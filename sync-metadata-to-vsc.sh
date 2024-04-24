#!/bin/bash
set -Eeo pipefail

# ...

# imports
# ------------------------------

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/vs.sh"

# main
# ------------------------------

# we encapsulate main to allow for local variable declarations
function main() {
    utils_check_host_requirements "false" "true" # 2nd true checks jq is available

    log "starting sync vs metadata to vsc matching label 'backup-ns.sh/type'"

    # Get all VolumeSnapshots that are ready and have the label key "backup-ns.sh/type"
    ready_snapshots=$(kubectl get volumesnapshot -lbackup-ns.sh/type -o=jsonpath='{range .items[*]}{.metadata.name} {.metadata.namespace}{"\n"}{end}')

    # vs_sync_labels_to_vsc "allaboutapps-go-starter-dev" "data-2024-04-17-001946-s6bah4" "backup-ns.sh/"

    while IFS= read -r line; do
        local vs_name
        vs_name=$(echo "$line" | awk '{print $1}')
        
        local vs_namespace
        vs_namespace=$(echo "$line" | awk '{print $2}')
        
        vs_sync_labels_to_vsc "$vs_namespace" "$vs_name" "backup-ns.sh/"

    done <<< "$ready_snapshots"

}

main