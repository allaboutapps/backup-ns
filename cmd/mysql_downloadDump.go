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
	customMySQLOutputFile string
	mysqlDownloadRetries  int
)

var mysqlDownloadDumpCmd = &cobra.Command{
	Use:   "downloadDump",
	Short: "Downloads the latest mysql dump from the container to the local filesystem",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.MySQL.Enabled {
			log.Fatal("BAK_DB_MYSQL=true must be set.")
		}

		runMySQLDownload(config)
	},
}

func init() {
	mysqlCmd.AddCommand(mysqlDownloadDumpCmd)
	mysqlDownloadDumpCmd.Flags().StringVarP(&customMySQLOutputFile, "output", "o", "", "Custom absolute output filepath")
	mysqlDownloadDumpCmd.Flags().IntVar(&mysqlDownloadRetries, "retries", 3, "Number of retries for kubectl cp")
}

func generateMySQLDumpFilename(namespace string, timestamp time.Time) string {
	return fmt.Sprintf("%s_%s_mysql_dump.sql.gz",
		namespace,
		timestamp.UTC().Format("2006-01-02T15-04-05Z"))
}

func runMySQLDownload(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.MySQL.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureMySQLAvailable(config.Namespace, config.MySQL); err != nil {
		log.Fatal(err)
	}

	podName, err := lib.GetPodFromResource(config.Namespace, config.MySQL.ExecResource)
	if err != nil {
		log.Fatal(err)
	}

	// Get dump file timestamp
	timestamp, err := lib.GetRemoteFileTimestamp(config.Namespace, config.MySQL.ExecResource, config.MySQL.ExecContainer, config.MySQL.DumpFile)
	if err != nil {
		log.Fatal(err)
	}

	// Determine local destination path
	localPath := customMySQLOutputFile
	if localPath != "" {
		if !filepath.IsAbs(localPath) {
			log.Fatal("Custom output path must be absolute")
		}
	} else {
		// Auto-generated name goes to current directory
		localPath = filepath.Join(".", generateMySQLDumpFilename(config.Namespace, timestamp))
	}

	log.Printf("Downloading mysql dump from namespace='%s' to %s", config.Namespace, localPath)

	// #nosec G204 -- kubectl cp is safe as config is validated through lib.LoadConfig()
	cmd := exec.Command("kubectl",
		"cp",
		"-c", config.MySQL.ExecContainer,
		fmt.Sprintf("--retries=%d", mysqlDownloadRetries),
		fmt.Sprintf("%s/%s:%s", config.Namespace, podName, config.MySQL.DumpFile),
		localPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to download dump: %v\nOutput: %s", err, output)
	}

	if info, err := os.Stat(localPath); err == nil {
		log.Printf("Successfully downloaded dump file (size: %d bytes)\n", info.Size())
		log.Printf("to unpack:\ngzip -dc %s > dump.sql\n", localPath)
		log.Printf("to import:\ngzip -dc %s | mysql --host=127.0.0.1 --port=3306 --user=%s --default-character-set=%s %s\n",
			localPath,
			config.MySQL.User,
			config.MySQL.DefaultCharacterSet,
			config.MySQL.DB)
	}
}
