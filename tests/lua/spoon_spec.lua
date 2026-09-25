-- The menu bar Spoon's decisions, run under Neovim with a stand-in for the few
-- hs.* functions they touch. Drawing the item is Hammerspoon's business.

local saved_hs = _G.hs
local files = {}
_G.hs = {
  json = {
    -- hs.json.decode turns JSON null into nil, which luanil mimics.
    decode = function(s)
      return vim.json.decode(s, { luanil = { object = true, array = true } })
    end,
  },
  fs = {
    attributes = function(path, attr)
      if attr == "mode" and files[path] then return "file" end
      return nil
    end,
  },
}
package.loaded.ZmkVimMode = nil
local spoon = require("ZmkVimMode")

T.section("ZmkVimMode:_view")
T.eq(spoon:_view(0, '{"text":"","tooltip":"Vim mode off","class":"off"}\n', ""), nil, "vim mode off hides the item")

local v = spoon:_view(0, '{"text":"NORMAL","tooltip":"Vim mode: normal (code 1)","class":"normal"}\n', "")
T.eq(v and v.label, "NORMAL", "the label is the bar text")
T.eq(v and v.tooltip, "Vim mode: normal (code 1)", "the tooltip passes through")

v = spoon:_view(0, '{"text":"VIM ?","tooltip":"zmk-vim-mode unavailable: daemon not reachable","class":"error"}\n', "")
T.eq(v and v.label, "VIM ?", "the daemon being down is shown as status --bar words it")

v = spoon:_view(2, "", "flag provided but not defined: -bar\n")
T.eq(v and v.label, "VIM ?", "a binary that fails shows VIM ?")
T.eq(v and v.tooltip, "zmk-vim-mode unavailable: flag provided but not defined: -bar", "its stderr explains why, trimmed")

v = spoon:_view(1, "", "  \n")
T.eq(v and v.tooltip, "zmk-vim-mode unavailable: status failed", "a blank stderr still says something")

v = spoon:_view(0, "not json", "")
T.eq(v and v.label, "VIM ?", "unreadable output shows VIM ?")

v = spoon:_view(0, '{"tooltip":"no text"}', "")
T.eq(v and v.label, "VIM ?", "output without text is unreadable")

v = spoon:_view(0, '{"text":"INSERT","tooltip":null}', "")
T.eq(v and v.tooltip, "", "a missing tooltip is empty, not nil")

T.section("ZmkVimMode:_resolveBinary")
local home = os.getenv("HOME") or ""
T.eq(spoon.binary:sub(1, 1) ~= "/", true, "the checked-in Spoon carries the placeholder, not a path")

files = { [home .. "/.local/bin/zmk-vim-mode"] = true, ["/opt/homebrew/bin/zmk-vim-mode"] = true }
T.eq(spoon:_resolveBinary(), home .. "/.local/bin/zmk-vim-mode", "an unfilled placeholder falls back to where make install puts it")

local installed = setmetatable({ binary = "/opt/zmk/bin/zmk-vim-mode" }, spoon)
files["/opt/zmk/bin/zmk-vim-mode"] = true
T.eq(installed:_resolveBinary(), "/opt/zmk/bin/zmk-vim-mode", "the path the installer wrote wins")

files["/opt/zmk/bin/zmk-vim-mode"] = nil
T.eq(installed:_resolveBinary(), home .. "/.local/bin/zmk-vim-mode", "a written path that is gone falls back too")

files = {}
T.eq(spoon:_resolveBinary(), nil, "no binary anywhere: nothing to run")

