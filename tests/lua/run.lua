-- Minimal test runner: nvim --headless -l tests/lua/run.lua
-- Exits non-zero on the first failure so `make test` is meaningful.
local root = vim.fn.fnamemodify(vim.fn.resolve(debug.getinfo(1, "S").source:sub(2)), ":h:h:h")
vim.opt.runtimepath:prepend(root)
package.path = root .. "/lua/?.lua;" .. root .. "/lua/?/init.lua;" .. package.path

local failures, checks = 0, 0

local T = {}

function T.eq(got, want, msg)
  checks = checks + 1
  if got ~= want then
    failures = failures + 1
    io.stderr:write(string.format("FAIL %s\n  got:  %s\n  want: %s\n", msg or "", vim.inspect(got), vim.inspect(want)))
  end
end

function T.ok(cond, msg)
  T.eq(cond and true or false, true, msg)
end

function T.section(name)
  io.stdout:write("-- " .. name .. "\n")
end

_G.T = T

for _, spec in ipairs({ "modes_spec", "context_spec" }) do
  dofile(root .. "/tests/lua/" .. spec .. ".lua")
end

io.stdout:write(string.format("\n%d checks, %d failures\n", checks, failures))
if failures > 0 then
  vim.cmd("cquit 1")
end
vim.cmd("quit")
