#!/bin/bash

if [[ "../.dev/git-hooks" = "$(readlink .git/hooks)" ]]; then
    exit 0
fi

rm -rf .git/hooks
ln -sf ../.dev/git-hooks .git/hooks
