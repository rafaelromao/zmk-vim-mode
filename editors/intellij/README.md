# zmk-vim-mode for IntelliJ

Reports [IdeaVim](https://plugins.jetbrains.com/plugin/164-ideavim)'s mode to
the daemon, so the keyboard follows IntelliJ the way it follows Neovim.

| IntelliJ state | reported |
|---|---|
| IdeaVim normal | normal |
| insert, replace | insert |
| visual, select | visual |
| command line (`:`) | cmdline |
| focus left the editor: project tree, settings, a dialog, the terminal, a console | raw |
| operator pending (`c` waiting for a motion) | nothing — the keyboard's own gesture layer owns that moment |

Without this plugin IntelliJ still works: the daemon classifies it as a
vim-like app with no mode feed (code 4) and the keyboard infers modes by
watching keys, exactly as it did before any of this existed.

## Requirements

**IdeaVim installed** — Settings → Plugins → Marketplace → *IdeaVim*. This
plugin declares a dependency on it, will not load without it, and the build
compiles against it.

That is the only thing you have to install. The Gradle wrapper is checked in
and the JDK comes from the IDE itself, which ships exactly the one the
platform expects.

## Install

```bash
zmk-vim-mode install --intellij
```

(`make install` does this too.) It finds every JetBrains IDE on the machine,
works out where each keeps its plugins, generates `gradle.properties` for it,
builds the plugin against that exact IDE and IdeaVim, and unpacks the result
where the IDE loads plugins from — the same place *Install Plugin from Disk*
would put it. Restart the IDE afterwards; plugins are only scanned at startup.

The build runs in `~/.cache/zmk-vim-mode/intellij` (`~/Library/Caches` on
macOS) rather than a throwaway directory, so Gradle's downloads are made once
and reused. The first run fetches Gradle itself and the Kotlin compiler and is
not quick; later runs are incremental.

An IDE without IdeaVim is reported and skipped, not guessed at.

### Building it by hand

Only needed if you are changing the plugin, or the installer could not.

```bash
cd editors/intellij
./gradlew buildPlugin
```

This uses the `gradle.properties` in the repo, which holds the values for
whatever machine last edited it — check it first:

| property | where to find it |
|---|---|
| `platformPath` | the IDE's path — `/Applications/IntelliJ IDEA.app` on macOS, the Toolbox directory on Linux |
| `ideaVimPath` | the IdeaVim plugin directory: `~/Library/Application Support/JetBrains/<IDE>/plugins/IdeaVim` on macOS, `~/.local/share/JetBrains/<IDE>/IdeaVim` on Linux |
| `sinceBuild` | Help → About → the `Build #IU-262.xxxx` number, first three digits |
| `org.gradle.java.installations.paths` | the IDE's bundled JVM, `<IDE>/Contents/jbr/Contents/Home` on macOS, `<IDE>/jbr` on Linux |

That last one exists because the build needs the JDK the IDE runs on (25 for
2026.2) and you probably have a different one; the IDE ships exactly that JVM,
so Gradle is pointed at it rather than downloading another. The result is
`build/distributions/zmk-vim-mode-intellij-0.1.0.zip` — install it with
Settings → Plugins → ⚙ → *Install Plugin from Disk…*, then restart.

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
and clicking the project tree — or the terminal — should drop the vim layers
entirely, showing `raw (code 6)`.

If nothing appears, *Help → Show Log in Finder* and search for
`zmk-vim-mode`: the plugin logs a warning when it cannot subscribe to
IdeaVim's listeners, and a debug line when the socket is not there.

## Limits

- **IntelliJ's terminal and consoles are editors**, so focus moving into one
  arrives as an ordinary editor focus rather than as leaving the editor. They
  are told apart by `EditorKind` — `CONSOLE` and `UNTYPED` report `raw`,
  anything else answers to vim — and mode changes are ignored while one is
  focused, since IdeaVim force-switches them to insert.
- `doctor` does not check this plugin, unlike the VSCode and Obsidian ones:
  once unpacked it is the IDE's to enable or disable, and the IDE's own
  Settings → Plugins is the honest answer to whether it is on.
- The installer builds against the IDE it finds. An IDE upgrade changes the
  platform the plugin was compiled for, so run `install --intellij` again
  after one.
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
- Built and confirmed working against IDEA 2026.2 (`IU-262.10315.125`) and
  IdeaVim 2.46.2, on macOS. Code instrumentation is off in `build.gradle.kts`:
  it exists for UI forms, there are none here, and leaving it on makes the
  build fetch an extra artifact from JetBrains for nothing.
