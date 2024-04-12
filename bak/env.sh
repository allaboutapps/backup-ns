#!/bin/bash
set -Eeo pipefail

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
BAK_START_DATE=$(date +"%Y-%m-%d-%H%M%S")

# BAK_VS_RAND: a random string to make the volume snapshot name unique (apart from the timestamp), fallback to nanoseconds
BAK_VS_RAND="${BAK_VS_RAND:=$((shuf -er -n6 {a..z} {0..9} | tr -d '\n') || date +"%6N")}"

# BAK_LABEL_VS_TYPE: "type" label value of volume snapshot (e.g. "adhoc" or custom backups, "scheduled" for recurring, etc.)
BAK_LABEL_VS_TYPE="${BAK_LABEL_VS_TYPE:=$(echo "adhoc")}"

# BAK_LABEL_VS_POD: "pod" label value of volume snapshot (this is used to identify the backup job that created the snapshot)
BAK_LABEL_VS_POD="${BAK_LABEL_VS_POD:=$(echo "")}"

# BAK_VS_NAME: the name of the volume snapshot
BAK_VS_NAME="${BAK_VS_NAME:=$(echo "${BAK_PVC_NAME}-${BAK_START_DATE}-${BAK_VS_RAND}")}"

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


# functions
# ------------------------------

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
