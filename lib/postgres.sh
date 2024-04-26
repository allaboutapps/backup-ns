#!/bin/bash
set -Eeo pipefail

# Application-aware volume snapshot for postgres databases: dump to disk
# ------------------------------

# functions
# ------------------------------

postgres_ensure_available() {
    local ns=$1
    local resource=$2
    local container=$3
    local pg_db=$4
    local pg_user=$5
    local pg_pass=$6

    log "check ns='${ns}' resource='${resource}' is available..."

    # print resource
    kubectl -n "$ns" get "$resource" -o wide \
        || fatal "resource='${resource}' not found."

    log "check exec /bin/bash into ns='${ns}' resource='${resource}' container='${container}' possible, tooling and pg_db='${pg_db}' is available for pg_user='${pg_user}'..."

    kubectl -n "$ns" exec -i --tty=false "$resource" -c "$container" -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', postgresql prerequisites not met!"
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

postgres_backup() {
    local ns=$1
    local resource=$2
    local container=$3
    local pg_db=$4
    local pg_user=$5
    local pg_pass=$6
    local dump_file=$7
    local dry_run=$8

    local dump_dir; dump_dir=$(dirname "$dump_file")

    log "creating dump inside ns='${ns}' resource='${resource}' container='${container}' pg_db='${pg_db}' pg_user='${pg_user}' dumpfile='${dump_file}' dry_run='${dry_run}'..."
    
    # dry-run mode? bail out early!
    if [ "$dry_run" == "true" ]; then
        warn "skipping - dry-run mode is active!"
        return
    fi
    
    # trigger postgres backup inside target container of target pod
    # TODO: ensure that pg_dump is no longer running if we kill the script (pod) that executes kubectl exec

    kubectl -n "$ns" exec -i --tty=false "$resource" -c "$container" -- /bin/bash <<- EOF || fatal "exit $? on ns='${ns}' resource='${resource}' container='${container}', postgresql dump/gzip on disk failed!"
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