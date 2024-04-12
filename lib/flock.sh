#!/bin/bash
set -Eeo pipefail

# functions
# ------------------------------

# gets the filepath to a random ([1..n].lock) lock file in the given dir
flock_shuffle_lock_file() {
    local dir=$1
    local count=$2

    echo "${dir}/$(shuf -i1-${count} -n1).lock"
}

flock_lock() {
    local lock_file=$1
    local timeout=$2
    local dry_run=$3

    log "trying to obtain lock on '${lock_file}' (timeout='${timeout}' dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    exec 99>${lock_file}
    flock --timeout "${timeout}" 99
    log "Got lock on '${lock_file}'!"
}

flock_unlock() {
    local lock_file=$1 
    local dry_run=$2

    log "releasing lock from '${lock_file}' (dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    # close lock fd
    exec 99>&-
}