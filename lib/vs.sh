#!/bin/bash
set -Eeo pipefail

# VolumeSnapshot (vs) related functions
# https://kubernetes.io/docs/concepts/storage/volume-snapshots/
# ------------------------------

# functions
# ------------------------------

# Retention related labeling. We directly flag the first hourly, daily, weekly, monthly snapshot.
# A (separate) retention worker can then use these labels to determine if a cleanup of this snapshot should happen.
#
# The following labels are used:
#    backup-ns.sh/retain: "hourly_daily_weekly_monthly"
#    backup-ns.sh/hourly: e.g. "2024-04-0900"
#    backup-ns.sh/daily: e.g. "2024-04-11"
#    backup-ns.sh/weekly: e.g. "2024-w15"
#    backup-ns.sh/monthly: e.g. "2024-04"
# 
# All dates use the **LOCAL TIMEZONE** of the machine executing the script!
#
# We simply try to kubectl get a prefixing snapshot with the same label and if it does not exist, we set the label on the new snapshot.
# This way we can ensure that the first snapshot of a day, week, month is always flagged.
vs_get_retain_labels() {
    local ns=$1

    # Note that even tough using printf for formatting dates might be a best practise (https://stackoverflow.com/questions/1401482/yyyy-mm-dd-format-date-in-shell-script)
    # we are still using date, as it is more portable (osx has no printf with date formatting support)
    local hourly_label
    hourly_label=$(date +"%Y-%m-%d-%H00")

    local daily_label
    daily_label=$(date +"%Y-%m-%d")
    
    local weekly_label
    weekly_label=$(date +"%Y-w%U")
    
    local monthly_label
    monthly_label=$(date +"%Y-%m")

    local labels=""

    read -r -d '' labels << EOF
backup-ns.sh/retain: "hourly_daily_weekly_monthly"
EOF

    if [ "$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/hourly="$hourly_label" -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/hourly: "${hourly_label}"
EOF
    fi

    if [ "$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/daily="$daily_label" -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/daily: "${daily_label}"
EOF
    fi

    if [ "$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/weekly="$weekly_label" -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/weekly: "${weekly_label}"
EOF
    fi

    if [ "$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/monthly="$monthly_label" -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/monthly: "${monthly_label}"
EOF
    fi

    echo "$labels"
}


vs_template() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_class_name=$4
    local labels=$5
    local annotations=$6

    # shellcheck disable=SC2001
    cat <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
    name: "${vs_name}"
    namespace: "${ns}"
    labels:
$(echo "${labels}" | sed 's/^/        /')
    annotations:
$(echo "${annotations}" | sed 's/^/        /')
spec:
    volumeSnapshotClassName: "${vs_class_name}"
    source:
        persistentVolumeClaimName: "${pvc_name}"
EOF
}


vs_create() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_object=$4 # the serialized k8s object
    local wait_until_ready=$5
    local wait_until_ready_timeout=$6
    local dry_run=$7

    log "creating ns='${ns}' pvc_name='${pvc_name}' 'VolumeSnapshot/${vs_name}' (dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "$dry_run" == "true" ]; then

        # at least validate the vs object
        echo "$vs_object" | kubectl -n "$ns" apply --validate=true --dry-run=client -f -

        warn "skipping - dry-run mode is active!"
        return
    fi

    echo "$vs_object" | kubectl -n "$ns" apply -f -

    # wait for the snapshot to be ready...
    if [ "$wait_until_ready" == "true" ]; then
        log "waiting for ns='${ns}' 'VolumeSnapshot/${vs_name}' to be ready (timeout='${wait_until_ready_timeout}')..."
        
        # give kubectl some time to actually have a status field to wait for
        # https://github.com/kubernetes/kubectl/issues/1204
        # https://github.com/kubernetes/kubernetes/pull/109525
        sleep 5
        
        # We ignore the exit code here, as we want to continue with the script even if the wait fails.
        kubectl -n "$ns" wait --for=jsonpath='{.status.readyToUse}'=true --timeout="$wait_until_ready_timeout" volumesnapshot/"$vs_name" || true
    fi

    kubectl -n "$ns" get volumesnapshot/"$vs_name"

    # TODO: supply additional checks to ensure the snapshot is actually ready and useable?

    # TODO: we should also annotate the created VSC - but not within this script (RBAC, only require access to VS)
    # instead move that to the retention worker, which must operate on VSCs anyways.
}