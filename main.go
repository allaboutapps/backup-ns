package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Config holds all the configuration options
type Config struct {
	DryRun                    bool
	Debug                     bool
	Namespace                 string
	PVCName                   string
	VSRand                    string
	LabelVSType               string
	LabelVSPod                string
	LabelVSRetain             string
	LabelVSRetainDays         int
	VSNameTemplate            string
	VSClassName               string
	VSWaitUntilReady          bool
	VSWaitUntilReadyTimeout   string
	ThresholdSpaceUsedPercent int
	DBSkip                    bool
	DBPostgres                bool
	DBPostgresExecResource    string
	DBPostgresExecContainer   string
	DBPostgresDumpFile        string
	DBPostgresUser            string
	DBPostgresPassword        string
	DBPostgresDB              string
	DBMySQL                   bool
	DBMySQLExecResource       string
	DBMySQLExecContainer      string
	DBMySQLDumpFile           string
	DBMySQLHost               string
	DBMySQLUser               string
	DBMySQLPassword           string
	DBMySQLDB                 string
	Flock                     bool
	FlockCount                int
	FlockDir                  string
	FlockTimeoutSec           int
}

func main() {
	config := loadConfig()

	if config.Debug {
		log.Println("Config:", config)
	}

	if config.DryRun {
		log.Println("Dry run mode is active, write operations are skipped!")
	}

	if !config.DBPostgres && !config.DBMySQL && !config.DBSkip {
		log.Fatal("Either BAK_DB_POSTGRES=true or BAK_DB_MYSQL=true or BAK_DB_SKIP=true must be set.")
	}

	if config.Flock {
		lockFile := flockShuffleLockFile(config.FlockDir, config.FlockCount)
		log.Printf("Using lock_file='%s'...", lockFile)

		unlock := flockLock(lockFile, config.FlockTimeoutSec, config.DryRun)
		defer unlock()
	}

	vsName := generateVSName(config)
	log.Println("VS Name:", vsName)

	ensurePVCAvailable(config)

	if config.DBPostgres {
		ensureResourceAvailable(config.Namespace, config.DBPostgresExecResource)
		ensurePostgresAvailable(config)
		ensureFreeSpace(config, config.DBPostgresExecResource, config.DBPostgresExecContainer, filepath.Dir(config.DBPostgresDumpFile))
		backupPostgres(config)
	}

	if config.DBMySQL {
		ensureResourceAvailable(config.Namespace, config.DBMySQLExecResource)
		ensureMySQLAvailable(config)
		ensureFreeSpace(config, config.DBMySQLExecResource, config.DBMySQLExecContainer, filepath.Dir(config.DBMySQLDumpFile))
		backupMySQL(config)
	}

	vsLabels := generateVSLabels(config)
	vsAnnotations := generateVSAnnotations(config)

	vsObject := generateVSObject(config, vsName, vsLabels, vsAnnotations)

	if config.Debug {
		log.Println("VS Object:", vsObject)
	}

	createVolumeSnapshot(config, vsName, vsObject)

	log.Printf("Finished backup vs_name='%s' in namespace='%s'!", vsName, config.Namespace)
}

