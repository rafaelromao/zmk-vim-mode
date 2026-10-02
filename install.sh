#!/bin/sh
# zmk-vim-mode installer.
#
#   curl -fsSL https://raw.githubusercontent.com/rafaelromao/zmk-vim-mode/main/install.sh | sh
#
# It puts the source tree in ~/.local/share/zmk-vim-mode and hands over to `make install` there,
# which builds the daemon, puts it in ~/.local/bin, starts the user service and sets up the editors
# and the status bar it finds. Nothing here runs as root: on Linux `make install` asks for sudo once,
# for the udev rule, and only where there is a terminal to ask on. $ZMK_VIM_MODE_HOME moves the
# tree; $ZMK_VIM_MODE_REF picks what is installed: `latest`, the newest release (the default), a
# release's tag such as v1.0.0, or a branch such as main.
#
# The daemon is built from source, so this needs Go 1.26 or newer and make, and on macOS the Xcode
# Command Line Tools for cgo.
#
# What you run is whatever that release or branch holds: there is no signature to check. Read it
# first if that matters to you (`curl -fsSL .../install.sh | less`), or clone the repository and
# run `make install` from the clone, which `git` then updates.
set -eu

REPO=rafaelromao/zmk-vim-mode
REF=${ZMK_VIM_MODE_REF:-latest}
HOME_DIR=${ZMK_VIM_MODE_HOME:-$HOME/.local/share/zmk-vim-mode}
TMP=
NEW=

# The tree this installs, as against any other directory that happens to be at $HOME_DIR: nothing
# below removes or moves a directory that does not have the daemon's source in it.
is_tree() { [ -f "$1/cmd/zmk-vim-mode/main.go" ]; }

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "zmk-vim-mode: $1 is needed${2:+ -- $2}" >&2; exit 1; }
}

# Everything in one function, called on the last line: under `curl | sh` a download cut short is
# then a syntax error that runs nothing, not a script that runs as far as it got.
main() {
  case "$(uname -s)" in
    Darwin|Linux) ;;
    *) echo "zmk-vim-mode: no daemon for $(uname -s); macOS and Linux only" >&2; exit 1 ;;
  esac

  # Checked before anything is fetched: a tree that cannot be built is a download for nothing.
  need tar
  need gzip
  need make
  need go "Go 1.26 or newer builds the daemon: https://go.dev/dl"
  if [ "$(uname -s)" = Darwin ] && ! xcode-select -p >/dev/null 2>&1; then
    echo "zmk-vim-mode: the Xcode Command Line Tools are needed for cgo -- xcode-select --install" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    fetch() { curl -fsSL "$1" -o "$2"; }
  elif command -v wget >/dev/null 2>&1; then
    fetch() { wget -qO "$2" "$1"; }
  else
    echo "zmk-vim-mode: curl or wget is needed" >&2; exit 1
  fi

  # A clone is updated with `git pull`; overwriting one would throw away whatever is uncommitted.
  if [ -d "$HOME_DIR/.git" ]; then
    echo "zmk-vim-mode: $HOME_DIR is a git clone -- update it with: git -C $HOME_DIR pull && make -C $HOME_DIR install" >&2
    exit 1
  fi
  if [ -e "$HOME_DIR" ] && ! is_tree "$HOME_DIR"; then
    echo "zmk-vim-mode: $HOME_DIR exists and is not a zmk-vim-mode tree; move it, or set ZMK_VIM_MODE_HOME" >&2
    exit 1
  fi
  if [ -e "$HOME_DIR.old" ] && ! is_tree "$HOME_DIR.old"; then
    echo "zmk-vim-mode: $HOME_DIR.old exists and is not a zmk-vim-mode tree; move it first" >&2
    exit 1
  fi

  TMP=$(mktemp -d)
  mkdir -p "$(dirname "$HOME_DIR")"
  # Staged beside the destination, not in $TMP: /tmp is often another filesystem, and the point of
  # staging is that the swap is two renames within one directory. mktemp names it, so nothing that
  # was already there is removed to make room.
  NEW=$(mktemp -d "$HOME_DIR.new.XXXXXX")
  trap 'rm -rf "$TMP" "$NEW"' EXIT INT TERM

  # `latest` is the newest release, as GitHub names it. The ref goes into a URL and into the
  # version the build stamps, so it is a plain name and nothing else, and one that starts with v
  # and a digit is a release's tag.
  if [ "$REF" = latest ]; then
    fetch "https://api.github.com/repos/$REPO/releases/latest" "$TMP/latest.json" || {
      echo "zmk-vim-mode: could not ask GitHub for the latest release; ZMK_VIM_MODE_REF=main installs the main branch" >&2; exit 1; }
    REF=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$TMP/latest.json" | head -n 1)
  fi
  case "$REF" in
    ""|*..*|[!A-Za-z0-9]*|*[!A-Za-z0-9._/-]*)
      echo "zmk-vim-mode: '$REF' is not a branch or a release's tag (ZMK_VIM_MODE_REF=main installs the main branch)" >&2; exit 1 ;;
  esac
  case "$REF" in
    v[0-9]*) KIND=tags ;;
    *) KIND=heads ;;
  esac

  echo "==> fetching $REPO@$REF"
  fetch "https://codeload.github.com/$REPO/tar.gz/refs/$KIND/$REF" "$TMP/tree.tar.gz"
  tar xzf "$TMP/tree.tar.gz" -C "$NEW" --strip-components=1
  is_tree "$NEW" || { echo "zmk-vim-mode: $REPO@$REF has no daemon to build" >&2; exit 1; }

  # A tarball has no .git for `git describe`, so the version the binary reports comes from here:
  # a release's tag as it is, a branch with the commit GitHub wrote into the archive's pax header
  # (the first line, once the header block's NULs are gone), as `main-06846f5`.
  VERSION=$REF
  if [ "$KIND" = heads ]; then
    COMMIT=$(gzip -dc "$TMP/tree.tar.gz" | tr -d '\000' | sed -n 's/.*comment=\([0-9a-f]\{40\}\)$/\1/p;q')
    if [ -n "$COMMIT" ]; then VERSION=$REF-$(echo "$COMMIT" | cut -c1-7); fi
  fi

  if [ -d "$HOME_DIR" ]; then
    rm -rf "$HOME_DIR.old"
    mv "$HOME_DIR" "$HOME_DIR.old"
  fi
  mv "$NEW" "$HOME_DIR"
  NEW=
  rm -rf "$HOME_DIR.old"
  echo "    tree at $HOME_DIR"

  # `make install` owns the build, the signing, the service and the editors, so there is one
  # implementation of each. Under `curl | sh` stdin is the pipe, so reopen the terminal where there
  # is one -- that is what lets it create the macOS signing certificate and ask sudo for the udev
  # rule. Whether /dev/tty opens is the test, not whether it is readable: without a controlling
  # terminal it is still mode 666, and the redirect would fail.
  echo
  echo "==> make install ($VERSION)"
  if (exec </dev/tty) 2>/dev/null; then
    make -C "$HOME_DIR" install VERSION="$VERSION" </dev/tty
  else
    make -C "$HOME_DIR" install VERSION="$VERSION"
  fi
}

main "$@"
