local T = _G.T
T.section("Neovim session identity")

local uv = vim.uv
local path = vim.fn.tempname()
local server = uv.new_pipe(false)
assert(server:bind(path))
local hellos, peers = {}, {}
assert(server:listen(8, function(err)
  assert(not err, err)
  local peer = uv.new_pipe(false)
  assert(server:accept(peer))
  peers[#peers + 1] = peer
  local pending = ""
  peer:read_start(function(readerr, chunk)
    assert(not readerr, readerr)
    if not chunk then return end
    pending = pending .. chunk
    while pending:find("\n", 1, true) do
      local nl = pending:find("\n", 1, true)
      local msg = vim.json.decode(pending:sub(1, nl - 1))
      pending = pending:sub(nl + 1)
      if msg.t == "hello" then hellos[#hellos + 1] = msg end
    end
  end)
end))

local plugin = require("zmk-vim-mode")
plugin._state.opts.socket = path
plugin.connect()
T.ok(vim.wait(2000, function() return #hellos >= 1 end), "initial hello arrives")
local first = hellos[1] or {}
T.ok(type(first.session) == "string" and first.session ~= "", "hello has a session token")
T.eq(first.pid, vim.fn.getpid(), "hello identifies the Neovim process")
plugin.reconnect()
T.ok(vim.wait(2000, function() return #hellos >= 2 end), "reconnect hello arrives")
T.eq((hellos[2] or {}).session, first.session, "reconnect preserves the session token")

plugin._state.stopped = true
plugin.reconnect() -- drops the old pipe without connecting again
for _, peer in ipairs(peers) do peer:close() end
server:close()
uv.fs_unlink(path)

package.loaded["zmk-vim-mode"] = nil
local reloaded = require("zmk-vim-mode")
T.eq(vim.g.zmk_vim_mode_session, first.session, "module reload preserves process identity")
reloaded._state.stopped = true
