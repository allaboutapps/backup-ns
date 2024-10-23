package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// syncMetadataToVscCmd represents the syncMetadataToVsc command
var syncMetadataToVscCmd = &cobra.Command{
	Use:   "syncMetadataToVsc",
	Short: "Synces vs label/annotations metadata to vsc",
	// Long:  `...`, // accidental namespace/vs deletion -> restore namespace...
	Run: func(_ *cobra.Command, _ []string) {
		log.Println("starting sync vs metadata to vsc matching label 'backup-ns.sh/type'")

		vss, err := lib.GetManagedVolumeSnapshots()
		if err != nil {
			log.Fatalf("Error getting ready snapshots: %v\n", err)
		}

		fails := 0

		for _, vs := range vss {
			if err := lib.SyncVSLabelsToVsc(vs.Namespace, vs.Name); err != nil {
				fails++
				log.Printf("fail#%d syncing metadata to vsc failed for vs_name='%s' in ns='%s'.\n", fails, vs.Name, vs.Namespace)
			}
		}

		if fails > 0 {
			log.Fatalf("syncing metadata to vsc failed with %d errors.\n", fails)
		}

		log.Println("syncing metadata to vsc done with", fails, "errors.")
	},
}

func init() {
	rootCmd.AddCommand(syncMetadataToVscCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncMetadataToVscCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncMetadataToVscCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
