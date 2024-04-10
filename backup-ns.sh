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


# env globals and defaults
# ------------------------------

# BAK_DRY_RUN: if true, no actual dump/backup is performed, just a dry run to check if everything is in place (still exec into the target container)
BAK_DRY_RUN="${BAK_DRY_RUN:=$(echo "false")}"

# BAK_DEBUG: if true, the script will print out every command before executing it
BAK_DEBUG="${BAK_DEBUG:=$(echo "false")}"

# BAK_NAMESPACE: the target namespace to backup
BAK_NAMESPACE="${BAK_NAMESPACE:=$((kubectl config view --minify | grep namespace | cut -d" " -f6) || echo "default")}"

# BAK_PVC_NAME: the name of the PVC to backup
BAK_PVC_NAME="${BAK_PVC_NAME:=$(echo "data")}"

# BAK_START_DATE: the start date of this script run
BAK_START_DATE=$(date -u +"%Y-%m-%d-%H%M%S")

# BAK_VS_RAND: a random string to make the volume snapshot name unique (apart from the timestamp), fallback to nanoseconds
BAK_VS_RAND="${BAK_VS_RAND:=$((shuf -er -n6 {a..z} {0..9} | tr -d '\n') || date -u +"%6N")}"

# BAK_VS_TYPE: "type" label value of volume snapshot (e.g. "adhoc" or custom backups, "scheduled" for recurring, etc.), also used for suffix of the volume snapshot name
BAK_VS_TYPE="${BAK_VS_TYPE:=$(echo "adhoc")}"

# BAK_VS_NAME: the name of the volume snapshot
BAK_VS_NAME="${BAK_VS_NAME:=$(echo "${BAK_PVC_NAME}-${BAK_START_DATE}-${BAK_VS_RAND}-${BAK_VS_TYPE}")}"

# BAK_VS_CLASS_NAME: the name of the volume snapshot class to use
BAK_VS_CLASS_NAME="${BAK_VS_CLASS_NAME:=$(echo "a3cloud-csi-gce-pd")}"

# BAK_VS_WAIT_UNTIL_READY: if true, the script will wait until the snapshot is actually ready (useable)
BAK_VS_WAIT_UNTIL_READY="${BAK_VS_WAIT_UNTIL_READY:=$(echo "true")}"

# BAK_VS_WAIT_UNTIL_READY_TIMEOUT: the timeout to wait for the snapshot to be ready (as go formatted duration spec)
BAK_VS_WAIT_UNTIL_READY_TIMEOUT="${BAK_VS_WAIT_UNTIL_READY_TIMEOUT:=$(echo "15m")}"

# BAK_THRESHOLD_SPACE_USED_PERCENTAGE: the max allowed used space of the disk mounted at the dump dir before the backup fails
BAK_THRESHOLD_SPACE_USED_PERCENTAGE="${BAK_THRESHOLD_SPACE_USED_PERCENTAGE:=$(echo "90")}"

# BAK_DB_*: application-aware backup settings
# Note that BAK_DB_* env vars are also serialized as VS annotation, to recreate the backup ENV later on within a restore job.

# BAK_DB_SKIP: if true, no application-aware backup is performed (no db - useful for testing the snapshot creation only)
BAK_DB_SKIP="${BAK_DB_SKIP:=$(echo "false")}"
if [ "$BAK_DB_SKIP" == "true" ]; then
    # Attention: we force BAK_DB_POSTGRES and BAK_DB_MYSQL to be "false" in this case!
    BAK_DB_POSTGRES="false"
    BAK_DB_MYSQL="false"
fi

