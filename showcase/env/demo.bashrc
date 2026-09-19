# Isolated Bash prompt and history for Linux showcase terminals.
export EDITOR=nvim
export LANG=en_US.UTF-8
HISTFILE="$(dirname "${BASH_SOURCE[0]}")/../run/.bash_history"
HISTSIZE=200
HISTFILESIZE=200
PS1='\[\e[34m\]\W\[\e[0m\] \[\e[32m\]❯\[\e[0m\] '
PROMPT_COMMAND='printf "\033]0;%s\007" "${PWD##*/}"'
alias ll='ls -la'

# Cards for the automated beats (env/cards/*.txt): `show title`, `show channel`, ...
SHOWCASE_CARDS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cards"
show() { clear; cat "$SHOWCASE_CARDS/$1.txt"; }
# doctor prints home paths; on camera they read as ~
doctor() { zmk-vim-mode doctor 2>&1 | sed "s#$HOME#~#g"; }
