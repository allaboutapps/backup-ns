#!/bin/bash
set -Eeo pipefail

# functions
# ------------------------------

# Retention related labeling. We directly flag the first hourly, daily, weekly, monthly snapshot.
# A (separate) retention worker can then use these labels to determine if a cleanup of this snapshot should happen.
#
# The following labels are used:
#    backup-ns.sh/retain: "hourly,daily,weekly,monthly"
#    backup-ns.sh/hourly: "$(date +"%Y-%m-%d-%H00")" # e.g. "2024-04-0900"
#    backup-ns.sh/daily: "$(date +"%Y-%m-%d")" # e.g. "2024-04-11"
#    backup-ns.sh/weekly: "$(date +"%Y-w%U")" # e.g. "2024-w15"
#    backup-ns.sh/monthly: "$(date +"%Y-%m")" # e.g. "2024-04"
#
# We simply try to kubectl get a prefixing snapshot with the same label and if it does not exist, we set the label on the new snapshot.
# This way we can ensure that the first snapshot of a day, week, month is always flagged.

label_get_retain_spec() {
    local ns=$1

    local hourly_label=$(date +"%Y-%m-%d-%H00")
    local daily_label=$(date +"%Y-%m-%d")
    local weekly_label=$(date +"%Y-w%U")
    local monthly_label=$(date +"%Y-%m")

    local labels=""

    read -r -d '' labels << EOF
backup-ns.sh/retain: "hourly,daily,weekly,monthly"
EOF

    if [ "$(kubectl -n ${ns} get volumesnapshot -l backup-ns.sh/hourly=${hourly_label} -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/hourly: "${hourly_label}"
EOF
    fi

    if [ "$(kubectl -n ${ns} get volumesnapshot -l backup-ns.sh/daily=${daily_label} -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/daily: "${daily_label}"
EOF
    fi

    if [ "$(kubectl -n ${ns} get volumesnapshot -l backup-ns.sh/weekly=${weekly_label} -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/weekly: "${weekly_label}"
EOF
    fi

    if [ "$(kubectl -n ${ns} get volumesnapshot -l backup-ns.sh/monthly=${monthly_label} -o name)" == "" ]; then
        read -r -d '' labels << EOF
${labels}
backup-ns.sh/monthly: "${monthly_label}"
EOF
    fi

    echo "${labels}"
}