func loadConfig() Config {
	return Config{
		// If true, no actual dump/backup is performed, just a dry run to check if everything is in place (still exec into the target container)
		DryRun: getBoolEnv("BAK_DRY_RUN", false),

		// If true, the script will print out every command before executing it
		Debug: getBoolEnv("BAK_DEBUG", false),

		// The target namespace to backup
		Namespace: getEnv("BAK_NAMESPACE", getCurrentNamespace()),

		// The name of the PVC to backup, the vs will also be labeled via the key "backup-ns.sh/pvc"
		PVCName: getEnv("BAK_PVC_NAME", "data"),

		// A random string to make the volume snapshot name unique (apart from the timestamp)
		VSRand: getEnv("BAK_VS_RAND", generateRandomString(6)),

		// "backup-ns.sh/type" label value of volume snapshot (e.g. "adhoc" or custom backups, "cronjob" for recurring, etc.)
		// This label is not used for any further selections and only for informational purposes.
		LabelVSType: getEnv("BAK_LABEL_VS_TYPE", "adhoc"),

		// "backup-ns.sh/pod" label value of volume snapshot (this is used to identify the backup job that created the snapshot)
		LabelVSPod: getEnv("BAK_LABEL_VS_POD", ""),

		// "backup-ns.sh/retain" label value. Currently supported values:
		// "daily_weekly_monthly": keep as long as these label keys (key "backup-ns.sh/daily|weekly|monthly") are available on the vs
		// "days": keep the vs for as long as the label value within key "backup-ns.sh/delete-after" says (YYYY-MM-DD)
		LabelVSRetain: getEnv("BAK_LABEL_VS_RETAIN", "daily_weekly_monthly"),

		// The number of days to retain the snapshot if BAK_LABEL_VS_RETAIN is set to "days"
		LabelVSRetainDays: getIntEnv("BAK_LABEL_VS_RETAIN_DAYS", 30),

		// The name of the volume snapshot can be templated (will be evaluated after having the flock lock, if enabled)
		VSNameTemplate: getEnv("BAK_VS_NAME_TEMPLATE", "${BAK_PVC_NAME}-$(date +\"%Y-%m-%d-%H%M%S\")-${BAK_VS_RAND}"),

		// The name of the volume snapshot class to use
		VSClassName: getEnv("BAK_VS_CLASS_NAME", "a3cloud-csi-gce-pd"), // should have "Retain" deletion policy!

		// If true, the script will wait until the snapshot is actually ready (useable)
		VSWaitUntilReady: getBoolEnv("BAK_VS_WAIT_UNTIL_READY", true),

		// The timeout to wait for the snapshot to be ready (as go formatted duration spec)
		VSWaitUntilReadyTimeout: getEnv("BAK_VS_WAIT_UNTIL_READY_TIMEOUT", "15m"),

		// The max allowed used space of the disk mounted at the dump dir before the backup fails
		ThresholdSpaceUsedPercent: getIntEnv("BAK_THRESHOLD_SPACE_USED_PERCENTAGE", 90),

		// If true, no application-aware backup is performed (no db - useful for testing the snapshot creation only)
		DBSkip: getBoolEnv("BAK_DB_SKIP", false),

		// If true, a postgresql dump is created before the snapshot
		DBPostgres: getBoolEnv("BAK_DB_POSTGRES", false),

		// The k8s resource to exec into to create the dump
		DBPostgresExecResource: getEnv("BAK_DB_POSTGRES_EXEC_RESOURCE", "deployment/app-base"),

		// The container inside the above resource to exec into to create the dump
		DBPostgresExecContainer: getEnv("BAK_DB_POSTGRES_EXEC_CONTAINER", "postgres"),

		// The file inside the container to store the dump
		DBPostgresDumpFile: getEnv("BAK_DB_POSTGRES_DUMP_FILE", "/var/lib/postgresql/data/dump.sql.gz"),

		// The postgresql user to use for connecting/creating the dump (psql and pg_dump must be allowed)
		DBPostgresUser: getEnv("BAK_DB_POSTGRES_USER", "${POSTGRES_USER}"),

		// The postgresql password to use for connecting/creating the dump
		DBPostgresPassword: getEnv("BAK_DB_POSTGRES_PASSWORD", "${POSTGRES_PASSWORD}"),

		// The postgresql database to use for connecting/creating the dump
		DBPostgresDB: getEnv("BAK_DB_POSTGRES_DB", "${POSTGRES_DB}"),

		// If true, a mysql dump is created before the snapshot
		DBMySQL: getBoolEnv("BAK_DB_MYSQL", false),

		// The k8s resource to exec into to create the dump
		DBMySQLExecResource: getEnv("BAK_DB_MYSQL_EXEC_RESOURCE", "deployment/app-base"),

		// The container inside the above resource to exec into to create the dump
		DBMySQLExecContainer: getEnv("BAK_DB_MYSQL_EXEC_CONTAINER", "mysql"),

		// The file inside the container to store the dump
		DBMySQLDumpFile: getEnv("BAK_DB_MYSQL_DUMP_FILE", "/var/lib/mysql/dump.sql.gz"),

		// The mysql host to use for connecting/creating the dump
		DBMySQLHost: getEnv("BAK_DB_MYSQL_HOST", "127.0.0.1"),

		// The mysql user to use for connecting/creating the dump
		DBMySQLUser: getEnv("BAK_DB_MYSQL_USER", "root"),

		// The mysql password to use for connecting/creating the dump
		DBMySQLPassword: getEnv("BAK_DB_MYSQL_PASSWORD", "${MYSQL_ROOT_PASSWORD}"),

		// The mysql database to use for connecting/creating the dump
		DBMySQLDB: getEnv("BAK_DB_MYSQL_DB", "${MYSQL_DATABASE}"),

		// If true, flock is used to coordinate concurrent backup script execution, e.g. controlling per k8s node backup script concurrency
		Flock: getBoolEnv("BAK_FLOCK", false),

		// The number of concurrent backup scripts allowed to run
		FlockCount: getIntEnv("BAK_FLOCK_COUNT", getDefaultFlockCount()),

		// The dir in which we will create file locks to coordinate multiple running backup-ns.sh jobs
		FlockDir: getEnv("BAK_FLOCK_DIR", "/mnt/host-backup-locks"),

		// The timeout in seconds to wait for the flock lock until we exit 1
		FlockTimeoutSec: getIntEnv("BAK_FLOCK_TIMEOUT_SEC", 3600),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	strValue := getEnv(key, fmt.Sprintf("%t", fallback))
	boolValue, err := strconv.ParseBool(strValue)
	if err != nil {
		log.Printf("Error parsing bool env var %s: %v", key, err)
		return fallback
	}
	return boolValue
}

func getIntEnv(key string, fallback int) int {
	strValue := getEnv(key, fmt.Sprintf("%d", fallback))
	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		log.Printf("Error parsing int env var %s: %v", key, err)
		return fallback
	}
	return intValue
}

