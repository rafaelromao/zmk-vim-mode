#!/usr/bin/env bash
# Cuts the landing page's media (docs/index.html) out of the showcase video:
# two GIFs, a still for each (shown while a GIF is paused, or instead of it
# when the reader asks for reduced motion) and the social card.
#
#   bash scripts/showcase-gif.sh
#   VIDEO=elsewhere.mp4 OUT=/tmp/img bash scripts/showcase-gif.sh
#
# The times belong to the 2026-09-24 cut of showcase/run/showcase.mp4 (8:07).
# A new cut moves them: find each clip's edges again and check them on frames
# decoded from the finished GIF, not on the video, since the fps filter picks
# its own frames. Needs ffmpeg with palettegen/paletteuse (any recent build).
set -euo pipefail

cd "$(dirname "$0")/.."

VIDEO=${VIDEO:-showcase/run/showcase.mp4}
OUT=${OUT:-docs/img}
WIDTH=${WIDTH:-1280}
FPS=${FPS:-15}

# Stretches of the video that show the author's full name: the title card in
# beat 1, and the file picker previewing LICENSE in beat 4. The page is public,
# so no cut may touch them, whatever a new edit does to the times.
FORBIDDEN=("36.0 80.0" "229.5 231.0")

die() { echo "error: $*" >&2; exit 1; }

# allowed START DURATION: dies when the cut overlaps a forbidden stretch.
allowed() {
  local w
  for w in "${FORBIDDEN[@]}"; do
    awk -v s="$1" -v d="$2" -v w="$w" 'BEGIN { split(w, x, " "); exit !(s < x[2] && s + d > x[1]) }' &&
      die "the cut at $1 s (+$2 s) overlaps $w s, which shows the author's name"
  done
  return 0
}

command -v ffmpeg >/dev/null || die "ffmpeg not found"
[[ -f "$VIDEO" ]] || die "no video at $VIDEO"
mkdir -p "$OUT"

# gif NAME START DURATION
# One pass per clip. The palette comes from the clip itself; stats_mode=full
# weighs the static editor text as much as what moves, so the code stays
# crisp. No dithering, because flat UI colours compress better without it, and
# diff_mode=rectangle re-encodes only the part of a frame that changed.
gif() {
  allowed "$2" "$3"
  ffmpeg -v error -y -ss "$2" -t "$3" -i "$VIDEO" \
    -filter_complex "fps=$FPS,scale=$WIDTH:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=256:stats_mode=full[p];[b][p]paletteuse=dither=none:diff_mode=rectangle" \
    -loop 0 "$OUT/$1"
}

# still NAME AT
still() {
  allowed "$2" 0.02
  ffmpeg -v error -y -ss "$2" -i "$VIDEO" -frames:v 1 \
    -vf "scale=$WIDTH:-1:flags=lanczos" "$OUT/$1"
}

# The cold open in Neovim, 20.8 -> 34.2 s: `:21` (the HUD reads Vim cmdline),
# Enter, `A` (Vim insert: the Romak base comes back), " no OS ever sets these",
# Esc (Vim normal) and `j j k l l h` lighting the right home row. It stops
# before the `u` at 34.5 s, which would undo the line on camera.
gif showcase-hero.gif 20.8 13.4
still showcase-hero.png 33.6

# Neovim going raw, 233.8 -> 240.4 s: modes.go in Vim normal; `<space>` opens
# which-key and the HUD drops to Alpha 1, no vim layers; `<space>e` opens the
# explorer and `j j k` move the tree, still raw; `<space>e` closes it and Vim
# normal is back. The edges are tight on purpose. The file picker closes at
# 233.75 s, and its preview had shown LICENSE a few seconds earlier. The
# command line for `:terminal` opens at 240.45 s.
gif showcase-raw.gif 233.8 6.6
still showcase-raw.png 237.0

# The social card, 1200x630: the hero still, padded with the TokyoNight
# background rather than cropped, so the top bar and lualine stay in.
allowed 33.6 0.02
ffmpeg -v error -y -ss 33.6 -i "$VIDEO" -frames:v 1 \
  -vf "scale=1120:630:flags=lanczos,pad=1200:630:40:0:color=0x1a1b26" "$OUT/og.png"

for f in showcase-hero.gif showcase-raw.gif showcase-hero.png showcase-raw.png og.png; do
  printf '%-18s %8s bytes\n' "$f" "$(wc -c <"$OUT/$f" | tr -d ' ')"
done
