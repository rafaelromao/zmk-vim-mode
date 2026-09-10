// zmk-vim-mode companion for VSCode.
//
// The vim *mode* comes from Neovim itself: vscode-neovim embeds a real nvim,
// which runs the zmk-vim-mode Neovim plugin, which talks to the daemon. This
// extension only adds what nvim cannot see -- that keyboard focus has left the
// text editor -- as the daemon's protocol "no opinion" (mode "none") or "raw":
//
//   - no active text editor (Settings, Extensions, a webview, an image, an
//     empty window)                                  → raw, until one is active
//   - a quick input we opened (Ctrl+P, the palette,
//     Go to Line/Symbol)                              → raw, with a TTL; cleared
//     when the editor shows activity again
//   - otherwise                                       → none: nvim decides
//
// The extension API has no event for focus moving to the terminal, the
// sidebar or a panel. Those come from the window title instead: set
//   "window.title": "${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]"
// and the daemon reads the focused view off the Hyprland title (see README).
//
// Plain JavaScript on purpose: no build step, symlink or vsce-package as is.
'use strict';

const vscode = require('vscode');
const net = require('net');
const os = require('os');
const path = require('path');

const PLUGIN = '0.1.3';
const RECONNECT_MIN = 250;
const RECONNECT_MAX = 5000;

let out = null;
let sock = null;
let connected = false;
let stopped = false;
let backoff = RECONNECT_MIN;
let reconnectTimer = null;

let noEditor = false; // no active text editor → raw
let windowFocused = true; // last seen WindowState.focused
let quickInput = ''; // name of the quick input we opened, '' when none → raw with TTL
let quickInputTimer = null;
let quickInputAt = 0; // when the hint was raised (ms)
// Opening a quick input makes the editor blur and vscode-neovim resync the
// selection; those synthetic events must not count as "the editor is active".
const SELECTION_GRACE_MS = 500;
let lastSent = null; // JSON of the last mode message, for dedupe

function cfg() {
  return vscode.workspace.getConfiguration('zmkVimMode');
}

function log(...parts) {
  if (out && cfg().get('debug')) {
    out.appendLine(`${new Date().toISOString()} ${parts.join(' ')}`);
  }
}

function socketPath() {
  const set = (cfg().get('socket') || '').trim();
  if (set) {
    return set.replace(/^~(?=$|\/)/, os.homedir());
  }
  if (process.env.ZMK_VIM_MODE_SOCKET) {
    return process.env.ZMK_VIM_MODE_SOCKET;
  }
  return path.join(os.homedir(), '.local', 'state', 'zmk-vim-mode', 'daemon.sock');
}

// current returns what we tell the daemon right now.
function current() {
  if (noEditor) {
    return { mode: 'raw', reason: 'no text editor' };
  }
  if (quickInput) {
    return { mode: 'raw', ttl_ms: Number(cfg().get('quickInputTtlMs')) || 20000, reason: quickInput };
  }
  return { mode: 'none', reason: 'editor' };
}

function send(msg) {
  if (!connected || !sock) {
    return;
  }
  try {
    sock.write(JSON.stringify(msg) + '\n');
  } catch (e) {
    log('write failed:', e && e.message);
    scheduleReconnect();
  }
}

function hello() {
  const c = current();
  send({
    v: 1,
    t: 'hello',
    client: 'vscode',
    app: 'vscode',
    pid: process.pid,
    mode: c.mode,
    ttl_ms: c.ttl_ms,
    plugin: PLUGIN,
  });
  lastSent = JSON.stringify([c.mode, c.ttl_ms]);
  log('hello', c.mode, c.reason);
}

function report(force) {
  const c = current();
  const key = JSON.stringify([c.mode, c.ttl_ms]);
  if (!force && key === lastSent) {
    return;
  }
  lastSent = key;
  send({ t: 'mode', mode: c.mode, ttl_ms: c.ttl_ms });
  log('mode', c.mode, c.reason);
}

function drop() {
  connected = false;
  if (sock) {
    try {
      sock.destroy();
    } catch (e) {
      // already gone
    }
    sock = null;
  }
  lastSent = null;
}

function scheduleReconnect() {
  drop();
  if (stopped || reconnectTimer) {
    return;
  }
  const delay = backoff;
  backoff = Math.min(backoff * 2, RECONNECT_MAX);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, delay);
}

function connect() {
  if (stopped || connected || sock) {
    return;
  }
  const p = socketPath();
  const s = net.createConnection(p);
  sock = s;
  s.setEncoding('utf8');
  s.on('connect', () => {
    connected = true;
    backoff = RECONNECT_MIN;
    log('connected', p);
    hello();
  });
  let pending = '';
  s.on('data', (chunk) => {
    pending += String(chunk);
    let nl;
    while ((nl = pending.indexOf('\n')) >= 0) {
      const line = pending.slice(0, nl);
      pending = pending.slice(nl + 1);
      let msg;
      try {
        msg = JSON.parse(line);
      } catch (e) {
        continue;
      }
      if (msg.t === 'resync') {
        // The daemon asks us to re-send our state (e.g. after a restart).
        hello();
      } else if (msg.t === 'editor_focus' && msg.focused === true) {
        // The accessibility bus saw focus return to the text editor: any
        // quick-input hint is stale.
        clearQuickInput('editor focused (a11y)');
      }
    }
  });
  s.on('error', (e) => {
    log('socket error:', e && e.code);
    scheduleReconnect();
  });
  s.on('close', () => {
    if (connected) {
      log('disconnected');
    }
    scheduleReconnect();
  });
}

