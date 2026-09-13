# zmk-vim-mode for IntelliJ

Reports [IdeaVim](https://plugins.jetbrains.com/plugin/164-ideavim)'s mode to
the daemon, so the keyboard follows IntelliJ the way it follows Neovim.

| IntelliJ state | reported |
|---|---|
| IdeaVim normal | normal |
| insert, replace | insert |
| visual, select | visual |
| command line (`:`) | cmdline |
| focus left the editor: project tree, settings, a dialog | raw |
| operator pending (`c` waiting for a motion) | nothing — the keyboard's own gesture layer owns that moment |

Without this plugin IntelliJ still works: the daemon classifies it as a
vim-like app with no mode feed (code 4) and the keyboard infers modes by
watching keys, exactly as it did before any of this existed.

## Requirements

- **IdeaVim installed** — Settings → Plugins → Marketplace → *IdeaVim*. This
  plugin declares a dependency on it and will not load without it.
- A JDK 21 or newer, and network access for the build: Gradle fetches IdeaVim
  and the Kotlin compiler, while the IDE itself is used from disk. The build
  targets Java 21 bytecode using whatever JDK runs Gradle, so no specific
  version has to be installed.

## Build and install

There is no Gradle wrapper checked in, so use one of these.

**From IntelliJ, with no installs** — it bundles Gradle:

1. *File → Open…* → `editors/intellij` → open as a project. IDEA sees
   `build.gradle.kts` and loads it as a Gradle project.
2. If it asks, point *Gradle JVM* at a JDK 21 or newer.
3. Gradle tool window → *Tasks → intellij platform → buildPlugin*.

**From the shell**, if you have or want the Gradle CLI:

```bash
brew install gradle          # once
cd editors/intellij
gradle wrapper               # so ./gradlew exists next time
./gradlew buildPlugin
```

Either way check `gradle.properties` first, and the result is
`build/distributions/zmk-vim-mode-intellij-0.1.0.zip`. Install it with
Settings → Plugins → ⚙ → *Install Plugin from Disk…*, then restart the IDE.

`gradle.properties` ships with the values for IntelliJ IDEA 2026.2
(`IU-262.10315.125`) and its installed IdeaVim. Change them for another
machine:

| property | where to find it |
|---|---|
| `platformPath` | the IDE's path — `/Applications/IntelliJ IDEA.app` on macOS, the Toolbox directory on Linux |
| `ideaVimPath` | the IdeaVim plugin directory: `~/Library/Application Support/JetBrains/<IDE>/plugins/IdeaVim` on macOS, `~/.local/share/JetBrains/<IDE>/IdeaVim` on Linux |
| `sinceBuild` | Help → About → the `Build #IU-262.xxxx` number, first three digits |

The Kotlin version in `build.gradle.kts` has to be able to read the metadata
in the IDE's jars — IntelliJ 2026.2 ships Kotlin 2.4, so the build asks for
2.4.0. Using an older compiler produces hundreds of *"was compiled with an
incompatible version of Kotlin"* errors followed by unresolved references to
`runCatching` and friends, which looks like a broken project but is only that
mismatch.

Both the IDE and IdeaVim are read from disk, so the build downloads only the
Kotlin compiler and the Gradle plugin. That also sidesteps a proxy that
intercepts TLS: the JDK keeps its own truststore, so it will refuse handshakes
that `curl` and the browser accept, and JetBrains' `cache-redirector` is the
usual casualty.

## Verify

```bash
zmk-vim-mode status | grep intellij
```

A client `intellij app=intellij` appears once a project is open. Then: `i` in
the editor should put the keyboard in its INSERT layer, `Esc` back to NORMAL,
and clicking the project tree should drop the vim layers entirely.

If nothing appears, *Help → Show Log in Finder* and search for
`zmk-vim-mode`: the plugin logs a warning when it cannot subscribe to
IdeaVim's listeners, and a debug line when the socket is not there.

## Limits

- **IntelliJ's terminal and consoles are IdeaVim editors**, force-switched to
  insert, so they report `insert` rather than `raw`. Keys reach them either
  way — the vim layers are transparent in insert — but Esc belongs to the
  editor, not the keyboard.
- Two IDEs open at once both report; the daemon uses the most recent one, and
  the frontmost window decides which application is in charge anyway.
- The mode listener uses IdeaVim's internal notifier
  (`injector.listenersNotifier.modeChangeListeners`). IdeaVim 2.46.2 also ships
  a nicer public API — `com.intellij.vim.api.scopes.ListenersScope` has exactly
  the `onModeChange`, `onEditorFocusGain` and `onEditorFocusLost` callbacks this
  plugin wants — but nothing outside its own jar exposes a way for a
  third-party plugin to reach that scope yet. Worth revisiting: it would make
  the whole `Startup.kt` dance disappear.
- If an IdeaVim release moves the notifier, the plugin logs a warning and keeps
  reporting focus only, rather than failing to load.
  [Modes.kt](src/main/kotlin/dev/rafaelromao/zmkvimmode/Modes.kt) matches mode
  *names* rather than importing the type, and accepts both the engine's
  `CMD_LINE` and the newer API's `COMMAND_LINE`, so renames there are harmless.
- **This plugin has never been compiled.** The API signatures were read out of
  the installed IdeaVim jars with `javap`, so the IdeaVim side is accurate, but
  the IntelliJ platform calls (`FocusChangeListener`, `ProjectActivity`) are
  written from memory. Expect the first `./gradlew buildPlugin` to want a fix
  or two.
