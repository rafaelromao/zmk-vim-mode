-- Command registration only. setup() is called by the plugin manager
-- (lazy.nvim `opts = {}`); calling it here too would run it twice.
if vim.g.loaded_zmk_vim_mode then
  return
end
vim.g.loaded_zmk_vim_mode = 1

vim.api.nvim_create_user_command("ZmkVimMode", function(cmd)
  local m = require("zmk-vim-mode")
  local action = cmd.args ~= "" and cmd.args or "status"
  if action == "status" then
    local s = m.status()
    local lines = { "zmk-vim-mode" }
    for _, k in ipairs({ "socket", "connected", "effective", "last_sent", "focused", "leader_pending", "nested", "tmux" }) do
      lines[#lines + 1] = string.format("  %-14s %s", k, tostring(s[k]))
    end
    vim.notify(table.concat(lines, "\n"), vim.log.levels.INFO)
  elseif action == "reconnect" then
    m.reconnect()
    vim.notify("[zmk-vim-mode] reconnecting to " .. m.status().socket, vim.log.levels.INFO)
  else
    vim.notify("[zmk-vim-mode] unknown action: " .. action .. " (status|reconnect)", vim.log.levels.ERROR)
  end
end, {
  nargs = "?",
  complete = function()
    return { "status", "reconnect" }
  end,
  desc = "zmk-vim-mode: status or reconnect",
})
