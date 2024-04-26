#!/bin/bash
set -Eeo pipefail

# ...

# imports
# ------------------------------

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/vs.sh"

# functions
# ------------------------------


# main
# ------------------------------

# we encapsulate main to allow for local variable declarations
function main() {
    utils_check_host_requirements "false" "true" # 2nd true checks jq is available

    # ---
    # first mark...
    # ---

    mark_query="backup-ns.sh/retain=daily_weekly_monthly,!backup-ns.sh/daily,!backup-ns.sh/weekly,!backup-ns.sh/monthly,!backup-ns.sh/delete-after"
    log "querying for volumesnapshots to mark for deletion with mark_query='${mark_query}'..."
    
    snapshots_to_mark=$(kubectl get vs --all-namespaces -l"$mark_query" -o=jsonpath='{range .items[*]}{.metadata.name} {.metadata.namespace}{"\n"}{end}')
    kubectl get vs --all-namespaces -l"$mark_query" -Lbackup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after

    local marked_date
    marked_date=$(date +"%Y-%m-%d")
    
    if [ "$snapshots_to_mark" == "" ]; then
        log "no volumesnapshots found to mark for deletion."
    else
        while IFS= read -r line; do
            local vs_name
            vs_name=$(echo "$line" | awk '{print $1}')

            local vs_namespace
            vs_namespace=$(echo "$line" | awk '{print $2}')

            log "labling vs_name='${vs_name}' in ns='${vs_namespace}' with 'backup-ns.sh/delete-after=${marked_date}'..."
            kubectl label -n "$vs_namespace" vs/"$vs_name" "backup-ns.sh/delete-after=${marked_date}"
            kubectl get vs "$vs_name" -n "$vs_namespace" -Lbackup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after

            # do not race through
            sleep 0.5

        done <<< "$snapshots_to_mark"
    fi

    # ---
    # then delete all vs that were not marked **today**
    # ---

    delete_query="backup-ns.sh/retain=daily_weekly_monthly,backup-ns.sh/delete-after,backup-ns.sh/delete-after!=${marked_date}"
    log "querying for volumesnapshots to mark for deletion with delete_query='${delete_query}'..."

    snapshots_to_delete=$(kubectl get vs --all-namespaces -l"$delete_query" -o=jsonpath='{range .items[*]}{.metadata.name} {.metadata.namespace}{"\n"}{end}')
    kubectl get vs --all-namespaces -l"$delete_query" -Lbackup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after

    if [ "$snapshots_to_delete" == "" ]; then
        log "no volumesnapshots found to delete."
    else
        while IFS= read -r line; do
            local vs_name
            vs_name=$(echo "$line" | awk '{print $1}')

            local vs_namespace
            vs_namespace=$(echo "$line" | awk '{print $2}')

            warn "deleting vs_name='${vs_name}' in ns='${vs_namespace}'..."
            kubectl get vs "$vs_name" -n "$vs_namespace" -Lbackup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after

            vs_delete "$vs_namespace" "$vs_name"

            # we are doing quite destructive operations, so lets sleep a bit until we do the new delete!
            log "deleted vs_name='${vs_name}' in ns='${vs_namespace}'!"
            sleep 5

        done <<< "$snapshots_to_delete"
    fi

    # if [ "$fails" -gt 0 ]; then
    #     fatal "marking deletion failed with $fails errors."
    # fi

    # log "syncing metadata to vsc done with $fails errors."
}

main
