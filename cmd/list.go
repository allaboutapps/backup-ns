package cmd

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var (
	allNamespaces bool
	filterDaily   bool
	filterWeekly  bool
	filterMonthly bool
	filterAdhoc   bool
	filterCronjob bool
)

var vsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List volume snapshots with backup-ns labels",
	Run: func(_ *cobra.Command, args []string) {
		namespace := ""
		if !allNamespaces {
			var err error
			namespace, err = lib.GetCurrentNamespace()
			if err != nil {
				log.Fatal(err)
			}
		}

		// Build label selector
		labelSelector := "backup-ns.sh/retain"
		if filterDaily {
			labelSelector += ",backup-ns.sh/daily"
		}
		if filterWeekly {
			labelSelector += ",backup-ns.sh/weekly"
		}
		if filterMonthly {
			labelSelector += ",backup-ns.sh/monthly"
		}
		if filterAdhoc {
			labelSelector += ",backup-ns.sh/type=adhoc"
		}
		if filterCronjob {
			labelSelector += ",backup-ns.sh/type=cronjob"
		}

		// Build kubectl command
		args = []string{
			"get",
			"vs",
			"-l" + labelSelector,
			"-Lbackup-ns.sh/type,backup-ns.sh/retain,backup-ns.sh/daily,backup-ns.sh/weekly,backup-ns.sh/monthly,backup-ns.sh/delete-after",
		}

		if allNamespaces {
			args = append(args, "--all-namespaces")
		} else if namespace != "" {
			args = append(args, "-n", namespace)
		}

		// #nosec G204
		cmd := exec.Command("kubectl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to list volume snapshots: %v\nOutput: %s", err, output)
		}

		if namespace != "" {
			fmt.Printf("Namespace: %s\n", namespace)
		}

		fmt.Printf("Listing volume snapshots with labels: %s\n", labelSelector)
		fmt.Print(string(output))
	},
}

func init() {
	rootCmd.AddCommand(vsListCmd)
	vsListCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List volume snapshots in all namespaces")
	vsListCmd.Flags().BoolVar(&filterDaily, "daily", false, "Filter daily snapshots")
	vsListCmd.Flags().BoolVar(&filterWeekly, "weekly", false, "Filter weekly snapshots")
	vsListCmd.Flags().BoolVar(&filterMonthly, "monthly", false, "Filter monthly snapshots")
	vsListCmd.Flags().BoolVar(&filterAdhoc, "adhoc", false, "Filter type adhoc snapshots")
	vsListCmd.Flags().BoolVar(&filterCronjob, "cronjob", false, "Filter type cronjob snapshots")
}
