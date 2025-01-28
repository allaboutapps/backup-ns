#!/bin/bash
set -Eeox pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Exit immediately if the current kubectl context is not kind-backup-ns
if [[ $(kubectl config current-context) != "kind-backup-ns" ]]; then
  echo "Current kubectl context is not kind-backup-ns"
  exit 1
fi

BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_SKIP=true BAK_NAMESPACE=generic-test backup-ns create
BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns create
BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns create

BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres dump
BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres info
BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres downloadDump -o "$SCRIPT_DIR/postgres-test.tar.gz"
rm -f "$SCRIPT_DIR/postgres-test.tar.gz"
BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres restore --force

# BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres shell

BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql dump
BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql info
BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql downloadDump -o "$SCRIPT_DIR/mysql-test.tar.gz"
rm -f "$SCRIPT_DIR/mysql-test.tar.gz"
BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql restore -f

# BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql shell

# Accessing external postgres dbs via another container "postgres-access" with psql tooling:
BAK_DB_POSTGRES_HOST=postgres.postgres-test.svc.cluster.local BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres-access BAK_DB_POSTGRES_EXEC_CONTAINER=postgres-access backup-ns postgres dump
# BAK_DB_POSTGRES_HOST=postgres.postgres-test.svc.cluster.local BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres-access BAK_DB_POSTGRES_EXEC_CONTAINER=postgres-access backup-ns postgres shell

# Accessing external mysql dbs via another container "mysql-access" with mysql tooling:
BAK_DB_MYSQL_HOST=mysql.mysql-test.svc.cluster.local BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql-access BAK_DB_MYSQL_EXEC_CONTAINER=mysql-access backup-ns mysql dump
# BAK_DB_MYSQL_HOST=mysql.mysql-test.svc.cluster.local BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql-access BAK_DB_MYSQL_EXEC_CONTAINER=mysql-access backup-ns mysql shell

backup-ns list -A