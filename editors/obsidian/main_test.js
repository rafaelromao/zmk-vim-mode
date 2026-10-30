// Tests for the Obsidian plugin's state(), with the obsidian module stubbed.
// Run: make test-js (node --test editors/obsidian/main_test.js)
'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');

class MarkdownView {}
const stub = {
  Plugin: class {},
  MarkdownView,
  Platform: { isDesktopApp: true },
  Notice: class {},
};
const load = Module._load;
Module._load = function (request, ...rest) {
  return request === 'obsidian' ? stub : load.call(this, request, ...rest);
};
const ZmkVimModePlugin = require('./main.js');
Module._load = load;

// plugin builds a plugin whose active view is a source-mode markdown editor
// with the text focused, in a document that does or does not have focus.
function plugin({ hasFocus }) {
  const text = { closest: (sel) => (sel === '.cm-content' ? {} : null) };
  const doc = { activeElement: text, hasFocus: () => hasFocus };
  const view = Object.assign(new MarkdownView(), {
    getMode: () => 'source',
    editMode: { editor: { cm: { cm: { state: { vim: {} } } } } },
    containerEl: { ownerDocument: doc },
    contentEl: { contains: (el) => el === text },
  });
  const p = new ZmkVimModePlugin();
  p.app = {
    workspace: { getActiveViewOfType: () => view },
    vault: { getConfig: () => true },
  };
  return p;
}

test('the focused editor reports its vim mode', () => {
  assert.equal(plugin({ hasFocus: true }).state(), 'normal');
});

test('another window of the app (Settings) has the keys → raw', () => {
  assert.equal(plugin({ hasFocus: false }).state(), 'raw');
});
