-- Window placement shared by the rehearsal runner and prepare.sh.
--
--   local P = dofile(".../showcase/hud/place.lua")
--   P.recordingScreen()   the external display when there is one, else the primary
--   P.place(win)          fill the recording display minus the bottom band of the typed-keys strip
--   P.all()               place every window of the showcase apps that is currently open

local P = {}

P.BOTTOM_BAND = 72 + 2 * 24 -- the typed-keys strip (72 pt) with its margins

function P.recordingScreen()
  if zmkhud and zmkhud.screen then return zmkhud.screen end
  local primary = hs.screen.primaryScreen()
  for _, s in ipairs(hs.screen.allScreens()) do if s ~= primary then return s end end
  return primary
end

function P.place(w)
  if not w then return false end
  local f = P.recordingScreen():frame()
  w:setFrame(hs.geometry.rect(f.x, f.y, f.w, f.h - P.BOTTOM_BAND), 0)
  return true
end

P.APPS = { "Code", "IntelliJ IDEA", "Obsidian" }

function P.all()
  local n = 0
  for _, name in ipairs(P.APPS) do
    local app = hs.application.get(name)
    if app then
      for _, w in ipairs(app:allWindows()) do
        if w:isStandard() and P.place(w) then n = n + 1 end
      end
    end
  end
  return n
end

return P
