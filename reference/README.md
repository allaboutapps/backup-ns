# backup-ns.sh

> This is the previous bash reference implementation of backup-ns that is fully replaced by the go implementation in the parent directory.
> This is mostly here for historical reasons and to document the previous implementation.

k8s application-aware snapshots.


### Development

```bash
# https://github.com/koalaman/shellcheck
# https://github.com/anordal/shellharden
brew install shellharden shellcheck
make
```


### Deployment

We require a VolumeSnapshotClass to be present in the cluster:
```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: my-default-snapshot-class
  annotations:
    # If you don't set this snapshot class as the default, you will have to specify it within the BAK_VS_CLASS_NAME env var.
    snapshot.storage.kubernetes.io/is-default-class: "true"
driver: pd.csi.storage.gke.io
parameters:
  storage-locations: europe-west3
# The retain controller automatically patches VolmeSnapshotContents that are "released" with the Delete deletionPolicy.
# However, normally you should always run with "Retain" to ensure that an (accidental) namespace delete does not delete the actual disk snapshot. 
deletionPolicy: Retain
```

`backup-ns.sh` requires the following ClusterRole as a base:
```yaml
# This ClusterRole is bound via namespace isolated RoleBindings + ServiceAccounts in each consuming namespaces.
# The main usecase of these service-accounts is allow perform streaming data (via k exec) from different pods (e.g. a SQL dump).
#
# Note: A RoleBinding can also reference a ClusterRole to grant the permissions defined in that ClusterRole to resources inside the RoleBinding's namespace.
# This kind of reference lets you define a set of common roles across your cluster, then reuse them within multiple namespaces.
# https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-example

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: a3cloud-backup
rules:
- apiGroups: [""]
  resources: ["pods", "persistentvolumeclaims"]
  verbs: ["get", "list"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["get", "create"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshots"]
  verbs: ["get", "create", "list", "watch"]
```

`backup-ns.sh` requires the following RBAC objects in each consuming namespace:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: a3cloud-backup
  namespace: <NAMESPACE>
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: a3cloud-backup
  namespace: <NAMESPACE>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: a3cloud-backup
subjects:
  - kind: ServiceAccount
    name: a3cloud-backup
    namespace: <NAMESPACE>
```

A `backup-ns.sh` cronjob can then be configured like the following (e.g. to backup a postgres database in a namespace):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-env
  namespace: <NAMESPACE>
data:
  # BAK_DRY_RUN: "true"
  # BAK_PVC_NAME: data
  BAK_LABEL_VS_TYPE: cronjob
  
  BAK_FLOCK: "true"
  # BAK_FLOCK_DIR: /mnt/host-backup-locks

  # BAK_DB_SKIP: "true" # no db in this namespace?!

  BAK_DB_POSTGRES: "true"
  # BAK_DB_POSTGRES_EXEC_RESOURCE: deployment/app-base
  # BAK_DB_POSTGRES_EXEC_CONTAINER: postgres

  # BAK_DB_MYSQL: "true"
  # BAK_DB_MYSQL_EXEC_RESOURCE: deployment/app-base # deployment/wordpress-base
  # BAK_DB_MYSQL_EXEC_CONTAINER: mariadb # mysql
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
  namespace: <NAMESPACE>
  annotations:
    a3c-validate-prefer-explicit-pod-strategy: "Recreate"
spec:
  timeZone: 'Europe/Vienna'
  schedule: "xx 0 * * *"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 2 # retry three times.
      activeDeadlineSeconds: 3600 # 1h max time before considered failed
      template:
        metadata:
          labels:
            app: backup
        spec:
          restartPolicy: Never # do not try to restart the container itself (full pod instead)

          # this container uses kubectl
          serviceAccountName: a3cloud-backup

          affinity:
            podAffinity:
              # force exe on same node as app-base pod
              requiredDuringSchedulingIgnoredDuringExecution:
              - labelSelector:
                  matchExpressions:
                  - key: app
                    operator: In
                    values: ["app-base"]
                topologyKey: kubernetes.io/hostname

          initContainers:
          - name: init-permissions
            image: busybox:1.26.2
            command: ["sh", "-c", "chmod 777 /mnt/host-backup-locks"]
            volumeMounts:
            - name: host-backup-locks
              mountPath: /mnt/host-backup-locks

          containers:
          - image: <THE_BACKUP_IMAGE>
            name: backup
            envFrom:
            - configMapRef:
                name: backup-env
            env:
            - name: BAK_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: BAK_LABEL_VS_POD
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name

            volumeMounts:
            - name: timezone
              mountPath: /etc/localtime
            - name: host-backup-locks
              mountPath: /mnt/host-backup-locks

          volumes:
          - name: timezone
            hostPath:
              path: /usr/share/zoneinfo/Europe/Vienna
          - name: host-backup-locks
            hostPath:
              path: /tmp/backup-ns-locks
              type: DirectoryOrCreate # control parallelism from multiple k8s hosts (flock)
```

You may then trigger a immediate execution via `kubectl create job --from=cronjob.batch/backup <unique-job-name>`.
