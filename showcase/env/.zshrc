# Demo shell for the showcase recording. Loaded when ZDOTDIR points at showcase/env.
# No user@host, no corporate prompt, no shell history sync, no dotfiles.

export PATH="$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export EDITOR=nvim
export LANG=en_US.UTF-8
export TERM=xterm-ghostty

# History stays inside the workspace and is thrown away with it.
HISTFILE="${ZDOTDIR}/../run/.zsh_history"
HISTSIZE=200
SAVEHIST=200

autoload -Uz colors && colors
setopt PROMPT_SUBST
# Directory only: /Users/<you>/… never appears on screen.
PROMPT='%F{blue}%1~%f %F{green}❯%f '
RPROMPT=''

# Window title = the current directory's basename (nvim overrides it while running).
precmd() { print -Pn "\e]0;%1~\a"; }

alias ll='ls -la'
cd "${ZDOTDIR}/../demo-go" 2>/dev/null || true
