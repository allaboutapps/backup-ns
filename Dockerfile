# which kubectl version to install (should be in sync with the kubernetes version used by GKE)
# https://hub.docker.com/r/bitnami/kubectl/tags
FROM bitnami/kubectl:1.25 as kubectl

WORKDIR /app
COPY --chmod=0777 backup-ns.sh /app/backup-ns.sh
COPY lib /app/lib

# sanity check all the required cli tools are installed in the image
RUN bash -c "source /app/lib/utils.sh && utils_check_host_requirements true"

ENTRYPOINT ["/app/backup-ns.sh"]