# BAK_DB_POSTGRES: if true, a postgresql dump is created before the snapshot
BAK_DB_POSTGRES="${BAK_DB_POSTGRES:=$(echo "false")}"
if [ "$BAK_DB_POSTGRES" == "true" ]; then

    # BAK_DB_POSTGRES_EXEC_RESOURCE: the k8s resource to exec into to create the dump
    BAK_DB_POSTGRES_EXEC_RESOURCE="${BAK_DB_POSTGRES_EXEC_RESOURCE:=$(echo "deployment/app-base")}"

    # BAK_DB_POSTGRES_EXEC_CONTAINER: the container inside the above resource to exec into to create the dump
    BAK_DB_POSTGRES_EXEC_CONTAINER="${BAK_DB_POSTGRES_EXEC_CONTAINER:=$(echo "postgres")}"

    # BAK_DB_POSTGRES_DUMP_DIR: the directory inside the container to store the dump
    BAK_DB_POSTGRES_DUMP_DIR="${BAK_DB_POSTGRES_DUMP_DIR:=$(echo "/var/lib/postgresql/data")}"

    # BAK_DB_POSTGRES_DUMP_FILE: the file inside the container to store the dump
    BAK_DB_POSTGRES_DUMP_FILE="${BAK_DB_POSTGRES_DUMP_FILE:=$(echo "${BAK_DB_POSTGRES_DUMP_DIR}/dump.sql.gz")}"

    # BAK_DB_POSTGRES_USER: the postgresql user to use for connecting/creating the dump (psql and pg_dump must be allowed)
    BAK_DB_POSTGRES_USER="${BAK_DB_POSTGRES_USER:=$(echo "\${POSTGRES_USER}")}" # defaults to env var within the target container

    # BAK_DB_POSTGRES_PASSWORD: the postgresql password to use for connecting/creating the dump
    BAK_DB_POSTGRES_PASSWORD="${BAK_DB_POSTGRES_PASSWORD:=$(echo "\${POSTGRES_PASSWORD}")}" # defaults to env var within the target container

    # BAK_DB_POSTGRES_DB: the postgresql database to use for connecting/creating the dump
    BAK_DB_POSTGRES_DB="${BAK_DB_POSTGRES_DB:=$(echo "\${POSTGRES_DB}")}" # defaults to env var within the target container
fi

# BAK_DB_MYSQL: if true, a mysql dump is created before the snapshot
BAK_DB_MYSQL="${BAK_DB_MYSQL:=$(echo "false")}"
if [ "$BAK_DB_MYSQL" == "true" ]; then

    # BAK_DB_MYSQL_EXEC_RESOURCE: the k8s resource to exec into to create the dump
    BAK_DB_MYSQL_EXEC_RESOURCE="${BAK_DB_MYSQL_EXEC_RESOURCE:=$(echo "deployment/app-base")}"

    # BAK_DB_MYSQL_EXEC_CONTAINER: the container inside the above resource to exec into to create the dump
    BAK_DB_MYSQL_EXEC_CONTAINER="${BAK_DB_MYSQL_EXEC_CONTAINER:=$(echo "mysql")}"

    # BAK_DB_MYSQL_DUMP_DIR: the directory inside the container to store the dump
    BAK_DB_MYSQL_DUMP_DIR="${BAK_DB_MYSQL_DUMP_DIR:=$(echo "/var/lib/mysql")}"

    # BAK_DB_MYSQL_DUMP_FILE: the file inside the container to store the dump
    BAK_DB_MYSQL_DUMP_FILE="${BAK_DB_MYSQL_DUMP_FILE:=$(echo "${BAK_DB_MYSQL_DUMP_DIR}/dump.sql.gz")}"

    # BAK_DB_MYSQL_HOST: the mysql host to use for connecting/creating the dump
    BAK_DB_MYSQL_HOST="${BAK_DB_MYSQL_HOST:=$(echo "127.0.0.1")}"

    # BAK_DB_MYSQL_USER: the mysql user to use for connecting/creating the dump
    BAK_DB_MYSQL_USER="${BAK_DB_MYSQL_USER:=$(echo "root")}"

    # BAK_DB_MYSQL_PASSWORD: the mysql password to use for connecting/creating the dump
    BAK_DB_MYSQL_PASSWORD="${BAK_DB_MYSQL_PASSWORD:=$(echo "\${MYSQL_ROOT_PASSWORD}")}" # defaults to env var within the target container

    # BAK_DB_MYSQL_DB: the mysql database to use for connecting/creating the dump
    BAK_DB_MYSQL_DB="${BAK_DB_MYSQL_DB:=$(echo "\${MYSQL_DATABASE}")}" # defaults to env var within the target container
fi

# BAK_FLOCK: if true, flock is used to coordinate concurrent backup script execution, e.g. controlling per k8s node backup script concurrency
BAK_FLOCK="${BAK_FLOCK:=$(echo "false")}"
if [ "$BAK_FLOCK" == "true" ]; then

    # BAK_FLOCK_COUNT: the number of concurrent backup scripts allowed to run
    BAK_FLOCK_COUNT="${BAK_FLOCK_COUNT:=$(NPROC=$(nproc --all) && awk -v nproc="$NPROC" 'BEGIN {if (nproc < 2) print 1; else print int(nproc / 2)}' || echo "2")}"

    # BAK_FLOCK_DIR: the dir in which we will create file locks to coordinate multiple running backup-ns.sh jobs
    #   Lock files like 1.lock, 2.lock, 3.lock will be created depending on the above count, jobs shuffle based on COUNT and select one of these files.
    #   If you use this to coordinate backup jobs on k8s node, ensure to use a hostDir volume DirectoryOrCreate mount.
    BAK_FLOCK_DIR="${BAK_FLOCK_DIR:=$(echo "/mnt/host-backup-locks")}"

    # BAK_FLOCK_TIMEOUT_SEC: the timeout in seconds to wait for the flock lock until we exit 1
    BAK_FLOCK_TIMEOUT_SEC="${BAK_FLOCK_TIMEOUT_SEC:=$(echo "3600")}" # default 1h
