#!/bin/bash
set -Eeox pipefail

# Exit immediately if the current kubectl context is not kind-backup-ns
if [[ $(kubectl config current-context) != "kind-backup-ns" ]]; then
  echo "Current kubectl context is not kind-backup-ns"
  exit 1
fi

BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_SKIP=true BAK_NAMESPACE=generic-test app create
BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/postgres app create
BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/mysql app create