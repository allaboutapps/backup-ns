#!/bin/bash
set -Eeo pipefail

# Utility functions
# ------------------------------

COLOR_RED=$([ "$BAK_COLORS" == "true" ] && printf "\033[0;31m\n" || printf "\n")
COLOR_GREEN=$([ "$BAK_COLORS" == "true" ] && printf "\033[0;32m\n" || printf "\n")
COLOR_YELLOW=$([ "$BAK_COLORS" == "true" ] && printf "\033[0;33m\n" || printf "\n")
COLOR_GRAY=$([ "$BAK_COLORS" == "true" ] && printf "\033[0;90m\n" || printf "\n")
COLOR_END=$([ "$BAK_COLORS" == "true" ] && printf "\033[0m\n" || printf "\n")

# functions
# ------------------------------

log() {
    local msg=$1
    printf '%b%s: %s%b\n' "$COLOR_GREEN" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
}

verbose() {
    local msg=$1
    printf '%b%s%b\n' "$COLOR_GRAY" "$msg" "$COLOR_END"
}

warn() {
    local msg=$1
    printf '%b%s: %s%b\n' "$COLOR_YELLOW" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
}

fatal() {
    local msg=$1
    >&2 printf '%b%s: %s%b\n' "$COLOR_RED" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
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

    if [ "$flock_required" == "true" ]; then
        command -v flock >/dev/null || fatal "flock is required but not found."
        command -v shuf >/dev/null || fatal "shuf is required (for flock) but not found."
    fi
}
