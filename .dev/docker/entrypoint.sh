#!/bin/bash
set -e

source ${HOME}/.profile

cd ${WORKDIR}

mkdir -p /go/pkg/mod
mkdir -p /go/cache/go-build
touch ${HOME}/.bash_history

sleep infinity
