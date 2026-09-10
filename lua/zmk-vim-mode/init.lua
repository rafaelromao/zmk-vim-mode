-- zmk-vim-mode: report Neovim's effective vim state to the zmk-vim-mode daemon,
-- which drives the ZMK keyboard's layers.
--
-- Transport is a persistent unix socket via vim.uv; never a process per event.
local modes = require("zmk-vim-mode.modes")
local context = require("zmk-vim-mode.context")
local vscode = require("zmk-vim-mode.vscode")

local M = {}

M.version = "0.1.0"

M.defaults = {
  -- Socket path. XDG is deliberately ignored so systemd/launchd/tmux agree.
  socket = nil,
  -- State reported for terminal-job mode ("raw" keeps Esc going to the job).
  terminal_state = "raw",
  -- Report "raw" while a <leader> sequence is pending (which-key menus).
  leader_raw = true,
  raw_filetypes = nil, -- nil = context.default_raw_filetypes
  raw_exceptions = nil, -- nil = context.default_raw_exceptions
  -- classify(buf, win, mode) -> state|nil overrides everything.
  classify = nil,
  -- Inside vscode-neovim: VSCode commands (Lua patterns) that move keyboard
  -- focus into a quick input or widget the title cannot see. nil = the
  -- defaults in zmk-vim-mode.vscode; {} disables the hook.
  vscode_raw_actions = nil,
  vscode_raw_ttl_ms = 20000, -- fallback when the close is never observed
  vscode_raw_grace_ms = 300, -- keys within this window are the mapping's own tail
  debug = false,
  -- Reconnect backoff bounds (ms).
  reconnect_min = 250,
  reconnect_max = 2000,
}

local S = {
  opts = vim.deepcopy(M.defaults),
  pipe = nil,
  connected = false,
  connecting = false,
  backoff = 250,
  last_sent = nil,
  focused = nil, -- nil = unknown; terminals send no initial focus report
  leader_pending = false,
  leader_timer = nil,
  vscode_raw = nil, -- VSCode command that took the keys, nil when none
  vscode_raw_at = 0,
  vscode_raw_timer = nil,
  companion = nil, -- nil = not asked yet
  warned_tmux = false,
  stopped = false,
}

