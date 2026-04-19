#!/usr/bin/env bash
set -euo pipefail

CLONE_ROOT=".dev-data/clones"

REPOSITORIES=(
    "https://github.com/precision-soft/doctrine-type"
    "https://github.com/precision-soft/doctrine-utility"
    "https://github.com/precision-soft/symfony-console"
    "https://github.com/precision-soft/symfony-doctrine-audit"
    "https://github.com/precision-soft/symfony-doctrine-encrypt"
    "https://github.com/precision-soft/symfony-json-form"
    "https://github.com/precision-soft/symfony-phpunit"
    "https://github.com/precision-soft/melody"
)

mkdir -p "${CLONE_ROOT}"

for REPOSITORY_URL in "${REPOSITORIES[@]}"; do
    NAME="${REPOSITORY_URL##*/}"
    TARGET="${CLONE_ROOT}/${NAME}"

    if [ -d "${TARGET}/.git" ]; then
        echo "reset ${NAME}"
        git -C "${TARGET}" fetch --tags --prune origin
        git -C "${TARGET}" remote set-head origin --auto >/dev/null
        DEFAULT_BRANCH="$(git -C "${TARGET}" symbolic-ref --short refs/remotes/origin/HEAD)"
        DEFAULT_BRANCH="${DEFAULT_BRANCH#origin/}"
        git -C "${TARGET}" checkout --quiet "${DEFAULT_BRANCH}"
        git -C "${TARGET}" reset --hard "origin/${DEFAULT_BRANCH}"
        git -C "${TARGET}" clean -fdx
    else
        echo "clone ${NAME}"
        git clone "${REPOSITORY_URL}" "${TARGET}"
    fi
done
