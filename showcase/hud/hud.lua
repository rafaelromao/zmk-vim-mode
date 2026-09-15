-- ZMK layer HUD for the zmk-vim-mode showcase video (macOS / Hammerspoon host).
--
-- Two always-on-top, borderless webviews on the recording display:
--   * index.html  (top-right)    Rafael Romão's Diamond keymap, rendered from the keyboards
--                                repo's keymap-drawer YAML via keymap/build.py; keys and combos
--                                light as they are typed; the banner names the active layer.
--   * keys.html   (bottom-left)  the typed-keys strip.
--
-- Load without touching ~/.hammerspoon (needs `require("hs.ipc")` there once):
--   hs -c "zmkhud = dofile('/ABS/PATH/showcase/hud/hud.lua')"
--   hs -c "zmkhud.stop()"            -- or the ✕ button on the panel
--
-- Feeds:
--   * keys      hs.eventtap on keyDown/keyUp/flagsChanged, forwarded to both pages. The host
--               sees keycodes, not key positions, so the page emulates layers (see hud.js).
--   * vim mode  the daemon log (~/Library/Logs/zmk-vim-mode.log, launchd's stderr): one INFO
--               line `msg=decision mode=… code=… reason="…"` per transition. Its size is
--               polled every 200 ms (FSEvents are unreliable for a file launchd keeps open).
--               Fallback when the log is missing: `zmk-vim-mode status --json` polled.
--
-- The Linux host is showcase/linux/ (the same pages, driven over a WebSocket).

local M = {}

local HERE = debug.getinfo(1, "S").source:sub(2):match("(.*/)") or "./"
local HOME = os.getenv("HOME")
local LOG = HOME .. "/Library/Logs/zmk-vim-mode.log"
local BINARY = HOME .. "/.local/bin/zmk-vim-mode"

-- panel: 2 hands × (4×62 + 3×6) + 34 gap + 2×14 padding + 2 px border = 598
M.width, M.height = 598, 392
M.margin = 24
M.screen = nil -- hs.screen; nil = the external display, else the primary
M.lastKeys = {}

local wv, kv, tap, watcher, pollTimer, logTimer, readyTimer, task, ucc
local ready = false
local queue = {}
local lastSize = -1

-- A JavaScript string literal. hs.json.encode only takes tables, so strings need their own
-- quoting (the feed's mode and reason, the HUD's manual mode).
local function jsstr(v)
  v = tostring(v or "")
  return '"' .. v:gsub("\\", "\\\\"):gsub('"', '\\"'):gsub("\n", "\\n"):gsub("\r", "") .. '"'
end

local function js(code)
  if not wv then return end
  if not ready then queue[#queue + 1] = code; return end
  wv:evaluateJavaScript(code)
end

local function flush()
  ready = true
  for _, code in ipairs(queue) do wv:evaluateJavaScript(code) end
  queue = {}
end

-- ---------- daemon feed ----------

local lastLine

local function applyDecision(line)
  if line == lastLine then return end
  lastLine = line
  local mode = line:match("mode=(%S+)")
  local code = line:match("code=(%d)")
  local reason = line:match('reason="([^"]*)"') or line:match("reason=(%S+)") or ""
  if code then
    js(string.format("hud.setMode(%s, %s, %s)", code, jsstr(mode), jsstr(reason)))
  end
end

local function readTail()
  local f = io.open(LOG, "rb")
  if not f then return end
  local size = f:seek("end")
  f:seek("set", math.max(0, size - 16384))
  local chunk = f:read("*a") or ""
  f:close()
  local last
  for line in chunk:gmatch("[^\n]+") do
    if line:find("msg=decision", 1, true) then last = line end
  end
  if last then applyDecision(last) end
end

local function pollStatus()
  if task and task:isRunning() then return end
  task = hs.task.new(BINARY, function(exit, out)
    if exit ~= 0 then return end
    local ok, st = pcall(hs.json.decode, out or "")
    if ok and type(st) == "table" and st.code then
      js(string.format("hud.setMode(%d, %s, %s)", st.code, jsstr(st.mode), jsstr(st.reason)))
    end
  end, { "status", "--json" })
  task:start()
end

-- ---------- key feed ----------

local types = hs.eventtap.event.types

-- Always a JSON object with the four booleans: an empty Lua table would encode as [] and
-- JavaScript arrays have a method called "shift".
local function flagsOf(ev)
  local f = ev:getFlags()
  return { cmd = f.cmd == true, ctrl = f.ctrl == true, alt = f.alt == true, shift = f.shift == true, fn = f.fn == true }
end

local function onKey(ev)
  local t = ev:getType()
  local name = hs.keycodes.map[ev:getKeyCode()]
  local chars = ""
  if t ~= types.flagsChanged then
    local ok, c = pcall(function() return ev:getCharacters(true) end)
    if ok and c then chars = c end
  end
  local rep = ev:getProperty(hs.eventtap.event.properties.keyboardEventAutorepeat) or 0
  local payload = hs.json.encode({
    type = (t == types.keyDown and "keyDown") or (t == types.keyUp and "keyUp") or "flagsChanged",
    name = name, chars = chars, flags = flagsOf(ev), ["repeat"] = rep ~= 0,
    code = ev:getKeyCode(),
  })
  if t == types.keyDown then
    -- zmkhud.lastKeys: the last raw events, newest last, for diagnosis:
    --   hs -c "return table.concat(zmkhud.lastKeys, '\n')"
    M.lastKeys[#M.lastKeys + 1] = os.date("%H:%M:%S ") .. payload
    if #M.lastKeys > 10 then table.remove(M.lastKeys, 1) end
    M.lastKey = payload
  end
  js("hud.key(" .. payload .. ")")
  if kv and t == types.keyDown then kv:evaluateJavaScript("keys.key(" .. payload .. ")") end
  return false -- never swallow the key
end

-- ---------- windows ----------

-- The recording display is the external one when there is one (the laptop keeps OBS and the
-- script); otherwise the screen under the mouse.
local function recordingScreen()
  local primary = hs.screen.primaryScreen()
  for _, s in ipairs(hs.screen.allScreens()) do
    if s ~= primary then return s end
  end
  return hs.mouse.getCurrentScreen() or primary
end

local function frame()
  local f = (M.screen or recordingScreen()):frame()
  -- top-right corner, M.margin from both edges (the typed-keys strip owns the bottom band)
  return hs.geometry.rect(f.x + f.w - M.width - M.margin, f.y + M.margin, M.width, M.height)
end

local function stripFrame()
  local f = (M.screen or recordingScreen()):frame()
  return hs.geometry.rect(f.x + M.margin, f.y + f.h - 72 - M.margin, 900, 72)
end

local function overlay(w)
  w:windowStyle({ "borderless", "nonactivating" })
  w:level(hs.drawing.windowLevels.overlay)
  w:behavior(hs.drawing.windowBehaviors.canJoinAllSpaces + hs.drawing.windowBehaviors.stationary)
  w:transparent(true)
  w:allowTextEntry(false)
  w:shadow(false)
  return w
end

function M.start()
  if wv then return M end
  -- The page's ✕ button posts "close" through this controller.
  ucc = hs.webview.usercontent.new("zmkhud"):setCallback(function(msg)
    if msg and msg.body == "close" then M.stop() end
  end)
  wv = overlay(hs.webview.new(frame(), { developerExtrasEnabled = false }, ucc))
  wv:navigationCallback(function(action)
    if action == "didFinishNavigation" then flush(); readTail() end
  end)
  wv:url("file://" .. HERE .. "index.html")
  wv:show()

  -- Typed-keys visualizer: bottom-left of the recording display, fed by the same event tap.
  kv = overlay(hs.webview.new(stripFrame(), { developerExtrasEnabled = false }))
  kv:url("file://" .. HERE .. "keys.html")
  kv:show()

  tap = hs.eventtap.new({ types.keyDown, types.keyUp, types.flagsChanged }, onKey)
  tap:start()

  -- If WebKit never reports the load, unblock the bridge anyway: keymap.js is inline, so
  -- the page is ready long before two seconds.
  readyTimer = hs.timer.doAfter(2, function() if not ready then flush(); readTail() end end)

  if hs.fs.attributes(LOG) then
    watcher = hs.pathwatcher.new(LOG, readTail):start()
    logTimer = hs.timer.doEvery(0.2, function()
      local size = hs.fs.attributes(LOG, "size") or -1
      if size ~= lastSize then lastSize = size; readTail() end
    end)
  else
    pollTimer = hs.timer.doEvery(0.2, pollStatus)
  end
  return M
end

function M.stop()
  if tap then tap:stop(); tap = nil end
  if watcher then watcher:stop(); watcher = nil end
  if pollTimer then pollTimer:stop(); pollTimer = nil end
  if logTimer then logTimer:stop(); logTimer = nil end
  if readyTimer then readyTimer:stop(); readyTimer = nil end
  if wv then wv:delete(); wv = nil end
  if kv then kv:delete(); kv = nil end
  ready = false
end

-- Move to another screen: zmkhud.moveTo(hs.screen.allScreens()[2])
function M.moveTo(screen)
  M.screen = screen
  if wv then wv:frame(frame()) end
  if kv then kv:frame(stripFrame()) end
end

-- Resize (points), keeping the panel's aspect (598×392): zmkhud.resize(480, 315)
function M.resize(w, h)
  M.width, M.height = w, h
  if wv then
    wv:frame(frame())
    js(string.format("document.documentElement.style.zoom = %.3f", w / 598))
  end
end

-- For rehearsals without the keyboard: zmkhud.mode(2) etc.
function M.mode(code, reason)
  js(string.format("hud.setMode(%d, '', %s)", code, jsstr(reason or "manual")))
end

-- zmkhud.selftest(): what the HUD sees — for `hs -c "print(zmkhud.selftest())"`.
function M.selftest()
  local size = hs.fs.attributes(LOG, "size")
  return string.format("ready=%s tap=%s log=%s size=%s last=%s",
    tostring(ready), tostring(tap and tap:isEnabled()), LOG, tostring(size), tostring(lastLine))
end

-- KeyCastr draws its own stacking bubbles wherever it was last placed; the strip replaces it.
local kc = hs.application.get("KeyCastr")
if kc then kc:kill(); print("zmkhud: quit KeyCastr (the HUD's typed-keys strip replaces it)") end

return M.start()
