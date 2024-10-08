#!/bin/bash
set -Eeo pipefail

# ...

# imports
# ------------------------------

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/vs.sh"

RETAIN_DRY_RUN="${RETAIN_DRY_RUN:="false"}"
RETAIN_LAST_DAILY="${RETAIN_LAST_DAILY:="7"}"
RETAIN_LAST_WEEKLY="${RETAIN_LAST_WEEKLY:="4"}"
RETAIN_LAST_MONTHLY="${RETAIN_LAST_MONTHLY:="12"}"

# main
# ------------------------------

# print RETAIN_* env vars
( set -o posix ; set ) | grep "RETAIN_"

# we encapsulate main to allow for local variable declarations
function main() {
    utils_check_host_requirements "false" "true" # 2nd true checks jq is available

    log "starting retain, getting namespaces with 'backup-ns.sh/retain' set..."

    local fails; fails=$((0))

    # Get all namespaces with snapshots that have the "backup-ns.sh/retain" label set
    local retain_namespaces; retain_namespaces=$(kubectl get volumesnapshot --all-namespaces -lbackup-ns.sh/retain -o=jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' | uniq)

    verbose "$retain_namespaces"
    
    while IFS= read -r ns; do

        local source_disks; source_disks=$(kubectl get volumesnapshot -n "$ns" -lbackup-ns.sh/retain,backup-ns.sh/pvc -o go-template --template '{{range .items}}{{index .metadata.labels "backup-ns.sh/pvc" -}} {{"\n"}}{{end}}' | uniq)
        # verbose "$source_disks"

        while IFS= read -r source_disk; do
            
            if ! vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/daily" "$RETAIN_LAST_DAILY" "$RETAIN_DRY_RUN"; then
                err "err apply daily retention policy ns='${ns}' backup-ns.sh/pvc=${source_disk}"
                ((fails+=1))
            fi

            if ! vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/weekly" "$RETAIN_LAST_WEEKLY" "$RETAIN_DRY_RUN"; then
                err "err apply weekly retention policy ns='${ns}' backup-ns.sh/pvc=${source_disk}"
                ((fails+=1))
            fi

            if ! vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/monthly" "$RETAIN_LAST_MONTHLY" "$RETAIN_DRY_RUN"; then
                err "err apply monthly retention policy ns='${ns}' backup-ns.sh/pvc=${source_disk}"
                ((fails+=1))
            fi

            log "processed ns='${ns}' backup-ns.sh/pvc='${source_disk}'!"
            sleep 0.5

        done <<< "$source_disks"

    done <<< "$retain_namespaces"

    if [ "$fails" -gt 0 ]; then
        fatal "retain labeler failed with $fails errors."
    fi

    log "retain labeler done with $fails errors."

}

main