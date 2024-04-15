#!/bin/bash
set -Eeo pipefail

# This script manages application-aware backups of a single k8s namespace.
# Backups are volume snapshot based, thus must use the underlying k8s CSI driver to finally create the snapshot of a disk.

# The fine part of this script is: You can execute this script directly from your local machine 
# and also from a k8s cronjob or trigger a k8s job via a CI/CD pipeline.
# The only requirement is having kubectl access to the target namespace via a serviceaccount.

# The target namespace is determined by the current kubectl context but may be overridden by setting the BAK_NAMESPACE env var.

# You may want to test how your variables are evaluated by running the script with the following commands in dry-run mode:
# BAK_DRY_RUN=true ./backup-ns.sh
# BAK_DRY_RUN=true BAK_DB_POSTGRES=true ./backup-ns.sh
# BAK_DRY_RUN=true BAK_DB_MYSQL=true ./backup-ns.sh
# BAK_DRY_RUN=true BAK_DB_SKIP=true ./backup-ns.sh

# To test flock, you might want to limit concurrency to 1 and simply specify /tmp as the lock dir:
# BAK_DRY_RUN=true BAK_FLOCK=true BAK_FLOCK_COUNT=1 BAK_FLOCK_DIR=/tmp BAK_DB_SKIP=true ./backup-ns.sh

# imports
# ------------------------------

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

source "${SCRIPT_DIR}/lib/env.sh"
source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/flock.sh"
source "${SCRIPT_DIR}/lib/mysql.sh"
source "${SCRIPT_DIR}/lib/postgres.sh"
source "${SCRIPT_DIR}/lib/pvc.sh"
source "${SCRIPT_DIR}/lib/vs.sh"

# main
# ------------------------------

# print the parsed env config
verbose "$(env_bak_config)"

# check host requirements before starting to ensure we are not missing any required tools on the host
utils_check_host_requirements ${BAK_FLOCK}

log "starting backup in namespace='${BAK_NAMESPACE}'..."

if [ "$BAK_DEBUG" == "true" ]; then
    set -Eeox pipefail
fi

if [ "$BAK_DRY_RUN" == "true" ]; then
    warn "dry-run mode is active, write operations are skipped!"
fi

if [ "$BAK_DB_POSTGRES" == "false" ] && [ "$BAK_DB_MYSQL" == "false" ] && [ "$BAK_DB_SKIP" == "false" ]; then
    fatal "either BAK_DB_POSTGRES=true or BAK_DB_MYSQL=true or BAK_DB_SKIP=true must be set."
fi

# if we are using flock, we immediately ensure the lock on the node before proceeding with any other checks (reduce the risks of perf. hits)
if [ "$BAK_FLOCK" == "true" ]; then

    LOCK_FILE=$(flock_shuffle_lock_file \
        ${BAK_FLOCK_DIR} \
        ${BAK_FLOCK_COUNT} \
    )

    log "using lock='${LOCK_FILE}'..."

    # we trap the unlock to ensure we always release the lock
    trap "flock_unlock ${LOCK_FILE} ${BAK_DRY_RUN}" EXIT
    flock_lock ${LOCK_FILE} ${BAK_FLOCK_TIMEOUT_SEC} ${BAK_DRY_RUN}
fi

# is the PVC available?
pvc_ensure_available ${BAK_NAMESPACE} ${BAK_PVC_NAME}

# check postgresql?
if [ "$BAK_DB_POSTGRES" == "true" ]; then
    postgres_ensure_available \
        ${BAK_NAMESPACE} \
        ${BAK_DB_POSTGRES_EXEC_RESOURCE} \
        ${BAK_DB_POSTGRES_EXEC_CONTAINER} \
        ${BAK_DB_POSTGRES_DB} \
        ${BAK_DB_POSTGRES_USER} \
        ${BAK_DB_POSTGRES_PASSWORD}

    pvc_ensure_free_space \
        ${BAK_NAMESPACE} \
        ${BAK_DB_POSTGRES_EXEC_RESOURCE} \
        ${BAK_DB_POSTGRES_EXEC_CONTAINER} \
        ${BAK_DB_POSTGRES_DUMP_DIR} \
        ${BAK_THRESHOLD_SPACE_USED_PERCENTAGE}
