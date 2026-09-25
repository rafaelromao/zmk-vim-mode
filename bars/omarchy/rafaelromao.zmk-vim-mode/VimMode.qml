import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

// The vim mode your ZMK keyboard is in, as the zmk-vim-mode daemon decided it:
// NORMAL, INSERT, VISUAL, CMDLINE, VIM or RAW behind a Neovim glyph, and no
// tile at all while vim mode is off.
//
// `zmk-vim-mode status --bar` does the deciding and the wording (the macOS
// menu bar item renders the same answer), so this file only draws it. The
// glyph is plain text in the bar's Nerd Font; a bar theme that colours icons
// can match this widget by its id, rafaelromao.zmk-vim-mode.
BarWidget {
  id: root
  moduleName: "rafaelromao.zmk-vim-mode"

  // `zmk-vim-mode install --omarchy` writes the path of the binary that
  // installed the plugin here. Anything that is not an absolute path means a
  // hand-copied plugin, which falls back to where make install puts the
  // binary: the shell does not start commands through a login shell, so a
  // bare name may not resolve.
  readonly property string installedBinary: "__ZMK_VIM_MODE_BIN__"
  readonly property string binary: installedBinary.charAt(0) === "/"
    ? installedBinary
    : Quickshell.env("HOME") + "/.local/bin/zmk-vim-mode"

  property string label: ""
  property string tooltip: ""
  // Looks in a row that produced no output. A binary that cannot start may
  // report neither output nor an exit, so counting silent looks is what
  // notices it, and hides a label that would otherwise go stale.
  property int silentPolls: 0

  // status --bar answers even while the daemon is down, so output that does
  // not parse means the binary itself is missing or too old: hide.
  function render(raw) {
    silentPolls = 0
    var view = null
    try {
      view = JSON.parse(raw)
    } catch (e) {
      view = null
    }
    if (view && typeof view.text === "string") {
      label = view.text
      tooltip = typeof view.tooltip === "string" ? view.tooltip : ""
    } else {
      label = ""
      tooltip = ""
    }
  }

  function poll() {
    // Skip rather than queue while the previous look is still out.
    if (statusProc.running) return
    silentPolls += 1
    if (silentPolls > 3) label = ""
    statusProc.running = true
  }

  visible: label !== ""
  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  Process {
    id: statusProc
    command: [root.binary, "status", "--bar"]
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.render(text)
    }
  }

  Timer {
    interval: 1000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.poll()
  }

  // A direct child, not a Loader's: bars style their widgets by walking the
  // tree once when the widget is placed.
  WidgetButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: " " + root.label
    tooltipText: root.tooltip
  }
}
