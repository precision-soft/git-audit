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

AUDIT_REQUESTED_STRING="false"
E2E_REQUESTED_STRING="false"
STAGED_ONLY_STRING="false"

for FLAG_STRING in "$@"; do
    case "${FLAG_STRING}" in
        -h)
            println "usage: all.sh [-h] [--all | --staged] [--e2e] [--audit]"
            println ""
            println "  -h         show this help and exit"
            println "  --all      validate all packages (default)"
            println "  --staged   validate only if staged .go changes exist"
            println "  --e2e      also vet and run the end-to-end suite ( builds the binary )"
            println "  --audit    also scan for known vulnerabilities ( needs: network )"
            exit 0
            ;;
        --all)
            :
            ;;
        --staged)
            STAGED_ONLY_STRING="true"
            ;;
        --e2e)
            E2E_REQUESTED_STRING="true"
            ;;
        --audit)
            AUDIT_REQUESTED_STRING="true"
            ;;
        *)
            fail "unknown flag: ${FLAG_STRING}"
            ;;
    esac
done

SERVICE_NAME_STRING="dev"

require_docker
require_docker_daemon

if ! docker_compose_service_exists "${SERVICE_NAME_STRING}"; then
    fail "missing docker compose service: ${SERVICE_NAME_STRING}"
fi

ensure_service_running "${SERVICE_NAME_STRING}"

if [[ "true" = "${STAGED_ONLY_STRING}" ]]; then
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

if [[ "true" = "${E2E_REQUESTED_STRING}" ]]; then
    run_section "go vet e2e" "${TAG_VALIDATE}" "go" -- \
        run_in_service_shell "${SERVICE_NAME_STRING}" "go vet -tags=e2e ./..."

    run_section "go test e2e" "${TAG_VALIDATE}" "go" -- \
        run_in_service_shell "${SERVICE_NAME_STRING}" "go test -tags=e2e -count=1 ./..."
fi

run_section "staticcheck" "${TAG_VALIDATE}" "go" -- \
    run_in_service_shell "${SERVICE_NAME_STRING}" \
    "if command -v staticcheck > /dev/null 2>&1; then staticcheck ./... && staticcheck -tags=e2e ./...; else echo 'staticcheck is not installed in the image — rebuild it (./dc build) to enable this section'; fi"

if [[ "true" = "${AUDIT_REQUESTED_STRING}" ]]; then
    run_section "govulncheck" "${TAG_VALIDATE}" "go" -- \
        run_in_service_shell "${SERVICE_NAME_STRING}" \
        "if command -v govulncheck > /dev/null 2>&1; then govulncheck ./... && govulncheck -tags=e2e ./...; else echo 'govulncheck is not installed in the image — rebuild it (./dc build) to enable this section'; fi"
fi
