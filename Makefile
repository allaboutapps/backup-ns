### -----------------------
# --- Building
### -----------------------

# first is default target when running "make" without args
.PHONY: build
build: ##- Default 'make' target: go-format, go-build and lint.
	@$(MAKE) go-format
	@$(MAKE) helm
	@$(MAKE) go-build
	@$(MAKE) lint
	@$(MAKE) reference-build

# useful to ensure that everything gets resetuped from scratch
.PHONY: all
all: init ##- Runs all of our common make targets: clean, init, build and test.
	@$(MAKE) build
	@$(MAKE) test

.PHONY: watch
watch: ##- Watches for changes and runs 'make build' on modifications.
	@echo Watching. Use Ctrl-c to exit.
	watchexec -r -w . --exts go,yaml -i *.tmp.yaml -i *.rendered.yaml $(MAKE) build

.PHONY: info
info: ##- Prints info about go.mod updates and current go version.
	@$(MAKE) get-go-outdated-modules
	@go version

.PHONY: lint
lint: ##- (opt) Runs golangci-lint.
	golangci-lint run --timeout 5m

.PHONY: go-format
go-format: ##- (opt) Runs go format.
	go fmt ./...

.PHONY: helm
helm:
	helm template ./deploy/backup-ns > ./deploy/backup-ns.tmp.yaml
	helm template ./deploy/backup-ns -n customer-namespace -f deploy/samples/customer-namespace-backup.values.yaml > ./deploy/samples/customer-namespace-backup.rendered.yaml
	helm package ./deploy/backup-ns -d ./deploy

.PHONY: go-build
go-build: ##- (opt) Runs go build.
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/app-linux-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/app-linux-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/app-darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/app-darwin-amd64

	CGO_ENABLED=0 go build -o bin/app

# https://github.com/gotestyourself/gotestsum#format 
# w/o cache https://github.com/golang/go/issues/24573 - see "go help testflag"
# note that these tests should not run verbose by default (e.g. use your IDE for this)
# TODO: add test shuffling/seeding when landed in go v1.15 (https://github.com/golang/go/issues/28592)
# tests by pkgname
.PHONY: test
test: ##- Run tests, output by package, print coverage.
	@$(MAKE) go-test-by-pkg
	@$(MAKE) go-test-print-coverage

# tests by testname
.PHONY: test-by-name
test-by-name: ##- Run tests, output by testname, print coverage.
	@$(MAKE) go-test-by-name
	@$(MAKE) go-test-print-coverage

# note that we explicitly don't want to use a -coverpkg=./... option, per pkg coverage take precedence
.PHONY: go-test-by-pkg
go-test-by-pkg: ##- (opt) Run tests, output by package.
	gotestsum --format pkgname-and-test-fails --jsonfile /tmp/test.log -- -race -cover -count=1 -coverprofile=/tmp/coverage.out ./...

.PHONY: go-test-by-name
go-test-by-name: ##- (opt) Run tests, output by testname.
	gotestsum --format testname --jsonfile /tmp/test.log -- -race -cover -count=1 -coverprofile=/tmp/coverage.out ./...

.PHONY: go-test-print-coverage
go-test-print-coverage: ##- (opt) Print overall test coverage (must be done after running tests).
	@printf "coverage "
	@go tool cover -func=/tmp/coverage.out | tail -n 1 | awk '{$$1=$$1;print}'

.PHONY: go-test-print-slowest
go-test-print-slowest: ##- Print slowest running tests (must be done after running tests).
	gotestsum tool slowest --jsonfile /tmp/test.log --threshold 2s

# TODO: switch to "-m direct" after go 1.17 hits: https://github.com/golang/go/issues/40364
.PHONY: get-go-outdated-modules
get-go-outdated-modules: ##- (opt) Prints outdated (direct) go modules (from go.mod). 
	@((go list -u -m -f '{{if and .Update (not .Indirect)}}{{.}}{{end}}' all) 2>/dev/null | grep " ") || echo "go modules are up-to-date."

.PHONY: watch-tests
watch-tests: ##- Watches *.go files and runs package tests on modifications.
	gotestsum --format testname --watch -- -race -count=1

### -----------------------
# --- Initializing
### -----------------------

.PHONY: init
init: ##- Runs make modules, tools and tidy.
	@$(MAKE) modules
	@$(MAKE) tools
	@$(MAKE) tidy