local function log(...)
  if S.opts.debug then
    local parts = {}
    for _, v in ipairs({ ... }) do
      parts[#parts + 1] = type(v) == "string" and v or vim.inspect(v)
    end
    vim.notify("[zmk-vim-mode] " .. table.concat(parts, " "), vim.log.levels.DEBUG)
  end
end

local function socket_path()
  if S.opts.socket and S.opts.socket ~= "" then
    return vim.fn.expand(S.opts.socket)
  end
  local env = vim.env.ZMK_VIM_MODE_SOCKET
  if env and env ~= "" then
    return env
  end
  local home = vim.env.HOME or vim.fn.expand("~")
  return home .. "/.local/state/zmk-vim-mode/daemon.sock"
end

---Current effective state: leader-pending → raw; raw context → raw; else the
---coarse mode map. Returns nil when the state should not change.
---@return string|nil
function M.effective_state()
  local mode = vim.api.nvim_get_mode().mode -- api-fast

  if S.opts.classify then
    local ok, custom = pcall(S.opts.classify, vim.api.nvim_get_current_buf(), vim.api.nvim_get_current_win(), mode)
    if ok and custom then
      return custom
    end
  end

  -- A pending <leader> means a menu/prompt is about to take the keys.
  if S.opts.leader_raw and S.leader_pending and (mode:sub(1, 1) == "n" or mode:sub(1, 1) == "v") then
    return "raw"
  end
  -- A VSCode quick input opened from a mapping has the keys until further notice.
  if S.vscode_raw then
    return "raw"
  end

  local coarse = modes.coarse(mode, S.opts)
  if coarse == nil then
    return nil
  end
  -- Insert-like prompts (pickers, vim.ui.input) stay insert: typing works and
  -- Esc → normal is the right behaviour there.
  if coarse == "normal" or coarse == "visual" then
    local raw, reason = context.is_raw(S.opts)
    if raw then
      log("raw context:", reason)
      return "raw"
    end
  end
  return coarse
end

local function send(tbl)
  if not S.connected or not S.pipe then
    return
  end
  local ok, encoded = pcall(vim.json.encode, tbl)
  if not ok then
    return
  end
  S.pipe:write(encoded .. "\n", function(err)
    if err then
      vim.schedule(M.reconnect)
    end
  end)
end

local function hello()
  send({
    v = 1,
    t = "hello",
    client = "nvim",
    app = vim.g.vscode ~= nil and "vscode" or nil,
    pid = vim.fn.getpid(),
    mode = M.effective_state() or "normal",
    focused = S.focused,
    nested = vim.env.NVIM ~= nil,
    tmux = vim.env.TMUX ~= nil,
    plugin = M.version,
  })
end

local function drop()
  S.connected = false
  S.connecting = false
  if S.pipe then
    pcall(function()
      S.pipe:read_stop()
    end)
    pcall(function()
      S.pipe:close()
    end)
    S.pipe = nil
  end
  S.last_sent = nil
end

---Connect (or reconnect) to the daemon. Silent no-op when it is not running.
function M.connect()
  if S.stopped or S.connected or S.connecting then
    return
  end
  S.connecting = true
  local pipe = vim.uv.new_pipe(false)
  if not pipe then
    S.connecting = false
    return
  end
  S.pipe = pipe
  pipe:connect(socket_path(), function(err)
    if err then
      vim.schedule(function()
        drop()
        M.schedule_reconnect()
      end)
      return
    end
    S.connected = true
    S.connecting = false
    S.backoff = S.opts.reconnect_min
    local pending = ""
    pipe:read_start(function(rerr, chunk)
      if rerr or not chunk then
        vim.schedule(function()
          drop()
          M.schedule_reconnect()
        end)
        return
      end
      pending = pending .. chunk
      while true do
        local nl = pending:find("\n", 1, true)
        if not nl then
          break
        end
        local line = pending:sub(1, nl - 1)
        pending = pending:sub(nl + 1)
        local ok, msg = pcall(vim.json.decode, line) -- plain Lua, safe in a uv callback
        if ok and type(msg) == "table" then
          if msg.t == "resync" then
            -- The daemon asks us to re-send our state (e.g. after a restart).
            vim.schedule(function()
              S.last_sent = nil
              hello()
              M.report()
            end)
          elseif msg.t == "editor_focus" and msg.focused == true then
            -- The accessibility bus saw focus return to the text editor: any
            -- quick-input hint we raised is stale.
            vim.schedule(function()
              M.clear_vscode_raw("editor focused (a11y)")
            end)
          end
        end
      end
    end)
    -- hello on every connect, not just VimEnter, so a restarted daemon is seeded.
    vim.schedule(function()
      S.last_sent = nil
      hello()
      M.report()
    end)
  end)
end

---Schedule a reconnect with exponential backoff.
function M.schedule_reconnect()
  if S.stopped then
    return
  end
  local delay = S.backoff
  S.backoff = math.min(S.backoff * 2, S.opts.reconnect_max)
  vim.defer_fn(M.connect, delay)
end

---Force an immediate reconnect.
function M.reconnect()
  drop()
  S.backoff = S.opts.reconnect_min
  M.connect()
end

---Compute the effective state and send it if it changed.
function M.report()
  local st = M.effective_state()
  if st == nil or st == S.last_sent then
    return
  end
  S.last_sent = st
  send({ t = "mode", mode = st })
end

local function report_focus(focused)
  S.focused = focused
  local st = M.effective_state() or S.last_sent
  send({ t = "focus", focused = focused, mode = st })
end

-- <leader> tracking: vim.on_key sees the typed key before the mapping resolves.
local function setup_leader_tracking()
  if not S.opts.leader_raw then
    return
  end
  local ns = vim.api.nvim_create_namespace("zmk_vim_mode_leader")
  vim.on_key(function(_, typed)
    if typed == nil or typed == "" or S.leader_pending then
      return
    end
    local leader = vim.g.mapleader
    local localleader = vim.g.maplocalleader
    if typed ~= leader and typed ~= localleader then
      return
    end
    local m = vim.api.nvim_get_mode().mode:sub(1, 1) -- api-fast
    if m ~= "n" and m ~= "v" and m ~= "V" and m ~= "\22" then
      return
    end
    S.leader_pending = true
    vim.schedule(M.report)
    -- Safety net: SafeState does not fire while a mapping is pending, but if
    -- the sequence is abandoned we must not stay raw forever.
    if S.leader_timer then
      S.leader_timer:stop()
    end
    S.leader_timer = vim.defer_fn(function()
      if S.leader_pending then
        S.leader_pending = false
        M.report()
      end
    end, (vim.o.timeoutlen or 1000) + 50)
  end, ns)
end

local function clear_leader()
  if S.leader_pending then
    S.leader_pending = false
    if S.leader_timer then
      S.leader_timer:stop()
      S.leader_timer = nil
    end
    M.report()
  end
end

---A VSCode command is about to take keyboard focus away from the editor.
---@param name string
function M.raise_vscode_raw(name)
  S.vscode_raw = name
  S.vscode_raw_at = vim.uv.now()
  if S.vscode_raw_timer then
    S.vscode_raw_timer:stop()
  end
  S.vscode_raw_timer = vim.defer_fn(function()
    if S.vscode_raw == name then
      M.clear_vscode_raw("ttl")
    end
  end, S.opts.vscode_raw_ttl_ms)
  log("vscode raw:", name)
  M.report()
  -- The companion, when present, adds Escape/cursor/active-editor detection
  -- and calls clear_vscode_raw back through vscode-neovim.lua.
  if S.companion == nil then
    S.companion = vscode.companion_available(500)
    log("vscode companion:", S.companion)
  end
  if S.companion then
    vscode.forward(name)
  end
end

---Keys reach Neovim again (or the companion saw the quick input close).
---@param why string
function M.clear_vscode_raw(why)
  if not S.vscode_raw then
    return
  end
  log("vscode raw cleared:", why)
  S.vscode_raw = nil
  if S.vscode_raw_timer then
    S.vscode_raw_timer:stop()
    S.vscode_raw_timer = nil
  end
  M.report()
end

local function setup_vscode_hook()
  local patterns = S.opts.vscode_raw_actions or vscode.default_raw_actions
  if vim.g.vscode == nil or #patterns == 0 then
    return
  end
  if not vscode.install(patterns, M.raise_vscode_raw) then
    return
  end
  -- A typed key reaching Neovim proves the editor has focus again -- except in
  -- the first instants, when the mapping's own remaining keys still arrive.
  local ns = vim.api.nvim_create_namespace("zmk_vim_mode_vscode")
  vim.on_key(function(_, typed)
    if typed == nil or typed == "" or not S.vscode_raw then
      return
    end
    if vim.uv.now() - S.vscode_raw_at < S.opts.vscode_raw_grace_ms then
      return
    end
    vim.schedule(function()
      M.clear_vscode_raw("key reached neovim")
    end)
  end, ns)
end

local function warn_tmux()
  if S.warned_tmux or vim.env.TMUX == nil then
    return
  end
  S.warned_tmux = true
  vim.system({ "tmux", "show", "-gv", "focus-events" }, { text = true }, function(res)
    if res.code == 0 and vim.trim(res.stdout or "") ~= "on" then
      vim.schedule(function()
        vim.notify(
          "[zmk-vim-mode] tmux focus-events is off; Neovim will not see FocusLost.\n"
            .. "Add to ~/.tmux.conf:  set -g focus-events on",
          vim.log.levels.WARN
        )
      end)
    end
  end)
end

---Set up autocmds and connect. Called by lazy.nvim via `opts = {}`.
---@param opts table|nil
function M.setup(opts)
  S.opts = vim.tbl_deep_extend("force", vim.deepcopy(M.defaults), opts or {})
  S.stopped = false
  S.backoff = S.opts.reconnect_min

  -- Headless (no UI): nothing to report until a UI attaches.
  if #vim.api.nvim_list_uis() == 0 then
    vim.api.nvim_create_autocmd("UIEnter", {
      once = true,
      callback = function()
        M.setup(opts)
      end,
    })
    return
  end

  local group = vim.api.nvim_create_augroup("zmk_vim_mode", { clear = true })
  local au = function(events, cb)
    vim.api.nvim_create_autocmd(events, { group = group, callback = cb })
  end

  au("ModeChanged", function()
    clear_leader()
    M.report()
  end)
  au({ "BufEnter", "WinEnter", "FileType", "TermEnter", "TermLeave", "CmdlineLeave" }, function()
    M.report()
  end)
  au("SafeState", function()
    clear_leader()
    M.report()
  end)
  au({ "FocusGained", "VimResume" }, function()
    report_focus(true)
  end)
  au({ "FocusLost", "VimSuspend" }, function()
    report_focus(false)
  end)
  au("VimLeavePre", function()
    send({ t = "bye" })
    S.stopped = true
    drop()
  end)

  setup_leader_tracking()
  setup_vscode_hook()
  warn_tmux()
  M.connect()
end

---Human-readable status (`:ZmkVimMode status`).
function M.status()
  return {
    socket = socket_path(),
    connected = S.connected,
    last_sent = S.last_sent,
    effective = M.effective_state(),
    focused = S.focused,
    leader_pending = S.leader_pending,
    vscode_raw = S.vscode_raw,
    vscode_companion = S.companion,
    nested = vim.env.NVIM ~= nil,
    tmux = vim.env.TMUX ~= nil,
  }
end

-- Exposed for tests.
M._state = S

return M
