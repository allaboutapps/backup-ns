package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var (
	customPostgresOutputFile string
	postgresDownloadRetries  int
)

var postgresDownloadDumpCmd = &cobra.Command{
	Use:   "downloadDump",
	Short: "Downloads the latest postgres dump from the container to the local filesystem",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.Postgres.Enabled {
			log.Fatal("BAK_DB_POSTGRES=true must be set.")
		}

		runPostgresDownload(config)
	},
}

func init() {
	postgresCmd.AddCommand(postgresDownloadDumpCmd)
	postgresDownloadDumpCmd.Flags().StringVarP(&customPostgresOutputFile, "output", "o", "", "Custom absolute output filepath")
	postgresDownloadDumpCmd.Flags().IntVar(&postgresDownloadRetries, "retries", 3, "Number of retries for kubectl cp")
}

func generateDumpFilename(namespace string, timestamp time.Time) string {
	return fmt.Sprintf("%s_%s_postgres_dump.tar.gz",
		namespace,
		timestamp.UTC().Format("2006-01-02T15-04-05Z"))
}

func runPostgresDownload(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsurePostgresAvailable(config.Namespace, config.Postgres); err != nil {
		log.Fatal(err)
	}

	podName, err := lib.GetPodFromResource(config.Namespace, config.Postgres.ExecResource)
	if err != nil {
		log.Fatal(err)
	}

	// Get dump file timestamp
	timestamp, err := lib.GetRemoteFileTimestamp(config.Namespace, config.Postgres.ExecResource, config.Postgres.ExecContainer, config.Postgres.DumpFile)
	if err != nil {
		log.Fatal(err)
	}

	// Determine local destination path
	localPath := customPostgresOutputFile
	if localPath != "" {
		if !filepath.IsAbs(localPath) {
			log.Fatal("Custom output path must be absolute")
		}
	} else {
		// Auto-generated name goes to current directory
		localPath = filepath.Join(".", generateDumpFilename(config.Namespace, timestamp))
	}

	log.Printf("Downloading postgres dump from namespace='%s' to %s", config.Namespace, localPath)

	// #nosec G204 -- kubectl cp is safe as config is validated through lib.LoadConfig()
	cmd := exec.Command("kubectl",
		"cp",
		"-c", config.Postgres.ExecContainer,
		fmt.Sprintf("--retries=%d", postgresDownloadRetries),
		fmt.Sprintf("%s/%s:%s", config.Namespace, podName, config.Postgres.DumpFile),
		localPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to download dump: %v\nOutput: %s", err, output)
	}

	if info, err := os.Stat(localPath); err == nil {
		log.Printf("Successfully downloaded dump file (size: %d bytes)\n", info.Size())
		log.Printf("to unpack:\ngzip -dc %s > dump.sql\n", localPath)
		log.Printf("to import:\ngzip -dc %s | psql --host 127.0.0.1 --port 5432 --username=%s %s\n", localPath, config.Postgres.User, config.Postgres.DB)
	}
}
