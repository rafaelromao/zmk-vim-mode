/*
 * Copyright (c) 2026 Rafael Romão
 *
 * SPDX-License-Identifier: MIT
 *
 * Decode a small integer from HID LED indicator bits and apply the matching
 * set of layers.
 *
 * Two properties of ZMK's indicator event shape this code:
 *
 *   - zmk_hid_indicators_changed is raised on *every* LED output report, not
 *     only on change, and also on endpoint/profile switch. Successive reports
 *     coalesce into one event carrying the latest value (a single k_work), so
 *     the wire protocol must be state, never a stream: we act only when the
 *     decoded code actually differs from the last one received.
 *
 *   - The host's view of the editor is always slightly stale. If the keyboard
 *     itself just changed a managed layer (the user typed Esc or an operator),
 *     an in-flight host code describes the world before that keystroke.
 *     Applying it would undo a correct local transition, so host codes are
 *     parked for local-guard-ms after any local change and only the most
 *     recent one is applied when the guard expires.
 */

#define DT_DRV_COMPAT zmk_hid_indicator_code_listener

#include <zephyr/device.h>
#include <zephyr/kernel.h>
#include <zephyr/logging/log.h>
#include <zephyr/sys/util.h>

#include <drivers/behavior.h>
#include <zmk/behavior.h>
#include <zmk/behavior_queue.h>
#include <zmk/event_manager.h>
#include <zmk/events/hid_indicators_changed.h>
#include <zmk/events/layer_state_changed.h>
#include <zmk/events/position_state_changed.h>
#include <zmk/hid_indicators.h>
#include <zmk/keymap.h>

#include "code_policy.h"

LOG_MODULE_DECLARE(zmk, CONFIG_ZMK_LOG_LEVEL);

#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)

#define OFF_DELAY_MS DT_INST_PROP(0, off_delay_ms)
#define GUARD_MS DT_INST_PROP(0, local_guard_ms)
#define TAP_MS DT_INST_PROP(0, tap_ms)
#define WAIT_MS DT_INST_PROP(0, wait_ms)

/* HID LED usages carrying the code bits, least significant first. */
static const uint8_t indicators[] = DT_INST_PROP(0, indicators);
#define N_INDICATORS ARRAY_SIZE(indicators)

/* Every layer this node may activate or deactivate. */
static const uint8_t managed[] = DT_INST_PROP(0, managed_layers);
#define N_MANAGED ARRAY_SIZE(managed)

BUILD_ASSERT(N_INDICATORS >= 1 && N_INDICATORS <= 5,
             "indicators must list between 1 and 5 HID LED usages");
BUILD_ASSERT(N_MANAGED >= 1, "managed-layers must not be empty");

struct code_entry {
    uint8_t code;
    const uint8_t *layers;
    uint8_t n_layers;
    const struct zmk_behavior_binding *bindings;
    uint8_t n_bindings;
};

/* Per-child arrays. DT_DEP_ORD keeps the generated symbols unique. */
#define LAYERS_SYM(node) _CONCAT(zvm_layers_, DT_DEP_ORD(node))
#define BINDINGS_SYM(node) _CONCAT(zvm_bindings_, DT_DEP_ORD(node))

#define DEFINE_CHILD_ARRAYS(node)                                                                  \
    static const uint8_t LAYERS_SYM(node)[] =                                                      \
        COND_CODE_1(DT_NODE_HAS_PROP(node, layers), (DT_PROP(node, layers)), ({0}));               \
    static const struct zmk_behavior_binding BINDINGS_SYM(node)[] = COND_CODE_1(                   \
        DT_NODE_HAS_PROP(node, bindings),                                                          \
        ({LISTIFY(DT_PROP_LEN(node, bindings), ZMK_KEYMAP_EXTRACT_BINDING, (, ), node)}),          \
        ({}));

#define CHILD_ENTRY(node)                                                                          \
    {                                                                                              \
        .code = DT_PROP(node, code),                                                               \
        .layers = LAYERS_SYM(node),                                                                \
        .n_layers = DT_PROP_LEN_OR(node, layers, 0),                                               \
        .bindings = BINDINGS_SYM(node),                                                            \
        .n_bindings = DT_PROP_LEN_OR(node, bindings, 0),                                           \
    },

DT_INST_FOREACH_CHILD_STATUS_OKAY(0, DEFINE_CHILD_ARRAYS)

static const struct code_entry entries[] = {DT_INST_FOREACH_CHILD_STATUS_OKAY(0, CHILD_ENTRY)};

#define N_ENTRIES ARRAY_SIZE(entries)

/* State. Everything below runs on the system workqueue: the indicators event is
 * raised from a k_work, and the deferred apply is a k_work_delayable. */
static uint8_t last_received;      /* last code decoded from an event */
static uint8_t last_applied;       /* last code whose state we actually applied */
static uint8_t pending;            /* code waiting for the guard/off delay */
static bool applying;              /* suppress our own layer_state_changed events */
static int64_t last_local_change;  /* uptime of the last keyboard-caused layer change */

static void apply_work_cb(struct k_work *work);
static K_WORK_DELAYABLE_DEFINE(apply_work, apply_work_cb);

static uint8_t decode(zmk_hid_indicators_t ind) {
    return zvm_decode_code((uint8_t)ind, indicators, N_INDICATORS);
}

