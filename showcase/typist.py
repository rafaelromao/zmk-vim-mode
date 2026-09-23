"""Romak layer-selection rules shared by the rehearsal feed and its typing pace."""

import unicodedata

ALPHA2_CHARS = frozenset("qkyzxwj_'") | {
    chr(n) for n in (0x00f4, 0x00f3, 0x00fa, 0x00e3, 0x00e1, 0x00e9,
                     0x00ed, 0x00e7, 0x00f5, 0x00e2, 0x00ea)
}
VOWELS = frozenset("aeiou") | {
    chr(n) for n in (0x00e1, 0x00e0, 0x00e2, 0x00e4, 0x00e3, 0x00e5,
                     0x00e9, 0x00e8, 0x00ea, 0x00eb, 0x00ed, 0x00ec,
                     0x00ee, 0x00ef, 0x00f3, 0x00f2, 0x00f4, 0x00f6,
                     0x00f5, 0x00fa, 0x00f9, 0x00fb, 0x00fc, 0x00fd,
                     0x00ff)
}


def is_vowel(char):
    return bool(char) and char.casefold() in VOWELS


def remember_char(chars):
    if len(chars) == 1 and chars.isalpha():
        return unicodedata.normalize("NFC", chars).casefold()
    return None


def uses_alpha2(chars, previous_char=None):
    """Whether a character needs the sticky Alpha 2 thumb from the current word context."""
    if not chars:
        return False
    lower = chars.casefold()
    if lower == "h":
        return is_vowel(previous_char)
    if lower == "v":
        return not is_vowel(previous_char)
    return lower in ALPHA2_CHARS


def type_gap(char, previous_char, base_gap, alpha2_extra_gap, slow_alpha2=True):
    """Return a character delay with extra time for a sticky Alpha 2 activation."""
    if slow_alpha2 and uses_alpha2(char, previous_char):
        return base_gap + alpha2_extra_gap
    return base_gap
