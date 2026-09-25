--- === ZmkVimMode ===
---
--- A menu bar item showing the vim mode your ZMK keyboard is in: NORMAL,
--- INSERT, VISUAL, CMDLINE, VIM or RAW behind a Neovim glyph, and nothing at
--- all while vim mode is off.
---
--- The zmk-vim-mode daemon decides the mode from the focused window and from
--- the editor plugins reporting in, then encodes it into the keyboard's HID LED
--- report, so the menu bar shows the decision the keyboard just acted on rather
--- than a second guess at it. `zmk-vim-mode status --bar` words that decision
--- for bars (the Omarchy widget renders the same answer); this Spoon only draws
--- it.
---
--- `zmk-vim-mode install --hammerspoon`, part of `make install`, installs the
--- Spoon and adds the line that starts it to init.lua:
---
---     hs.loadSpoon("ZmkVimMode"):start()
---
--- Download: https://github.com/rafaelromao/zmk-vim-mode

local obj = {}
obj.__index = obj

-- Metadata
obj.name = "ZmkVimMode"
obj.version = "1.0"
obj.author = "Rafael Romão"
obj.homepage = "https://github.com/rafaelromao/zmk-vim-mode"
obj.license = "MIT - https://opensource.org/licenses/MIT"

--- ZmkVimMode.interval
--- Variable
--- Seconds between two looks at the daemon. Defaults to 1.
obj.interval = 1

--- ZmkVimMode.binary
--- Variable
--- The zmk-vim-mode binary. The installer writes the path of the binary that
--- installed the Spoon here; anything that is not an absolute path falls back
--- to where make install and Homebrew put it.
obj.binary = "__ZMK_VIM_MODE_BIN__"

--- ZmkVimMode.socket
--- Variable
--- The daemon's socket, or nil for its default. Hammerspoon does not see your
--- shell's environment, so a ZMK_VIM_MODE_SOCKET set there has to be repeated
--- here.
obj.socket = nil

--- ZmkVimMode.iconFont
--- Variable
--- The Nerd Font family the glyph is drawn in, or nil (the default) for the
--- first installed family with "Nerd Font" in its name. The menu bar's own font
--- has nothing in the Nerd Font private-use area, so the glyph carries its own
--- font or it renders as a box; with no Nerd Font at all, the item shows the
--- label alone.
obj.iconFont = nil

