#!/bin/bash
set -e

source ${HOME}/.profile

cd ${WORKDIR}

if [[ -d ".git" ]] && [[ -f ".dev/git-hook/install.sh" ]]; then
    bash .dev/git-hook/install.sh || true
fi

mkdir -p /go/pkg/mod
mkdir -p /go/cache/go-build
touch ${HOME}/.bash_history

# `exec` so tini (compose `init: true`) has sleep as its direct child. Without it bash stays PID 1,
# the kernel discards a default-action SIGTERM aimed at PID 1 in a namespace, and `stop` SIGKILLs.
exec sleep infinity
