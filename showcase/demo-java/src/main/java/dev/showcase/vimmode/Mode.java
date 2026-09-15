package dev.showcase.vimmode;

import java.util.List;

/** One of the eight 3-bit codes the host writes into the keyboard's LED report. */
public enum Mode {
    OFF(0, List.of()),
    NORMAL(1, List.of("VIM_NORMAL")),
    INSERT(2, List.of("VIM_INSERT")),
    VISUAL(3, List.of("VIM_NORMAL", "VIM_VISUAL")),
    LEGACY(4, List.of("VIM_NORMAL")),
    CMDLINE(5, List.of("VIM_CMDLINE")),
    RAW(6, List.of()),
    LEGACY_SILENT(7, List.of("VIM_NORMAL"));

    private final int code;
    private final List<String> layers;

    Mode(int code, List<String> layers) {
        this.code = code;
        this.layers = layers;
    }

    public int code() {
        return code;
    }

    /** The keyboard layers this mode activates; every other managed layer is switched off. */
    public List<String> layers() {
        return layers;
    }

    public static Mode ofCode(int code) {
        for (Mode m : values()) {
            if (m.code == code) {
                return m;
            }
        }
        throw new IllegalArgumentException("no mode with code " + code);
    }
}
