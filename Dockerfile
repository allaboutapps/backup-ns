# our actual base image
FROM debian:12 as base

RUN apt-get update \
    && apt-get install -y \
    curl \
    make \
    shellcheck \
    # apt cleanup
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN set -x; ARCH="$(uname -m)" \
    && SHELLHARDEN_TMP="$(mktemp -d)" \
    && SHELLHARDEN_VERSION="4.3.1" \
    && cd "${SHELLHARDEN_TMP}" \
    && curl -fsSLO "https://github.com/anordal/shellharden/releases/download/v${SHELLHARDEN_VERSION}/shellharden-${ARCH}-unknown-linux-gnu.tar.gz" \
    && tar zxvf "shellharden-${ARCH}-unknown-linux-gnu.tar.gz" \
    && chmod +x shellharden \
    && cp shellharden /usr/local/bin/shellharden \
    && rm -rf "${SHELLHARDEN_TMP}"

WORKDIR /app
COPY . /app/

RUN make

# which kubectl version to install (should be in sync with you kubernetes version)
# https://hub.docker.com/r/bitnami/kubectl/tags
FROM bitnami/kubectl:1.25 as kubectl

WORKDIR /app
COPY --from=base --chmod=0777 /app/backup-ns.sh /app/backup-ns.sh
COPY --from=base /app/lib /app/lib

# sanity check all the required cli tools are installed in the image
RUN bash -c "source /app/lib/utils.sh && utils_check_host_requirements true"

ENTRYPOINT ["/app/backup-ns.sh"]