#!/bin/bash
set -Eeo pipefail

# functions
# ------------------------------

mysql_ensure_available() {
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

mysql_backup() {
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