func getCurrentNamespace() string {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting current namespace: %v", err)
		return "default"
	}
	return string(output)
}

func generateRandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if err != nil {
			log.Fatalf("generateRandomString: Failed to generate secure random number: %v", err)
		}
		b[i] = letterRunes[num.Int64()]
	}
	return string(b)
}

func getDefaultFlockCount() int {
	cmd := exec.Command("nproc", "--all")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting nproc: %v", err)
		return 2
	}
	nproc, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		log.Printf("Error parsing nproc: %v", err)
		return 2
	}
	if nproc < 2 {
		return 1
	}
	return nproc / 2
}

func generateVSName(config Config) string {
	vsName := config.VSNameTemplate
	vsName = strings.ReplaceAll(vsName, "${BAK_PVC_NAME}", config.PVCName)
	vsName = strings.ReplaceAll(vsName, "${BAK_VS_RAND}", config.VSRand)
	vsName = strings.ReplaceAll(vsName, "$(date +\"%Y-%m-%d-%H%M%S\")", time.Now().Format("2006-01-02-150405"))
	return vsName
}

func flockShuffleLockFile(dir string, count int) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
	if err != nil {
		log.Fatalf("flockShuffleLockFile: Failed to generate secure random number: %v", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%d.lock", n.Int64()+1))
}

func flockLock(lockFile string, timeoutSec int, dryRun bool) func() {
	if dryRun {
		log.Println("Skipping flock - dry run mode is active")
		return func() {}
	}

	lockFd, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Failed to open lock file: %v", err)
	}

	_, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	err = syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX)
	if err != nil {
		log.Fatalf("Failed to acquire lock: %v", err)
	}

	log.Printf("Got lock on '%s'!", lockFile)

	return func() {
		err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_UN)
		if err != nil {
			log.Printf("Warning: Failed to release lock: %v", err)
		}
		lockFd.Close()
		log.Printf("Released lock from '%s'", lockFile)
	}
}

