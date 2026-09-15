-- Rehearsal runner for the showcase: performs the scripted actions of SCRIPT.md with
-- synthesized keystrokes, asks the daemon what it decided after each one, and writes a
-- PASS/FAIL report to showcase/run/rehearsal.log.
--
--   bash showcase/rehearse.sh [3|4|5|6|7|8|all]
--
-- Run showcase/prepare.sh first: it puts every editor into a clean state (no leftover tabs,
-- panels or view modes), which the expectations below assume. Report: showcase/run/rehearsal.log.
-- Keep your hands off the keyboard and mouse while it runs: every step types into the
-- frontmost window. Synthesized keys do not pass through the Diamond, so the keyboard's
-- layers still follow the daemon, but nothing here proves what the physical keys do; that
-- is what the recording is for.

local M = {}

local HERE = debug.getinfo(1, "S").source:sub(2):match("(.*/)") or "./"
local ROOT = HERE .. "../"          -- showcase/
local HOME = os.getenv("HOME")
local BINARY = HOME .. "/.local/bin/zmk-vim-mode"
local REPORT = ROOT .. "run/rehearsal.log"
local VAULT = "Demo"                -- showcase/Demo, registered in Obsidian under that folder name

local steps, i, results, logf = {}, 0, { pass = 0, fail = 0 }, nil
-- hs.timer objects are garbage-collected if nothing references them, which kills a
-- doAfter chain mid-run; every timer this module creates is parked here.
local timers = {}
local function after(delay, fn)
  local t
  t = hs.timer.doAfter(delay, function() timers[t] = nil; fn() end)
  timers[t] = true
  return t
end

local function log(line)
  print("[rehearse] " .. line)
  if logf then logf:write(os.date("%H:%M:%S "), line, "\n"); logf:flush() end
end

local function status()
  local out = hs.execute(BINARY .. " status --json 2>&1")
  local ok, st = pcall(hs.json.decode, out or "")
  if ok and type(st) == "table" then return st end
  return { mode = "?", reason = "status failed: " .. tostring(out) }
end

-- ---------- step builders ----------

