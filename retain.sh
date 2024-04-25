#!/bin/bash
set -Eeo pipefail

# ...

# imports
# ------------------------------

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/vs.sh"

RETAIN_DRY_RUN="${RETAIN_DRY_RUN:="false"}"
RETAIN_LAST_DAILY="${RETAIN_LAST_DAILY:="7"}"
RETAIN_LAST_WEEKLY="${RETAIN_LAST_WEEKLY:="4"}"
RETAIN_LAST_MONTHLY="${RETAIN_LAST_MONTHLY:="12"}"

# main
# ------------------------------

# print RETAIN_* env vars
( set -o posix ; set ) | grep "RETAIN_"

# we encapsulate main to allow for local variable declarations
function main() {
    utils_check_host_requirements "false" "true" # 2nd true checks jq is available

    log "starting retain, getting namespaces with 'backup-ns.sh/retain' set..."

    # Get all namespaces with snapshots that have the "backup-ns.sh/retain" label set
    retain_namespaces=$(kubectl get volumesnapshot --all-namespaces -lbackup-ns.sh/retain -o=jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' | uniq)

    verbose "$retain_namespaces"

    fails=$((0))
    while IFS= read -r ns; do

        log "processing namespace='${ns}'..."

        # backup-ns.sh/daily...
        # we keep the last $RETAIN_LAST_DAILY daily snapshots, let's get all current volumesnapshots with the label "backup-ns.sh/daily" sorted by latest
        daily=$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/daily -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' --sort-by=.metadata.creationTimestamp | tac)
        daily_head=$(echo "$daily" | head -n "$RETAIN_LAST_DAILY")
        daily_kept_count=$(echo "$daily_head" | wc -l | xargs)

        daily_unlabel=$(sort <(echo "$daily" | sort) <(echo "$daily_head" | sort) | uniq -u)

        verbose "backup-ns.sh/daily - we will keep these ${daily_kept_count}/${RETAIN_LAST_DAILY}:"
        verbose "$daily_head"

        if [ "$daily_unlabel" != "" ]; then
            while IFS= read -r vs_name; do
                warn "backup-ns.sh/daily - unlabeling '${vs_name}' in ns='${ns}'..."

                cmd="kubectl label -n $ns vs/${vs_name} backup-ns.sh/daily-"

                # dry-run mode? bail out early!
                if [ "$RETAIN_DRY_RUN" == "true" ]; then
                    warn "skipping - dry-run mode is active, cmd='${cmd}'"
                    continue
                fi

                if ! eval "$cmd"; then
                    ((fails+=1))
                    err "fail#${fails} unlabeling failed for vs_name='${vs_name}' in ns='${ns}'."
                fi
            done <<< "$daily_unlabel"
        fi

        # backup-ns.sh/weekly...
        # we keep the last 4 weekly snapshots, let's get all current volumesnapshots with the label "backup-ns.sh/weekly" sorted by latest
        weekly=$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/weekly -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' --sort-by=.metadata.creationTimestamp | tac)
        weekly_head=$(echo "$weekly" | head -n "$RETAIN_LAST_WEEKLY")
        weekly_kept_count=$(echo "$weekly_head" | wc -l | xargs)

        weekly_unlabel=$(sort <(echo "$weekly" | sort) <(echo "$weekly_head" | sort) | uniq -u)

        verbose "backup-ns.sh/weekly - we will keep these ${weekly_kept_count}/${RETAIN_LAST_WEEKLY}:"
        verbose "$weekly_head"

        if [ "$weekly_unlabel" != "" ]; then
            while IFS= read -r vs_name; do
                warn "backup-ns.sh/weekly - unlabeling '${vs_name}' in ns='${ns}'..."

                cmd="kubectl label -n $ns vs/${vs_name} backup-ns.sh/weekly-"

                # dry-run mode? bail out early!
                if [ "$RETAIN_DRY_RUN" == "true" ]; then
                    warn "skipping - dry-run mode is active, cmd='${cmd}'"
                    continue
                fi

                if ! eval "$cmd"; then
                    ((fails+=1))
                    err "fail#${fails} unlabeling failed for vs_name='${vs_name}' in ns='${ns}'."
                fi
            done <<< "$weekly_unlabel"
        fi

        # backup-ns.sh/monthly...
        # we keep the last 12 monthly snapshots, let's get all current volumesnapshots with the label "backup-ns.sh/monthly" sorted by latest
        monthly=$(kubectl -n "$ns" get volumesnapshot -l backup-ns.sh/monthly -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' --sort-by=.metadata.creationTimestamp | tac)
        monthly_head=$(echo "$monthly" | head -n "$RETAIN_LAST_MONTHLY")
        monthly_kept_count=$(echo "$monthly_head" | wc -l | xargs)

        monthly_unlabel=$(sort <(echo "$monthly" | sort) <(echo "$monthly_head" | sort) | uniq -u)

        verbose "backup-ns.sh/monthly - we will keep these ${monthly_kept_count}/${RETAIN_LAST_MONTHLY}:"
        verbose "$monthly_head"

        if [ "$monthly_unlabel" != "" ]; then
            while IFS= read -r vs_name; do
                warn "backup-ns.sh/monthly - unlabeling '${vs_name}' in ns='${ns}'..."

                cmd="kubectl label -n $ns vs/${vs_name} backup-ns.sh/monthly-"

                # dry-run mode? bail out early!
                if [ "$RETAIN_DRY_RUN" == "true" ]; then
                    warn "skipping - dry-run mode is active, cmd='${cmd}'"
                    continue
                fi

                if ! eval "$cmd"; then
                    ((fails+=1))
                    err "fail#${fails} unlabeling failed for vs_name='${vs_name}' in ns='${ns}'."
                fi
            done <<< "$monthly_unlabel"
        fi


    done <<< "$retain_namespaces"

    if [ "$fails" -gt 0 ]; then
        fatal "retain labeler failed with $fails errors."
    fi

    log "retain labeler done with $fails errors."

}

main