static const struct code_entry *find_entry(uint8_t code) {
    for (size_t i = 0; i < N_ENTRIES; i++) {
        if (entries[i].code == code) {
            return &entries[i];
        }
    }
    return NULL;
}

static bool entry_wants_layer(const struct code_entry *entry, uint8_t layer) {
    if (entry == NULL) {
        return false;
    }
    for (size_t i = 0; i < entry->n_layers; i++) {
        if (entry->layers[i] == layer) {
            return true;
        }
    }
    return false;
}

static void invoke_bindings(const struct code_entry *entry) {
    for (size_t i = 0; i < entry->n_bindings; i++) {
        struct zmk_behavior_binding_event event = {
            .position = INT32_MAX,
            .timestamp = k_uptime_get(),
#if IS_ENABLED(CONFIG_ZMK_SPLIT)
            .source = ZMK_POSITION_STATE_CHANGE_SOURCE_LOCAL,
#endif
        };

        int err = zmk_behavior_queue_add(&event, entry->bindings[i], true, TAP_MS);
        if (err < 0) {
            LOG_WRN("behavior queue full (%d); binding %d for code %d dropped", err, (int)i,
                    entry->code);
            return;
        }
        err = zmk_behavior_queue_add(&event, entry->bindings[i], false, WAIT_MS);
        if (err < 0) {
            LOG_WRN("behavior queue full (%d) between press and release of binding %d", err,
                    (int)i);
            return;
        }
    }
}

static void apply(uint8_t code) {
    const struct code_entry *entry = find_entry(code);

    if (code != 0 && entry == NULL) {
        /* An unknown code means host and firmware disagree about the table.
         * Treat it as "no vim layers" rather than leaving a stale state on. */
        LOG_WRN("no entry for code %d; clearing managed layers", code);
    }

    applying = true;
    for (size_t i = 0; i < N_MANAGED; i++) {
        if (!entry_wants_layer(entry, managed[i]) && zmk_keymap_layer_active(managed[i])) {
            zmk_keymap_layer_deactivate(managed[i], false);
        }
    }
    for (size_t i = 0; i < N_MANAGED; i++) {
        if (entry_wants_layer(entry, managed[i]) && !zmk_keymap_layer_active(managed[i])) {
            zmk_keymap_layer_activate(managed[i], false);
        }
    }
    applying = false;

    last_applied = code;
    LOG_DBG("applied code %d", code);

    if (entry != NULL && entry->n_bindings > 0) {
        invoke_bindings(entry);
    }
}

static void apply_work_cb(struct k_work *work) {
    ARG_UNUSED(work);
    if (pending != last_applied) {
        apply(pending);
    }
}

/* Decide when to apply a newly received code (policy in code_policy.h). */
static void schedule(uint8_t code) {
    pending = code;

    struct zvm_plan plan =
        zvm_plan(code, last_applied, k_uptime_get() - last_local_change, GUARD_MS, OFF_DELAY_MS);

    switch (plan.action) {
    case ZVM_ACTION_CANCEL:
        k_work_cancel_delayable(&apply_work);
        return;
    case ZVM_ACTION_APPLY_NOW:
        k_work_cancel_delayable(&apply_work);
        apply(code);
        return;
    case ZVM_ACTION_DEFER:
        LOG_DBG("deferring code %d by %d ms", code, plan.delay_ms);
        k_work_reschedule(&apply_work, K_MSEC(plan.delay_ms));
        return;
    }
}

static int hid_indicator_code_listener(const zmk_event_t *eh) {
    const struct zmk_hid_indicators_changed *ind_ev = as_zmk_hid_indicators_changed(eh);

    if (ind_ev != NULL) {
        uint8_t code = decode(ind_ev->indicators);

        if (code != last_received) {
            LOG_DBG("indicators 0x%02x -> code %d", ind_ev->indicators, code);
            last_received = code;
            schedule(code);
        }
        return ZMK_EV_EVENT_BUBBLE;
    }

    const struct zmk_layer_state_changed *layer_ev = as_zmk_layer_state_changed(eh);

    if (layer_ev != NULL && !applying) {
        for (size_t i = 0; i < N_MANAGED; i++) {
            if (managed[i] == layer_ev->layer) {
                last_local_change = k_uptime_get();
                /* Push out an already-parked code: the keyboard is mid-roll. */
                if (k_work_delayable_is_pending(&apply_work)) {
                    k_work_reschedule(&apply_work, K_MSEC(GUARD_MS));
                }
                break;
            }
        }
    }
    return ZMK_EV_EVENT_BUBBLE;
}

ZMK_LISTENER(hid_indicator_code_listener, hid_indicator_code_listener);
ZMK_SUBSCRIPTION(hid_indicator_code_listener, zmk_hid_indicators_changed);
ZMK_SUBSCRIPTION(hid_indicator_code_listener, zmk_layer_state_changed);

static int hid_indicator_code_listener_init(void) {
    /* Seed from the selected endpoint so a reconnect does not look like a
     * transition from 0. */
    last_received = decode(zmk_hid_indicators_get_current_profile());
    last_applied = last_received;
    LOG_DBG("initialised with code %d (%zu codes, %zu managed layers)", last_received, N_ENTRIES,
            N_MANAGED);
    return 0;
}

SYS_INIT(hid_indicator_code_listener_init, APPLICATION, CONFIG_APPLICATION_INIT_PRIORITY);

#endif /* DT_HAS_COMPAT_STATUS_OKAY */
