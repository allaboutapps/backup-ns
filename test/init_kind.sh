#!/bin/bash
set -Eeox pipefail

kubectl version
kubectl get nodes
kubectl config current-context

# Exit immediately if the current kubectl context is not kind-backup-ns
if [[ $(kubectl config current-context) != "kind-backup-ns" ]]; then
  echo "Current kubectl context is not kind-backup-ns"
  exit 1
fi

echo "Installing resouces..."
# sleep 1

kubectl config set-context kind-backup-ns --namespace kube-system

cd /app/test/kube-system
kubectl kustomize . | kubectl apply -f - || true

cd /tmp
git clone --depth 1 --branch v1.15.0 https://github.com/kubernetes-csi/csi-driver-host-path.git || true
cd /tmp/csi-driver-host-path/deploy/kubernetes-1.28
./deploy.sh || true

kubectl rollout status statefulset csi-hostpathplugin -n default

# reapply, potentially from class could not be installed immediately
cd /app/test/kube-system
kubectl kustomize . | kubectl apply -f - || true

cd /app/test/postgres-test
kubectl apply -f namespace.yaml

kubectl config set-context kind-backup-ns --namespace postgres-test

kubectl apply -f ./

cd /app/test/mysql-test
kubectl apply -f namespace.yaml

kubectl config set-context kind-backup-ns --namespace mysql-test

kubectl apply -f ./

kubectl rollout status deployment database -n postgres-test
kubectl rollout status deployment database -n mysql-test

# e.g.
# BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_POSTGRES=true BAK_NAMESPACE=postgres-test BAK_DB_POSTGRES_EXEC_RESOURCE=deployment/database app create
# BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_DB_MYSQL=true BAK_NAMESPACE=mysql-test BAK_DB_MYSQL_EXEC_RESOURCE=deployment/database app create