-- nf-custom-neovim, U+E6AE, written as raw UTF-8 so the file stays Lua 5.1
-- readable (the tests run it under Neovim's LuaJIT).
local ICON = "\238\154\174"

-- Where make install and Homebrew put the binary. Hammerspoon does not inherit
-- the login shell's PATH, so a bare name would not resolve.
local function candidates()
  local home = os.getenv("HOME") or ""
  return {
    home .. "/.local/bin/zmk-vim-mode",
    "/opt/homebrew/bin/zmk-vim-mode",
    "/usr/local/bin/zmk-vim-mode",
  }
end

local function isFile(path)
  return hs.fs.attributes(path, "mode") == "file"
end

-- The family to draw the glyph in: the one asked for when it is installed,
-- else the first Nerd Font installed (every one carries the glyph), else nil.
local function pickIconFont(wanted)
  local ok, families = pcall(hs.styledtext.fontFamilies)
  if not ok or type(families) ~= "table" then return nil end
  local nerd = {}
  for _, family in ipairs(families) do
    if family == wanted then return family end
    if family:find("Nerd Font", 1, true) then nerd[#nerd + 1] = family end
  end
  table.sort(nerd)
  return nerd[1]
end

function obj:_log()
  self.logger = self.logger or hs.logger.new("ZmkVimMode")
  return self.logger
end

--- ZmkVimMode:_resolveBinary() -> string or nil
--- Method
--- The binary to run: the configured one when it is an absolute path to a
--- file, else the first candidate that exists.
function obj:_resolveBinary()
  local configured = self.binary
  if type(configured) == "string" and configured:sub(1, 1) == "/" and isFile(configured) then
    return configured
  end
  for _, path in ipairs(candidates()) do
    if isFile(path) then return path end
  end
  return nil
end

--- ZmkVimMode:_view(exitCode, stdout, stderr) -> table or nil
--- Method
--- What one look at the daemon means for the menu bar: nil to hide the item,
--- or a table with `label` and `tooltip`. `status --bar` answers even when the
--- daemon is down, so a failure here is the binary itself (missing, or too old
--- to know --bar).
function obj:_view(exitCode, stdout, stderr)
  if exitCode ~= 0 then
    local why = (stderr or ""):gsub("^%s+", ""):gsub("%s+$", "")
    if why == "" then why = "status failed" end
    return { label = "VIM ?", tooltip = "zmk-vim-mode unavailable: " .. why }
  end
  local ok, view = pcall(hs.json.decode, stdout or "")
  if not ok or type(view) ~= "table" or type(view.text) ~= "string" then
    return { label = "VIM ?", tooltip = "zmk-vim-mode returned an unreadable status" }
  end
  if view.text == "" then return nil end
  return { label = view.text, tooltip = type(view.tooltip) == "string" and view.tooltip or "" }
end

-- Icon and label need different fonts, so the title is styled text rather
-- than a plain string. The label keeps the menu bar's own font and size so it
-- sits with the rest of the bar.
function obj:_title(label)
  if not self._iconFont then return label end
  local font = self._iconFont
  local ok, styled = pcall(function()
    local base = hs.styledtext.defaultFonts.menuBar
    return hs.styledtext.new(ICON .. " ", { font = { name = font, size = base.size } })
      .. hs.styledtext.new(label, { font = base })
  end)
  if ok and styled then return styled end
  return ICON .. " " .. label
end

function obj:_show(view)
  if not self._menu then return end
  if not self._visible then
    self._menu:returnToMenuBar()
    self._visible = true
  end
  self._menu:setTitle(self:_title(view.label))
  self._menu:setTooltip(view.tooltip)
end

function obj:_hide()
  if self._menu and self._visible then
    self._menu:removeFromMenuBar()
    self._visible = false
  end
end

function obj:_render(view)
  if view then self:_show(view) else self:_hide() end
end

-- hs.task rather than hs.execute: hs.execute blocks Hammerspoon's main
-- thread, and once a second that is a stutter you can feel.
function obj:_poll()
  -- Skip rather than queue: a slow answer must not build up a backlog of one
  -- task per tick.
  if self._task and self._task:isRunning() then return end
  local generation = self._generation
  local args = { "status", "--bar" }
  if self.socket then
    args[#args + 1] = "--socket"
    args[#args + 1] = self.socket
  end
  local task = hs.task.new(self._binary, function(exitCode, stdout, stderr)
    -- A look that was under way when stop() ran must not bring the item back.
    if generation ~= self._generation then return end
    self:_render(self:_view(exitCode, stdout, stderr))
  end, args)
  if not task or not task:start() then
    -- Nothing to show when the binary cannot even be started (uninstalled
    -- since start(), most likely): hide, and say so once rather than every
    -- tick.
    self._task = nil
    if not self._startFailed then
      self:_log().e("cannot run " .. tostring(self._binary))
      self._startFailed = true
    end
    self:_hide()
    return
  end
  self._startFailed = false
  self._task = task
end

--- ZmkVimMode:init() -> self
--- Method
--- Called by hs.loadSpoon. Does nothing until :start().
function obj:init()
  self._generation = 0
  self._visible = false
  return self
end

--- ZmkVimMode:start() -> self
--- Method
--- Starts watching the daemon; the item appears whenever vim mode is on. When
--- the binary is not installed the item stays out of the menu bar, and the
--- console says why.
function obj:start()
  self:stop()
  self._binary = self:_resolveBinary()
  if not self._binary then
    self:_log().w("zmk-vim-mode is not installed (run make install in the zmk-vim-mode repo, then reload)")
    return self
  end
  self._iconFont = pickIconFont(self.iconFont)
  -- The autosave name lets macOS keep the item where you dragged it.
  self._menu = self._menu or hs.menubar.new(false, "ZmkVimMode")
  self._timer = hs.timer.doEvery(self.interval, function() self:_poll() end)
  self:_poll()
  return self
end

--- ZmkVimMode:stop() -> self
--- Method
--- Stops watching the daemon and removes the item from the menu bar.
function obj:stop()
  self._generation = (self._generation or 0) + 1
  if self._timer then
    self._timer:stop()
    self._timer = nil
  end
  if self._task and self._task:isRunning() then self._task:terminate() end
  self._task = nil
  self:_hide()
  return self
end

return obj
