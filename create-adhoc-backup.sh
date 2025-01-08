#!/bin/bash
set -Eeo pipefail

# This script creates an adhoc backup job for the (current) namespace in kubectl context.
# $ BAK_DRY_RUN=true ./create-adhoc-backup.sh

# -------------
# env

# You can provide the following env vars to this script:
NAMESPACE="${NAMESPACE:=$( (kubectl config view --minify | grep namespace | cut -d" " -f6) || kubectl get sa -o=jsonpath='{.items[0]..metadata.namespace}' || echo "default")}"
BACKUP_JOB_WAIT="${BACKUP_JOB_WAIT:="true"}"
BACKUP_JOB_WAIT_TIMEOUT="${BACKUP_JOB_WAIT_TIMEOUT:="1h"}"

# BAK_* variables are injected as ENV into the job container, the following are the default overrides
# see https://git.allaboutapps.at/projects/AW/repos/backup-ns/browse/lib/bak_env.sh

# BAK_LABEL_VS_TYPE: adhoc (flag for adhoc backup job)
BAK_LABEL_VS_TYPE="${BAK_LABEL_VS_TYPE:="adhoc"}"
# BAK_LABEL_VS_RETAIN: days (flag for retention policy, currently only days is supported)
BAK_LABEL_VS_RETAIN="${BAK_LABEL_VS_RETAIN:="days"}"

# -------------
# main

echo "Creating adhoc backup in ns=${NAMESPACE}..."

# Check deps
command -v awk >/dev/null || fatal "awk is required but not found."
command -v yq >/dev/null || fatal "yq is required but not found."

# Collect BAK_* environment variables and construct yq commands to inject the explicit BAK_* env vars into the kubectl job definition
backup_env_vars=$(( set -o posix ; set ) | grep "BAK_" | awk -F= '{print $1}')
yq_cmd=""

while IFS= read -r bak_key; do
    bak_value=$(eval "echo \$${bak_key}")
    yq_cmd+=" | yq eval '.spec.template.spec.containers[0].env += [{\"name\": \"${bak_key}\", \"value\": \"${bak_value}\"}]' -"
done <<< "$backup_env_vars"

# create the command so we can print it right before executing it
timestamp=$(date +"%Y-%m-%d-%H%M%S")
backup_cmd="kubectl create job --from=cronjob.batch/backup \"backup-adhoc-${timestamp}\" -o yaml --dry-run=client -n \"${NAMESPACE}\" \
${yq_cmd} \
| kubectl apply -f -"

echo "Prepared backup command:"
echo "$backup_cmd"

echo "Ensuring there is no other backup job running within ns=${NAMESPACE}..."

if [ "$BACKUP_JOB_WAIT" == "true" ]; then
    sleep 3
fi

# kubectl get job -l app=backup

# ensure there is currently no other backup job running, if that is the case, exit 1
if kubectl -n "${NAMESPACE}" get job -l app=backup -o jsonpath='{.items[*].status.active}' | grep -q "1"; then
    >&2 echo "Another backup job is currently running, exit 1!"
    exit 1
fi

# Create the backup job with the overwritten env vars
echo "Creating job/backup-adhoc-${timestamp} for ns=${NAMESPACE}..."
eval "$backup_cmd"

echo "Follow logs with:"
echo "  kubectl logs -n ${NAMESPACE} -f job/backup-adhoc-${timestamp}"

if [ "$BACKUP_JOB_WAIT" == "true" ]; then

    sleep 2
    echo "Waiting for backup job/backup-adhoc-${timestamp} to complete for ns=${NAMESPACE}..."

    kubectl -n "${NAMESPACE}" wait --for=condition=complete --timeout="$BACKUP_JOB_WAIT_TIMEOUT" "job/backup-adhoc-${timestamp}"
fi

echo ""
echo "List all snapshots in this namespace via:"
echo "kubectl -n ${NAMESPACE} get vs -lbackup-ns.sh/retain -Lbackup-ns.sh/type,backup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after"

echo ""
echo "Adhoc backups are only kept for 30days by default, you can delete this auto-retention flag manually by running:"
echo "kubectl -n ${NAMESPACE} label vs/<snapshot-name> backup-ns.sh/retain- backup-ns.sh/delete-after-"
