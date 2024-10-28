# backup-ns

k8s application-aware snapshots.

- [backup-ns](#backup-ns)
  - [Introduction](#introduction)
  - [Process](#process)
  - [Labels](#labels)
  - [Development setup](#development-setup)
  - [Maintainers](#maintainers)
  - [License](#license)
  - [Alternative projects](#alternative-projects)


## Introduction

> Please note that this project is currently in a **alpha** state, only used internally at allaboutapps and thus is not yet ready for production use!
> Expect breaking changes, especially when it comes to the configuration and label handling.

This project extends Kubernetes CSI-based snapshots with an application-aware (also called application-consistent) creation mechanisms. It is designed to be used in a multi-tenant cluster environments where namespaces are used to separate different customer applications. 

Current focus:
* Simple cli util for backup and restore without the need for operators or custom resource definitions (CRDs).
* Using the right `mysql` and `pg_dump` version is crucial, by executing the backup in the same pod as the database container, this is always guranteed.
* Stick to the primitives. Just use cronjobs for daily backup, handle retention with labels. 
* Control backup job concurrency on the node-level via flock.
* Mark and sweep like handling, giving you time between marking and deleting.
* Low-dependency, only `kubectl` must be in the `PATH`.

## Process

> TODO add mermaid diagram

## Labels

Here are some typical labels backup-ns currently uses and how to manipulate them manually:

```bash
# remove a deleteAfter labeled vs (this prevents the backup-ns pruner from deleting the vs):
kubectl label vs/<vs> "backup-ns.sh/delete-after"-

# remove a specific label for daily/weekly/monthly retention
kubectl label vs/<vs> "backup-ns.sh/daily"-
kubectl label vs/<vs> "backup-ns.sh/weekly"-
kubectl label vs/<vs> "backup-ns.sh/monthly"-

# add a specific label daily/weekly/monthly
kubectl label vs/<vs> "backup-ns.sh/daily"="YYYY-MM-DD"
kubectl label vs/<vs> "backup-ns.sh/weekly"="YYYY-w04"
kubectl label vs/<vs> "backup-ns.sh/monthly"="YYYY-MM"

# add a specific deleteAfter label (the pruner will delete the vs after the specified date)! 
kubectl label vs/<vs> "backup-ns.sh/delete-after"="YYYY-MM-DD"
```

## Development setup

Requires the following local setup for development:

- [Docker CE](https://docs.docker.com/install/) (19.03 or above)
- [Docker Compose](https://docs.docker.com/compose/install/) (1.25 or above)
- [kind (Kubernetes in Docker)](https://kind.sigs.k8s.io/)
- [VSCode Extension: Remote - Containers](https://code.visualstudio.com/docs/remote/containers) (`ms-vscode-remote.remote-containers`)

This project makes use of the [Remote - Containers extension](https://code.visualstudio.com/docs/remote/containers) provided by [Visual Studio Code](https://code.visualstudio.com/). A local installation of the Go tool-chain is **no longer required** when using this setup.

Please refer to the [official installation guide](https://code.visualstudio.com/docs/remote/containers) how this works for your host OS and head to our [FAQ: How does our VSCode setup work?](https://github.com/allaboutapps/go-starter/wiki/FAQ#how-does-our-vscode-setup-work) if you encounter issues.

We test the functionality of the backup-ns tool against a [kind (Kubernetes in Docker)](https://kind.sigs.k8s.io/) cluster.

```bash
# Ensure you have docker (for mac) and kind installed on your **local** host.
# This project requires kind (Kubernetes in Docker) to do the testing.

# Launch a new kind cluster on your *LOCAL* host:
brew install kind
make kind-cluster-init

# the dev container is autoconfigured to use the above kind cluster
./docker-helper --up

development@f4a7ad3b5e3d:/app$ k get nodes
# NAME                      STATUS   ROLES           AGE   VERSION
# backup-ns-control-plane   Ready    control-plane   69s   v1.28.13
development@f4a7ad3b5e3d:/app$ k version
# Client Version: v1.28.14
# Kustomize Version: v5.0.4-0.20230601165947-6ce0bf390ce3
# Server Version: v1.28.13
development@f4a7ad3b5e3d:/app$ make all

# Print all available make targets
development@f4a7ad3b5e3d:/app$ make help

# Shortcut for make init, make build, make info and make test
development@f4a7ad3b5e3d:/app$ make all

# Init install/cache dependencies and install tools to bin
development@f4a7ad3b5e3d:/app$ make init

# Rebuild only after changes to files
development@f4a7ad3b5e3d:/app$ make

# Execute all tests
development@f4a7ad3b5e3d:/app$ make test

# Watch pipeline (rebuilds all after any change)
development@f4a7ad3b5e3d:/app$ make watch
```

## Maintainers

- [Mario Ranftl - @majodev](https://github.com/majodev)

## License

[MIT](LICENSE) © 2024 aaa – all about apps GmbH | Mario Ranftl and the backup-ns project contributors

## Alternative projects

* [backube/snapscheduler](https://github.com/backube/snapscheduler): Based on CSI snapshots, but using CRDs and without the option to do application consistent snapshots (no pre/post hooks).
* [k8up-io/k8up](https://github.com/k8up-io/k8up): Based on Restic, requires launching pods with direct access to the PVC to backup and custom CRDs.
* [vmware-tanzu/velero](https://github.com/vmware-tanzu/velero): Global cluster disaster recovery (difficult to target singular namespaces) and custom CRDs.