fi

# BAK_COLORS_ENABLED: if true, colored output is enabled
BAK_COLORS_ENABLED="${BAK_COLORS_ENABLED:=$((which tput > /dev/null 2>&1 && [[ $(tput -T$TERM colors) -ge 8 ]] && echo "true") || echo "false")}" 
COLOR_RED=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;31m" || echo "")
COLOR_GREEN=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;32m" || echo "")
COLOR_YELLOW=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;33m" || echo "")
COLOR_GRAY=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0;90m" || echo "")
COLOR_END=$([ "$BAK_COLORS_ENABLED" == "true" ] && echo "\033[0m" || echo "")


# functions
# ------------------------------

log() {
    local msg=$1
    echo -e "${COLOR_GREEN}[I] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
}

verbose() {
    local msg=$1
    echo -e "${COLOR_GRAY}${msg}${COLOR_END}"
}

warn() {
    local msg=$1
    echo -e "${COLOR_YELLOW}[W] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
}

fatal() {
    local msg=$1
    >&2 echo -e "${COLOR_RED}[E] ${FUNCNAME[1]}: ${msg}${COLOR_END}"
    exit 1
}

check_host_requirements() {
    local flock_required=$1

    # check required cli tooling is available on the system that executes this script
    command -v cat >/dev/null || fatal "cat is required but not found."
    command -v sed >/dev/null || fatal "sed is required but not found."
    command -v awk >/dev/null || fatal "awk is required but not found."
    command -v grep >/dev/null || fatal "grep is required but not found."
    command -v dirname >/dev/null || fatal "dirname is required but not found."
    command -v kubectl >/dev/null || fatal "kubectl is required but not found."

    if [ "${flock_required}" == "true" ]; then
        command -v flock >/dev/null || fatal "flock is required but not found."
        command -v shuf >/dev/null || fatal "shuf is required (for flock) but not found."
    fi
}

get_bak_env_config() {
    # log set globals by prefix
    # automatically strip out *_PASSWORD vars for security reasons 
    ( set -o posix ; set ) | grep "BAK_" | grep -v "_PASSWORD"
}

serialize_bak_env_config() {
    # serialize truthy BAK_DB_* vars
    # this is used to store the env config as vs annotation
    get_bak_env_config | grep "BAK_DB_" | grep -v "=false"
}

ensure_pvc_available() {
    local ns=$1
    local pvc=$2

    log "check ns='${ns}' pvc='${pvc}' exists..."

    # ensure target pvc exists
    kubectl -n ${ns} get pvc ${pvc} \
        || fatal "ns='${ns}' pvc='${pvc}' not found."
}

# gets the filepath to a random ([1..n].lock) lock file in the given dir
flock_shuffle_lock_file() {
    local dir=$1
    local count=$2

    echo "${dir}/$(shuf -i1-${count} -n1).lock"
}

flock_lock() {
    local lock_file=$1
    local timeout=$2
    local dry_run=$3

    log "trying to obtain lock on '${lock_file}' (timeout='${timeout}' dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    exec 99>${lock_file}
    flock --timeout "${timeout}" 99
    log "Got lock on '${lock_file}'!"
}