func ensurePVCAvailable(config Config) {
	log.Printf("Checking if PVC '%s' exists in namespace '%s'...", config.PVCName, config.Namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "pvc", config.PVCName, "-n", config.Namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("PVC '%s' not found in namespace '%s'", config.PVCName, config.Namespace)
	}
	log.Printf("PVC '%s' is available in namespace '%s'. Output:\n%s", config.PVCName, config.Namespace, string(output))
}

func ensureResourceAvailable(namespace string, resource string) {
	log.Printf("Checking if resource '%s' exists in namespace '%s'...", resource, namespace)

	cmd := exec.Command("kubectl", "get", "-n", namespace, resource, "-o", "wide")
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Error checking resource availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("Resource '%s' not available in namespace '%s'", resource, namespace)
	}
	log.Printf("Resource '%s' is available in namespace '%s'. Output:\n%s", resource, namespace, string(output))
}

func ensurePostgresAvailable(config Config) {
	log.Printf("Checking if Postgres is available in namespace '%s'...", config.Namespace)

	script := fmt.Sprintf(`
		# inject default PGPASSWORD into current env (before cmds are visible in logs)
		export PGPASSWORD=%s
		
		set -Eeox pipefail

		# check clis are available
		command -v gzip
		psql --version
		pg_dump --version

		# check db is accessible
		psql --username=%s %s -c "SELECT 1;" >/dev/null
	`, config.DBPostgresPassword, config.DBPostgresUser, config.DBPostgresDB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking Postgres availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("Postgres not available in namespace '%s'", config.Namespace)
	}
	log.Printf("Postgres is available in namespace '%s'. Output:\n%s", config.Namespace, string(output))
}

func ensureMySQLAvailable(config Config) {
	log.Printf("Checking if MySQL is available in namespace '%s'...", config.Namespace)

	script := fmt.Sprintf(`
		# inject default MYSQL_PWD into current env (before cmds are visible in logs)
		export MYSQL_PWD=%s

		set -Eeox pipefail

		# check clis are available
		command -v gzip
		mysql --version
		mysqldump --version

		# check db is accessible (default password injected via above MYSQL_PWD)
		mysql \
			--host %s \
			--user %s \
			--default-character-set=utf8 \
			%s \
			-e "SELECT 1;" >/dev/null
	`, config.DBMySQLPassword, config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking MySQL availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("MySQL not available in namespace '%s'", config.Namespace)
	}
	log.Printf("MySQL is available in namespace '%s'. Output:\n%s", config.Namespace, string(output))
}

func ensureFreeSpace(config Config, resource, container, dir string) {
	log.Printf("Checking free space on %s in namespace '%s'...", dir, config.Namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, resource, "-c", container, "--", "df", "-h", dir)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error checking free space: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		log.Fatalf("Unexpected df output: %s", string(output))
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		log.Fatalf("Unexpected df output: %s", string(output))
	}
	usedPercent, err := strconv.Atoi(strings.TrimRight(fields[4], "%"))
	if err != nil {
		log.Fatalf("Error parsing used percentage: %v", err)
	}
	if usedPercent >= config.ThresholdSpaceUsedPercent {
		log.Fatalf("Not enough free space. Used: %d%%, Threshold: %d%%", usedPercent, config.ThresholdSpaceUsedPercent)
	}

	log.Printf("Free space check succeeded. Used: %d%%, Threshold: %d%%. Output:\n%s", usedPercent, config.ThresholdSpaceUsedPercent, string(output))
}