function setQuickInput(name) {
  quickInput = name;
  quickInputAt = Date.now();
  if (quickInputTimer) {
    clearTimeout(quickInputTimer);
  }
  const ttl = Number(cfg().get('quickInputTtlMs')) || 20000;
  quickInputTimer = setTimeout(() => {
    quickInputTimer = null;
    if (quickInput === name) {
      clearQuickInput('ttl');
    }
  }, ttl);
  report();
}

function clearQuickInput(why) {
  if (!quickInput) {
    return;
  }
  log('quick input closed:', why);
  quickInput = '';
  if (quickInputTimer) {
    clearTimeout(quickInputTimer);
    quickInputTimer = null;
  }
  report();
  // The embedded Neovim raises its own hint for quick inputs opened from
  // mappings; tell it the close we observed. Best effort: vscode-neovim may
  // not be installed, or the plugin not loaded.
  vscode.commands
    .executeCommand('vscode-neovim.lua', [
      "pcall(function() require('zmk-vim-mode').clear_vscode_raw('companion') end)",
    ])
    .then(undefined, (e) => log('vscode-neovim.lua unavailable:', e && e.message));
}

function updateEditor(editor) {
  noEditor = editor === undefined;
  // A pick that changed the active editor closed the quick input.
  if (quickInput) {
    clearQuickInput('active editor changed');
  }
  report();
}

// wrap registers a command that raises the raw hint, then runs the original.
function wrap(context, name, original) {
  context.subscriptions.push(
    vscode.commands.registerCommand(name, async (...args) => {
      setQuickInput(original);
      try {
        await vscode.commands.executeCommand(original, ...args);
      } catch (e) {
        log('command failed:', original, e && e.message);
      }
    }),
  );
}

function statusText() {
  const c = current();
  return `zmk-vim-mode: ${connected ? 'connected' : 'not connected'} (${socketPath()}); reporting ${c.mode} (${c.reason})`;
}

function activate(context) {
  out = vscode.window.createOutputChannel('ZMK Vim Mode');
  context.subscriptions.push(out);
  stopped = false;

  noEditor = vscode.window.activeTextEditor === undefined;
  windowFocused = vscode.window.state.focused;

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(updateEditor),
    // Cursor moved in the editor: keys are reaching it again -- unless it is
    // the blur/resync burst that opening the quick input itself causes.
    vscode.window.onDidChangeTextEditorSelection((e) => {
      if (Date.now() - quickInputAt < SELECTION_GRACE_MS) {
        log('ignoring selection change during grace, kind', e.kind);
        return;
      }
      clearQuickInput('selection changed');
    }),
    // Escape inside a quick input: the one close we can observe directly.
    vscode.commands.registerCommand('zmkVimMode.quickInputEscape', async () => {
      clearQuickInput('escape');
      try {
        await vscode.commands.executeCommand('workbench.action.closeQuickOpen');
      } catch (e) {
        log('closeQuickOpen failed:', e && e.message);
      }
    }),
    // Quick inputs close when the window loses focus. Only `focused` counts:
    // the event also fires when `active` flips, i.e. on the very keypress
    // that opens the palette after a pause, which would clear the hint we
    // have just raised.
    vscode.window.onDidChangeWindowState((e) => {
      if (e.focused !== windowFocused) {
        windowFocused = e.focused;
        clearQuickInput('window focus changed');
      }
    }),
    vscode.commands.registerCommand('zmkVimMode.status', () => {
      vscode.window.showInformationMessage(statusText());
    }),
    // Raised by the Neovim plugin when a mapping opens a quick input, so the
    // close detection here (Escape, cursor, active editor, TTL) applies too.
    vscode.commands.registerCommand('zmkVimMode.rawHint', (name) => {
      setQuickInput(typeof name === 'string' && name ? name : 'nvim');
    }),
  );
  wrap(context, 'zmkVimMode.quickOpen', 'workbench.action.quickOpen');
  wrap(context, 'zmkVimMode.showCommands', 'workbench.action.showCommands');
  wrap(context, 'zmkVimMode.gotoLine', 'workbench.action.gotoLine');
  wrap(context, 'zmkVimMode.gotoSymbol', 'workbench.action.gotoSymbol');
  // F2: the rename widget is an input inside the editor; the title cannot see it.
  wrap(context, 'zmkVimMode.rename', 'editor.action.rename');
  context.subscriptions.push(
    vscode.commands.registerCommand('zmkVimMode.renameEscape', async () => {
      clearQuickInput('rename escape');
      try {
        await vscode.commands.executeCommand('cancelRenameInput');
      } catch (e) {
        log('cancelRenameInput failed:', e && e.message);
      }
    }),
  );

  connect();
}

function deactivate() {
  stopped = true;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (quickInputTimer) {
    clearTimeout(quickInputTimer);
    quickInputTimer = null;
  }
  send({ t: 'bye' });
  drop();
}

module.exports = { activate, deactivate };
