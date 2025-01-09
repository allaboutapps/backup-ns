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
BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres downloadDump -o "$SCRIPT_DIR/postgres-test.tar.gz"
rm -f "$SCRIPT_DIR/postgres-test.tar.gz"
BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres backup-ns postgres restore --force

BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql dump
BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql backup-ns mysql restore --force
