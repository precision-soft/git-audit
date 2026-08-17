#!/bin/bash

export HISTFILE="${HOME}/.bash_history"
export HISTSIZE=10000
export HISTFILESIZE=10000
if [ -n "${BASH_VERSION:-}" ]; then
    shopt -s histappend
fi

if [ -n "${BASH_VERSION:-}" ]; then
    if [[ -f /app/.dev/utility.sh ]]; then
        . /app/.dev/utility.sh
    fi
fi

alias ll="ls -al"
alias app="cd /app/"

if [ -n "${BASH_VERSION:-}" ]; then
    gv() {
        go vet "$@" ./...
    }

    gt() {
        go test "$@" ./...
    }

    goa() {
        gv "$@"
        gt "$@"
    }

    go_build() {
        local outputName="$1"
        shift

        if [ -z "$outputName" ]; then
            echo "missing output name"
            return 1
        fi

        go build -o "$outputName" "$@" .

        local buildExitCode="$?"
        if [ 0 -ne "$buildExitCode" ]; then
            return "$buildExitCode"
        fi

        chmod +x "$outputName"
    }

    audit() {
        go run . audit "$@"
    }

    if command -v git > /dev/null 2>&1; then
        git config --global alias.st status
        git config --global alias.ci commit
        git config --global alias.co checkout
        git config --global alias.br branch
        git config --global color.branch auto
        git config --global color.diff auto
        git config --global color.interactive auto
        git config --global color.status auto
        git config --global push.default current
        git config --global init.defaultBranch master
        git config --global core.autocrlf input
        git config --global pull.rebase false
        git config --global --add safe.directory /app/
    fi

    if [[ -f ~/.bash_aliases_local ]]; then
        . ~/.bash_aliases_local
    fi
fi
