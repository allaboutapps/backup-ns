# backup-ns

k8s application-aware snapshots.


Focus:
* Swiss-army knife for application-aware k8s backup and restore without the need for operators or CRDs.
* Cronjobs for backup and retention.
* Low-dependency (e.g. only `testify` for testing, cobra for cli entrypoint), only `kubectl` must be in the `PATH` and working.
* Use the clis within the already running container to do the backup (same version)

> Note that the README is WIP!

## Development setup

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
```

## TODO

Document manual labeling steps
```bash

# remove a deleteAfter labeled vs:
kubectl label vs/<vs> "backup-ns.sh/delete-after"-

# remove a specific label daily/weekly/monthly
kubectl label vs/<vs> "backup-ns.sh/daily"-
kubectl label vs/<vs> "backup-ns.sh/weekly"-
kubectl label vs/<vs> "backup-ns.sh/monthly"-

# add a specific label daily/weekly/monthly
kubectl label vs/<vs> "backup-ns.sh/daily"="YYYY-MM-DD"
kubectl label vs/<vs> "backup-ns.sh/weekly"="YYYY-w04"
kubectl label vs/<vs> "backup-ns.sh/monthly"="YYYY-MM"

# add a specific deleteAfter label:
kubectl label vs/<vs> "backup-ns.sh/delete-after"="YYYY-MM-DD"
```

**ToC**:

- [backup-ns](#backup-ns)
  - [Development setup](#development-setup)
  - [TODO](#todo)
    - [Requirements](#requirements)
    - [Quickstart](#quickstart)
    - [Visual Studio Code](#visual-studio-code)
    - [Building and testing](#building-and-testing)
    - [Uninstall](#uninstall)
  - [Maintainers](#maintainers)
  - [License](#license)

### Requirements

Requires the following local setup for development:

- [Docker CE](https://docs.docker.com/install/) (19.03 or above)
- [Docker Compose](https://docs.docker.com/compose/install/) (1.25 or above)
- [VSCode Extension: Remote - Containers](https://code.visualstudio.com/docs/remote/containers) (`ms-vscode-remote.remote-containers`)

This project makes use of the [Remote - Containers extension](https://code.visualstudio.com/docs/remote/containers) provided by [Visual Studio Code](https://code.visualstudio.com/). A local installation of the Go tool-chain is **no longer required** when using this setup.

Please refer to the [official installation guide](https://code.visualstudio.com/docs/remote/containers) how this works for your host OS and head to our [FAQ: How does our VSCode setup work?](https://github.com/allaboutapps/go-starter/wiki/FAQ#how-does-our-vscode-setup-work) if you encounter issues.

### Quickstart

Create a new git repository through the GitHub template repository feature ([use this template](https://github.com/majodev/go-docker-vscode/generate)). You will then start with a **single initial commit** in your own repository. 

```bash
# Clone your new repository, cd into it, then easily start the docker-compose dev environment through our helper
./docker-helper.sh --up
```

You should be inside the 'service' docker container with a bash shell.

```bash
development@94242c61cf2b:/app$ # inside your container...

# Shortcut for make init, make build, make info and make test
make all

# Print all available make targets
make help
```

### Visual Studio Code

> If you are new to VSCode Remote - Containers feature, see our [FAQ: How does our VSCode setup work?](https://github.com/allaboutapps/go-starter/wiki/FAQ#how-does-our-vscode-setup-work).

Run `CMD+SHIFT+P` `Go: Install/Update Tools` **after** attaching to the container with VSCode to auto-install all golang related vscode extensions.

### Building and testing

Other useful commands while developing your service:

```bash
development@94242c61cf2b:/app$ # inside your container...

# Print all available make targets
make help

# Shortcut for make init, make build, make info and make test
make all

# Init install/cache dependencies and install tools to bin
make init

# Rebuild only after changes to files
make

# Execute all tests
make test
```

Full docker build:

```bash
docker build . -t go-docker-vscode

docker run go-docker-vscode
# Hello World
```

### Uninstall

Simply run `./docker-helper --destroy` in your working directory (on your host machine) to wipe all docker related traces of this project (and its volumes!).

## Maintainers

- [Mario Ranftl - @majodev](https://github.com/majodev)

## License

[MIT](LICENSE) © Mario Ranftl