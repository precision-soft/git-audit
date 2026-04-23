#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

REPOSITORY_ROOT_DIRECTORY_STRING="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ "" = "${REPOSITORY_ROOT_DIRECTORY_STRING}" ]]; then
    SCRIPT_DIRECTORY_STRING="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    DEV_DIRECTORY_STRING="$(cd -P "${SCRIPT_DIRECTORY_STRING}/.." && pwd)"
    REPOSITORY_ROOT_DIRECTORY_STRING="$(cd -P "${DEV_DIRECTORY_STRING}/.." && pwd)"
fi

. "${REPOSITORY_ROOT_DIRECTORY_STRING}/.dev/utility.sh"

if [[ "" = "${1-}" ]]; then
    :
elif [[ "-h" = "${1-}" ]]; then
    println "usage: all.sh [-h] [--all | --staged]"
    println ""
    println "  -h         show this help and exit"
    println "  --all      validate all packages (default)"
    println "  --staged   validate only if staged .go changes exist"
    exit 0
elif [[ "--staged" = "${1-}" || "--all" = "${1-}" ]]; then
    :
else
    fail "unknown flag: ${1}"
fi

SERVICE_NAME_STRING="dev"

require_docker
require_docker_daemon

if ! docker_compose_service_exists "${SERVICE_NAME_STRING}"; then
    fail "missing docker compose service: ${SERVICE_NAME_STRING}"
fi

ensure_service_running "${SERVICE_NAME_STRING}"

if [[ "--staged" = "${1-}" ]]; then
    if ! git --no-pager diff --cached --name-only --diff-filter=d | grep -q '\.go$'; then
        exit 0
    fi
fi

run_section "go vet" "${TAG_VALIDATE}" "go" -- \
    run_in_service_shell "${SERVICE_NAME_STRING}" "go vet ./..."

run_section "go build" "${TAG_VALIDATE}" "go" -- \
    run_in_service_shell "${SERVICE_NAME_STRING}" "go build ./..."

run_section "go test" "${TAG_VALIDATE}" "go" -- \
    run_in_service_shell "${SERVICE_NAME_STRING}" "go test ./..."