fi

# check mysql?
if [ "$BAK_DB_MYSQL" == "true" ]; then
    mysql_ensure_available \
        ${BAK_NAMESPACE} \
        ${BAK_DB_MYSQL_EXEC_RESOURCE} \
        ${BAK_DB_MYSQL_EXEC_CONTAINER} \
        ${BAK_DB_MYSQL_HOST} \
        ${BAK_DB_MYSQL_DB} \
        ${BAK_DB_MYSQL_USER} \
        ${BAK_DB_MYSQL_PASSWORD}

    pvc_ensure_free_space \
        ${BAK_NAMESPACE} \
        ${BAK_DB_MYSQL_EXEC_RESOURCE} \
        ${BAK_DB_MYSQL_EXEC_CONTAINER} \
        ${BAK_DB_MYSQL_DUMP_DIR} \
        ${BAK_THRESHOLD_SPACE_USED_PERCENTAGE}
fi

# backup postgresql?
if [ "$BAK_DB_POSTGRES" == "true" ]; then
    postgres_backup \
        ${BAK_NAMESPACE} \
        ${BAK_DB_POSTGRES_EXEC_RESOURCE} \
        ${BAK_DB_POSTGRES_EXEC_CONTAINER} \
        ${BAK_DB_POSTGRES_DB} \
        ${BAK_DB_POSTGRES_USER} \
        ${BAK_DB_POSTGRES_PASSWORD} \
        ${BAK_DB_POSTGRES_DUMP_FILE} \
        ${BAK_DRY_RUN}
fi

# backup mysql?
if [ "$BAK_DB_MYSQL" == "true" ]; then
    mysql_backup \
        ${BAK_NAMESPACE} \
        ${BAK_DB_MYSQL_EXEC_RESOURCE} \
        ${BAK_DB_MYSQL_EXEC_CONTAINER} \
        ${BAK_DB_MYSQL_HOST} \
        ${BAK_DB_MYSQL_DB} \
        ${BAK_DB_MYSQL_USER} \
        ${BAK_DB_MYSQL_PASSWORD} \
        ${BAK_DB_MYSQL_DUMP_FILE} \
        ${BAK_DRY_RUN}

fi

# setup k8s volume snapshot labels
VS_LABELS=$(
cat <<EOF
backup-ns.sh/type: "${BAK_LABEL_VS_TYPE}"
EOF
)

# dynamically set backup-ns.sh/pod label
if [ "${BAK_LABEL_VS_POD}" != "" ]; then
    VS_LABELS="${VS_LABELS}
backup-ns.sh/pod: \"${BAK_LABEL_VS_POD}\""
fi

# dynamically set retain labels
VS_RETAIN_LABELS=$(vs_retain_labels ${BAK_NAMESPACE})
if [ "${VS_RETAIN_LABELS}" != "" ]; then
    VS_LABELS="${VS_LABELS}
$(echo "${VS_RETAIN_LABELS}")"
fi

# setup k8s volume snapshot annotations
VS_ANNOTATIONS=$(
cat <<EOF
backup-ns.sh/env-config: |-
$(env_bak_config_serialize | sed 's/^/    /')
EOF
)

# template the k8s volume snapshot object
VS_OBJECT=$(vs_template \
    ${BAK_NAMESPACE} \
    ${BAK_PVC_NAME} \
    ${BAK_VS_NAME} \
    ${BAK_VS_CLASS_NAME} \
    "${VS_LABELS}" \
    "${VS_ANNOTATIONS}" \
)

# print the to be created object
verbose "${VS_OBJECT}"

# snapshot the disk!
vs_create \
    ${BAK_NAMESPACE} \
    ${BAK_PVC_NAME} \
    ${BAK_VS_NAME} \
    "${VS_OBJECT}" \
    ${BAK_VS_WAIT_UNTIL_READY} \
    ${BAK_VS_WAIT_UNTIL_READY_TIMEOUT} \
    ${BAK_DRY_RUN}

log "finished backup in namespace='${BAK_NAMESPACE}'!"
