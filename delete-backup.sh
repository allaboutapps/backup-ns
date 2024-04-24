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

# Check if all required arguments are provided
if [ $# -ne 2 ]; then
    echo "Usage: $0 <namespace> <volumesnapshot>" 
    exit 1
fi

# Call the function with provided arguments
vs_delete "$1" "$2"
