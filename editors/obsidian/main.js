// zmk-vim-mode for Obsidian.
//
// Obsidian's vim keybindings come from @replit/codemirror-vim, which exposes a
// CodeMirror-5-style adapter on every editor (view.editMode.editor.cm.cm) with
// a `vim-mode-change` event and the live mode in `cm.state.vim`. This plugin
// reports, over the daemon's unix socket:
//
//   normal / insert / visual   the vim mode, while the text has keyboard focus
//   cmdline                    the vim ":" / "/" dialog has focus
//   raw                        keys must reach Obsidian untouched: no markdown
//                              view, reading view, vim keybindings disabled,
//                              or focus outside the text (sidebar, search,
//                              settings, note title, properties)
//
// Plain CommonJS on purpose: no build step, copy two files into the vault.
'use strict';

const { Plugin, MarkdownView, Platform, Notice } = require('obsidian');

const PLUGIN = '0.1.0';
const RECONNECT_MIN = 250;
const RECONNECT_MAX = 5000;

class ZmkVimModePlugin extends Plugin {
  async onload() {
    if (!Platform.isDesktopApp) {
      return;
    }
    this.net = require('net');
    this.os = require('os');
    this.path = require('path');

    this.sock = null;
    this.connected = false;
    this.stopped = false;
    this.backoff = RECONNECT_MIN;
    this.reconnectTimer = null;
    this.lastSent = null;
    this.attached = new WeakSet(); // adapters we already listen to
    this.eventCm = null; // adapter of the last vim-mode-change
    this.eventMode = null; // its mode: normal | insert | visual | replace

    this.addCommand({
      id: 'status',
      name: 'Status',
      callback: () => new Notice(this.statusText(), 6000),
    });

    const refresh = () => {
      this.attach();
      this.report();
    };
    this.registerEvent(this.app.workspace.on('active-leaf-change', refresh));
    this.registerEvent(this.app.workspace.on('file-open', refresh));
    this.registerEvent(this.app.workspace.on('layout-change', refresh));
    // Focus moving between the text, the vim dialog, the title, the sidebar...
    this.registerDomEvent(document, 'focusin', () => this.report());
    this.registerEvent(
      this.app.workspace.on('window-open', (win) => {
        this.registerDomEvent(win.doc, 'focusin', () => this.report());
      }),
    );

    this.app.workspace.onLayoutReady(() => {
      this.attach();
      this.connect();
    });
  }

  onunload() {
    this.stopped = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.send({ t: 'bye' });
    this.drop();
  }

  // ---- editor state -------------------------------------------------------

  activeView() {
    return this.app.workspace.getActiveViewOfType(MarkdownView);
  }

  // getCM returns the codemirror-vim adapter of a markdown view, or null.
  getCM(view) {
    if (!view) {
      return null;
    }
    const candidates = [
      () => view.editMode.editor.cm.cm,
      () => view.editor.cm.cm,
      () => view.sourceMode.cmEditor.cm.cm,
    ];
    for (const get of candidates) {
      try {
        const cm = get();
        if (cm && cm.state) {
          return cm;
        }
      } catch (e) {
        // shape not present in this Obsidian version; try the next
      }
    }
    return null;
  }

  vimEnabled() {
    try {
      const v = this.app.vault.getConfig('vimMode');
      return v !== false;
    } catch (e) {
      return true;
    }
  }

  // attach listens for vim-mode-change on the active editor (once per adapter).
  attach() {
    const cm = this.getCM(this.activeView());
    if (!cm || this.attached.has(cm)) {
      return;
    }
    this.attached.add(cm);
    try {
      cm.on('vim-mode-change', (ev) => {
        this.eventCm = cm;
        this.eventMode = ev && ev.mode ? String(ev.mode) : null;
        this.report();
      });
    } catch (e) {
      this.attached.delete(cm);
    }
  }

  // state returns what we tell the daemon right now.
  state() {
    const view = this.activeView();
    if (!view || !this.vimEnabled()) {
      return 'raw';
    }
    if (typeof view.getMode === 'function' && view.getMode() === 'preview') {
      return 'raw';
    }
    const cm = this.getCM(view);
    if (!cm) {
      return 'raw';
    }
    const doc = (view.containerEl && view.containerEl.ownerDocument) || document;
    const active = doc.activeElement;
    const editorEl = view.contentEl || view.containerEl;
    if (!active || !editorEl || !editorEl.contains(active)) {
      return 'raw'; // sidebar, search, settings, another pane
    }
    if (active.closest && active.closest('.cm-content')) {
      return this.vimMode(cm);
    }
    // Inside the editor but not in the text: the ":" / "/" dialog.
    if (active.closest && active.closest('.cm-vim-panel, .cm-panels')) {
      return 'cmdline';
    }
    return 'raw'; // inline title, properties, embeds
  }

  vimMode(cm) {
    let mode = this.eventCm === cm ? this.eventMode : null;
    if (!mode) {
      const vs = cm.state && cm.state.vim;
      if (vs && vs.insertMode) {
        mode = 'insert';
      } else if (vs && vs.visualMode) {
        mode = 'visual';
      } else {
        mode = 'normal';
      }
    }
    switch (mode) {
      case 'insert':
      case 'replace':
        return 'insert';
      case 'visual':
        return 'visual';
      default:
        return 'normal';
    }
  }

  // ---- transport ----------------------------------------------------------

  socketPath() {
    if (process.env.ZMK_VIM_MODE_SOCKET) {
      return process.env.ZMK_VIM_MODE_SOCKET;
    }
    return this.path.join(this.os.homedir(), '.local', 'state', 'zmk-vim-mode', 'daemon.sock');
  }

  send(msg) {
    if (!this.connected || !this.sock) {
      return;
    }
    try {
      this.sock.write(JSON.stringify(msg) + '\n');
    } catch (e) {
      this.scheduleReconnect();
    }
  }

  hello() {
    const st = this.state();
    this.send({
      v: 1,
      t: 'hello',
      client: 'obsidian',
      app: 'obsidian',
      pid: process.pid,
      mode: st,
      plugin: PLUGIN,
    });
    this.lastSent = st;
  }

  report(force) {
    const st = this.state();
    if (!force && st === this.lastSent) {
      return;
    }
    this.lastSent = st;
    this.send({ t: 'mode', mode: st });
  }

  drop() {
    this.connected = false;
    if (this.sock) {
      try {
        this.sock.destroy();
      } catch (e) {
        // already gone
      }
      this.sock = null;
    }
    this.lastSent = null;
  }

  scheduleReconnect() {
    this.drop();
    if (this.stopped || this.reconnectTimer) {
      return;
    }
    const delay = this.backoff;
    this.backoff = Math.min(this.backoff * 2, RECONNECT_MAX);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  connect() {
    if (this.stopped || this.connected || this.sock) {
      return;
    }
    const s = this.net.createConnection(this.socketPath());
    this.sock = s;
    s.setEncoding('utf8');
    s.on('connect', () => {
      this.connected = true;
      this.backoff = RECONNECT_MIN;
      this.hello();
    });
    s.on('data', (chunk) => {
      if (String(chunk).includes('"resync"')) {
        this.hello();
      }
    });
    s.on('error', () => this.scheduleReconnect());
    s.on('close', () => this.scheduleReconnect());
  }

  statusText() {
    return `zmk-vim-mode: ${this.connected ? 'connected' : 'not connected'} (${this.socketPath()}); reporting ${this.state()}`;
  }
}

module.exports = ZmkVimModePlugin;
