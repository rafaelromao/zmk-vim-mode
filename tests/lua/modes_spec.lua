-- Every mode() value documented in :h mode() on Neovim 0.11.5.
local modes = require("zmk-vim-mode.modes")
local T = _G.T

T.section("modes.coarse")

local CTRL_V = "\22"
local CTRL_S = "\19"

local cases = {
  -- normal family
  { "n", "normal" },
  { "niI", "normal" },
  { "niR", "normal" },
  { "niV", "normal" },
  { "nt", "normal" }, -- normal inside :terminal
  { "ntT", "normal" },
  -- operator-pending: skipped so the keyboard's CHANGE sticky layer survives
  { "no", nil },
  { "nov", nil },
  { "noV", nil },
  { "no" .. CTRL_V, nil },
  -- insert family
  { "i", "insert" },
  { "ic", "insert" },
  { "ix", "insert" },
  -- replace maps to insert (VIM_REPLACE on the keyboard is a different concept)
  { "R", "insert" },
  { "Rc", "insert" },
  { "Rx", "insert" },
  { "Rv", "insert" },
  { "Rvc", "insert" },
  { "Rvx", "insert" },
  -- visual / select
  { "v", "visual" },
  { "vs", "visual" },
  { "V", "visual" },
  { "Vs", "visual" },
  { CTRL_V, "visual" },
  { CTRL_V .. "s", "visual" },
  { "s", "visual" },
  { "S", "visual" },
  { CTRL_S, "visual" },
  -- cmdline and prompts
  { "c", "cmdline" },
  { "cr", "cmdline" },
  { "cv", "cmdline" },
  { "cvr", "cmdline" },
  { "r", "cmdline" }, -- hit-enter
  { "rm", "cmdline" }, -- -- more --
  { "r?", "cmdline" }, -- :confirm
  -- terminal job
  { "t", "raw" },
  -- shell running / unknown / empty
  { "!", nil },
  { "zz", nil },
  { "", nil },
}

for _, c in ipairs(cases) do
  local raw, want = c[1], c[2]
  T.eq(modes.coarse(raw, {}), want, string.format("coarse(%q)", raw))
end

T.eq(modes.coarse("t", { terminal_state = "insert" }), "insert", "terminal_state option honoured")
T.eq(modes.coarse(nil, {}), nil, "nil input is safe")
T.eq(modes.coarse(42, {}), nil, "non-string input is safe")
