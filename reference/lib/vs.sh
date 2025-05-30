#!/bin/bash
set -Eeo pipefail

# VolumeSnapshot (vs) related functions
# https://kubernetes.io/docs/concepts/storage/volume-snapshots/
# ------------------------------

# functions
# ------------------------------

vs_get_default_labels() {
    local pvc=$1 # name of the PVC that the volume snapshot is taken from
    local type=$2 # e.g. "adhoc" or "cronjob"
    local pod=$3 # might be empty string

    local labels; labels=$(
    cat <<EOF
backup-ns.sh/pvc: "${pvc}"
backup-ns.sh/type: "${type}"
EOF
)

    # dynamically set backup-ns.sh/pod label
    if [ "$pod" != "" ]; then
        labels="${labels}
backup-ns.sh/pod: \"${pod}\""
    fi

    echo "$labels"
}

# Retention related labeling. We directly flag the first daily, weekly, monthly snapshot.
# A (separate) retention worker can then use these labels to determine if a cleanup of this snapshot should happen.
#
# The following labels are used:
#    backup-ns.sh/retain: "daily_weekly_monthly"
#    backup-ns.sh/monthly: e.g. "2024-04"
#    backup-ns.sh/weekly: e.g. "w15" # ISO: Monday as first day of week, see https://unix.stackexchange.com/questions/282609/how-to-use-the-date-command-to-display-week-number-of-the-year
#    backup-ns.sh/daily: e.g. "2024-04-11"
#### #    backup-ns.sh/hourly: e.g. "2024-04-0900" # disabled for now, as it is not really useful for most use-cases
# 
# All dates use the **LOCAL TIMEZONE** of the machine executing the script!
#
# We simply try to kubectl get a prefixing snapshot with the same label and if it does not exist, we set the label on the new snapshot.
# This way we can ensure that the first snapshot of a day, week, month is always flagged.
# The assumption here is that the backup procedure is run at least once a day.
vs_get_retain_labels_daily_weekly_monthly() {
    local ns=$1

    # Note that even tough using printf for formatting dates might be a best practise (https://stackoverflow.com/questions/1401482/yyyy-mm-dd-format-date-in-shell-script)
    # we are still using date, as it is more portable (osx has no printf with date formatting support)
    # local hourly_label
    # hourly_label=$(date +"%Y-%m-%d-%H00")

    local daily_label; daily_label=$(date +"%Y-%m-%d")
    local weekly_label; weekly_label=$(date +"w%0V")
    local monthly_label; monthly_label=$(date +"%Y-%m")

    local labels=""

    read -r -d '' labels << EOF
backup-ns.sh/retain: "daily_weekly_monthly"
EOF

#     if [ "$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/hourly="$hourly_label" -o name)" == "" ]; then
#         read -r -d '' labels << EOF
# ${labels}
# backup-ns.sh/hourly: "${hourly_label}"
# EOF
#     fi

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

vs_get_retain_labels_delete_after_days() {
    local date_add=$1 # must be a numer, e.g. "30" - for 30 days

    # support for osx via eval (after gdate)
    local delete_after_label; delete_after_label=$(date -d "+${date_add} days" +"%Y-%m-%d" || eval "date -v +${date_add}d +'%Y-%m-%d'")

    local labels=""

    read -r -d '' labels << EOF
backup-ns.sh/retain: "days"
backup-ns.sh/retain-days: "${date_add}"
backup-ns.sh/delete-after: "${delete_after_label}"
EOF

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

# TODO backup-ns.sh/retain label config is not actually parsed
vs_apply_retain_policy() {

    local ns=$1
    local pvc_label=$2 # e.g. backup-ns.sh/pvc=data
    local retain_label=$3 # e.g. backup-ns.sh/daily or backup-ns.sh/weekly or backup-ns.sh/monthly
    local retain_count=$4 # e.g. 7
    local dry_run=$5

    local fails; fails=$((0))

    log "processing ns='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' retain_count='${retain_count}'..."

    # we keep the last $retain_count daily snapshots, let's get all current volumesnapshots with the label "$retain_label" sorted by latest asc
    # ensure to use .status.creationTime instead of .metadata.creationTimestamp as restored vs from dangling vsc (new preprovided vsc) are correctly sorted again!
    local sorted_snapshots; sorted_snapshots=$(kubectl -n "$ns" get volumesnapshot -l"$retain_label","$pvc_label" -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' --sort-by=.status.creationTime | tac)

    local snapshots_retained; snapshots_retained=$(echo "$sorted_snapshots" | head -n "$retain_count")

    local snapshot_kept_count; snapshot_kept_count=$(echo "$snapshots_retained" | wc -l | xargs)

    local snapshots_to_unlabel; snapshots_to_unlabel=$(sort <(echo "$sorted_snapshots" | sort) <(echo "$snapshots_retained" | sort) | uniq -u)

    verbose "ns='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - we will keep it ${snapshot_kept_count}/${retain_count}:"
    verbose "$snapshots_retained"

    if [ "$snapshots_to_unlabel" != "" ]; then

        verbose "ns='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - we will remove it from:"
        verbose "$snapshots_to_unlabel"

        while IFS= read -r vs_name; do
            warn "ns='${ns}' pvc_label='${pvc_label}' retain_label='${retain_label}' - unlabeling '${vs_name}' in ns='${ns}'..."

            local cmd; cmd="kubectl label -n $ns vs/${vs_name} ${retain_label}-"

            # dry-run mode? bail out early!
            if [ "$dry_run" == "true" ]; then
                warn "skipping - dry-run mode is active, cmd='${cmd}'"
                continue
            fi

            if ! eval "$cmd"; then
                ((fails+=1))
                err "unlabeling failed for vs_name='${vs_name}' pvc_label='${pvc_label} in ns='${ns}'."
            fi
        done <<< "$snapshots_to_unlabel"
    fi

    if [ "$fails" -gt 0 ]; then
        return 1
    fi
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
}

vs_sync_labels_to_vsc() {
    local ns=$1
    local vs_name=$2
    local search_prefix=$3 # e.g. "backup-ns.sh/"
    local dry_run=$4
    
    log "syncing labels from VolumeSnapshot to VolumeSnapshotContent for ns='${ns}' vs_name='${vs_name}' search_prefix='${search_prefix}'..."

    # Get the VolumeSnapshotContent name referenced by the VolumeSnapshot
    local vsc_name; vsc_name=$(kubectl get volumesnapshot "$vs_name" -n "$ns" -o jsonpath='{.status.boundVolumeSnapshotContentName}')

    if [ "$vsc_name" == "" ]; then
        err "volumeSnapshot vs_name='$vs_name' in ns='$ns' not found or does not have a boundVolumeSnapshotContentName."
        return 1
    fi

    # Get labels of the VolumeSnapshot, space separated key=value pairs
    local vs_labels; vs_labels=$(kubectl get volumesnapshot "$vs_name" -n "$ns" -o jsonpath='{.metadata.labels}' \
        | jq --arg search_prefix "$search_prefix" -r 'to_entries[] | select(.key | startswith($search_prefix)) | "\(.key)=\(.value)"')

    if [ "$vs_labels" == "" ]; then
        err "volumeSnapshot vs_name='$vs_name' in ns='$ns' does not have any labels we are interested in."
        return 1
    fi

    # Get labels of the VolumeSnapshotContent (if any), only the keys are needed
    local vsc_labels; vsc_labels=$(kubectl get volumesnapshotcontent "$vsc_name" -o jsonpath='{.metadata.labels}' \
        | jq --arg search_prefix "$search_prefix" -r 'to_entries[] | select(.key | startswith($search_prefix)) | "\(.key)=\(.value)"')

    local label_diff; label_diff=$(sort <(echo "$vs_labels" | sort) <(echo "$vsc_labels" | sort) | uniq -u | tr '\n' ' ' | xargs)

    if [ "$label_diff" != "" ]; then
        verbose "label_diff=${label_diff}"
    fi

    # there is a diff - we simply delete and apply all the labels again on the vsc, first get it in a comma separated format (xargs to trim whitespace)...
    local label_del; label_del=$(printf '%s\n' "${vsc_labels[@]}" | sed 's/=.*$/-/' | tr '\n' ' ' | xargs)

    local label_add; label_add=$(printf '%s\n' "${vs_labels[@]}" | tr '\n' ' ' | xargs)

    verbose "label_add=${label_add}"

    if [ "$label_diff" = "" ]; then
        warn "noop VolumeSnapshotContent vsc_name='${vsc_name}' already in sync."
        return
    fi
     
    if [ "$label_del" != "" ]; then
        verbose "label_del=${label_del}"
    fi

    if [ "$dry_run" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    if [ "$label_del" != "" ]; then
        log "deleting preexisting labels from VolumeSnapshotContent vsc_name='${vsc_name}' matchin search_prefix='${search_prefix}'..."
        if ! eval "kubectl label volumesnapshotcontent ${vsc_name} ${label_del}"; then
            err "failed to delete labels from VolumeSnapshotContent vsc_name='${vsc_name}'!"
            return 1
        fi
    fi

    # Apply labels and annotations to the VolumeSnapshotContent
    log "syncing labels from VolumeSnapshot ns='${ns}' vs_name='${vs_name}' to VolumeSnapshotContent vsc_name='${vsc_name}'..."
    if ! eval "kubectl label volumesnapshotcontent ${vsc_name} ${label_add}"; then
        err "failed to apply labels to VolumeSnapshotContent vsc_name='${vsc_name}'!"
        return 1
    fi

    # kubectl get volumesnapshot "$vs_name" -o custom-columns=NAME:.metadata.name,LABELS:.metadata.labels
    kubectl get volumesnapshotcontent "$vsc_name" -o custom-columns=NAME:.metadata.name,LABELS:.metadata.labels
}

# Dangerous!
# Delete a VolumeSnapshot, its associated VolumeSnapshotContent and the underlying storage!
# This is a destructive operation and should be used with caution!
# This function will set the deletionPolicy of the VolumeSnapshotContent to "Delete" before deleting the VolumeSnapshot, thus ensuring the underlying storage is also deleted.
vs_delete() {
    local ns=$1
    local vs_name=$2
    # local dry_run=$3

    kubectl get volumesnapshot "$vs_name" -n "$ns" --show-labels

    # Get the VolumeSnapshotContent name referenced by the VolumeSnapshot
    local vsc_name; vsc_name=$(kubectl get volumesnapshot "$vs_name" -n "$ns" -o jsonpath='{.status.boundVolumeSnapshotContentName}')

    if [ "$vsc_name" == "" ]; then
        fatal "volumeSnapshot vs_name='$vs_name' in ns='$ns' not found or does not have a boundVolumeSnapshotContentName."
    fi

    kubectl get volumesnapshotcontent "$vsc_name" -n "$ns" --show-labels

    warn "Patching vsc_name='${vsc_name}' deletionPolicy to 'Delete' before deleting VolumeSnapshot vs_name='${vs_name}' in ns='${ns}'..." 
    kubectl patch "vsc/${vsc_name}" --type='json' -p='[{"op": "replace", "path": "/spec/deletionPolicy", "value":"Delete"}]'

    warn "Deleting VolumeSnapshot vs_name='${vs_name}' in ns='${ns}'..."
    kubectl -n "$ns" delete volumesnapshot "$vs_name"

    # kubectl get volumesnapshot "$vs_name" -n "$ns" || true
    # kubectl get volumesnapshotcontent "$vsc_name" || true
}
