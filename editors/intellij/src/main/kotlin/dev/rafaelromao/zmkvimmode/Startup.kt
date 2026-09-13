package dev.rafaelromao.zmkvimmode

import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.Service
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.editor.EditorFactory
import com.intellij.openapi.editor.EditorKind
import com.intellij.openapi.editor.ex.EditorEventMulticasterEx
import com.intellij.openapi.editor.ex.FocusChangeListener
import com.intellij.openapi.project.Project
import com.intellij.openapi.startup.ProjectActivity
import com.maddyhome.idea.vim.api.VimEditor
import com.maddyhome.idea.vim.api.injector
import com.maddyhome.idea.vim.common.ModeChangeListener
import com.maddyhome.idea.vim.state.mode.Mode

private val LOG = logger<ZmkVimModeService>()

/**
 * One socket client and one set of listeners for the whole IDE, however many
 * projects are open.
 *
 * The listeners are IdeaVim's internal notifier rather than the newer
 * `com.intellij.vim.api` listeners scope: as of IdeaVim 2.46.2 that scope is
 * declared in the api jar but nothing outside it exposes a way to reach it, so
 * the notifier is the only path available to a third-party plugin. Both calls
 * are wrapped, and a failure downgrades the plugin to focus reporting instead
 * of breaking the IDE.
 */
@Service(Service.Level.APP)
class ZmkVimModeService : Disposable {
    val client: Client = Client(PLUGIN_VERSION)

    private var installed = false

    /**
     * Whether the focused editor answers to vim. A mode change reported from a
     * terminal -- IdeaVim forces those to insert -- would otherwise put the
     * keyboard back into a vim layer for a window that is just a shell.
     */
    @Volatile
    private var vimHasTheKeys = true

    fun install() {
        if (installed) return
        installed = true
        client.start()
        subscribeToModes()
        subscribeToFocus()
        reportCurrentMode()
    }

    private fun subscribeToModes() {
        try {
            val listener = object : ModeChangeListener {
                // The argument is the mode being left; the one we want is the
                // current one, read below.
                override fun modeChanged(editor: VimEditor, oldMode: Mode) = reportCurrentMode()
            }
            injector.listenersNotifier.modeChangeListeners.add(listener)
        } catch (e: Throwable) {
            LOG.warn("cannot follow IdeaVim's mode changes; reporting focus only", e)
        }
    }

    /**
     * Keys that reach a tool window, a dialog or the project tree never reach
     * the editor, so the keyboard must drop its vim layers for them.
     */
    private fun subscribeToFocus() {
        try {
            val multicaster = EditorFactory.getInstance().eventMulticaster
            if (multicaster !is EditorEventMulticasterEx) {
                LOG.warn("editor multicaster does not support focus events; tool windows will keep the vim layers")
                return
            }
            multicaster.addFocusChangeListener(object : FocusChangeListener {
                override fun focusGained(editor: Editor) {
                    vimHasTheKeys = takesVimKeys(editor)
                    if (vimHasTheKeys) reportCurrentMode() else client.report("raw")
                }

                override fun focusLost(editor: Editor) {
                    vimHasTheKeys = false
                    client.report("raw")
                }
            }, this)
        } catch (e: Throwable) {
            LOG.warn("cannot follow editor focus", e)
        }
    }

    /**
     * The terminal and the run/debug consoles are editors too, so focus moving
     * into one arrives here as an ordinary focusGained -- which would report a
     * vim mode for a window where the keys belong to a shell. The editor kind
     * is what separates them: only a file being edited, a diff or a preview
     * answers to vim.
     */
    private fun takesVimKeys(editor: Editor): Boolean = when (editor.editorKind) {
        EditorKind.CONSOLE, EditorKind.UNTYPED -> false
        else -> true
    }

    /** Read IdeaVim's current mode and send it. */
    fun reportCurrentMode() {
        if (!vimHasTheKeys) return client.report("raw")
        val name = try {
            injector.vimState.mode.javaClass.simpleName
        } catch (e: Throwable) {
            LOG.debug("IdeaVim state unavailable: ${e.message}")
            null
        }
        client.report(Modes.wireName(name))
    }

    override fun dispose() = client.stop()

    companion object {
        const val PLUGIN_VERSION = "0.1.0"

        fun getInstance(): ZmkVimModeService =
            ApplicationManager.getApplication().getService(ZmkVimModeService::class.java)
    }
}

/** Installs the listeners once, when the first project finishes opening. */
class Startup : ProjectActivity {
    override suspend fun execute(project: Project) {
        ZmkVimModeService.getInstance().install()
    }
}
