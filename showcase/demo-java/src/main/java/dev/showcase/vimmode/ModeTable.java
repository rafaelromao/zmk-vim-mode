package dev.showcase.vimmode;

/**
 * Packs a mode into the HID LED indicator byte and reads it back.
 *
 * <p>Compose carries bit 0, Kana bit 1 and Scroll Lock bit 2. Num Lock and Caps
 * Lock belong to the operating system and are never touched.
 */
public final class ModeTable {
    static final int SCROLL_LOCK = 0x04;
    static final int COMPOSE = 0x08;
    static final int KANA = 0x10;

    private ModeTable() {
    }

    public static int encode(Mode mode, int leds) {
        int out = leds & ~(COMPOSE | KANA | SCROLL_LOCK);
        int code = mode.code();
        if ((code & 1) != 0) out |= COMPOSE;
        if ((code & 2) != 0) out |= KANA;
        if ((code & 4) != 0) out |= SCROLL_LOCK;
        return out;
    }

    public static Mode decode(int leds) {
        int code = 0;
        if ((leds & COMPOSE) != 0) code |= 1;
        if ((leds & KANA) != 0) code |= 2;
        if ((leds & SCROLL_LOCK) != 0) code |= 4;
        return Mode.ofCode(code);
    }
}
