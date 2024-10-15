External Attacher, Provisioner, and Resizer based on:
* https://github.com/kubernetes-csi/external-provisioner/blob/release-5.1/deploy/kubernetes/deployment.yaml
* https://github.com/kubernetes-csi/external-resizer/blob/v1.12.0/deploy/kubernetes
* https://github.com/kubernetes-csi/external-attacher/blob/release-4.7/deploy/kubernetes/deployment.yaml

Snapshot CRDs and Controller based on:
* https://github.com/kubernetes-csi/external-snapshotter/tree/release-8.1/client/config/crd
* https://github.com/kubernetes-csi/external-snapshotter/tree/release-8.1/deploy/kubernetes/snapshot-controller
* https://github.com/kubernetes-csi/external-snapshotter/tree/release-8.1/deploy/kubernetes/csi-snapshotter

Test CSI Driver with Snapshots support based on:
* `csi-hostpath-*.yaml` https://github.com/kubernetes-csi/csi-driver-host-path/tree/release-1.15/deploy/kubernetes-1.27/hostpath

```bash
ks # switch to namespace kube-system
cd /tmp
git clone https://github.com/kubernetes-csi/csi-driver-host-path.git
cd csi-driver-host-path/deploy/kubernetes-1.28
./deploy.sh

kubectl kustomize . | kubectl apply -f -
```