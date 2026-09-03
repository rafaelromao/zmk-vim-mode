-- Buffer/window context classification.
--
-- The keyboard's NORMAL layer remaps physical keys to vim letters, so any
-- buffer where letters are plugin commands (dashboard, file explorer, lazy,
-- mason, trouble) or where the user is typing free text into a non-vim widget
-- must report "raw" instead: no vim layers, keys pass through untouched.
local M = {}

-- Filetypes whose single-letter keymaps are the UI. Letters must reach them.
M.default_raw_filetypes = {
  "aerial",
  "alpha",
  "dap-repl",
  "dashboard",
  "DiffviewFiles",
  "DressingSelect",
  "fugitive",
  "grug-far",
  "lazy",
  "lspinfo",
  "mason",
  "neo-tree",
  "NvimTree",
  "neotest-summary",
  "noice",
  "notify",
  "oil",
  "Outline",
  "snacks_dashboard",
  "snacks_explorer",
  "snacks_layout_box",
  "snacks_picker_list",
  "starter",
  "TelescopeResults",
  "Trouble",
  "trouble",
  "undotree",
}

-- Filetypes that look like scratch buffers but are ordinary vim buffers:
-- navigation there is vim navigation, so keep the real mode.
M.default_raw_exceptions = {
  "checkhealth",
  "help",
  "man",
  "qf",
  "gitcommit",
  "gitrebase",
}

local function has(list, v)
  for _, x in ipairs(list) do
    if x == v then
      return true
    end
  end
  return false
end

-- Prefix match for generated filetypes like dapui_watches, snacks_picker_*.
local function has_prefixed(list, ft)
  for _, x in ipairs(list) do
    if x == ft or (#x > 0 and ft:sub(1, #x) == x and ft:sub(#x + 1, #x + 1) == "_") then
      return true
    end
  end
  return false
end

---Decide whether the current buffer/window is a raw-keys context.
---@param opts table { raw_filetypes, raw_exceptions }
---@param api table|nil injection point for tests
---@return boolean raw, string reason
function M.is_raw(opts, api)
  api = api or {
    buf = function()
      return vim.api.nvim_get_current_buf()
    end,
    win = function()
      return vim.api.nvim_get_current_win()
    end,
    bo = function(buf, name)
      return vim.api.nvim_get_option_value(name, { buf = buf })
    end,
    win_config = function(win)
      return vim.api.nvim_win_get_config(win)
    end,
  }
  local buf, win = api.buf(), api.win()
  local ft = api.bo(buf, "filetype") or ""
  local bt = api.bo(buf, "buftype") or ""

  local exceptions = opts.raw_exceptions or M.default_raw_exceptions
  if has(exceptions, ft) then
    return false, "exception filetype " .. ft
  end

  local raw_fts = opts.raw_filetypes or M.default_raw_filetypes
  if has_prefixed(raw_fts, ft) or has_prefixed({ "dapui", "neotest" }, ft) then
    return true, "raw filetype " .. ft
  end

  -- Read-only scratch buffers: their letters are almost always mappings.
  if (bt == "nofile" or bt == "prompt") and api.bo(buf, "modifiable") == false then
    return true, "unmodifiable " .. bt .. " buffer"
  end

  -- Floating windows with a plugin filetype (pickers, popups). A float showing
  -- a normal file (peek definition) keeps vim behaviour.
  local cfg = api.win_config(win) or {}
  if cfg.relative ~= nil and cfg.relative ~= "" and ft ~= "" and not has(exceptions, ft) then
    return true, "floating window " .. ft
  end

  return false, ""
end

return M
