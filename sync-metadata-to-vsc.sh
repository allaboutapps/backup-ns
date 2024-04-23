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

    vs_sync_labels_to_vsc "allaboutapps-go-starter-dev" "data-2024-04-17-001946-s6bah4" "backup-ns.sh/"
}

main