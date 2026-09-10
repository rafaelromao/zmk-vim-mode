-- zmk-vim-mode: VSCode (vscode-neovim) specifics.
--
-- Inside vscode-neovim, mappings open VSCode quick inputs -- the palette, Go
-- to File, rename... -- through require("vscode").action/call, which end in
-- vim.rpcnotify/rpcrequest(channel, "vscode-action", name, opts). Neovim then
-- stops receiving keys until the quick input closes, and nothing tells it so.
--
-- This module watches those calls. A matching action raises a "raw" hint that
-- the plugin reports instead of the mode; the hint is cleared by the next key
-- that actually reaches Neovim, by the companion extension (Escape, cursor,
-- active editor), or by a timer. Views and the terminal are deliberately not
-- listed: the window title already reports them, instantly, both ways.
local M = {}

-- Lua patterns matched against the VSCode command name.
M.default_raw_actions = {
  "^workbench%.action%.quickOpen", -- quickOpen, quickOpenWithModes, quickOpenView...
  "^workbench%.action%.showCommands$",
  "^workbench%.action%.gotoLine$",
  "^workbench%.action%.gotoSymbol$",
  "^workbench%.action%.showAllSymbols$",
  "^workbench%.action%.quickTextSearch$",
  "^workbench%.action%.openRecent$",
  "^workbench%.action%.showAllEditors",
  "^workbench%.action%.quickSwitchWindow$",
  "^workbench%.action%.tasks%.runTask$",
  "^editor%.action%.rename$",
}

---@param name any
---@param patterns string[]
---@return boolean
function M.is_raw_action(name, patterns)
  if type(name) ~= "string" then
    return false
  end
  for _, p in ipairs(patterns) do
    if name:find(p) then
      return true
    end
  end
  return false
end

---Wrap vscode-neovim's outgoing calls. on_hint(name) runs for every matching
---action. Two layers, because either alone can be bypassed by a cached
---function reference: the rpc functions (what the vscode module uses) and the
---module's own action/call (what mappings and :Find use).
---@param patterns string[]
---@param on_hint fun(name: string)
---@return boolean installed
function M.install(patterns, on_hint)
  if vim.g.vscode == nil then
    return false
  end
  local function watch(method, name)
    if method == "vscode-action" and M.is_raw_action(name, patterns) then
      pcall(on_hint, name)
    end
  end
  local notify, request = vim.rpcnotify, vim.rpcrequest
  vim.rpcnotify = function(chan, method, name, ...)
    watch(method, name)
    return notify(chan, method, name, ...)
  end
  vim.rpcrequest = function(chan, method, name, ...)
    watch(method, name)
    return request(chan, method, name, ...)
  end

  local ok, vscode = pcall(require, "vscode")
  if ok and type(vscode) == "table" then
    for _, fn in ipairs({ "action", "call" }) do
      local orig = vscode[fn]
      if type(orig) == "function" then
        vscode[fn] = function(name, ...)
          watch("vscode-action", name)
          return orig(name, ...)
        end
      end
    end
  end
  return true
end

---Ask VSCode once whether the companion extension is installed. Blocks for at
---most `timeout` ms; false whenever vscode-neovim cannot answer.
---@param timeout integer|nil
---@return boolean
function M.companion_available(timeout)
  local ok, vscode = pcall(require, "vscode")
  if not ok or type(vscode) ~= "table" or type(vscode.eval) ~= "function" then
    return false
  end
  local ok2, res =
    pcall(vscode.eval, "return !!vscode.extensions.getExtension('rafaelromao.zmk-vim-mode')", {}, timeout or 500)
  return ok2 and res == true
end

---Tell the companion about a hint so its close detection applies too.
---@param name string
function M.forward(name)
  local ok, vscode = pcall(require, "vscode")
  if ok and type(vscode) == "table" and type(vscode.action) == "function" then
    pcall(vscode.action, "zmkVimMode.rawHint", { args = { name } })
  end
end

return M
