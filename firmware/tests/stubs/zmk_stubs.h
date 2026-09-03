/*
 * Copyright (c) 2026 Rafael Romão
 * SPDX-License-Identifier: MIT
 *
 * Host-side stubs for the Zephyr/ZMK APIs used by
 * firmware/src/hid_indicator_code_listener.c, so the decode and
 * guard/off-delay state machine can be unit-tested with a plain C compiler.
 *
 * This is a test harness, not a Zephyr emulation: time is virtual and the
 * "work queue" is driven explicitly by the test.
 */

#ifndef ZMK_VIM_MODE_TEST_STUBS_H
#define ZMK_VIM_MODE_TEST_STUBS_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

/* ---------- Zephyr basics ---------- */

#define ARRAY_SIZE(a) (sizeof(a) / sizeof((a)[0]))
#define BIT(n) (1UL << (n))
#define ARG_UNUSED(x) ((void)(x))
#define BUILD_ASSERT(cond, msg) _Static_assert(cond, msg)
#define IS_ENABLED(x) 0
#define K_MSEC(ms) (ms)
#define INT32_MAX_VALUE 2147483647

#define LOG_MODULE_DECLARE(...)
#define LOG_DBG(...)                                                                               \
    do {                                                                                           \
    } while (0)
#define LOG_WRN(...)                                                                               \
    do {                                                                                           \
    } while (0)

#define SYS_INIT(fn, level, prio) int (*zvm_test_init_fn)(void) = fn
#define APPLICATION 0
#define CONFIG_APPLICATION_INIT_PRIORITY 90

/* Virtual clock, driven by the test. */
extern int64_t zvm_test_now;
static inline int64_t k_uptime_get(void) { return zvm_test_now; }

/* ---------- work queue ---------- */

struct k_work;
struct k_work_delayable {
    void (*handler)(struct k_work *);
    bool pending;
    int64_t due;
};

#define K_WORK_DELAYABLE_DEFINE(name, fn) static struct k_work_delayable name = {.handler = fn}

static inline bool k_work_delayable_is_pending(struct k_work_delayable *w) { return w->pending; }

static inline int k_work_cancel_delayable(struct k_work_delayable *w) {
    w->pending = false;
    return 0;
}

static inline int k_work_reschedule(struct k_work_delayable *w, int32_t delay_ms) {
    w->pending = true;
    w->due = zvm_test_now + delay_ms;
    return 0;
}

/* Advance virtual time and run the work item if it came due. */
void zvm_test_advance(struct k_work_delayable *w, int64_t ms);

/* ---------- ZMK events ---------- */

typedef uint8_t zmk_hid_indicators_t;
typedef int zmk_event_t;

struct zmk_hid_indicators_changed {
    zmk_hid_indicators_t indicators;
};

struct zmk_layer_state_changed {
    uint8_t layer;
    bool state;
};

/* The test sets which event the current dispatch represents. */
extern const struct zmk_hid_indicators_changed *zvm_test_ind_ev;
extern const struct zmk_layer_state_changed *zvm_test_layer_ev;

static inline const struct zmk_hid_indicators_changed *
as_zmk_hid_indicators_changed(const zmk_event_t *eh) {
    ARG_UNUSED(eh);
    return zvm_test_ind_ev;
}
static inline const struct zmk_layer_state_changed *
as_zmk_layer_state_changed(const zmk_event_t *eh) {
    ARG_UNUSED(eh);
    return zvm_test_layer_ev;
}

#define ZMK_EV_EVENT_BUBBLE 0
#define ZMK_LISTENER(name, fn) static int (*zvm_test_listener_##name)(const zmk_event_t *) = fn
#define ZMK_SUBSCRIPTION(name, ev)

/* ---------- ZMK keymap ---------- */

#define ZVM_MAX_LAYER 32
extern bool zvm_test_layer_state[ZVM_MAX_LAYER];
extern int zvm_test_layer_ops; /* count of activate/deactivate calls */

static inline bool zmk_keymap_layer_active(uint8_t layer) {
    return layer < ZVM_MAX_LAYER && zvm_test_layer_state[layer];
}
static inline int zmk_keymap_layer_activate(uint8_t layer, bool locking) {
    ARG_UNUSED(locking);
    if (layer < ZVM_MAX_LAYER) {
        zvm_test_layer_state[layer] = true;
        zvm_test_layer_ops++;
    }
    return 0;
}
static inline int zmk_keymap_layer_deactivate(uint8_t layer, bool locking) {
    ARG_UNUSED(locking);
    if (layer < ZVM_MAX_LAYER) {
        zvm_test_layer_state[layer] = false;
        zvm_test_layer_ops++;
    }
    return 0;
}

/* ---------- ZMK behaviors ---------- */

struct zmk_behavior_binding {
    const char *behavior_dev;
    uint32_t param1;
    uint32_t param2;
};

struct zmk_behavior_binding_event {
    int layer;
    uint32_t position;
    int64_t timestamp;
};

extern int zvm_test_binding_invocations;

static inline int zmk_behavior_queue_add(const struct zmk_behavior_binding_event *event,
                                         const struct zmk_behavior_binding binding, bool press,
                                         uint32_t wait) {
    ARG_UNUSED(event);
    ARG_UNUSED(binding);
    ARG_UNUSED(wait);
    if (press) {
        zvm_test_binding_invocations++;
    }
    return 0;
}

/* ---------- HID indicators ---------- */

extern zmk_hid_indicators_t zvm_test_current_profile;
static inline zmk_hid_indicators_t zmk_hid_indicators_get_current_profile(void) {
    return zvm_test_current_profile;
}

/* ---------- devicetree shim ----------
 * The real file gets these from the generated devicetree header. The test
 * supplies the same values through the ZVM_TEST_* macros below.
 */

#define DT_HAS_COMPAT_STATUS_OKAY(c) 1
#define DT_INST_PROP(inst, prop) ZVM_TEST_##prop
#define INT32_MAX 2147483647

#endif /* ZMK_VIM_MODE_TEST_STUBS_H */
