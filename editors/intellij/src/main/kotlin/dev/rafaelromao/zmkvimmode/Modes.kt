package dev.rafaelromao.zmkvimmode

/**
 * IdeaVim's modes mapped onto the daemon's wire names.
 *
 * This is the only place that knows IdeaVim's vocabulary, and it matches on
 * the *name* of the mode rather than importing the type, so a release that
 * moves or renames it cannot stop the plugin from compiling.
 *
 * The engine's `com.maddyhome.idea.vim.state.mode.Mode` is a sealed interface
 * whose implementations are NORMAL, INSERT, REPLACE, VISUAL, SELECT,
 * OP_PENDING and CMD_LINE (verified against IdeaVim 2.46.2). The newer
 * `com.intellij.vim.api.models.Mode` enum spells the last one COMMAND_LINE
 * and adds NORMAL_FROM_INSERT and friends, so both spellings are accepted.
 */
object Modes {
    /**
     * @return the wire mode, or null when this state should not be reported.
     *
     * Operator-pending is the deliberate null: the keyboard is mid-gesture
     * (`c` waiting for a motion) and a correction from the host would clear
     * the layer that gesture depends on.
     */
    fun wireName(modeName: String?): String? {
        val name = modeName?.uppercase() ?: return null
        return when {
            name.startsWith("OP_PENDING") -> null
            // NORMAL, and the api enum's NORMAL_FROM_INSERT and friends.
            name.startsWith("NORMAL") -> "normal"
            // Replace types over existing text; on the keyboard that is
            // insert's shape, and it is what the daemon reports for Neovim's R.
            name.startsWith("INSERT") || name.startsWith("REPLACE") -> "insert"
            name.startsWith("VISUAL") || name.startsWith("SELECT") -> "visual"
            name.startsWith("CMD_LINE") || name.startsWith("COMMAND_LINE") -> "cmdline"
            // A mode we do not model: normal is the safe guess, and the
            // keyboard's own inference corrects it on the next keystroke.
            else -> "normal"
        }
    }
}
