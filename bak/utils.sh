#!/bin/bash
set -Eeo pipefail

# env globals and defaults
# ------------------------------

COLOR_RED=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;31m" || echo "")
COLOR_GREEN=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;32m" || echo "")
COLOR_YELLOW=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;33m" || echo "")
COLOR_GRAY=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;90m" || echo "")
COLOR_END=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0m" || echo "")

# functions
# ------------------------------

log() {
    local msg=$1
    echo -e "${COLOR_GREEN}[I] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
}

verbose() {
    local msg=$1
    echo -e "${COLOR_GRAY}${msg}${COLOR_END}"
}

warn() {
    local msg=$1
    echo -e "${COLOR_YELLOW}[W] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
}

fatal() {
    local msg=$1
    >&2 echo -e "${COLOR_RED}[E] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
    exit 1
}

utils_check_host_requirements() {
    local flock_required=$1

    # check required cli tooling is available on the system that executes this script
    command -v cat >/dev/null || fatal "cat is required but not found."
    command -v sed >/dev/null || fatal "sed is required but not found."
    command -v awk >/dev/null || fatal "awk is required but not found."
    command -v grep >/dev/null || fatal "grep is required but not found."
    command -v dirname >/dev/null || fatal "dirname is required but not found."
    command -v kubectl >/dev/null || fatal "kubectl is required but not found."

    if [ "${flock_required}" == "true" ]; then
        command -v flock >/dev/null || fatal "flock is required but not found."
        command -v shuf >/dev/null || fatal "shuf is required (for flock) but not found."
    fi
}
