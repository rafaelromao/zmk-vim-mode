# Isolated Bash prompt and history for Linux showcase terminals.
export EDITOR=nvim
export LANG=en_US.UTF-8
HISTFILE="$(dirname "${BASH_SOURCE[0]}")/../run/.bash_history"
HISTSIZE=200
HISTFILESIZE=200
PS1='\[\e[34m\]\W\[\e[0m\] \[\e[32m\]❯\[\e[0m\] '
PROMPT_COMMAND='printf "\033]0;%s\007" "${PWD##*/}"'
alias ll='ls -la'