T.section("ZmkVimMode lifecycle")
-- Stand-ins that record what the Spoon asks of Hammerspoon.
local ICON = "\238\154\174"
local menu = { inBar = true }
function menu:returnToMenuBar() self.inBar = true; return self end
function menu:removeFromMenuBar() self.inBar = false; return self end
function menu:setTitle(t) self.title = t; return self end
function menu:setTooltip(t) self.tooltip = t; return self end
hs.menubar = {
  new = function(inBar, autosaveName)
    menu.inBar, menu.autosaveName = inBar, autosaveName
    return menu
  end,
}
local timers, tasks = {}, {}
hs.timer = {
  doEvery = function(interval, fn)
    local t = { interval = interval, fn = fn }
    function t:stop() self.stopped = true end
    timers[#timers + 1] = t
    return t
  end,
}
hs.task = {
  new = function(path, callback, args)
    local t = { path = path, callback = callback, args = args }
    function t:start() self.running = true; return self end
    function t:isRunning() return self.running == true end
    function t:terminate() self.running = false; return self end
    function t:finish(code, out, err) self.running = false; self.callback(code, out, err) end
    tasks[#tasks + 1] = t
    return t
  end,
}
local fonts = { "Hack Nerd Font" }
local styled = { __concat = function(a, b) return setmetatable({ text = a.text .. b.text }, getmetatable(a)) end }
hs.styledtext = {
  fontFamilies = function() return fonts end,
  defaultFonts = { menuBar = { name = ".AppleSystemUIFont", size = 13 } },
  new = function(text) return setmetatable({ text = text }, styled) end,
}
hs.logger = { new = function() return { w = function() end, e = function() end } end }

files = { ["/opt/zmk/bin/zmk-vim-mode"] = true }
local s = setmetatable({ binary = "/opt/zmk/bin/zmk-vim-mode" }, spoon):init()
s:start()
T.eq(menu.inBar, false, "the item starts out of the menu bar")
T.eq(menu.autosaveName, "ZmkVimMode", "it keeps its place in the menu bar across reloads")
T.eq(timers[1] and timers[1].interval, 1, "it looks once a second")
T.eq(#tasks, 1, "start looks at once")
T.eq(tasks[1].path, "/opt/zmk/bin/zmk-vim-mode", "it runs the binary it resolved")
T.eq(table.concat(tasks[1].args, " "), "status --bar", "and asks status --bar")

s:_poll()
T.eq(#tasks, 1, "a look still out is not doubled")

tasks[1]:finish(0, '{"text":"INSERT","tooltip":"Vim mode: insert (code 2)"}\n', "")
T.eq(menu.inBar, true, "a mode shows the item")
T.eq(menu.title and menu.title.text, ICON .. " INSERT", "glyph and label")
T.eq(menu.tooltip, "Vim mode: insert (code 2)", "with the tooltip")

timers[1].fn()
tasks[2]:finish(0, '{"text":"","tooltip":"Vim mode off"}\n', "")
T.eq(menu.inBar, false, "vim mode off hides it")

timers[1].fn()
s:stop()
T.eq(timers[1].stopped, true, "stop stops the timer")
T.eq(tasks[3].running, false, "and ends the look under way")
tasks[3]:finish(15, '{"text":"NORMAL"}\n', "")
T.eq(menu.inBar, false, "a look that lands after stop does not bring the item back")

fonts = {}
s.socket = "/tmp/zmk.sock"
s:start()
tasks[#tasks]:finish(0, '{"text":"NORMAL"}\n', "")
T.eq(menu.title, "NORMAL", "without the Nerd Font the label stands alone")
T.eq(table.concat(tasks[#tasks].args, " "), "status --bar --socket /tmp/zmk.sock", "a socket setting is passed on")
s:stop()

files = {}
local before = #tasks
local none = setmetatable({}, spoon):init()
none:start()
T.eq(#tasks, before, "without a binary it never runs anything")
T.eq(none._timer, nil, "and keeps no timer")

T.section("ZmkVimMode icon font")
files = { ["/opt/zmk/bin/zmk-vim-mode"] = true }
local f = setmetatable({ binary = "/opt/zmk/bin/zmk-vim-mode" }, spoon):init()
fonts = { "Menlo", "JetBrainsMono Nerd Font", "FiraCode Nerd Font Mono" }
f:start()
T.eq(f._iconFont, "FiraCode Nerd Font Mono", "no iconFont: the first Nerd Font installed")
f.iconFont = "JetBrainsMono Nerd Font"
f:start()
T.eq(f._iconFont, "JetBrainsMono Nerd Font", "iconFont names the family")
f.iconFont = "Hack Nerd Font"
f:start()
T.eq(f._iconFont, "FiraCode Nerd Font Mono", "an iconFont that is not installed falls back to one that is")
fonts = { "Menlo" }
f:start()
T.eq(f._iconFont, nil, "no Nerd Font at all: the label alone")
f:stop()

_G.hs = saved_hs
package.loaded.ZmkVimMode = nil
