/*
 * Copyright (c) 2026 Rafael Romão
 * SPDX-License-Identifier: MIT
 *
 * Host-side unit tests for the pure decode/timing policy of the ZMK module.
 * Build and run:  make firmware-test   (or see the Makefile target)
 */

#include "../src/code_policy.h"

#include <stdio.h>
#include <string.h>

static int failures;
static int checks;

static void eq_u8(uint8_t got, uint8_t want, const char *what) {
    checks++;
    if (got != want) {
        failures++;
        printf("FAIL %s: got %u, want %u\n", what, got, want);
    }
}

static void eq_plan(struct zvm_plan got, enum zvm_action action, int32_t delay, const char *what) {
    checks++;
    if (got.action != action || (action == ZVM_ACTION_DEFER && got.delay_ms != delay)) {
        failures++;
        printf("FAIL %s: got action=%d delay=%d, want action=%d delay=%d\n", what, (int)got.action,
               got.delay_ms, (int)action, delay);
    }
}

/* Usages: Compose (0x04) = b0, Kana (0x05) = b1, Scroll Lock (0x03) = b2. */
static const uint8_t usages[] = {0x04, 0x05, 0x03};
#define N_USAGES (sizeof(usages) / sizeof(usages[0]))

/* Indicator bit positions (usage - 1). */
#define IND_NUM (1u << 0)
#define IND_CAPS (1u << 1)
#define IND_SCROLL (1u << 2)
#define IND_COMPOSE (1u << 3)
#define IND_KANA (1u << 4)

static void test_decode(void) {
    puts("-- decode");
    eq_u8(zvm_decode_code(0, usages, N_USAGES), 0, "no bits -> 0 (off)");
    eq_u8(zvm_decode_code(IND_COMPOSE, usages, N_USAGES), 1, "compose -> 1 (normal)");
    eq_u8(zvm_decode_code(IND_KANA, usages, N_USAGES), 2, "kana -> 2 (insert)");
    eq_u8(zvm_decode_code(IND_COMPOSE | IND_KANA, usages, N_USAGES), 3, "compose+kana -> 3 (visual)");
    eq_u8(zvm_decode_code(IND_SCROLL, usages, N_USAGES), 4, "scroll -> 4 (legacy)");
    eq_u8(zvm_decode_code(IND_SCROLL | IND_COMPOSE, usages, N_USAGES), 5, "scroll+compose -> 5 (cmdline)");
    eq_u8(zvm_decode_code(IND_SCROLL | IND_KANA, usages, N_USAGES), 6, "scroll+kana -> 6 (raw)");
    eq_u8(zvm_decode_code(IND_SCROLL | IND_KANA | IND_COMPOSE, usages, N_USAGES), 7,
          "all three -> 7 (legacy silent)");

    /* The OS-owned indicators must not disturb the code. */
    eq_u8(zvm_decode_code(IND_NUM | IND_CAPS, usages, N_USAGES), 0,
          "num+caps alone decode to 0");
    eq_u8(zvm_decode_code(IND_NUM | IND_CAPS | IND_COMPOSE | IND_KANA, usages, N_USAGES), 3,
          "num+caps do not corrupt the code");

    /* A single-bit configuration still works (e.g. legacy num-lock style). */
    static const uint8_t one[] = {0x01};
    eq_u8(zvm_decode_code(IND_NUM, one, 1), 1, "single indicator on");
    eq_u8(zvm_decode_code(0, one, 1), 0, "single indicator off");
}

static void test_plan_steady_state(void) {
    puts("-- plan: steady state");
    /* No local activity: apply immediately. */
    eq_plan(zvm_plan(2, 1, 100000, 150, 60), ZVM_ACTION_APPLY_NOW, 0,
            "new code with no recent local change applies now");
    /* Same code as applied: nothing to do. */
    eq_plan(zvm_plan(2, 2, 100000, 150, 60), ZVM_ACTION_CANCEL, 0, "unchanged code is a no-op");
}

static void test_plan_off_holdoff(void) {
    puts("-- plan: OFF hold-off");
    eq_plan(zvm_plan(0, 1, 100000, 150, 60), ZVM_ACTION_DEFER, 60,
            "code 0 waits for the off delay");
    /* The clobber-and-restore sequence: OFF parked, then the real code returns.
     * The state was never changed, so this only cancels the parked apply. */
    eq_plan(zvm_plan(1, 1, 100000, 150, 60), ZVM_ACTION_CANCEL, 0,
            "restored code cancels the parked OFF without re-applying");
    /* A genuinely different code while OFF is parked still applies. */
    eq_plan(zvm_plan(2, 1, 100000, 150, 60), ZVM_ACTION_APPLY_NOW, 0,
            "different code supersedes the parked OFF");
    /* Off delay of 0 disables the hold-off. */
    eq_plan(zvm_plan(0, 1, 100000, 150, 0), ZVM_ACTION_APPLY_NOW, 0,
            "off-delay-ms=0 applies immediately");
}

static void test_plan_local_guard(void) {
    puts("-- plan: local transition guard");
    /* The user just pressed Esc (local change 10 ms ago): a host code that
     * describes the previous keystroke must wait out the guard. */
    eq_plan(zvm_plan(2, 1, 10, 150, 60), ZVM_ACTION_DEFER, 140,
            "host code within the guard is deferred by the remainder");
    eq_plan(zvm_plan(2, 1, 149, 150, 60), ZVM_ACTION_DEFER, 1,
            "guard remainder just before expiry");
    eq_plan(zvm_plan(2, 1, 150, 150, 60), ZVM_ACTION_APPLY_NOW, 0, "guard expired exactly");
    eq_plan(zvm_plan(2, 1, 151, 150, 60), ZVM_ACTION_APPLY_NOW, 0, "guard expired");
    /* Longest of the two delays wins. */
    eq_plan(zvm_plan(0, 1, 10, 150, 60), ZVM_ACTION_DEFER, 140, "guard longer than off delay wins");
    eq_plan(zvm_plan(0, 1, 140, 150, 60), ZVM_ACTION_DEFER, 60,
            "off delay longer than guard remainder wins");
    /* An unchanged code is still a no-op inside the guard. */
    eq_plan(zvm_plan(1, 1, 10, 150, 60), ZVM_ACTION_CANCEL, 0,
            "unchanged code inside the guard is a no-op");
    /* Guard disabled. */
    eq_plan(zvm_plan(2, 1, 0, 0, 60), ZVM_ACTION_APPLY_NOW, 0,
            "local-guard-ms=0 applies immediately");
}

int main(void) {
    test_decode();
    test_plan_steady_state();
    test_plan_off_holdoff();
    test_plan_local_guard();
    printf("\n%d checks, %d failures\n", checks, failures);
    return failures == 0 ? 0 : 1;
}
