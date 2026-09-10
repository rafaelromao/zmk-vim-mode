-- The vscode-neovim hook: which commands raise the raw hint, and that both
-- call layers are watched without changing what reaches VSCode.
local vscode = require("zmk-vim-mode.vscode")
local T = _G.T

T.section("vscode.is_raw_action")

local P = vscode.default_raw_actions
for _, name in ipairs({
  "workbench.action.quickOpen",
  "workbench.action.quickOpenWithModes",
  "workbench.action.showCommands",
  "workbench.action.gotoLine",
  "workbench.action.gotoSymbol",
  "workbench.action.showAllSymbols",
  "workbench.action.quickTextSearch",
  "workbench.action.openRecent",
  "workbench.action.showAllEditorsByAppearance",
  "workbench.action.tasks.runTask",
  "editor.action.rename",
}) do
  T.ok(vscode.is_raw_action(name, P), name .. " raises the hint")
end
for _, name in ipairs({
  "workbench.action.terminal.focus", -- the title covers views and the terminal
  "workbench.view.explorer",
  "workbench.action.findInFiles",
  "editor.action.formatDocument",
  "workbench.action.files.save",
  "zmkVimMode.rawHint", -- our own forward must never recurse
}) do
  T.ok(not vscode.is_raw_action(name, P), name .. " does not raise the hint")
end
T.ok(not vscode.is_raw_action(nil, P), "nil name")
T.ok(not vscode.is_raw_action({}, P), "non-string name")
T.ok(vscode.is_raw_action("editor.action.rename", { "rename$" }), "custom patterns")
T.ok(not vscode.is_raw_action("editor.action.rename", {}), "empty patterns disable")

T.section("vscode.install")

-- Outside VSCode the hook stays out of the way.
vim.g.vscode = nil
T.eq(vscode.install(P, function() end), false, "not installed outside vscode")

-- Inside: rpc calls are watched and passed through untouched.
vim.g.vscode = 1
local seen = {}
local calls = {}
local orig_notify, orig_request = vim.rpcnotify, vim.rpcrequest
vim.rpcnotify = function(...)
  calls[#calls + 1] = { "notify", ... }
  return 1
end
vim.rpcrequest = function(...)
  calls[#calls + 1] = { "request", ... }
  return "reply"
end
T.eq(vscode.install(P, function(name) seen[#seen + 1] = name end), true, "installed inside vscode")

T.eq(vim.rpcnotify(7, "vscode-action", "workbench.action.showCommands", { args = {} }), 1, "notify passes through")
T.eq(vim.rpcrequest(7, "vscode-action", "editor.action.rename"), "reply", "request passes through")
vim.rpcnotify(7, "vscode-action", "editor.action.formatDocument")
vim.rpcnotify(7, "redraw", "workbench.action.showCommands") -- wrong method: ignored
T.eq(#seen, 2, "two hints raised")
T.eq(seen[1], "workbench.action.showCommands", "first hint")
T.eq(seen[2], "editor.action.rename", "second hint")
T.eq(#calls, 4, "every call reached the rpc layer")
T.eq(calls[1][3], "vscode-action", "method forwarded")
T.eq(calls[1][4], "workbench.action.showCommands", "name forwarded")

vim.rpcnotify, vim.rpcrequest = orig_notify, orig_request
vim.g.vscode = nil
