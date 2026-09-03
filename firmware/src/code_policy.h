/*
 * Copyright (c) 2026 Rafael Romão
 * SPDX-License-Identifier: MIT
 *
 * Pure decode and timing policy for the HID indicator code listener.
 *
 * This header deliberately depends on nothing but stdint/stdbool so the
 * decisions that are easy to get wrong (bit decoding, the local-transition
 * guard, the OFF hold-off) can be unit-tested on the host, away from Zephyr.
 */

#ifndef ZMK_VIM_MODE_CODE_POLICY_H
#define ZMK_VIM_MODE_CODE_POLICY_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

/**
 * Decode the mode code from a HID indicator bitmap.
 *
 * @param indicators bitmap as delivered by zmk_hid_indicators_changed, where
 *                   bit (usage - 1) is set when that LED is on.
 * @param usages     HID LED usages carrying the code bits, least significant
 *                   first (e.g. Compose, Kana, Scroll Lock).
 * @param n_usages   number of entries in @p usages (1..5).
 * @return the decoded code.
 */
static inline uint8_t zvm_decode_code(uint8_t indicators, const uint8_t *usages, size_t n_usages) {
    uint8_t code = 0;

    for (size_t i = 0; i < n_usages; i++) {
        if (usages[i] >= 1 && usages[i] <= 8 && (indicators & (1u << (usages[i] - 1))) != 0) {
            code |= (uint8_t)(1u << i);
        }
    }
    return code;
}

/** What the listener should do with a freshly received code. */
enum zvm_action {
    /** Nothing changes; cancel any parked apply. */
    ZVM_ACTION_CANCEL = 0,
    /** Apply the code now. */
    ZVM_ACTION_APPLY_NOW,
    /** Park the code and apply it after the returned delay. */
    ZVM_ACTION_DEFER,
};

/** Result of zvm_plan(). */
struct zvm_plan {
    enum zvm_action action;
    int32_t delay_ms; /* valid when action == ZVM_ACTION_DEFER */
};

/**
 * Decide when a received code should be applied.
 *
 * Two delays combine, longest wins:
 *
 *   - local guard: after the keyboard changed a managed layer itself, host
 *     codes describe a state older than that keystroke, so they wait.
 *   - OFF hold-off: code 0 waits, because on Linux the compositor briefly
 *     zeroes our LED bits whenever the kernel emits its own LED report and the
 *     daemon restores them a moment later.
 *
 * @param code           newly received code.
 * @param last_applied   code whose state is currently applied.
 * @param since_local_ms milliseconds since the last keyboard-caused change to
 *                       a managed layer (large when there was none).
 * @param guard_ms       local-guard-ms from devicetree.
 * @param off_delay_ms   off-delay-ms from devicetree.
 */
static inline struct zvm_plan zvm_plan(uint8_t code, uint8_t last_applied, int64_t since_local_ms,
                                       int32_t guard_ms, int32_t off_delay_ms) {
    struct zvm_plan plan = {ZVM_ACTION_CANCEL, 0};

    if (code == last_applied) {
        /* The state is already applied, so there is nothing to do beyond
         * cancelling any parked apply. This is the common Linux case: the
         * compositor zeroed our bits, we parked an OFF, and the daemon
         * restored the real code before the hold-off elapsed. */
        return plan;
    }

    int32_t delay = 0;

    if (since_local_ms < (int64_t)guard_ms) {
        delay = (int32_t)((int64_t)guard_ms - since_local_ms);
    }
    if (code == 0 && delay < off_delay_ms) {
        delay = off_delay_ms;
    }

    if (delay <= 0) {
        plan.action = ZVM_ACTION_APPLY_NOW;
        return plan;
    }
    plan.action = ZVM_ACTION_DEFER;
    plan.delay_ms = delay;
    return plan;
}

#endif /* ZMK_VIM_MODE_CODE_POLICY_H */
