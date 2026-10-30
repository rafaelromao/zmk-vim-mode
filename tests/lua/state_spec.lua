-- effective_state: the plugin's final verdict for the current window and mode.
local T = _G.T

T.section("effective_state")

local plugin = require("zmk-vim-mode")
local real_get_mode = vim.api.nvim_get_mode

---Run fn with nvim_get_mode reporting mode.
local function in_mode(mode, fn)
  vim.api.nvim_get_mode = function()
    return { mode = mode, blocking = false }
  end
  local ok, err = pcall(fn)
  vim.api.nvim_get_mode = real_get_mode
  assert(ok, err)
end

---Open a floating window on a scratch buffer with the given filetype/buftype.
local function float(ft, bt)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.bo[buf].buftype = bt
  vim.bo[buf].filetype = ft
  local win = vim.api.nvim_open_win(buf, true, { relative = "editor", row = 1, col = 1, width = 20, height = 1 })
  return win
end

local prompt = float("snacks_picker_input", "prompt")
in_mode("i", function()
  T.eq(plugin.effective_state(), "raw", "picker prompt in insert mode")
end)
in_mode("n", function()
  T.eq(plugin.effective_state(), "raw", "picker prompt in normal mode")
end)
vim.api.nvim_win_close(prompt, true)

local file = vim.api.nvim_create_buf(true, false)
vim.api.nvim_set_current_buf(file)
in_mode("i", function()
  T.eq(plugin.effective_state(), "insert", "ordinary buffer in insert mode")
end)