local function add(desc, fn, wait) steps[#steps + 1] = { desc = desc, fn = fn, wait = wait or 0.6 } end

-- The window that started the run (your terminal) is never typed into: if focus lands back on
-- it the run aborts instead of spraying keys into your shell.
local origin

local function guarded(fn)
  return function()
    local w = hs.window.focusedWindow()
    if origin and w and w:id() == origin then
      log("ABORT  focus is on the terminal that started the rehearsal; not typing into it")
      steps = {}
      return
    end
    fn()
  end
end

-- Type text. Each "\n" becomes its own Return step, after the text before it has been
-- typed: keyStrokes paces characters, so a Return posted in the same step would overtake them.
local function keys(text, wait)
  local parts = {}
  for piece, nl in text:gmatch("([^\n]*)(\n?)") do
    if piece ~= "" then parts[#parts + 1] = { text = piece } end
    if nl == "\n" then parts[#parts + 1] = { ret = true } end
  end
  -- One step per character, so every key is seen landing on the HUD and the strip.
  local CHAR_GAP = 0.16
  local total = 0
  for _, part in ipairs(parts) do total = total + (part.ret and 1 or utf8.len(part.text)) end
  local n = 0
  for _, part in ipairs(parts) do
    if part.ret then
      n = n + 1
      add("⏎", guarded(function() hs.eventtap.keyStroke({}, "return", 0) end), (n == total) and wait or 0.35)
    else
      for _, cp in utf8.codes(part.text) do
        n = n + 1
        local ch = utf8.char(cp)
        add("type " .. ch, guarded(function() hs.eventtap.keyStrokes(ch) end), (n == total) and wait or CHAR_GAP)
      end
    end
  end
end

local function key(mods, k, wait)
  add((#mods > 0 and (table.concat(mods, "+") .. "+") or "") .. k, guarded(function() hs.eventtap.keyStroke(mods, k, 0) end), wait)
end

local function esc(wait) key({}, "escape", wait) end

local function shell(cmd, wait)
  add("$ " .. cmd, function() hs.execute(cmd) end, wait)
end

local P = dofile(HERE .. "place.lua")
local recordingScreen = P.recordingScreen

-- Fill the recording display with the window, minus the typed-keys band (see place.lua).
local function place(w)
  if not w or w:id() == origin then return end
  P.place(w)
end

-- Focus an app that owns its windows (VS Code, IntelliJ, Obsidian, Safari).
local function focus(bundleOrName, wait)
  add("focus " .. bundleOrName, function()
    if not hs.application.launchOrFocusByBundleID(bundleOrName) then hs.application.launchOrFocus(bundleOrName) end
    after(0.6, function() place(hs.window.focusedWindow()) end)
  end, wait or 2.0)
end

-- Open something with `open …` and place the app's window on the recording display as soon as
-- it appears, instead of after the whole start-up wait.
local function openAndPlace(appName, cmd, wait)
  add("$ " .. cmd, function()
    hs.execute(cmd)
    local tries = 0
    local function find()
      tries = tries + 1
      local app = hs.application.get(appName)
      local w = app and (app:focusedWindow() or app:mainWindow())
      if w and w:id() ~= origin then place(w); return end
      if tries < 40 then after(0.5, find) end
    end
    after(0.5, find)
  end, wait)
end

-- Open a NEW window of an app that may already be running (Ghostty): remember the app's
-- windows, launch, then focus the one that appeared — never an existing one.
local function openNewWindow(appName, launch, wait)
  add("new " .. appName .. " window", function()
    local before = {}
    for _, w in ipairs(hs.window.allWindows()) do
      local a = w:application()
      if a and a:name() == appName then before[w:id()] = true end
    end
    launch()
    local tries = 0
    local function find()
      tries = tries + 1
      for _, w in ipairs(hs.window.allWindows()) do
        local a = w:application()
        if a and a:name() == appName and not before[w:id()] and w:id() ~= origin then
          w:focus(); place(w)
          log("using new " .. appName .. " window #" .. w:id())
          return
        end
      end
      if tries < 10 then after(0.5, find) else log("WARN  no new " .. appName .. " window appeared") end
    end
    after(1.0, find)
  end, wait or 6)
end

-- expect(mode, reasonPattern): mode is strict, the reason is a plain-text substring and is
-- reported but does not fail the step when the mode matched.
local function expect(mode, reason)
  add("expect " .. mode .. (reason and (" (" .. reason .. ")") or ""), function()
    local st = status()
    local okMode = st.mode == mode
    local okReason = (reason == nil) or (tostring(st.reason or ""):find(reason, 1, true) ~= nil)
    local verdict = okMode and "PASS" or "FAIL"
    if okMode and not okReason then verdict = "PASS (reason differs)" end
    results[okMode and "pass" or "fail"] = results[okMode and "pass" or "fail"] + 1
    log(string.format("%s  want mode=%s%s  got mode=%s code=%s reason=%q",
      verdict, mode, reason and (" reason~" .. reason) or "", tostring(st.mode), tostring(st.code), tostring(st.reason)))
  end, 0.1)
end

local function pause(s) add("wait " .. s .. "s", function() end, s) end

-- Click at a fraction of the focused window (fx, fy in 0..1): how a person moves focus back
-- into an editor pane when no hotkey does it.
local function click(fx, fy, wait)
  add(string.format("click %.2f,%.2f", fx, fy), function()
    local w = hs.window.focusedWindow()
    if not w or w:id() == origin then return end
    local f = w:frame()
    hs.eventtap.leftClick({ x = f.x + f.w * fx, y = f.y + f.h * fy }, 0)
  end, wait or 0.8)
end

-- The vim tour every editor gets: motions on the home row (h j k l), word motions (w e b),
-- the insert commands (i, a) — one key at a time so each one lights on the HUD's vim layer.
local function vimTour()
  for _, k in ipairs({ "j", "j", "j", "k", "l", "l", "l", "h", "h" }) do keys(k, 0.35) end
  expect("normal")
  for _, k in ipairs({ "w", "w", "e", "b", "b" }) do keys(k, 0.35) end
  for _, k in ipairs({ "0", "$", "0" }) do keys(k, 0.35) end        -- line start/end
  keys("i", 0.5);            expect("insert")                       -- insert before the cursor
  esc(0.5);                  expect("normal")
  keys("a", 0.5);            expect("insert")                       -- append after it
  esc(0.5);                  expect("normal")
  keys("gg", 0.5)
end

-- ---------- segments ----------

local S = {}

-- Beat 3: the daemon's manual override moves the keyboard and the HUD.
S["3"] = function()
  log("--- segment 3: how it works (set overrides)")
  shell(BINARY .. " set insert", 0.5);  expect("insert")
  shell(BINARY .. " set normal", 0.5);  expect("normal")
  shell(BINARY .. " set off", 0.5);     expect("off")
  shell(BINARY .. " set off", 0.5)      -- same mode again returns to auto
  add("check no override", function()
    local st = status()
    log((st.override == nil and "PASS" or "FAIL") .. "  override cleared: " .. (st.override and hs.json.encode(st.override) or "none"))
    results[st.override == nil and "pass" or "fail"] = results[st.override == nil and "pass" or "fail"] + 1
  end)
end

-- Beat 4: Neovim in a fresh Ghostty window (demo shell), LazyVim.
S["4"] = function()
  log("--- segment 4: Neovim")
  openNewWindow("Ghostty", function()
    local t = hs.task.new("/usr/bin/open", nil, { "-na", "Ghostty", "--args",
      "--config-file=" .. ROOT .. "env/ghostty-demo.conf", "--working-directory=" .. ROOT .. "demo-go" })
    t:setEnvironment({ ZDOTDIR = ROOT .. "env", HOME = HOME, PATH = os.getenv("PATH") or "/usr/bin:/bin" })
    t:start()
  end, 7)
  keys("nvim\n", 4)
  expect("raw", "nvim client")                       -- dashboard
  keys("f", 1.5)                                     -- find file (dashboard)
  keys("modes.go", 1.0); key({}, "return", 1.5)
  expect("normal", "nvim client")
  vimTour()
  keys("/Compose\n", 1.0);  expect("normal")               -- "/" showed CMDLINE on the way
  keys("A", 0.5);            expect("insert")               -- append at the end of that line
  keys(" // bit 0 of the code", 0.5)
  esc(0.5);                  expect("normal")
  keys("u", 0.4)                                            -- the file is unchanged again
  keys("v", 0.5);            expect("visual")
  keys("jj", 0.3); keys("y", 0.5); expect("normal")
  keys(":", 0.5);            expect("cmdline")
  esc(0.5);                  expect("normal")
  keys(" ", 0.25);           expect("raw", "nvim client")   -- leader pending: raw for timeoutlen (~350 ms)
  esc(0.5);                  expect("normal")
  keys(" ff", 1.2);          expect("insert")               -- picker input is insert mode
  keys("readme", 0.8);       expect("insert")               -- r is "replace" on the vim layer
  esc(0.4); esc(0.6);        expect("normal")
  keys(" e", 1.2);           expect("raw")                  -- explorer
  keys("j", 0.4); keys("j", 0.4); keys("k", 0.5); expect("raw")   -- j/k move in the tree, not in vim
  keys(" e", 1.0);           expect("normal")
  keys(":terminal\n", 1.2); keys("i", 0.6); expect("raw")   -- terminal job mode
  keys("go test ./...\n", 2.5); expect("raw")                -- typed on the default layer
  key({ "ctrl" }, "\\", 0.2); key({ "ctrl" }, "n", 0.6); expect("normal")   -- <C-\><C-n>
  keys(":bd!\n", 0.8)
  keys(" l", 1.2);           expect("raw")                  -- Lazy
  keys("q", 0.8);            expect("normal")
  keys(":qa!\n", 2.0)                                      -- let nvim exit and the shell redraw
  keys("exit\n", 0.8)
end

-- Beat 5: VS Code with vscode-neovim on the Go workspace.
S["5"] = function()
  log("--- segment 5: VS Code")
  openAndPlace("Code", "open -a 'Visual Studio Code' '" .. ROOT .. "demo-go.code-workspace'", 5)
  focus("com.microsoft.VSCode", 1)
  key({ "cmd" }, "p", 0.8); keys("modes.go", 0.6); key({}, "return", 2.0)
  esc(0.8);                  expect("normal", "client")
  vimTour()
  keys("/Kana\n", 1.0);     expect("normal")
  keys("A", 0.5);            expect("insert")              -- append at the end of that line
  keys(" // bit 1 of the code", 0.4)
  esc(0.5);                  expect("normal")
  keys("v", 0.5);            expect("visual")
  esc(0.5);                  expect("normal")
  key({ "ctrl" }, "`", 1.5); expect("raw", "tool window")   -- terminal
  keys("go run ./cmd/vimmode\n", 2.5)                        -- typed on the default layer
  expect("raw", "tool window")
  key({ "ctrl" }, "`", 1.2); expect("normal")
  key({}, "F1", 1.2);        expect("raw")                  -- command palette (companion)
  keys("keyboard", 0.9);     expect("raw")                  -- k is "up" on the vim layer
  esc(0.8);                  expect("normal")
  key({ "cmd", "shift" }, "e", 1.2); expect("raw", "tool window")   -- Explorer
  key({}, "down", 0.4); key({}, "down", 0.5); expect("raw")   -- arrows only: a letter here creates a file in this setup
  key({ "cmd", "shift" }, "e", 1.2); expect("normal")               -- again: back to the editor
  keys("u", 0.4)             -- undo the comment: the file is clean again
end

-- Beat 6: IntelliJ IDEA with IdeaVim on the Java project.
S["6"] = function()
  log("--- segment 6: IntelliJ IDEA")
  openAndPlace("IntelliJ IDEA", "open -a 'IntelliJ IDEA' '" .. ROOT .. "demo-java'", 10)
  focus("com.jetbrains.intellij", 1)
  key({ "cmd", "shift" }, "o", 1.0); keys("ModeTable", 0.8); key({}, "return", 2.0)
  -- no Esc here: the editor opens in normal mode and IdeaVim rings the bell on Esc in normal
  expect("normal", "intellij")
  vimTour()
  keys("/COMPOSE\n", 1.0)                                   -- jump to the constant
  keys("A", 0.5);            expect("insert")              -- append at the end of that line
  keys(" // Compose is bit 0", 0.4)
  esc(0.5);                  expect("normal")
  keys("v", 0.5);            expect("visual")
  esc(0.5);                  expect("normal")
  -- IdeaVim's ex line is a separate component: the plugin sees focus leave the editor and
  -- reports raw, not cmdline (see editors/intellij/README.md; a plugin follow-up).
  keys(":", 0.6);            expect("raw", "intellij client raw")
  esc(0.6);                  expect("normal")
  -- Project tool window through the MEHS keymap (Meh+B = ActivateProjectToolWindow in
  -- editors/intellij/Mehs-macos.xml); IntelliJ ignored a synthesized ⌘1.
  key({ "ctrl", "alt", "shift" }, "b", 2.0);  expect("raw", "intellij")
  keys("readme", 0.9);       expect("raw")                  -- speed search; r is "replace" on the vim layer
  esc(0.5); esc(1.0);        expect("normal")               -- close the search popup, then back to the editor
  key({ "alt" }, "F12", 1.5); expect("raw")                      -- terminal
  keys("ls\n", 1.5);         expect("raw")                      -- typed on the default layer
  key({ "alt" }, "F12", 1.2); expect("normal")
  keys("u", 0.4)
end

-- Beat 7: Obsidian, the showcase/Demo vault, note Tasks.

S["7"] = function()
  log("--- segment 7: Obsidian (vault " .. VAULT .. ")")
  openAndPlace("Obsidian", "open 'obsidian://open?vault=" .. VAULT .. "&file=Tasks'", 3)
  focus("md.obsidian", 1)
  esc(0.8);                  expect("normal", "obsidian")
  vimTour()
  keys("jj", 0.3); keys("A", 0.5); expect("insert")
  keys(" (rehearsal)", 0.4)
  esc(0.5);                  expect("normal")
  keys("u", 0.4)                                        -- the note is unchanged again
  keys(":", 0.6);            expect("cmdline")
  esc(0.6);                  expect("normal")
  key({ "cmd" }, "e", 1.2);  expect("raw")            -- reading view
  key({ "cmd" }, "e", 1.2);  expect("normal")
  key({ "cmd", "shift" }, "f", 1.2); expect("raw")    -- search pane
  keys("layer", 1.0);        expect("raw")            -- typed on the default layer
  esc(0.4)
  click(0.6, 0.22, 1.0);     expect("normal")         -- back into the note: the first task line
end

-- Beat 8: a non-editor app, then the manual override.
S["8"] = function()
  log("--- segment 8: everywhere else")
  -- a terminal without vim is the everyday non-editor case, and it is already on screen
  openNewWindow("Ghostty", function()
    local t = hs.task.new("/usr/bin/open", nil, { "-na", "Ghostty", "--args",
      "--config-file=" .. ROOT .. "env/ghostty-demo.conf", "--working-directory=" .. ROOT .. "demo-go" })
    t:setEnvironment({ ZDOTDIR = ROOT .. "env", HOME = HOME, PATH = os.getenv("PATH") or "/usr/bin:/bin" })
    t:start()
  end, 6)
  expect("off", "terminal without nvim client")
  shell(BINARY .. " set raw", 0.5);         expect("raw")
  shell(BINARY .. " set raw", 0.5)          -- back to auto
  add("check no override", function()
    local st = status()
    log((st.override == nil and "PASS" or "FAIL") .. "  override cleared")
    results[st.override == nil and "pass" or "fail"] = results[st.override == nil and "pass" or "fail"] + 1
  end)
  expect("off")
  keys("exit\n", 0.8)
end

-- ---------- sequencer ----------

local function runNext()
  i = i + 1
  local s = steps[i]
  if not s then
    log(string.format("DONE  pass=%d fail=%d", results.pass, results.fail))
    if logf then logf:close(); logf = nil end
    return
  end
  if M.verbose and not s.desc:match("^expect") then log("  · " .. s.desc) end
  local ok, err = pcall(s.fn)
  if not ok then log("ERROR " .. s.desc .. ": " .. tostring(err)) end
  after(s.wait or 0.6, runNext)
end

function M.run(which, verbose)
  M.verbose = verbose and true or false
  steps, i, results = {}, 0, { pass = 0, fail = 0 }
  logf = io.open(REPORT, "w")
  local w = hs.window.focusedWindow()
  origin = w and w:id() or nil
  log("rehearsal " .. tostring(which) .. " — hands off the keyboard (never typing into window #" .. tostring(origin) .. ")")
  local order = (which == nil or which == "all") and { "3", "4", "5", "6", "7", "8" } or { tostring(which) }
  for _, k in ipairs(order) do
    if S[k] then pause(1.5); S[k]() else log("unknown segment " .. k) end
  end
  runNext()
  return M
end

function M.stop()
  steps, i = {}, 0
  for t in pairs(timers) do t:stop() end
  timers = {}
  log("stopped by user")
  if logf then logf:close(); logf = nil end
end

return M
