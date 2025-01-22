# Changelog

Note that versions before v1 may have breaking changes during minor version upgrades. We do our best to document these changes in the here.

- [Changelog](#changelog)
  - [v0.2.0: Go binary release](#v020-go-binary-release)
    - [Migration Steps for the `backup-ns.sh/weekly` label](#migration-steps-for-the-backup-nsshweekly-label)
  - [v0.1.0: Initial release](#v010-initial-release)


## v0.2.0: Go binary release

* Most of the bash scripts have been rewritten in Go (apart from mark and delete scripts): the `backup-ns` binary.
* `backup-ns` is now both useful in the k8s pod/job/cronjob context and while running locally (especially in combination with [`kubectl envx`](https://github.com/majodev/kubectl-envx)).
* GitHub Release automation as been added, including Helm chart and binary release.
* `v0.2.0` places final binary from under `/app/backup-ns` (instead of `/app/app`), any references to it within your manifests must be updated.
* All controller related subcommands are now under the `controller` subcommand. e.g. `backup-ns controller syncMetadataToVsc`.
* `v0.2.0` introduces a change in how the value of the `backup-ns.sh/weekly` label is generated. e.g. the value of the label is now `w04` instead of `YYYY-w04` (where 04 is the current week number). 

### Migration Steps for the `backup-ns.sh/weekly` label

For the retention logic to properly work, it's necessary to manually update the value of the `backup-ns.sh/weekly` label for all existing snapshots. The following steps describe how to do this:

```bash
# List all snapshots with weekly label
kubectl get vs --all-namespaces -l"backup-ns.sh/weekly" -Lbackup-ns.sh/retain,backup-ns.sh/weekly

# outputs e.g.:
# NAME                            READYTOUSE   SOURCEPVC   SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS        SNAPSHOTCONTENT                                    CREATIONTIME   AGE     RETAIN                 WEEKLY
# data-2024-12-23-001716-teefnb   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-89b2b720-0ebc-4ad1-8ad5-cf1815deff16   15d            15d     daily_weekly_monthly   2024-w52
# data-2024-12-30-002012-izgqoi   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-3aa08d3a-584e-4b8a-bb3d-1d2d4a1f48bd   8d             8d      daily_weekly_monthly   2024-w53
# data-2025-01-01-001709-sivtji   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-d3075820-6eb2-4fa5-a5f3-afee76887441   6d18h          6d18h   daily_weekly_monthly   2025-w01
# data-2025-01-06-003219-meaukv   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-08a8ebb9-3322-4b10-9461-04448e9f8ca3   42h            42h     daily_weekly_monthly   2025-w02

# Generate migration commands (review before executing)
kubectl get vs --all-namespaces -l"backup-ns.sh/weekly" -o json | \
  jq -r '.items[] | 
    select(.metadata.labels."backup-ns.sh/weekly" | test("^[0-9]{4}-w[0-9]{2}$")) | 
    "kubectl label vs -n \(.metadata.namespace) \(.metadata.name) backup-ns.sh/weekly=\(.metadata.labels."backup-ns.sh/weekly" | split("-")[1]) --overwrite"'

# outputs e.g.:
# kubectl label vs -n go-starter-dev data-2024-12-23-001716-teefnb backup-ns.sh/weekly=w52 --overwrite
# kubectl label vs -n go-starter-dev data-2024-12-30-002012-izgqoi backup-ns.sh/weekly=w53 --overwrite
# kubectl label vs -n go-starter-dev data-2025-01-01-001709-sivtji backup-ns.sh/weekly=w01 --overwrite
# kubectl label vs -n go-starter-dev data-2025-01-06-003219-meaukv backup-ns.sh/weekly=w02 --overwrite

# Review and execute the generated commands

# Finally verify changes
kubectl get vs --all-namespaces -l"backup-ns.sh/weekly" -Lbackup-ns.sh/retain,backup-ns.sh/weekly

# outputs e.g.:
# NAME                            READYTOUSE   SOURCEPVC   SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS        SNAPSHOTCONTENT                                    CREATIONTIME   AGE     RETAIN                 WEEKLY
# data-2024-12-23-001716-teefnb   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-89b2b720-0ebc-4ad1-8ad5-cf1815deff16   15d            15d     daily_weekly_monthly   w52
# data-2024-12-30-002012-izgqoi   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-3aa08d3a-584e-4b8a-bb3d-1d2d4a1f48bd   8d             8d      daily_weekly_monthly   w53
# data-2025-01-01-001709-sivtji   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-d3075820-6eb2-4fa5-a5f3-afee76887441   6d18h          6d18h   daily_weekly_monthly   w01
# data-2025-01-06-003219-meaukv   true         data                                10Gi          a3cloud-csi-gce-pd   snapcontent-08a8ebb9-3322-4b10-9461-04448e9f8ca3   42h            42h     daily_weekly_monthly   w02

# Done (see the WEEKLY column)
```

## v0.1.0: Initial release

- Initial release of the `backup-ns` scripts - mostly in bash.