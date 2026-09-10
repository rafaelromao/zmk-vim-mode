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

const PLUGIN = '0.1.0';
const RECONNECT_MIN = 250;
const RECONNECT_MAX = 5000;

let out = null;
let sock = null;
let connected = false;
let stopped = false;
let backoff = RECONNECT_MIN;
let reconnectTimer = null;

let noEditor = false; // no active text editor → raw
let quickInput = ''; // name of the quick input we opened, '' when none → raw with TTL
let quickInputTimer = null;
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
  s.on('data', (chunk) => {
    // The daemon may ask us to re-send our state (e.g. after a restart).
    if (String(chunk).includes('"resync"')) {
      hello();
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

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(updateEditor),
    // Cursor moved in the editor: keys are reaching it again.
    vscode.window.onDidChangeTextEditorSelection(() => clearQuickInput('selection changed')),
    // Quick inputs close when the window loses focus.
    vscode.window.onDidChangeWindowState(() => clearQuickInput('window state changed')),
    vscode.commands.registerCommand('zmkVimMode.status', () => {
      vscode.window.showInformationMessage(statusText());
    }),
  );
  wrap(context, 'zmkVimMode.quickOpen', 'workbench.action.quickOpen');
  wrap(context, 'zmkVimMode.showCommands', 'workbench.action.showCommands');
  wrap(context, 'zmkVimMode.gotoLine', 'workbench.action.gotoLine');
  wrap(context, 'zmkVimMode.gotoSymbol', 'workbench.action.gotoSymbol');

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
