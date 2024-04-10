# which kubectl version to install (should be in sync with the kubernetes version used by GKE)
# https://hub.docker.com/r/bitnami/kubectl/tags
FROM bitnami/kubectl:1.25 as kubectl

WORKDIR /
COPY --chmod=0777 backup-ns.sh /backup-ns.sh

ENTRYPOINT ["/backup-ns.sh"]