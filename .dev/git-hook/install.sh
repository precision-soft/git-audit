#!/bin/bash

rm -rf .git/hooks
ln -sf ../.dev/git-hooks .git/hooks
