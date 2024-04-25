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

FAILS=$((0))

# functions
# ------------------------------

vs_apply_retain_policy() {

    local ns=$1
    local pvc_label=$2 # e.g. backup-ns.sh/pvc=data
    local retain_label=$3 # e.g. backup-ns.sh/daily
    local retain_count=$4 # e.g. 7
    local dry_run=$5

    log "processing namespace='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' retain_count='${retain_count}'..."

    # we keep the last $retain_count daily snapshots, let's get all current volumesnapshots with the label "$retain_label" sorted by latest asc
    sorted_snapshots=$(kubectl -n "$ns" get volumesnapshot -l"$retain_label","$pvc_label" -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' --sort-by=.metadata.creationTimestamp | tac)
    snapshots_retained=$(echo "$sorted_snapshots" | head -n "$retain_count")
    snapshot_kept_count=$(echo "$snapshots_retained" | wc -l | xargs)

    snapshots_to_unlabel=$(sort <(echo "$sorted_snapshots" | sort) <(echo "$snapshots_retained" | sort) | uniq -u)

    verbose "namespace='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - we will keep it ${snapshot_kept_count}/${retain_count}:"
    verbose "$snapshots_retained"

    if [ "$snapshots_to_unlabel" != "" ]; then

        log "namespace='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - we will remove it from:"
        verbose "$snapshots_to_unlabel"

        while IFS= read -r vs_name; do
            warn "namespace='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - unlabeling '${vs_name}' in ns='${ns}'..."

            cmd="kubectl label -n $ns vs/${vs_name} ${retain_label}-"

            # dry-run mode? bail out early!
            if [ "$dry_run" == "true" ]; then
                warn "skipping - dry-run mode is active, cmd='${cmd}'"
                continue
            fi

            if ! eval "$cmd"; then
                ((FAILS+=1))
                err "fail#${FAILS} unlabeling failed for vs_name='${vs_name}' in ns='${ns}'."
            fi
        done <<< "$snapshots_to_unlabel"
    fi

}

# TODO backup-ns.sh/retain label config is not actually parsed

# main
# ------------------------------

# print RETAIN_* env vars
( set -o posix ; set ) | grep "RETAIN_"

# we encapsulate main to allow for local variable declarations
function main() {
    utils_check_host_requirements "false" "true" # 2nd true checks jq is available

    log "starting retain, getting namespaces with 'backup-ns.sh/retain' set..."

    # Get all namespaces with snapshots that have the "backup-ns.sh/retain" label set
    retain_namespaces=$(kubectl get volumesnapshot --all-namespaces -lbackup-ns.sh/retain -o=jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' | uniq)

    verbose "$retain_namespaces"
    
    while IFS= read -r ns; do

        source_disks=$(kubectl get volumesnapshot -n "$ns" -lbackup-ns.sh/retain,backup-ns.sh/pvc -o go-template --template '{{range .items}}{{index .metadata.labels "backup-ns.sh/pvc" -}} {{"\n"}}{{end}}' | uniq)
        # verbose "$source_disks"

        while IFS= read -r source_disk; do

            vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/daily" "$RETAIN_LAST_DAILY" "$RETAIN_DRY_RUN"
            vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/weekly" "$RETAIN_LAST_WEEKLY" "$RETAIN_DRY_RUN"
            vs_apply_retain_policy "$ns" "backup-ns.sh/pvc=${source_disk}" "backup-ns.sh/monthly" "$RETAIN_LAST_MONTHLY" "$RETAIN_DRY_RUN"

        done <<< "$source_disks"

    done <<< "$retain_namespaces"

    if [ "$FAILS" -gt 0 ]; then
        fatal "retain labeler failed with $FAILS errors."
    fi

    log "retain labeler done with $FAILS errors."

}

main