flock_unlock() {
    local lock_file=$1 
    local dry_run=$2

    log "releasing lock from '${lock_file}' (dry_run='${dry_run}')..."

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    # close lock fd
    exec 99>&-
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

ensure_postgres_available() {
    local ns=$1
    local resource=$2
    local container=$3
    local pg_db=$4
    local pg_user=$5
    local pg_pass=$6

    log "check ns='${ns}' resource='${resource}' is available..."

    # print resource
    kubectl -n ${ns} get ${resource} -o wide \
        || fatal "resource='${resource}' not found."

    log "check exec /bin/bash into ns='${ns}' resource='${resource}' container='${container}' possible, tooling and pg_db='${pg_db}' is available for pg_user='${pg_user}'..."

    kubectl -n ${ns} exec -i --tty=false ${resource} -c ${container} -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', postgresql prerequisites not met!"
        # inject default PGPASSWORD into current env (before cmds are visible in logs)
        export PGPASSWORD=${pg_pass}
        
        set -Eeox pipefail

        # check clis are available
        command -v gzip
        psql --version
        pg_dump --version

        # check db is accessible
        psql --username=${pg_user} ${pg_db} -c "SELECT 1;" >/dev/null
EOF
}

backup_postgres() {
    local ns=$1
    local resource=$2
    local container=$3
    local pg_db=$4
    local pg_user=$5
    local pg_pass=$6
    local dump_file=$7
    local dry_run=$8

    local dump_dir=$(dirname $dump_file)

    log "creating dump inside ns='${ns}' resource='${resource}' container='${container}' pg_db='${pg_db}' pg_user='${pg_user}' dumpfile='${dump_file}' dry_run='${dry_run}'..."
    
    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi
    
    # trigger postgres backup inside target container of target pod
    # TODO: ensure that pg_dump is no longer running if we kill the script (pod) that executes kubectl exec
    
    kubectl -n ${ns} exec -i --tty=false ${resource} -c ${container} -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', postgresql dump/gzip on disk failed!"
        # inject default PGPASSWORD into current env (before cmds are visible in logs)
        export PGPASSWORD=${pg_pass}

        set -Eeox pipefail
        
        # setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
        trap 'exit_code=\$?; [ \$exit_code -ne 0 ] \
            && echo "TRAP!" \
            && rm -f ${dump_file} \
            && df -h ${dump_dir}; \
            exit \$exit_code' EXIT

        # create dump and pipe to gzip archive
        pg_dump --username=${pg_user} --format=p --clean --if-exists ${pg_db} \
            | gzip -c > ${dump_file}

        # print dump file info
        ls -lha ${dump_file}

        # ensure generated file is bigger than 0 bytes
        [ -s ${dump_file} ] || exit 1

        # print mounted disk space
        df -h ${dump_dir}
EOF
}

ensure_mysql_available() {
    local ns=$1
    local resource=$2
    local container=$3
    local mysql_host=$4
    local mysql_db=$5
    local mysql_user=$6
    local mysql_pass=$7

    log "check ns='${ns}' resource='${resource}' is available..."

    # print resource
    kubectl -n ${ns} get ${resource} -o wide \
        || fatal "resource='${resource}' not found."

    log "check exec /bin/bash into ns='${ns}' resource='${resource}' container='${container}' possible, tooling and mysql_host='${mysql_host}' mysql_db='${mysql_db}' is available for mysql_user='${mysql_user}'..."

    # Do not use "which" in clis, unsupported in mysql:8.x images -> command -v is supported
    kubectl -n ${ns} exec -i --tty=false ${resource} -c ${container} -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', mysql prerequisites not met!"
        # inject default MYSQL_PWD into current env (before cmds are visible in logs)
        export MYSQL_PWD=${mysql_pass}

        set -Eeox pipefail
        
        # check clis are available
        command -v gzip
        mysql --version
        mysqldump --version

        # check db is accessible (default password injected via above MYSQL_PWD)
        mysql \
            --host ${mysql_host} \
            --user ${mysql_user} \
            --default-character-set=utf8 \
            ${mysql_db} \
            -e "SELECT 1;" >/dev/null
EOF
}

backup_mysql() {
    local ns=$1
    local resource=$2
    local container=$3
    local mysql_host=$4
    local mysql_db=$5
    local mysql_user=$6
    local mysql_pass=$7
    local dump_file=$8
    local dry_run=$9

    local dump_dir=$(dirname $dump_file)

    log "creating dump inside ns='${ns}' resource='${resource}' container='${container}' mysql_host='${mysql_host}' mysql_db='${mysql_db}' mysql_user='${mysql_user}' dumpfile='${dump_file}' dry_run='${dry_run}'..."
    
    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    # trigger mysql backup inside target container of target pod 
    kubectl -n ${ns} exec -i --tty=false ${resource} -c ${container} -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', mysqldump/gzip on disk failed!"
        # inject default MYSQL_PWD into current env (before cmds are visible in logs)
        export MYSQL_PWD=${mysql_pass}

        set -Eeox pipefail

        # setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
        trap 'exit_code=\$?; [ \$exit_code -ne 0 ] \
            && echo "TRAP!" \
            && rm -f ${dump_file} \
            && df -h ${dump_dir}; \
            exit \$exit_code' EXIT

        # create dump and pipe to gzip archive (default password injected via above MYSQL_PWD)
        mysqldump \
            --host ${mysql_host} \
            --user ${mysql_user} \
            --default-character-set=utf8 \
            --add-locks \
            --set-charset \
            --compact \
            --create-options \
            --add-drop-table \
            --lock-tables \
            ${mysql_db} \
            | gzip -c > ${dump_file}

        # print dump file info
        ls -lha ${dump_file}

        # ensure generated file is bigger than 0 bytes
        [ -s ${dump_file} ] || exit 1

        # print mounted disk space
        df -h ${dump_dir}
EOF
}

snapshot_disk_template() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_class_name=$4
    local vs_type=$5

    # BAK_* env vars are serialized into the annotation for later reference
    cat <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
    name: ${vs_name}
    namespace: ${ns}
    labels:
        backup-ns.sh/type: ${vs_type}
    annotations:
        backup-ns.sh/env-config: |-
$(serialize_bak_env_config | sed 's/^/            /')
spec:
    volumeSnapshotClassName: ${vs_class_name}
    source:
        persistentVolumeClaimName: ${pvc_name}
EOF
}

