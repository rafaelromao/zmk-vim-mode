-- Coarse mapping from Neovim's mode() string to the states the daemon knows.
-- Reference: :h mode() on Neovim 0.11.5. Match on leading characters, as the
-- docs advise, because more specific modes may be added.
local M = {}

---Map a raw mode() string to a wire state.
---@param raw string value of nvim_get_mode().mode / v:event.new_mode
---@param opts table|nil { terminal_state = "raw"|"insert" }
---@return string|nil state one of normal|insert|visual|cmdline|raw, or nil to keep the previous state
function M.coarse(raw, opts)
  opts = opts or {}
  if type(raw) ~= "string" or raw == "" then
    return nil
  end
  local c = raw:sub(1, 1)

  if c == "n" then
    -- n, niI, niR, niV, nt, ntT → normal.
    -- no, nov, noV, no^V (operator-pending) → nil: the keyboard's own CHANGE
    -- sticky layer is handling this, and a host correction would fight it.
    if raw:sub(1, 2) == "no" then
      return nil
    end
    return "normal"
  elseif c == "i" then
    return "insert" -- i, ic, ix
  elseif c == "R" then
    -- Replace maps to insert on purpose: VIM_REPLACE on the keyboard is the
    -- sticky "next key is a literal" layer for r<char>/q<reg>, not Neovim's R.
    return "insert"
  elseif c == "v" or c == "V" or c == "\22" or c == "s" or c == "S" or c == "\19" then
    return "visual" -- v vs V Vs ^V ^Vs s S ^S
  elseif c == "c" then
    return "cmdline" -- c cr cv cvr
  elseif c == "r" then
    return "cmdline" -- r (hit-enter), rm (-- more --), r? (:confirm)
  elseif c == "t" then
    return opts.terminal_state or "raw" -- keys belong to the job; Esc must reach it
  end
  -- "!" (shell running) and anything unknown: keep the previous state.
  return nil
end

return M
