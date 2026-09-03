-- Copy to ~/.config/nvim/lua/plugins/zmk-vim-mode.lua
return {
  {
    "rafaelromao/zmk-vim-mode",
    -- While developing, point at the checkout instead of fetching from GitHub:
    -- dir = vim.fn.expand("~/projects/zmk-vim-mode"),
    lazy = false, -- the daemon needs to know the mode from the first keystroke
    opts = {
      -- terminal_state = "raw",  -- "insert" if you prefer vim layers in :terminal
      -- leader_raw = true,       -- report raw while a <leader> sequence is pending
      -- debug = false,

      -- Extra filetypes whose single letters are plugin commands:
      -- raw_filetypes = vim.list_extend(
      --   vim.deepcopy(require("zmk-vim-mode.context").default_raw_filetypes),
      --   { "myplugin" }
      -- ),

      -- Last-resort hook; return a state string to override everything:
      -- classify = function(buf, win, mode)
      --   if vim.b[buf].my_special_ui then return "raw" end
      -- end,
    },
  },
}