snapshot_disk() {
    local ns=$1
    local pvc_name=$2
    local vs_name=$3
    local vs_class_name=$4
    local vs_type=$5
    local wait_until_ready=$6
    local wait_until_ready_timeout=$7
    local dry_run=$8
    
    local k8s_snapshot=$(snapshot_disk_template \
        ${ns} \
        ${pvc_name} \
        ${vs_name} \
        ${vs_class_name} \
        ${vs_type} \
    )

    log "creating ns='${ns}' pvc_name='${pvc_name}' 'VolumeSnapshot/${vs_name}' (vs_class_name='${vs_class_name}' vs_type='${vs_type}' dry_run='${dry_run}')..."

    verbose "${k8s_snapshot}"

    # dry-run mode? bail out early!
    if [ "${dry_run}" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi

    echo "${k8s_snapshot}" | kubectl -n ${ns} apply -f -

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


# main
# ------------------------------

# print the parsed env config
verbose "$(get_bak_env_config)"

# check host requirements before starting to ensure we are not missing any required tools on the host
check_host_requirements ${BAK_FLOCK}

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
ensure_pvc_available ${BAK_NAMESPACE} ${BAK_PVC_NAME}

# check postgresql?
if [ "$BAK_DB_POSTGRES" == "true" ]; then
    ensure_postgres_available \
        ${BAK_NAMESPACE} \
        ${BAK_DB_POSTGRES_EXEC_RESOURCE} \
        ${BAK_DB_POSTGRES_EXEC_CONTAINER} \
        ${BAK_DB_POSTGRES_DB} \
        ${BAK_DB_POSTGRES_USER} \
        ${BAK_DB_POSTGRES_PASSWORD}

    ensure_free_space \
        ${BAK_NAMESPACE} \
        ${BAK_DB_POSTGRES_EXEC_RESOURCE} \
        ${BAK_DB_POSTGRES_EXEC_CONTAINER} \
        ${BAK_DB_POSTGRES_DUMP_DIR} \
        ${BAK_THRESHOLD_SPACE_USED_PERCENTAGE}
fi

# check mysql?
if [ "$BAK_DB_MYSQL" == "true" ]; then
    ensure_mysql_available \
        ${BAK_NAMESPACE} \
        ${BAK_DB_MYSQL_EXEC_RESOURCE} \
        ${BAK_DB_MYSQL_EXEC_CONTAINER} \
        ${BAK_DB_MYSQL_HOST} \
        ${BAK_DB_MYSQL_DB} \
        ${BAK_DB_MYSQL_USER} \
        ${BAK_DB_MYSQL_PASSWORD}

    ensure_free_space \
        ${BAK_NAMESPACE} \
        ${BAK_DB_MYSQL_EXEC_RESOURCE} \
        ${BAK_DB_MYSQL_EXEC_CONTAINER} \
        ${BAK_DB_MYSQL_DUMP_DIR} \
        ${BAK_THRESHOLD_SPACE_USED_PERCENTAGE}
fi

# backup postgresql?
if [ "$BAK_DB_POSTGRES" == "true" ]; then
    backup_postgres \
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
    backup_mysql \
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

# snapshot the disk!
snapshot_disk \
    ${BAK_NAMESPACE} \
    ${BAK_PVC_NAME} \
    ${BAK_VS_NAME} \
    ${BAK_VS_CLASS_NAME} \
    ${BAK_VS_TYPE} \
    ${BAK_VS_WAIT_UNTIL_READY} \
    ${BAK_VS_WAIT_UNTIL_READY_TIMEOUT} \
    ${BAK_DRY_RUN}

log "finished backup in namespace='${BAK_NAMESPACE}'!"