// TODO: find a way to kill the remote process (e.g. pgdump / mysqldump) the exec command started
// in the case if origin process on the host terminates (or if we lose connection?)
func backupPostgres(config Config) {
	if config.DryRun {
		log.Println("Skipping Postgres backup - dry run mode is active")
		return
	}
	log.Printf("Backing up Postgres database '%s' in namespace '%s'...", config.DBPostgresDB, config.Namespace)

	script := fmt.Sprintf(`
		# inject default PGPASSWORD into current env (before cmds are visible in logs)
		export PGPASSWORD=%s

		set -Eeox pipefail

		# setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
		trap 'exit_code=$?; [ $exit_code -ne 0 ] && echo "TRAP!" && rm -f %s && df -h %s; exit $exit_code' EXIT
		
		# create dump and pipe to gzip archive
		pg_dump --username=%s --format=p --clean --if-exists %s | gzip -c > %s
		
		# print dump file info
		ls -lha %s
		
		# ensure generated file is bigger than 0 bytes
		[ -s %s ] || exit 1
		
		# print mounted disk space
		df -h %s
	`, config.DBPostgresPassword, config.DBPostgresDumpFile, filepath.Dir(config.DBPostgresDumpFile),
		config.DBPostgresUser, config.DBPostgresDB, config.DBPostgresDumpFile,
		config.DBPostgresDumpFile, config.DBPostgresDumpFile, filepath.Dir(config.DBPostgresDumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up Postgres: %v\nOutput: %s", err, string(output))
		log.Fatal("Postgres backup failed")
	}
	log.Printf("Postgres backup completed. Output:\n%s", string(output))
}

func backupMySQL(config Config) {
	if config.DryRun {
		log.Println("Skipping MySQL backup - dry run mode is active")
		return
	}
	log.Printf("Backing up MySQL database '%s' in namespace '%s'...", config.DBMySQLDB, config.Namespace)

	script := fmt.Sprintf(`
		# inject default MYSQL_PWD into current env (before cmds are visible in logs)
		export MYSQL_PWD=%s

		set -Eeox pipefail
		
        # setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
		trap 'exit_code=$?; [ $exit_code -ne 0 ] && echo "TRAP!" && rm -f %s && df -h %s; exit $exit_code' EXIT
		
		# create dump and pipe to gzip archive (default password injected via above MYSQL_PWD)
		mysqldump \
            --host%s \
            --user %s \
            --default-character-set=utf8 \
            --add-locks \
            --set-charset \
            --compact \
            --create-options \
            --add-drop-table \
            --lock-tables \
            %s \
            | gzip -c > %s
		
		# print dump file info
		ls -lha %s
		
		# ensure generated file is bigger than 0 bytes
		[ -s %s ] || exit 1
		
		# print mounted disk space
		df -h %s
	`, config.DBMySQLPassword, config.DBMySQLDumpFile, filepath.Dir(config.DBMySQLDumpFile),
		config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB, config.DBMySQLDumpFile,
		config.DBMySQLDumpFile, config.DBMySQLDumpFile, filepath.Dir(config.DBMySQLDumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up MySQL: %v\nOutput: %s", err, string(output))
		log.Fatal("MySQL backup failed")
	}
	log.Printf("MySQL backup completed. Output:\n%s", string(output))
}

func generateVSLabels(config Config) map[string]string {
	labels := map[string]string{
		"backup-ns.sh/pvc":  config.PVCName,
		"backup-ns.sh/type": config.LabelVSType,
	}
	if config.LabelVSPod != "" {
		labels["backup-ns.sh/pod"] = config.LabelVSPod
	}
	if config.LabelVSRetain == "daily_weekly_monthly" {
		now := time.Now()
		labels["backup-ns.sh/retain"] = "daily_weekly_monthly"

		dailyLabel := now.Format("2006-01-02")

		_, week := time.Now().ISOWeek()
		weeklyLabel := now.Format("2006-") + fmt.Sprintf("w%02d", week)
		monthlyLabel := now.Format("2006-01")

		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/daily", dailyLabel) {
			labels["backup-ns.sh/daily"] = dailyLabel
		}
		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/weekly", weeklyLabel) {
			labels["backup-ns.sh/weekly"] = weeklyLabel
		}
		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/monthly", monthlyLabel) {
			labels["backup-ns.sh/monthly"] = monthlyLabel
		}
	} else if config.LabelVSRetain == "days" {
		deleteAfter := time.Now().AddDate(0, 0, config.LabelVSRetainDays).Format("2006-01-02")
		labels["backup-ns.sh/retain"] = "days"
		labels["backup-ns.sh/retain-days"] = strconv.Itoa(config.LabelVSRetainDays)
		labels["backup-ns.sh/delete-after"] = deleteAfter
	}
	return labels
}

