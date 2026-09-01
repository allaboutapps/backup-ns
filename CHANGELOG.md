# Changelog

Note that versions before v1 may have breaking changes during minor version upgrades. We do our best to document these changes in the here.

- [Changelog](#changelog)
  - [v0.3.2 - 2026-09-01](#v032---2026-09-01)
    - [Security](#security)
    - [Changed](#changed)
  - [v0.3.1 - 2026-05-08](#v031---2026-05-08)
    - [Fixed](#fixed)
    - [Changed](#changed-1)
  - [v0.3.0 2025-04-22](#v030-2025-04-22)
    - [Changed](#changed-2)
    - [Added](#added)
  - [v0.2.1 - 2025-04-22](#v021---2025-04-22)
    - [Changed](#changed-3)
  - [v0.2.0: Go binary release](#v020-go-binary-release)
    - [Migration Steps for the `backup-ns.sh/weekly` label](#migration-steps-for-the-backup-nsshweekly-label)
  - [v0.1.0: Initial release](#v010-initial-release)


## v0.3.2 - 2026-09-01
### Security
* Replaced the runtime base image. `bitnamilegacy/kubectl:1.29` was last rebuilt on 2025-07-20, and Bitnami no longer publishes version tags under `bitnami/kubectl`, so that base could never receive Debian security updates again. It carried 11 critical CVEs into every deployment:
  * `CVE-2025-68121` (CVSS 10.0) and `CVE-2026-27143` (CVSS 9.8) in the Go standard library -- once each for the bundled `kubectl` (go1.22.10), an unused `yq` (go1.22.9) and the `backup-ns` binary itself (go1.23.12).
  * `CVE-2024-56171`, `CVE-2025-49794`, `CVE-2025-49796` in `libxml2`.
  * `CVE-2026-33845`, `CVE-2026-42010` in `gnutls28`.

### Changed
* The deployment image is now built on `debian:12-slim`, which installs only `jq` and `ca-certificates` -- every other tool the bash reference implementation requires is already present in the base. `libxml2` is no longer present at all.
* `kubectl` is fetched from the upstream Kubernetes release channel with a sha256 checksum verification, pinned through the `KUBECTL_VERSION` build arg (currently `v1.35.8`). Keep it within one minor of your oldest cluster control plane.
* The Go toolchain used to build the binary moved from 1.23 to 1.25.
* `yq` is no longer present in the image. It was inherited from the Bitnami base and never used at runtime; only the host-side `create-adhoc-backup.sh` helper needs it.
* The image still runs as uid 1001, matching the previous Bitnami base.

> **Note:** v0.3.1 appears below but was never published as a container image -- its git tag was not pushed, so the release workflow never ran. v0.3.2 is the first image release after v0.3.0.

## v0.3.1 - 2026-05-08
### Fixed
* Retention labels (`backup-ns.sh/daily`, `/weekly`, `/monthly`) were skipped on a new VolumeSnapshot when another PVC in the same namespace had already received that label value on the same day/week/month. The existence check in `GenerateVSLabels` now scopes the kubectl label-selector by `backup-ns.sh/pvc=<pvcName>`, so each PVC is evaluated independently. Affected setups with multiple PVCs backed up by separate cronjobs in one namespace (the second cronjob's snapshots ended up unlabeled and therefore invisible to the retention pruner).
### Changed
* Dockerfile now pulls `bitnamilegacy/kubectl` instead of `bitnami/kubectl`.

## v0.3.0 2025-04-22
### Changed
* mysqldump no longer uses `--compact`
### Added
* add `backup-ns postgres shell` for interactive postgres `psql` shell access
* add `backup-ns mysql shell` for interactive `mysql` shell access

## v0.2.1 - 2025-04-22
### Changed
* Adjust free disk space check to use posix output format to ensure proper alignment
* Refactor flock to use non-blocking syscalls to fix waiting for mutex on timeout and move flock to separate package

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