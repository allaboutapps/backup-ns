#!/bin/bash
set -Eeo pipefail

# Disk / Persistent Volume Claims (pvc) related checks
# https://kubernetes.io/docs/concepts/storage/persistent-volumes/
# ------------------------------

# functions
# ------------------------------

pvc_ensure_available() {
    local ns=$1
    local pvc=$2

    log "check ns='${ns}' pvc='${pvc}' exists..."

    # ensure target pvc exists
    kubectl -n ${ns} get pvc ${pvc} \
        || fatal "ns='${ns}' pvc='${pvc}' not found."
}

# Depends on awk, sed and df available within the remote container
pvc_ensure_free_space() {
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
