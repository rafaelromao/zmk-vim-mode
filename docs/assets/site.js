/* zmk-vim-mode landing page: the page's own vim mode, the example keymap's
   board, copy buttons, the theme switch and the GIF pause buttons.
   No dependencies and no network. */
(() => {
  "use strict";

  const root = document.documentElement;
  const $ = (sel, from = document) => from.querySelector(sel);
  const $$ = (sel, from = document) => Array.from(from.querySelectorAll(sel));
  const media = (query) =>
    window.matchMedia ? window.matchMedia(query) : { matches: false, addEventListener() {} };
  const load = (key) => {
    try { return window.localStorage.getItem(key); } catch (e) { return null; }
  };
  const save = (key, value) => {
    try { window.localStorage.setItem(key, value); } catch (e) { /* storage is off: nothing to keep */ }
  };
  const reducedMotion = media("(prefers-reduced-motion: reduce)");

  // Jumps (gg, G, { }, a search, :help) land at once, as they do in vim. The
  // root's scroll-behavior is smooth for anchor links, so switch it off for
  // the one call instead of relying on behavior: "instant".
  function instant(scroll) {
    const style = document.documentElement.style;
    const before = style.scrollBehavior;
    style.scrollBehavior = "auto";
    scroll();
    style.scrollBehavior = before;
  }

  /* ---- the board: docs/example-keymap.md --------------------------------- */

  // Three rows of ten keys, left hand then right, then the four thumbs.
  // A key is { t: legend, h: hold legend, to: the mode it switches to }.
  const MOTIONS = new Set(["h", "j", "k", "l", "w", "e", "b", "$", "^", "%", "gg", "G", "^U", "^D"]);
  const isHjkl = (t) => t.length === 1 && "hjkl".includes(t);

  function base() {
    const rows = ["b l d c v j y o u ,", "n r t s g p h a e i", "x q m w z k f ' ; ."];
    const hold = { 10: "gui", 11: "alt", 12: "ctl", 13: "sft", 16: "sft", 17: "ctl", 18: "alt", 19: "gui" };
    const keys = rows.join(" ").split(" ").map((t, i) => ({ t, h: hold[i] || "", hjkl: isHjkl(t) }));
    keys.push({ t: "Esc", h: "nav" }, { t: "Spc" }, { t: "Ret" }, { t: "Bksp", h: "sym" });
    return keys;
  }

  function normal() {
    const rows = [
      ["Esc", "c", "o", "i", "a", "^U", "w", "e", "b", "$"],
      ["^R", "u", "v", "dd", "yy", ":", "h", "j", "k", "l"],
      [".", "x", "p", "/", "n", "^D", "gg", "G", "^", "%"],
    ];
    const to = { c: "insert", o: "insert", i: "insert", a: "insert", v: "visual", ":": "cmdline", "/": "cmdline" };
    const keys = rows.flat().map((t) => ({ t, vim: true, to: to[t] || "", motion: MOTIONS.has(t), hjkl: isHjkl(t) }));
    // The thumbs are &trans: the base layer's show through.
    base().slice(30).forEach((k) => keys.push(Object.assign({}, k, { trans: true })));
    return keys;
  }

  function visual() {
    // VIM_VISUAL's own keys, both Escs among them (the thumb still holds NAV);
    // every other key falls through to NORMAL underneath.
    const own = { 0: ["Esc", "normal"], 1: ["c", "insert"], 12: ["v", "normal"], 13: ["d", "normal"],
      14: ["y", "normal"], 15: [":", "cmdline"], 21: ["x", "normal"], 22: ["p", "normal"],
      30: ["Esc", "normal", "nav"] };
    const keys = normal();
    Object.keys(own).forEach((i) => {
      const [t, to, h = ""] = own[i];
      keys[i] = Object.assign({}, keys[i], { t, h, to, vim: true, own: true, motion: false, trans: false });
    });
    return keys;
  }

  function insert() {
    // Transparent: the base layout, plus an Esc that goes back to NORMAL.
    const keys = base();
    keys[30] = { t: "Esc", h: "nav", to: "normal" };
    return keys;
  }

  function cmdline() {
    // Esc abandons the line, Enter submits it; both end in NORMAL.
    const keys = base();
    keys[30] = { t: "Esc", h: "nav", to: "normal" };
    keys[32] = { t: "Ret", to: "normal" };
    return keys;
  }

  const LAYERS = {
    off: { name: "OFF", under: "no vim layers: the Gallium base", keys: base(),
      label: "The example keymap with vim mode off: only its base layer" },
    normal: { name: "NORMAL", under: "commands, not letters", keys: normal() },
    insert: { name: "INSERT", under: "transparent: the Gallium base", keys: insert() },
    visual: { name: "VISUAL", under: "on top of NORMAL", keys: visual() },
    cmdline: { name: "CMDLINE", under: "the base, with Esc and Enter", keys: cmdline() },
  };

  const board = $("#board");
  const cells = [];
  // Column stagger in key heights, pinky to inner column; the right hand mirrors it.
  const STAGGER = [0.18, 0.06, 0, 0.06, 0.12];

  if (board) {
    const halves = [0, 1].map(() => {
      const half = document.createElement("div");
      half.className = "half";
      board.appendChild(half);
      return half;
    });
    for (let i = 0; i < 34; i++) {
      const el = document.createElement("span");
      let half, col, row, dy;
      if (i < 30) {
        row = Math.floor(i / 10);
        half = i % 10 < 5 ? 0 : 1;
        col = i % 5;
        dy = STAGGER[half === 0 ? col : 4 - col];
      } else {
        row = 3;
        half = i < 32 ? 0 : 1;
        col = [3, 4, 0, 1][i - 30];
        dy = 0.4;
      }
      el.style.gridRow = String(row + 1);
      el.style.gridColumn = String(col + 1);
      el.style.setProperty("--dy", String(dy));
      halves[half].appendChild(el);
      cells.push(el);
    }
  }

  function paint(name) {
    const layer = LAYERS[name];
    if (!board || !layer) return;
    layer.keys.forEach((k, i) => {
      const el = cells[i];
      const cls = ["k"];
      if (k.vim && !k.trans) cls.push("vim");
      if (k.trans) cls.push("trans");
      if (k.motion) cls.push("motion");
      if (k.hjkl) cls.push("hjkl");
      if (k.to) cls.push("to-" + k.to, "tap");
      if (k.own) cls.push("own");
      el.className = cls.join(" ");
      el.textContent = k.t;
      if (k.h) {
        const hold = document.createElement("small");
        hold.textContent = k.h;
        el.appendChild(hold);
      }
    });
    $("#layer-name").textContent = layer.name;
    $("#layer-under").textContent = layer.under;
    board.setAttribute("aria-label", layer.label || "The " + layer.name + " layer of the example keymap");
  }

  const NAMED = { Escape: "Esc", " ": "Spc", Enter: "Ret", Backspace: "Bksp" };

  // The key that sends this keycode on the layer on screen: an exact legend
  // first, then the unshifted letter, then a doubled one (d lights dd).
  function cellFor(e) {
    const keys = LAYERS[mode].keys;
    const want = NAMED[e.key] || e.key;
    let i = keys.findIndex((k) => k.t === want);
    if (i < 0 && want.length === 1) {
      const lower = want.toLowerCase();
      i = keys.findIndex((k) => k.t === lower || k.t === lower + lower);
    }
    return i < 0 ? null : cells[i];
  }

  function light(el) {
    if (!el) return;
    el.classList.add("hit");
    clearTimeout(el.zvmTimer);
    el.zvmTimer = setTimeout(() => el.classList.remove("hit"), 160);
  }

  const strip = $("#strip");
  const SHOWN = { Escape: "⎋", " ": "␣", Enter: "↵", Backspace: "⌫" };

  function logKey(e) {
    if (!strip) return;
    const label = SHOWN[e.key] || (e.key.length === 1 ? e.key : "");
    if (!label) return;
    const chip = document.createElement("span");
    chip.textContent = label;
    strip.appendChild(chip);
    while (strip.children.length > 16) strip.firstElementChild.remove();
  }

  /* ---- messages ----------------------------------------------------------- */

  const lastline = $("#lastline");
  const msgEl = $("#msg");
  const live = $("#live");
  const MODE_MSG = { insert: "-- INSERT --", visual: "-- VISUAL --", off: "vim mode off: Esc turns it back on" };
  let msgTimer = 0;
  let liveTimer = 0;

  function announce(text) {
    clearTimeout(liveTimer);
    liveTimer = setTimeout(() => { live.textContent = text; }, 150);
  }

  function showMode() {
    clearTimeout(msgTimer);
    const text = MODE_MSG[mode] || "";
    msgEl.textContent = text;
    msgEl.className = "msg" + (text ? " mode" : "");
  }

  // A message on the last line, the way vim echoes one; it gives way to the
  // mode message after a few seconds.
  function say(text, kind) {
    clearTimeout(msgTimer);
    msgEl.textContent = text;
    msgEl.className = "msg" + (kind ? " " + kind : "");
    announce(text);
    msgTimer = setTimeout(showMode, 4000);
  }

  /* ---- modes -------------------------------------------------------------- */

  let mode = "normal";
  const modeEl = $("#mode");
  const LABEL = { off: "OFF", normal: "NORMAL", insert: "INSERT", visual: "VISUAL", cmdline: "COMMAND" };

  function setMode(next) {
    if (!LAYERS[next]) return;
    const changed = next !== mode;
    mode = next;
    root.dataset.mode = next;
    modeEl.textContent = LABEL[next];
    $$(".modebar button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.set === next)));
    paint(next);
    if (next !== "cmdline") showMode();
    if (changed) announce(next === "off" ? "vim mode off" : LABEL[next].toLowerCase() + " mode");
  }

  // Quitting vim is when the host sends code 0 and the keyboard drops every
  // vim layer (README: "that is the host's job, and it sends code 0").
  function quitVim() {
    setPending("");
    setMode("off");
    say("vim closed: code 0, no vim layers. Esc starts it again.");
  }

  /* ---- moving around ------------------------------------------------------ */

  const header = $(".statusline");
  const headerOffset = () => (header && getComputedStyle(header).position === "sticky" ? header.offsetHeight : 0);
  const toTop = () => instant(() => window.scrollTo(0, 0));
  const toBottom = () => instant(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const jump = (el) => el && instant(() => el.scrollIntoView({ block: "start" }));

  // j and k glide a step; a held key repeats without queueing animations.
  function scrollLines(n, repeat) {
    if (repeat || reducedMotion.matches) instant(() => window.scrollBy(0, n * 72));
    else window.scrollBy({ top: n * 72, behavior: "smooth" });
  }

  function sectionStep(dir) {
    const secs = $$("main section[id]");
    // Measured against each section's own scroll margin, so the section just
    // jumped to counts as the current one, not as the one above.
    const tops = secs.map((s) => s.getBoundingClientRect().top - parseFloat(getComputedStyle(s).scrollMarginTop || "0"));
    let i = -1;
    if (dir > 0) i = tops.findIndex((t) => t > 2);
    else tops.forEach((t, j) => { if (t < -2) i = j; });
    if (i >= 0) jump(secs[i]);
    else if (dir > 0) toBottom();
    else toTop();
  }

  const ruler = $("#ruler");
  function updateRuler() {
    const max = document.documentElement.scrollHeight - window.innerHeight;
    const y = window.scrollY;
    ruler.textContent = max <= 0 ? "All" : y <= 0 ? "Top" : y >= max - 1 ? "Bot" : Math.round((y / max) * 100) + "%";
  }
  window.addEventListener("scroll", updateRuler, { passive: true });
  window.addEventListener("resize", updateRuler);

  /* ---- search: / ? n N ---------------------------------------------------- */

  let lastPattern = null;
  let lastHit = null;

  function clearFound() {
    if (lastHit) lastHit.classList.remove("found");
    lastHit = null;
  }

  function search(text, dir) {
    const needle = text.toLowerCase();
    const els = $$("main h2, main h3, main p, main li, main td, main th, main pre, main figcaption")
      .filter((el) => el.textContent.toLowerCase().includes(needle));
    if (!els.length) {
      clearFound();
      say("E486: Pattern not found: " + text, "err");
      return;
    }
    let i = lastHit ? els.indexOf(lastHit) : -1;
    let wrapped = false;
    if (i >= 0) {
      i += dir;
      if (i >= els.length) { i = 0; wrapped = true; }
      if (i < 0) { i = els.length - 1; wrapped = true; }
    } else {
      const off = headerOffset() + 16;
      const tops = els.map((el) => el.getBoundingClientRect().top - off);
      if (dir > 0) i = tops.findIndex((t) => t > 1);
      else for (let j = tops.length - 1; j >= 0; j--) if (tops[j] < -1) { i = j; break; }
      if (i < 0) { i = dir > 0 ? 0 : els.length - 1; wrapped = true; }
    }
    if (lastHit) lastHit.classList.remove("found");
    lastHit = els[i];
    lastHit.classList.add("found");
    const hit = lastHit;
    instant(() => hit.scrollIntoView({ block: "center" }));
    if (wrapped) say(dir > 0 ? "search hit BOTTOM, continuing at TOP" : "search hit TOP, continuing at BOTTOM", "err");
    else say((dir > 0 ? "/" : "?") + text);
  }

  /* ---- the command line --------------------------------------------------- */

  const form = $("#cmdform");
  const input = $("#cmd");
  const prefixEl = $("#cmdprefix");
  let cmdOpen = false;
  let cmdPrefix = ":";

  function openCmd(prefix) {
    cmdPrefix = prefix;
    cmdOpen = true;
    syncLastline();
    prefixEl.textContent = prefix;
    input.setAttribute("aria-label", prefix === ":" ? "Command line" : "Search");
    input.value = "";
    msgEl.hidden = true;
    form.hidden = false;
    lastline.classList.add("active");
    setMode("cmdline");
    input.focus({ preventScroll: true });
  }

  function closeCmd() {
    if (!cmdOpen) return;
    cmdOpen = false;
    form.hidden = true;
    msgEl.hidden = false;
    lastline.classList.remove("active");
    input.blur();
    syncLastline();
    setMode("normal");
  }

  const TOPICS = { "": "#main", why: "#why", try: "#try", keys: "#keys", modes: "#modes",
    editors: "#editors", neovim: "#editors", how: "#how", install: "#install", bar: "#bar" };
  const REPO = "https://github.com/rafaelromao/zmk-vim-mode";

  function setTheme(theme) {
    root.dataset.theme = theme;
    save("zvm-theme", theme);
    syncThemeButton();
  }

  function run(line) {
    const cmd = line.trim().replace(/^:+/, "").trim();
    if (!cmd) return;
    let m;
    if ((m = cmd.match(/^h(?:e(?:lp?)?)?(?:\s+(.*))?$/))) {
      const arg = (m[1] || "").trim();
      const topic = arg ? arg.replace(/^zmk-vim-mode-?/, "") : "keys";
      if (!(topic in TOPICS)) { say("E149: Sorry, no help for " + arg, "err"); return; }
      jump($(TOPICS[topic]));
      say(":help " + (arg || "zmk-vim-mode-keys"));
    } else if (/^(q|qa|qa?!|wq|wqa|x|xa|quit|qall|exit)$/.test(cmd)) {
      quitVim();
    } else if (/^w!?$/.test(cmd)) {
      say("E45: 'readonly' option is set (add ! to override)", "err");
    } else if ((m = cmd.match(/^se(?:t)?\s+(bg|background)(?:=(\S*)|(\?))?$/))) {
      if (m[2] === "light" || m[2] === "dark") setTheme(m[2]);
      else if (m[2] !== undefined) { say("E474: Invalid argument: " + m[1] + "=" + m[2], "err"); return; }
      say("  background=" + currentTheme());
    } else if (/^se(t)?(\s|$)/.test(cmd)) {
      say("E518: Unknown option: " + cmd.replace(/^se(t)?\s*/, ""), "err");
    } else if ((m = cmd.match(/^colo(?:r(?:s(?:c(?:h(?:e(?:me?)?)?)?)?)?)?\s+(\S+)$/))) {
      if (m[1] === "tokyonight" || m[1] === "tokyonight-night") setTheme("dark");
      else if (m[1] === "tokyonight-day") setTheme("light");
      else { say("E185: Cannot find color scheme '" + m[1] + "'", "err"); return; }
      say("colorscheme " + m[1]);
    } else if (/^(gh|github|repo)$/.test(cmd)) {
      window.location.href = REPO;
    } else if (/^noh(l(s(e(a(r(ch?)?)?)?)?)?)?$/.test(cmd)) {
      clearFound();
    } else if (cmd === "0" || cmd === "1") {
      toTop();
    } else if (cmd === "$") {
      toBottom();
    } else {
      say("E492: Not an editor command: " + cmd, "err");
    }
  }

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const line = input.value;
    const prefix = cmdPrefix;
    closeCmd();
    if (prefix === ":") { run(line); return; }
    const dir = prefix === "/" ? 1 : -1;
    if (line) {
      lastPattern = { text: line, dir };
      clearFound();
      search(line, dir);
    } else if (lastPattern) {
      lastPattern.dir = dir;
      search(lastPattern.text, dir);
    }
  });

  input.addEventListener("keydown", (e) => {
    // The command line is a real input: the page's own keys stay out of it.
    e.stopPropagation();
    if (e.isComposing) return;
    light(cellFor(e));
    if (e.key === "Escape" || (e.key === "Backspace" && !input.value)) {
      e.preventDefault();
      closeCmd();
    }
  });
  input.addEventListener("blur", () => { if (cmdOpen) closeCmd(); });

  /* ---- keys --------------------------------------------------------------- */

  // The transitions a keyboard infers for an editor that cannot report its
  // mode (README, "Inferring modes on the keyboard"), plus page motions.
  const TO_INSERT = new Set(["i", "a", "o", "s", "c", "I", "A", "O", "S", "C", "R"]);
  const VISUAL_TO_NORMAL = new Set(["Escape", "v", "d", "x", "y", "p", "J", "=", "~", "u"]);
  const EDITABLE = "input, textarea, select, [contenteditable]:not([contenteditable='false'])";
  const INTERACTIVE = "a[href], button, summary, [tabindex]:not([tabindex='-1'])";

  let pending = "";
  let pendingTimer = 0;
  const showcmd = $("#showcmd");
  function setPending(key) {
    pending = key;
    showcmd.textContent = key;
    clearTimeout(pendingTimer);
    if (key) pendingTimer = setTimeout(() => setPending(""), 1000);
  }

  function motion(e) {
    const k = e.key;
    if (k === "g" || k === "Z") {
      if (k === "g" && pending === "g") { setPending(""); toTop(); return true; }
      if (k === "Z" && pending === "Z") { quitVim(); return true; }
      setPending(k);
      return true;
    }
    if (pending === "Z" && k === "Q") { quitVim(); return true; }
    setPending("");
    if (k === "j") scrollLines(1, e.repeat);
    else if (k === "k") scrollLines(-1, e.repeat);
    else if (k === "G") toBottom();
    else if (k === "}") sectionStep(1);
    else if (k === "{") sectionStep(-1);
    else if (k === "n" || k === "N") {
      if (!lastPattern) say("E35: No previous regular expression", "err");
      else search(lastPattern.text, k === "n" ? lastPattern.dir : -lastPattern.dir);
    } else if (k === "u") say("Already at oldest change");
    else return false;
    return true;
  }

  let keysOn = true;

  document.addEventListener("keydown", (e) => {
    if (!keysOn || cmdOpen || e.defaultPrevented || e.isComposing || e.keyCode === 229) return;
    // AltGr arrives as Ctrl+Alt on Windows, and types { and } on many layouts.
    const altGr = e.getModifierState && e.getModifierState("AltGraph");
    if (e.metaKey || ((e.ctrlKey || e.altKey) && !altGr)) return;
    const t = e.target instanceof Element ? e.target : null;
    if (t && t.closest(EDITABLE)) return;
    const k = e.key;
    // Space and Enter keep their meaning on links and buttons.
    if ((k === " " || k === "Enter") && t && t.closest(INTERACTIVE)) return;
    const printable = k.length === 1;
    const typing = mode === "insert" || mode === "off";
    const typed = typing && (k === "Enter" || k === "Backspace");
    if (!printable && k !== "Escape" && !typed) return;
    // A held key repeats only the motions: holding v must not flicker VISUAL.
    if (e.repeat && !typing && k !== "j" && k !== "k") {
      if (printable) e.preventDefault();
      return;
    }

    light(cellFor(e));
    logKey(e);
    // Vim mode off: the keyboard is just a keyboard, and the page lets every
    // key through untouched. Esc turns vim mode on, as focusing an editor would.
    if (mode === "off") {
      if (k === "Escape") setMode("normal");
      return;
    }
    // Space still pages down outside insert mode.
    if (k === " " && mode !== "insert") return;

    if (mode === "insert") {
      if (k === "Escape") setMode("normal");
    } else if (mode === "visual") {
      if (VISUAL_TO_NORMAL.has(k)) setMode("normal");
      else if (k === "c" || k === "s") setMode("insert");
      else if (k === ":") openCmd(":");
      else motion(e);
    } else if (TO_INSERT.has(k)) {
      setPending("");
      setMode("insert");
    } else if (k === "v" || k === "V") {
      setPending("");
      setMode("visual");
    } else if (k === ":" || k === "/" || k === "?") {
      setPending("");
      openCmd(k);
    } else if (k === "Escape") {
      setPending("");
      clearFound();
      showMode();
    } else {
      motion(e);
    }
    // Every printable key is the page's while vim keys are on, so the
    // browser's own single-key shortcuts (Firefox's ' and / find) stay quiet.
    // Esc keeps its default: it still stops a load or leaves full screen.
    if (k !== "Escape") e.preventDefault();
  });

  /* ---- switches: vim keys, theme ------------------------------------------ */

  const keysToggle = $("#keys-toggle");
  function syncLastline() {
    const show = keysOn || cmdOpen;
    lastline.hidden = !show;
    document.body.classList.toggle("has-lastline", show);
  }
  function setKeys(on) {
    keysOn = on;
    keysToggle.setAttribute("aria-pressed", String(on));
    $(".state", keysToggle).textContent = on ? "on" : "off";
    syncLastline();
  }
  const savedKeys = load("zvm-keys");
  // Single-key shortcuts start on where there is a keyboard to press them.
  setKeys(savedKeys ? savedKeys === "on" : media("(hover: hover) and (pointer: fine)").matches);
  keysToggle.hidden = false;
  keysToggle.addEventListener("click", () => {
    setKeys(!keysOn);
    save("zvm-keys", keysOn ? "on" : "off");
    if (!keysOn) setMode("normal");
    say(keysOn ? "vim keys on" : "vim keys off");
  });

  const themeBtn = $("#theme-toggle");
  const prefersLight = media("(prefers-color-scheme: light)");
  const currentTheme = () => root.dataset.theme || (prefersLight.matches ? "light" : "dark");
  function syncThemeButton() {
    const other = currentTheme() === "dark" ? "light" : "dark";
    themeBtn.textContent = "bg=" + other;
    themeBtn.setAttribute("aria-label", "Switch to the " + other + " theme");
  }
  themeBtn.hidden = false;
  syncThemeButton();
  themeBtn.addEventListener("click", () => setTheme(currentTheme() === "dark" ? "light" : "dark"));
  if (prefersLight.addEventListener) prefersLight.addEventListener("change", syncThemeButton);

  /* ---- mode buttons and taps on the board ---------------------------------- */

  $$(".modebar button").forEach((b) => {
    b.addEventListener("click", () => {
      if (b.dataset.set === "cmdline") { openCmd(":"); return; }
      // Leaving the command line by button must not depend on the input's blur.
      if (cmdOpen) closeCmd();
      setMode(b.dataset.set);
    });
  });

  if (board) {
    board.addEventListener("click", (e) => {
      const el = e.target instanceof Element ? e.target.closest(".k") : null;
      const k = el && LAYERS[mode].keys[cells.indexOf(el)];
      if (!k || !k.to) return;
      light(el);
      if (k.to === "cmdline") openCmd(k.t === "/" ? "/" : ":");
      else setMode(k.to);
    });
  }

  /* ---- copy buttons ------------------------------------------------------- */

  function fallbackCopy(text) {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    let ok = false;
    try { ok = document.execCommand("copy"); } catch (e) { ok = false; }
    area.remove();
    return ok;
  }

  $$("pre.copyable").forEach((pre) => {
    const box = document.createElement("div");
    box.className = "codebox";
    pre.parentNode.insertBefore(box, pre);
    box.appendChild(pre);
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy";
    btn.textContent = "copy";
    btn.setAttribute("aria-label", "Copy the code");
    btn.addEventListener("click", async () => {
      const code = $("code", pre).cloneNode(true);
      $$(".p", code).forEach((p) => p.remove());
      const text = code.textContent.replace(/\n+$/, "");
      let ok;
      try { await navigator.clipboard.writeText(text); ok = true; } catch (e) { ok = fallbackCopy(text); }
      btn.textContent = ok ? "yanked" : "select it";
      if (ok) {
        const lines = text.split("\n").length;
        say(lines + (lines === 1 ? " line" : " lines") + " yanked");
      }
      setTimeout(() => { btn.textContent = "copy"; }, 1600);
    });
    box.appendChild(btn);
  });

  /* ---- GIFs: pause, and stills for reduced motion ------------------------- */

  $$("figure.anim").forEach((fig) => {
    const img = $("img", fig);
    const source = $("source", fig);
    const btn = $(".anim-toggle", fig);
    if (!img || !btn) return;
    let playing = !reducedMotion.matches;
    const label = () => {
      btn.textContent = playing ? "pause" : "play";
      btn.setAttribute("aria-label", (playing ? "Pause" : "Play") + " the animation");
    };
    btn.hidden = false;
    label();
    btn.addEventListener("click", () => {
      playing = !playing;
      const src = playing ? img.dataset.gif : img.dataset.still;
      if (source) source.srcset = src;
      img.src = src;
      label();
    });
  });

  paint("normal");
  updateRuler();
})();
