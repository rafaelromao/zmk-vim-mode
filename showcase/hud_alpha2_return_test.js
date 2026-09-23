#!/usr/bin/env node
"use strict";

// Exercise the real HUD renderer with the take's key interval and recording press_ms.
// Override ZMK_LAYER_HUD when the checkout is not ~/projects/zmk-layer-hud.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const hudRoot = process.env.ZMK_LAYER_HUD || path.join(os.homedir(), "projects/zmk-layer-hud");
const { loadPage } = require(path.join(hudRoot, "hud/tests/dom.js"));
const keymap = JSON.parse(fs.readFileSync(
  path.join(hudRoot, "hud/tests/fixtures/diamond.json"), "utf8"));
const alpha2 = Number(Object.entries(keymap.zmk_layers)
  .find(([, layer]) => layer.drawer === "alpha2")[0]);
const position = Number(Object.keys(keymap.positions)[0]);
const alpha2ThumbPosition = Number(Object.entries(keymap.positions)
  .find(([, index]) => Number(index) === 22)[0]);
const qIndex = keymap.layers.alpha2.findIndex(key => key.tap === "q");
const nextCharacterMs = 60_000 / 70 / 5; // rehearse.py TYPE_GAP, ~171 ms

function layersAtNextCharacter(pressMs) {
  const page = loadPage();
  const data = JSON.parse(JSON.stringify(keymap));
  data.hud.press_ms = pressMs;
  page.hud.load(data);
  page.hud.setLayers([alpha2]);
  page.hud.pressAt(position);
  page.hud.releaseAt(position);
  page.hud.setLayers([]); // the feed's key-up restore to Alpha 1
  page.clock.advance(nextCharacterMs);
  return [...page.hud.state.live.ids];
}

function highlightedAfterCharacter(synthetic) {
  const page = loadPage();
  const data = JSON.parse(JSON.stringify(keymap));
  data.hud.press_ms = 100;
  page.hud.load(data);
  page.hud.setLayers([alpha2]);
  // Match rehearsal-feed.py: enter Alpha 2, type its character, then flash the
  // sticky thumb through pressAt/releaseAt. A following injected character must
  // still be resolved by the renderer despite that synthetic position report.
  page.hud.pressAt(alpha2ThumbPosition);
  page.hud.releaseAt(alpha2ThumbPosition);
  page.clock.advance(data.hud.release_ms + 1);
  page.hud.key({
    type: "keyDown", name: "q", chars: "q", code: 16, flags: {}, synthetic,
  });
  const pressed = [];
  page.hud.state.keyEls.forEach((element, index) => {
    if (element.classList.contains("pressed")) pressed.push(index);
  });
  return pressed;
}

assert.deepEqual(layersAtNextCharacter(500), [alpha2],
  "the old 500 ms hold should reproduce the stale Alpha 2 banner");
assert.deepEqual(layersAtNextCharacter(100), [],
  "Alpha 2 must be gone before the next character; [] draws Alpha 1");
assert.deepEqual(highlightedAfterCharacter(true), [qIndex],
  "a synthetic character after the Alpha 2 thumb flash must highlight its mapped key");
assert.deepEqual(highlightedAfterCharacter(false), [],
  "a hardware character after a position report must retain the freshness guard");
console.log("HUD layer matches the typed-key cadence: Alpha 1 is active before the next key.");
