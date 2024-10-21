package lib_test

import (
	"log"
	"os/exec"
	"strings"
)

// Immediately error out if kubectl is currently not in the context of our kind test k8s cluster
// We do no want to accidentally run tests against a production Kubernetes cluster (the one you might have configured on your host)
//
// $ kubectl config current-context
// # kind-backup-ns
func init() {
	if context, _ := exec.Command("kubectl", "config", "current-context").Output(); !strings.Contains(string(context), "kind-backup-ns") {
		log.Fatalf("kubectl is not currently running within the context of the kind test k8s cluster named 'kind-backup-ns', exit now!")
	}
}
