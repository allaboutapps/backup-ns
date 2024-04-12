#!/bin/bash
set -Eeo pipefail

# functions
# ------------------------------

ensure_pvc_available() {
    local ns=$1
    local pvc=$2

    log "check ns='${ns}' pvc='${pvc}' exists..."

    # ensure target pvc exists
    kubectl -n ${ns} get pvc ${pvc} \
        || fatal "ns='${ns}' pvc='${pvc}' not found."
}

# Depends on awk, sed and df.
ensure_free_space() {
    local ns=$1
    local resource=$2
    local container=$3
    local dir=$4
    local threshold=$5 # threshold in percent of used space

    log "check free space on ns='${ns}' resource='${resource}' container='${container}' dir='${dir}' threshold='${threshold}'..."

    kubectl -n ${ns} exec -i --tty=false ${resource} -c ${container} -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}' dir='${dir}'."
        set -Eeo pipefail

        # check clis are available
        command -v awk >/dev/null
        command -v sed >/dev/null
        command -v df >/dev/null

        df -h ${dir}

        # compute used space in percent
        used_percent=\$(df -h '${dir}' | awk 'NR==2 { print \$5 }' | sed 's/%//')
        
        exit_code=\$(echo \$((\${used_percent}>=${threshold})))
        echo "exit \${exit_code} as used_percent=\${used_percent}% and threshold=${threshold}%"
        exit \${exit_code}
EOF
}


volume_snapshot_template() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_class_name=$4
    local labels=$5
    local annotations=$6

    # BAK_* env vars are serialized into the annotation for later reference
    cat <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
    name: "${vs_name}"
    namespace: "${ns}"
$(echo "${labels}" | sed 's/^/    /')
$(echo "${annotations}" | sed 's/^/    /')
spec:
    volumeSnapshotClassName: "${vs_class_name}"
    source:
        persistentVolumeClaimName: "${pvc_name}"
EOF
}

snapshot_disk() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_object=$4 # the serialized k8s object
    local wait_until_ready=$5
    local wait_until_ready_timeout=$6
    local dry_run=$7

    log "creating ns='${ns}' pvc_name='${pvc_name}' 'VolumeSnapshot/${vs_name}' (dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    echo "${vs_object}" | kubectl -n ${ns} apply -f -

    # wait for the snapshot to be ready...
    if [ "${wait_until_ready}" == "true" ]; then
        log "waiting for ns='${ns}' 'VolumeSnapshot/${vs_name}' to be ready (timeout='${wait_until_ready_timeout}')..."
        
        # give kubectl some time to actually have a status field to wait for
        # https://github.com/kubernetes/kubectl/issues/1204
        # https://github.com/kubernetes/kubernetes/pull/109525
        sleep 5
        
        # We ignore the exit code here, as we want to continue with the script even if the wait fails.
        kubectl -n ${ns} wait --for=jsonpath='{.status.readyToUse}'=true --timeout=${wait_until_ready_timeout} volumesnapshot/${vs_name} || true
    fi

    kubectl -n ${ns} get volumesnapshot/${vs_name}

    # TODO: supply additional checks to ensure the snapshot is actually ready and useable

    # TODO: we should also annotate the created VSC - but not within this script (RBAC, only require access to VS)
    # instead move that to the retention worker, which must operate on VSCs anyways.
}