func volumeSnapshotWithLabelValueExists(namespace, labelKey, labelValue string) bool {
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "volumesnapshot", "-n", namespace, "-l", fmt.Sprintf("%s=%s", labelKey, labelValue), "-o", "name")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error checking for existing VolumeSnapshot: %v", err)
		return false
	}
	return len(output) > 0
}

func generateVSAnnotations(config Config) map[string]string {
	annotations := map[string]string{
		"backup-ns.sh/env-config": fmt.Sprintf(`BAK_DB_POSTGRES=%t
BAK_DB_POSTGRES_EXEC_RESOURCE=%s
BAK_DB_POSTGRES_EXEC_CONTAINER=%s
BAK_DB_POSTGRES_DUMP_FILE=%s
BAK_DB_POSTGRES_USER=%s
BAK_DB_POSTGRES_DB=%s
BAK_DB_MYSQL=%t
BAK_DB_MYSQL_EXEC_RESOURCE=%s
BAK_DB_MYSQL_EXEC_CONTAINER=%s
BAK_DB_MYSQL_DUMP_FILE=%s
BAK_DB_MYSQL_HOST=%s
BAK_DB_MYSQL_USER=%s
BAK_DB_MYSQL_DB=%s`,
			config.DBPostgres, config.DBPostgresExecResource, config.DBPostgresExecContainer, config.DBPostgresDumpFile, config.DBPostgresUser, config.DBPostgresDB,
			config.DBMySQL, config.DBMySQLExecResource, config.DBMySQLExecContainer, config.DBMySQLDumpFile, config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB),
	}
	return annotations
}

func generateVSObject(config Config, vsName string, labels, annotations map[string]string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]interface{}{
			"name":        vsName,
			"namespace":   config.Namespace,
			"labels":      labels,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": config.VSClassName,
			"source": map[string]interface{}{
				"persistentVolumeClaimName": config.PVCName,
			},
		},
	}
}

func createVolumeSnapshot(config Config, vsName string, vsObject map[string]interface{}) {
	stringifiedVSObject, err := json.MarshalIndent(vsObject, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	log.Printf("Creating VolumeSnapshot '%s' in namespace '%s'...\n%s", vsName, config.Namespace, string(stringifiedVSObject))

	if config.DryRun {
		log.Println("Skipping VolumeSnapshot creation - dry run mode is active")
		return
	}

	vsJSON, err := json.Marshal(vsObject)
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", config.Namespace)
	cmd.Stdin = bytes.NewReader(vsJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error creating VolumeSnapshot: %v. Output:\n%s", err, string(output))
	}

	if config.VSWaitUntilReady {
		log.Printf("Waiting for VolumeSnapshot '%s' to be ready (timeout: %s)...", vsName, config.VSWaitUntilReadyTimeout)

		// give kubectl some time to actually have a status field to wait for
		// https://github.com/kubernetes/kubectl/issues/1204
		// https://github.com/kubernetes/kubernetes/pull/109525
		time.Sleep(5 * time.Second)

		// #nosec G204
		cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyToUse}=true", "--timeout", config.VSWaitUntilReadyTimeout, "volumesnapshot/"+vsName, "-n", config.Namespace)
		// log.Println(cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Warning: VolumeSnapshot '%s' may not be ready: %v. Output:\n%s", vsName, err, string(output))
		}
	}

	// #nosec G204
	cmd = exec.Command("kubectl", "get", "volumesnapshot/"+vsName, "-n", config.Namespace)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Error getting VolumeSnapshot details: %v. Output:\n%s", err, string(output))
	} else {
		log.Printf("VolumeSnapshot details:\n%s", string(output))
	}
}
