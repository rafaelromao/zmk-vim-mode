package dev.showcase.vimmode;

public class Main {
    public static void main(String[] args) {
        System.out.println("code  byte  mode           layers");
        for (Mode mode : Mode.values()) {
            System.out.printf("%-5d 0x%02x  %-14s %s%n",
                    mode.code(), ModeTable.encode(mode, 0), mode, String.join(" ", mode.layers()));
        }
    }
}
