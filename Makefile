.PHONY: all
all: format lint

.PHONY: info
info:
	shellcheck --version
	shellharden --version

.PHONY: format
format:
	shellharden --replace *.sh
	shellharden --replace **/*.sh

.PHONY: lint
lint:
	shellcheck -x *.sh
	shellcheck -x **/*.sh

# normal POSIX bash shell mode
SHELL = /bin/bash
.SHELLFLAGS = -cEeuo pipefail