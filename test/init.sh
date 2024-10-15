#!/bin/bash
set -Eeox pipefail

kubectl version
kubectl get nodes

echo "Installing resouces..."
sleep 1

kubectl config set-context kind-backup-ns --namespace kube-system

cd /app/test/kube-system
kubectl kustomize . | kubectl apply -f - || true

cd /tmp
git clone https://github.com/kubernetes-csi/csi-driver-host-path.git || true
cd csi-driver-host-path/deploy/kubernetes-1.28
./deploy.sh || true

kubectl rollout status statefulset csi-hostpathplugin -n default

# reapply, potentially from class could not be installed immediately
cd /app/test/kube-system
kubectl kustomize . | kubectl apply -f - || true

cd /app/test/postgres-test
kubectl apply -f namespace.yaml

kubectl config set-context kind-backup-ns --namespace postgres-test

kubectl apply -f ./

kubectl rollout status deployment database -n postgres-test

BAK_DB_SKIP=true BAK_VS_CLASS_NAME=csi-hostpath-snapclass BAK_NAMESPACE=postgres-test app create
