#!/bin/bash
set -Eeo pipefail

# Utility functions
# ------------------------------

# env globals and defaults
# ------------------------------

# COLORS_ENABLED: if true, colored output is enabled
COLORS_ENABLED="${COLORS_ENABLED:=$( (which tput > /dev/null 2>&1 && [[ $(tput -T"$TERM" colors) -ge 8 ]] && echo "true") || echo "false")}" 

COLOR_RED=$([ "$COLORS_ENABLED" == "true" ] && printf "\033[0;31m\n" || printf "\n")
COLOR_GREEN=$([ "$COLORS_ENABLED" == "true" ] && printf "\033[0;32m\n" || printf "\n")
COLOR_YELLOW=$([ "$COLORS_ENABLED" == "true" ] && printf "\033[0;33m\n" || printf "\n")
COLOR_GRAY=$([ "$COLORS_ENABLED" == "true" ] && printf "\033[0;90m\n" || printf "\n")
COLOR_END=$([ "$COLORS_ENABLED" == "true" ] && printf "\033[0m\n" || printf "\n")

# functions
# ------------------------------

log() {
    local msg=$1
    printf '%b[I] %s: %s%b\n' "$COLOR_GREEN" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
}

verbose() {
    local msg=$1
    printf '%b%s%b\n' "$COLOR_GRAY" "$msg" "$COLOR_END"
}

warn() {
    local msg=$1
    printf '%b[W] %s: %s%b\n' "$COLOR_YELLOW" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
}

err() {
    local msg=$1
    >&2 printf '%b[E] %s: %s%b\n' "$COLOR_RED" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
}

fatal() {
    local msg=$1
    >&2 printf '%b[F] %s: %s%b\n' "$COLOR_RED" "${FUNCNAME[1]}" "$msg" "$COLOR_END"
    exit 1
}

utils_check_host_requirements() {
    local flock_required=$1
    local jq_required=$2

    # check required cli tooling is available on the system that executes this script
    command -v awk >/dev/null || fatal "awk is required but not found."
    command -v cat >/dev/null || fatal "cat is required but not found."
    command -v dirname >/dev/null || fatal "dirname is required but not found."
    command -v grep >/dev/null || fatal "grep is required but not found."
    command -v head >/dev/null || fatal "head is required but not found."
    command -v kubectl >/dev/null || fatal "kubectl is required but not found."
    command -v sed >/dev/null || fatal "sed is required but not found."
    command -v sort >/dev/null || fatal "sort is required but not found."
    command -v tac >/dev/null || fatal "tac is required but not found."
    command -v tr >/dev/null || fatal "tr is required but not found."
    command -v uniq >/dev/null || fatal "uniq is required but not found."
    command -v xargs >/dev/null || fatal "xargs is required but not found."

    if [ "$flock_required" == "true" ]; then
        command -v flock >/dev/null || fatal "flock is required but not found."
        command -v shuf >/dev/null || fatal "shuf is required (for flock) but not found."
    fi

    if [ "$jq_required" == "true" ]; then
        command -v jq >/dev/null || fatal "jq is required but not found."
    fi
}
