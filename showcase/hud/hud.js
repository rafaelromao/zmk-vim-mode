/* ZMK layer HUD — renderer + keymap emulator.
 *
 * Data comes from keymap.json (built from the keyboards repo's keymap-drawer YAML).
 * Hammerspoon drives it through window.hud:
 *   hud.load(data)                     keymap.json contents
 *   hud.setMode(code, mode, reason)    the daemon's decision (ground truth for vim layers)
 *   hud.key({type, chars, name, flags}) one keyboard event from the event tap
 *   hud.press([idx...])                light keys directly (tests)
 * In a browser (http://) it fetches keymap.json itself and accepts real key events,
 * so the page can be developed and checked without Hammerspoon.
 */
(function () {
  "use strict";

  const KEY = 62, GAP = 6;
  const PRESS_MS = 320;
  const MOMENTARY_MS = 700;

  // Layers searched when a key is not on the active stack, in order of likelihood.
  const SEARCH_ORDER = ["alpha2", "shifted1", "shifted2", "numbers", "symbols", "nav", "shortcuts",
    "media", "text", "func", "macros", "toggles", "ç-extension"];
  const ONE_SHOT = new Set(["alpha2", "shifted1", "shifted2", "ç-extension"]);

  // Named keys → the legend text the drawer YAML uses for them.
  const NAMED = {
    space: "␣", return: "↵", escape: "⎋", delete: "⌫", forwarddelete: "⌦", tab: "⇥",
    left: "←", right: "→", up: "↑", down: "↓", home: "⇱", end: "⇲",
    pagedown: "⇟", pageup: "⇞",
  };
  const ESC_LEGENDS = new Set(["⎋"]);

  const state = {
    data: null,
    code: 0, mode: "off", reason: "", provisional: false,
    baseLayers: ["alpha1"],
    momentary: [],           // [{layer, until}]
    oneShot: null,           // layer name
    keyEls: [],
    timers: new Map(),
  };

  // ---------- rendering ----------

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function buildBoard() {
    const lay = state.data.layout;
    for (const hand of ["L", "R"]) {
      const host = document.getElementById("hand-" + hand);
      host.innerHTML = "";
      const cols = hand === "L" ? lay.left_cols : lay.right_cols;
      host.style.width = (cols * (KEY + GAP) - GAP) + "px";
      host.style.height = ((lay.rows + 1) * (KEY + GAP) - GAP + 8) + "px";
    }
    state.keyEls = [];
    for (const k of lay.keys) {
      const host = document.getElementById("hand-" + k.hand);
      const e = el("div", "key");
      e.dataset.idx = k.idx;
      e.title = `#${k.idx} ${k.name} (ZMK ${k.zmk})`;
      const y = k.row * (KEY + GAP) + (k.thumb ? 8 : 0);
      e.style.left = (k.col * (KEY + GAP)) + "px";
      e.style.top = y + "px";
      e.appendChild(el("div", "shifted"));
      e.appendChild(el("div", "tap"));
      e.appendChild(el("div", "hold"));
      host.appendChild(e);
      state.keyEls[k.idx] = e;
    }
  }

  function stack() {
    // top first
    const s = [];
    if (state.oneShot) s.push(state.oneShot);
    for (let i = state.momentary.length - 1; i >= 0; i--) s.push(state.momentary[i].layer);
    for (let i = state.baseLayers.length - 1; i >= 0; i--) s.push(state.baseLayers[i]);
    return s;
  }

  // The binding that wins at idx, walking the stack through transparent keys.
  function resolveBinding(idx, layersTopFirst) {
    for (const name of layersTopFirst) {
      const k = state.data.layers[name][idx];
      if (!k) continue;
      if (k.type === "trans") continue;
      return { key: k, layer: name };
    }
    return null;
  }

  function fit(tapEl, text) {
    tapEl.textContent = text;
    tapEl.classList.remove("long", "mid");
    if (text.length > 4) tapEl.classList.add("long");
    else if (text.length > 2) tapEl.classList.add("mid");
  }

  function renderKeys() {
    const layers = stack();
    const activators = new Set();
    for (const m of state.momentary) for (const a of activatorsOf(m.layer)) activators.add(a);
    if (state.oneShot) for (const a of activatorsOf(state.oneShot)) activators.add(a);
    for (const k of state.data.layout.keys) {
      const e = state.keyEls[k.idx];
      const r = resolveBinding(k.idx, layers);
      e.classList.remove("trans", "blank", "held", "activator");
      if (!r) {
        e.classList.add("blank");
        fit(e.querySelector(".tap"), "");
        e.querySelector(".hold").textContent = "";
        e.querySelector(".shifted").textContent = "";
        continue;
      }
      const top = layers[0];
      if (r.layer !== top && state.data.layers[top][k.idx].type === "trans") e.classList.add("trans");
      if (r.key.type === "blank") e.classList.add("blank");
      if (r.key.type.startsWith("held")) e.classList.add("held");
      if (activators.has(k.idx)) e.classList.add("activator");
      fit(e.querySelector(".tap"), r.key.tap || "");
      e.querySelector(".hold").textContent = r.key.hold || "";
      e.querySelector(".shifted").textContent = r.key.shifted || "";
    }
  }

  const LAYER_LABEL = { alpha1: "Alpha 1", alpha2: "Alpha 2", shifted1: "Shift", shifted2: "Shift · Alpha 2",
    "ç-extension": "Ç extension", vim: "Vim", numbers: "Numbers", symbols: "Symbols", nav: "Navigation",
    shortcuts: "Shortcuts", media: "Media / mouse", text: "Text navigation", func: "Function keys",
    macros: "Macros", toggles: "Toggles", mehs: "Mehs" };

  // The layer the daemon's code puts the keyboard in.
  function baseSummary() {
    switch (state.code) {
      case 1: return { name: "Vim normal", cls: "vim", sub: "VIM_NORMAL" };
      case 3: return { name: "Vim visual", cls: "vim", sub: "VIM_VISUAL over VIM_NORMAL" };
      case 4: case 7: return { name: "Vim normal", cls: "vim", sub: "inferred by the keyboard (legacy)" };
      case 2: return { name: "Vim insert", cls: "vim-insert", sub: "VIM_INSERT is transparent: alpha 1 shows through" };
      case 5: return { name: "Vim cmdline", cls: "vim-cmdline", sub: "VIM_CMDLINE is transparent: alpha 1 shows through" };
      case 6: return { name: "Raw", cls: "raw", sub: "no vim layers: keys pass through untouched" };
      default: return { name: "Alpha 1", cls: "off", sub: "no vim editor focused" };
    }
  }

  // What to print on the banner: a held or one-shot layer on top of the base, else the base.
  function activeSummary() {
    const base = baseSummary();
    const top = state.oneShot || (state.momentary.length ? state.momentary[state.momentary.length - 1].layer : null);
    if (!top) return base;
    return { name: LAYER_LABEL[top] || top, cls: "momentary",
      sub: (state.oneShot ? "one shot" : "held") + " · over " + base.name.toLowerCase() };
  }

  function renderBanner() {
    const a = activeSummary();
    const layer = document.getElementById("layer");
    layer.className = a.cls + (state.provisional ? " provisional" : "");
    document.getElementById("layerName").textContent = a.name;
    document.getElementById("layerSub").textContent = a.sub;
    document.getElementById("reason").textContent = state.reason || "";
    document.getElementById("board").className = a.cls;
  }

  function render() { renderBanner(); renderKeys(); }

  // ---------- emulator ----------

  function activatorsOf(layer) {
    return state.data.activators.filter(a => a.layer === layer).map(a => a.idx);
  }

  // A combo is drawn the way keymap-drawer draws it: a pill with the combo's legend at the
  // midpoint of its keys, on top of the flashed keys, fading after a moment.
  const COMBO_MS = 1000;
  function showCombo(positions, key) {
    const board = document.getElementById("board");
    const bb = board.getBoundingClientRect();
    const centers = positions.map(idx => {
      const r = state.keyEls[idx].getBoundingClientRect();
      return { x: r.left + r.width / 2 - bb.left, y: r.top + r.height / 2 - bb.top };
    });
    const cx = centers.reduce((a, c) => a + c.x, 0) / centers.length;
    const cy = centers.reduce((a, c) => a + c.y, 0) / centers.length - 30;

    // Links from the output pill down to each key of the combo.
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("class", "combo-links");
    svg.setAttribute("width", bb.width); svg.setAttribute("height", bb.height);
    for (const c of centers) {
      const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
      line.setAttribute("x1", cx); line.setAttribute("y1", cy);
      line.setAttribute("x2", c.x); line.setAttribute("y2", c.y);
      svg.appendChild(line);
    }
    board.appendChild(svg);

    const pill = el("div", "combo-pill");
    pill.appendChild(el("span", "combo-label", "combo"));
    pill.appendChild(el("span", "combo-tap", key.tap || ""));
    if (key.hold || key.shifted) pill.appendChild(el("span", "combo-sub", key.hold || key.shifted));
    pill.style.left = cx + "px"; pill.style.top = cy + "px";
    board.appendChild(pill);
    requestAnimationFrame(() => { pill.classList.add("show"); svg.classList.add("show"); });
    for (const idx of positions) state.keyEls[idx].classList.add("combo-key");
    setTimeout(() => {
      pill.classList.remove("show"); svg.classList.remove("show");
      for (const idx of positions) state.keyEls[idx].classList.remove("combo-key");
      setTimeout(() => { pill.remove(); svg.remove(); }, 200);
    }, COMBO_MS);
  }

  function comboFor(layer, token) {
    return state.data.combos.find(c => c.layers.includes(layer) && c.key.tap === token) || null;
  }

  function flash(indices, cls) {
    for (const idx of indices) {
      const e = state.keyEls[idx];
      if (!e) continue;
      e.classList.add("pressed");
      if (cls) e.classList.add(cls);
      clearTimeout(state.timers.get(idx));
      state.timers.set(idx, setTimeout(() => e.classList.remove("pressed", "combo"), PRESS_MS));
    }
  }

  function setLast() { /* the key log was removed from the panel: it confused more than it helped */ }

  // Letter combos on alpha1 (ns=q, mg=k, …) are vim commands, never typing: outside the vim
  // layer they are ignored, so a typed q/x/z/… is attributed to the sticky alpha2 layer.
  function findOnLayer(layer, token, letterCombos) {
    const keys = state.data.layers[layer];
    for (let i = 0; i < keys.length; i++) if (keys[i].tap === token && keys[i].type !== "trans") return [i];
    const isLetter = /^[a-zA-ZçÇ]$/.test(token);
    for (const c of state.data.combos) {
      if (!c.layers.includes(layer) || c.key.tap !== token) continue;
      if (isLetter && layer === "alpha1" && !letterCombos) continue;
      return c.positions.slice();
    }
    // magic key: the drawer legend is "h|v"; either letter lights it
    if (layer === "alpha1" && (token === "h" || token === "v")) {
      const i = keys.findIndex(k => k.tap === "h|v");
      if (i >= 0) return [i];
    }
    return null;
  }

  function tokenFor(ev) {
    if (ev.name && NAMED[ev.name]) return NAMED[ev.name];
    if (ev.chars && ev.chars.length === 1) return ev.chars;
    return null;
  }

  function armMomentary(layer) {
    const now = Date.now();
    const m = state.momentary.find(x => x.layer === layer);
    if (m) m.until = now + MOMENTARY_MS; else state.momentary.push({ layer, until: now + MOMENTARY_MS });
    setTimeout(expire, MOMENTARY_MS + 20);
  }

  function expire() {
    const now = Date.now();
    const before = state.momentary.length;
    state.momentary = state.momentary.filter(m => m.until > now);
    if (state.momentary.length !== before) render();
  }

  // Local inference: the transitions the firmware itself performs, so the chip moves on the
  // keystroke; the daemon's decision replaces it a moment later.
  function inferVim(token) {
    if (!state.data.codes[String(state.code)].vim) return;
    const inNormal = state.code === 1 || state.code === 4 || state.code === 7;
    if (inNormal || state.code === 3) {
      if ("iaosc".includes(token)) return provisional(2);
      if (token === ":" || token === "/") return provisional(5);
      if (token === "v") return provisional(state.code === 3 ? 1 : 3);
    }
    if (ESC_LEGENDS.has(token) && state.code !== 1) return provisional(1);
    if (state.code === 5 && token === "↵") return provisional(1);
  }

  function provisional(code) {
    state.code = code;
    state.mode = state.data.codes[String(code)].mode;
    state.provisional = true;
    state.baseLayers = state.data.codes[String(code)].layers.slice();
    render();
  }

  function handleKey(ev) {
    if (!state.data) return;
    if (ev.type === "flagsChanged") return;
    if (ev.type !== "keyDown" || ev.repeat) return;
    const token = tokenFor(ev);
    if (!token) { setLast(`<span class="k">${ev.name || "?"}</span> not on the keymap`); return; }

    const layers = stack();
    const vimActive = layers.includes("vim");
    // 0. Typing goes through the two alpha layers: a letter that is not a plain alpha1 key
    //    comes from the sticky alpha2 layer (one shot), never from an alpha1 letter combo.
    if (/^[a-zçA-ZÇ]$/.test(token) && !vimActive && !state.oneShot) {
      const lower = token.toLowerCase();
      const onAlpha1 = state.data.layers.alpha1.some(k => k.tap === lower && k.type !== "trans");
      const direct = state.data.layers.alpha2.findIndex(k => k.tap === lower && k.type !== "trans");
      if (!onAlpha1 && direct >= 0) {
        state.oneShot = "alpha2";
        render();
        const extra = activatorsOf("alpha2").concat(/^[A-ZÇ]$/.test(token) ? activatorsOf("shifted1") : []);
        flash([direct].concat(extra));
        setLast(`<span class="k">${token}</span> alpha2 #${direct}`);
        inferVim(token);
        afterKey();
        return;
      }
    }
    // 1. the active stack, top first (uppercase letters live on shifted1)
    const searchStack = /^[A-ZÀ-Ý]$/.test(token) ? ["shifted1", ...layers] : layers;
    for (const layer of searchStack) {
      const hit = findOnLayer(layer, token, vimActive);
      if (hit) {
        const extra = [];
        if (layer === "shifted1" && !layers.includes("shifted1")) extra.push(...activatorsOf("shifted1"));
        flash(hit.concat(extra), hit.length > 1 ? "combo" : null);
        if (hit.length > 1) { const c = comboFor(layer, token); if (c) showCombo(hit, c.key); }
        setLast(`<span class="k">${token}</span> ${layer} ${hit.length > 1 ? "combo" : ""} #${hit.join("+")}`);
        touchLayer(layer);
        inferVim(token);
        afterKey();
        return;
      }
    }
    // 2. any other layer → a momentary or one-shot activation we could not see.
    //    Letters most likely came from the sticky alpha2 layer; anything else from a held
    //    numbers/symbols thumb (":" in normal mode is the symbols layer's th_colon_vim, not
    //    alpha2's combo).
    const order = /^[a-zA-ZçÇ]$/.test(token)
      ? SEARCH_ORDER
      : SEARCH_ORDER.filter(l => !ONE_SHOT.has(l)).concat(SEARCH_ORDER.filter(l => ONE_SHOT.has(l)));
    for (const layer of order) {
      if (!state.data.layers[layer]) continue;
      const hit = findOnLayer(layer, token, vimActive);
      if (hit) {
        if (ONE_SHOT.has(layer)) state.oneShot = layer; else armMomentary(layer);
        render();
        flash(hit.concat(activatorsOf(layer)), hit.length > 1 ? "combo" : null);
        if (hit.length > 1) { const c = comboFor(layer, token); if (c) showCombo(hit, c.key); }
        setLast(`<span class="k">${token}</span> ${layer} #${hit.join("+")}`);
        inferVim(token);
        afterKey();
        return;
      }
    }
    setLast(`<span class="k">${token}</span> not on the keymap`);
    afterKey();
  }

  function touchLayer(layer) {
    const m = state.momentary.find(x => x.layer === layer);
    if (m) { m.until = Date.now() + MOMENTARY_MS; return; }
    // a key that resolved only in the base drops any momentary layer
    if (state.baseLayers.includes(layer) && state.momentary.length) { state.momentary = []; render(); }
  }

  // A one-shot layer is consumed by the key, but stays on the banner long enough to be seen.
  let oneShotTimer = null;
  function afterKey() {
    if (!state.oneShot) return;
    clearTimeout(oneShotTimer);
    oneShotTimer = setTimeout(() => { state.oneShot = null; render(); }, 450);
  }

  // ---------- public API ----------

  const hud = {
    load(data) {
      state.data = data;
      buildBoard();
      hud.setMode(state.code, state.mode, state.reason);
    },
    setMode(code, mode, reason) {
      code = Number(code) || 0;
      const spec = state.data.codes[String(code)] || state.data.codes["0"];
      state.code = code; state.mode = mode || spec.mode; state.reason = reason || "";
      state.provisional = false;
      state.baseLayers = spec.layers.slice();
      render();
    },
    key(ev) { handleKey(typeof ev === "string" ? JSON.parse(ev) : ev); },
    press(indices) { flash(indices); },
    setLayers(names) { state.baseLayers = names.slice(); render(); },
    state,
  };
  window.hud = hud;

  // ✕: tell the host to close. Hammerspoon listens on a user-content controller; a
  // WebSocket host (showcase/linux) receives {"kind":"close"}.
  document.getElementById("close").addEventListener("click", () => {
    try { window.webkit.messageHandlers.zmkhud.postMessage("close"); } catch (e) { /* not WebKit */ }
    if (hud.socket && hud.socket.readyState === 1) hud.socket.send(JSON.stringify({ kind: "close" }));
  });

  // Generic host: index.html?ws=ws://127.0.0.1:8766 — messages are
  //   {"kind":"key", ...event}  {"kind":"mode","code":N,"mode":"…","reason":"…"}
  // (showcase/linux/keyfeed.py speaks this).
  const wsUrl = new URLSearchParams(location.search).get("ws");
  if (wsUrl) {
    const connect = () => {
      const s = new WebSocket(wsUrl);
      hud.socket = s;
      s.onmessage = e => {
        const m = JSON.parse(e.data);
        if (m.kind === "key") hud.key(m);
        else if (m.kind === "mode") hud.setMode(m.code, m.mode, m.reason);
      };
      s.onclose = () => setTimeout(connect, 1000);
    };
    connect();
  }

  // Data is inlined by keymap/build.py as keymap.js. Real key events are accepted too, so the
  // page can be exercised in a browser; inside the Hammerspoon webview the window never has
  // focus, so only the event-tap feed reaches it.
  if (window.KEYMAP) hud.load(window.KEYMAP);
  if (location.protocol.startsWith("http")) window.addEventListener("keydown", e => {   // dev only
    const name = e.key.length === 1 ? null : e.key.toLowerCase().replace("arrow", "").replace("backspace", "delete").replace("enter", "return");
    hud.key({ type: "keyDown", chars: e.key.length === 1 ? e.key : "", name, flags: {} });
    if (e.key !== "F5" && !e.metaKey) e.preventDefault();
  });
})();
