package lib_test

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/allaboutapps/backup-ns/internal/lib"
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

	cleanupTestVolumeSnapshots()
}

func cleanupTestVolumeSnapshots() {
	// Get all managed volume snapshots
	vss, err := lib.GetManagedVolumeSnapshots()
	if err != nil {
		log.Fatalf("failed to get volume snapshots: %v", err)
	}

	// Filter snapshots in test namespaces
	testNamespaces := map[string]bool{
		"generic-test":  true,
		"postgres-test": true,
		"mysql-test":    true,
	}

	var filteredVss []lib.NamespacedK8sObject
	for _, vs := range vss {
		if testNamespaces[vs.Namespace] {
			filteredVss = append(filteredVss, vs)
		}
	}

	log.Printf("del %d test volume snapshots...\n", len(filteredVss))

	const maxWorkers = 16
	var wg sync.WaitGroup
	jobs := make(chan lib.NamespacedK8sObject, len(filteredVss))
	errors := make(chan error, len(filteredVss))

	// Start workers
	for w := 0; w < maxWorkers; w++ {
		go func() {
			for vs := range jobs {
				if err := lib.PruneVolumeSnapshot(vs.Namespace, vs.Name, false); err != nil {
					errors <- fmt.Errorf("failed to prune %s/%s: %w", vs.Namespace, vs.Name, err)
				}
				wg.Done()
			}
		}()
	}

	// Queue jobs
	for _, vs := range filteredVss {
		wg.Add(1)
		jobs <- vs
	}
	close(jobs)

	// Wait for completion and collect errors
	go func() {
		wg.Wait()
		close(errors)
	}()

	// Check for errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		log.Fatalf("errors during volume snapshot cleanup: %v", errs)
	}

	log.Printf("deleted %d test volume snapshots\n", len(filteredVss))
}