# cache go modules (locally into .pkg)
.PHONY: modules
modules: ##- (opt) Cache packages as specified in go.mod.
	go mod download

# https://marcofranssen.nl/manage-go-tools-via-go-modules/
.PHONY: tools
tools: ##- (opt) Install packages as specified in tools.go.
	@cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -P $$(nproc) -tI % go install %

.PHONY: tidy
tidy: ##- (opt) Tidy our go.sum file.
	go mod tidy

### -----------------------
# --- Binary checks
### -----------------------

# Got license issues with some dependencies? Provide a custom lichen --config
# see https://github.com/uw-labs/lichen#config 
.PHONY: get-licenses
get-licenses: ##- Prints licenses of embedded modules in the compiled bin/app.
	lichen bin/app

.PHONY: get-embedded-modules
get-embedded-modules: ##- Prints embedded modules in the compiled bin/app.
	go version -m -v bin/app

.PHONY: get-embedded-modules-count
get-embedded-modules-count: ##- (opt) Prints count of embedded modules in the compiled bin/app.
	go version -m -v bin/app | grep $$'\tdep' | wc -l

### -----------------------
# --- Helpers
### -----------------------

# https://gist.github.com/prwhite/8168133 - based on comment from @m000
.PHONY: help
help: ##- Show common make targets.
	@echo "usage: make <target>"
	@echo "note: use 'make help-all' to see all make targets."
	@echo ""
	@sed -e '/#\{2\}-/!d; s/\\$$//; s/:[^#\t]*/@/; s/#\{2\}- *//' $(MAKEFILE_LIST) | grep --invert "(opt)" | sort | column -t -s '@'

.PHONY: help-all
help-all: ##- Show all make targets.
	@echo "usage: make <target>"
	@echo "note: make targets flagged with '(opt)' are part of a main target."
	@echo ""
	@sed -e '/#\{2\}-/!d; s/\\$$//; s/:[^#\t]*/@/; s/#\{2\}- *//' $(MAKEFILE_LIST) | sort | column -t -s '@'


### -----------------------
# --- Special targets
### -----------------------

# https://unix.stackexchange.com/questions/153763/dont-stop-makeing-if-a-command-fails-but-check-exit-status
# https://www.gnu.org/software/make/manual/html_node/One-Shell.html
# required to ensure make fails if one recipe fails (even on parallel jobs) and on pipefails
.ONESHELL:

# # normal POSIX bash shell mode
# SHELL = /bin/bash
# .SHELLFLAGS = -cEeuo pipefail

# wrapped make time tracing shell, use it via MAKE_TRACE_TIME=true make <target>
SHELL = ./rksh
.SHELLFLAGS = $@


### -----------------------
# --- Old bash reference implementation
### -----------------------

.PHONY: reference-build
reference-build: reference-format reference-lint

.PHONY: reference-info
reference-info:
	@shellcheck --version
	@shellharden --version

.PHONY: reference-format
reference-format:
	cd reference
	@shellharden --replace *.sh
	@shellharden --replace **/*.sh

.PHONY: reference-lint
reference-lint:
	cd reference
	@shellcheck -x *.sh
	@shellcheck -x **/*.sh
	@shellharden --check *.sh
	@shellharden --check **/*.sh

# kind, these steps are meant to run locally on your machine directly!
# 
# https://johnharris.io/2019/04/kubernetes-in-docker-kind-of-a-big-deal/

.PHONY: clean cluster compose context deploy

.PHONY: kind-cluster-clean
kind-cluster-clean:
	kind delete cluster --name backup-ns
	rm -rf .kube/**

# https://hub.docker.com/r/kindest/node/tags
.PHONY: kind-cluster-init
kind-cluster-init:
	kind create cluster --name backup-ns --config=kind.yaml --kubeconfig .kube/config --image "kindest/node:v1.28.13"
	$(MAKE) kind-fix-kubeconfig
	sleep 1
	$(MAKE) kind-cluster-init-script

.PHONY: kind-fix-kubeconfig
kind-fix-kubeconfig:
	sed -i.bak -e 's/127.0.0.1/host.docker.internal/' .kube/config

.PHONY: kind-cluster-init-script
kind-cluster-init-script:
	docker-compose up --no-start
	docker-compose start
	docker-compose exec service bash test/init_kind.sh

.PHONY: kind-cluster-reset
kind-cluster-reset:
	$(MAKE) kind-cluster-clean
	$(MAKE) kind-cluster-init

