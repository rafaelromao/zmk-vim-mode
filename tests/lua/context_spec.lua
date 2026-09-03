-- Context classifier: which buffers/windows must report "raw".
local context = require("zmk-vim-mode.context")
local T = _G.T

T.section("context.is_raw")

---Build a fake api for is_raw.
local function fake(ft, bt, modifiable, relative)
  return {
    buf = function()
      return 1
    end,
    win = function()
      return 1000
    end,
    bo = function(_, name)
      if name == "filetype" then
        return ft
      elseif name == "buftype" then
        return bt
      elseif name == "modifiable" then
        return modifiable
      end
    end,
    win_config = function()
      return { relative = relative or "" }
    end,
  }
end

local cases = {
  -- { filetype, buftype, modifiable, relative, want_raw, description }
  { "", "", true, "", false, "ordinary file buffer" },
  { "go", "", true, "", false, "code buffer" },
  { "snacks_dashboard", "nofile", false, "", true, "dashboard: letters are shortcuts" },
  { "dashboard", "nofile", false, "", true, "dashboard variant" },
  { "alpha", "nofile", false, "", true, "alpha start screen" },
  { "neo-tree", "nofile", false, "", true, "file explorer: a/d/r are commands" },
  { "NvimTree", "nofile", false, "", true, "nvim-tree" },
  { "oil", "acwrite", true, "", true, "oil.nvim" },
  { "lazy", "nofile", false, "", true, "lazy.nvim UI" },
  { "mason", "nofile", false, "", true, "mason UI" },
  { "trouble", "nofile", false, "", true, "trouble list" },
  { "Trouble", "nofile", false, "", true, "trouble list (capitalised)" },
  { "dapui_watches", "nofile", false, "", true, "generated dapui_* filetype" },
  { "neotest-summary", "nofile", false, "", true, "neotest summary" },
  { "TelescopeResults", "nofile", false, "", true, "telescope results list" },
  { "snacks_picker_list", "nofile", false, "", true, "snacks picker list" },
  -- exceptions: vim navigation is correct there
  { "help", "help", false, "", false, "help buffer stays vim" },
  { "qf", "quickfix", false, "", false, "quickfix stays vim" },
  { "man", "nofile", false, "", false, "man page stays vim" },
  { "checkhealth", "nofile", false, "", false, "checkhealth stays vim" },
  { "gitcommit", "", true, "", false, "commit message is a normal edit" },
  -- structural rules
  { "", "nofile", false, "", true, "unmodifiable scratch buffer" },
  { "", "prompt", false, "", true, "unmodifiable prompt buffer" },
  { "", "nofile", true, "", false, "modifiable scratch buffer is editable" },
  { "noice", "nofile", false, "win", true, "floating plugin window" },
  { "markdown", "", true, "win", true, "floating window with a filetype (peek/preview UI)" },
  { "", "", true, "win", false, "floating window without a filetype" },
  { "help", "help", false, "win", false, "floating help is still help" },
}

for _, c in ipairs(cases) do
  local ft, bt, modifiable, relative, want, desc = c[1], c[2], c[3], c[4], c[5], c[6]
  local got = context.is_raw({}, fake(ft, bt, modifiable, relative))
  T.eq(got, want, string.format("is_raw(ft=%q bt=%q): %s", ft, bt, desc))
end

T.section("context options")

-- User-supplied lists replace the defaults.
local got = context.is_raw({ raw_filetypes = { "mycustom" } }, fake("mycustom", "", true, ""))
T.eq(got, true, "custom raw_filetypes honoured")

got = context.is_raw({ raw_filetypes = { "mycustom" } }, fake("neo-tree", "", true, ""))
T.eq(got, false, "custom list replaces defaults")

got = context.is_raw({ raw_exceptions = { "neo-tree" } }, fake("neo-tree", "nofile", true, ""))
T.eq(got, false, "custom exception wins over raw filetype")
