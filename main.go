package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all the configuration options
type Config struct {
	DryRun                     bool
	Debug                      bool
	Namespace                  string
	PVCName                    string
	VSRand                     string
	LabelVSType                string
	LabelVSPod                 string
	LabelVSRetain              string
	LabelVSRetainDays          int
	VSNameTemplate             string
	VSClassName                string
	VSWaitUntilReady           bool
	VSWaitUntilReadyTimeout    string
	ThresholdSpaceUsedPercent  int
	DBSkip                     bool
	DBPostgres                 bool
	DBPostgresExecResource     string
	DBPostgresExecContainer    string
	DBPostgresDumpFile         string
	DBPostgresUser             string
	DBPostgresPassword         string
	DBPostgresDB               string
	DBMySQL                    bool
	DBMySQLExecResource        string
	DBMySQLExecContainer       string
	DBMySQLDumpFile            string
	DBMySQLHost                string
	DBMySQLUser                string
	DBMySQLPassword            string
	DBMySQLDB                  string
	Flock                      bool
	FlockCount                 int
	FlockDir                   string
	FlockTimeoutSec            int
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
		log.Fatal("Either DBPostgres=true or DBMySQL=true or DBSkip=true must be set.")
	}

	vsName := generateVSName(config)
	log.Println("VS Name:", vsName)

	ensurePVCAvailable(config)

	if config.DBPostgres {
		ensurePostgresAvailable(config)
		ensureFreeSpace(config, config.DBPostgresExecResource, config.DBPostgresExecContainer, filepath.Dir(config.DBPostgresDumpFile))
		backupPostgres(config)
	}

	if config.DBMySQL {
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
		DryRun:                    getBoolEnv("BAK_DRY_RUN", false),
		Debug:                     getBoolEnv("BAK_DEBUG", false),
		Namespace:                 getEnv("BAK_NAMESPACE", getCurrentNamespace()),
		PVCName:                   getEnv("BAK_PVC_NAME", "data"),
		VSRand:                    getEnv("BAK_VS_RAND", generateRandomString()),
		LabelVSType:               getEnv("BAK_LABEL_VS_TYPE", "adhoc"),
		LabelVSPod:                getEnv("BAK_LABEL_VS_POD", ""),
		LabelVSRetain:             getEnv("BAK_LABEL_VS_RETAIN", "daily_weekly_monthly"),
		LabelVSRetainDays:         getIntEnv("BAK_LABEL_VS_RETAIN_DAYS", 30),
		VSNameTemplate:            getEnv("BAK_VS_NAME_TEMPLATE", "${BAK_PVC_NAME}-$(date +\"%Y-%m-%d-%H%M%S\")-${BAK_VS_RAND}"),
		VSClassName:               getEnv("BAK_VS_CLASS_NAME", "a3cloud-csi-gce-pd"),
		VSWaitUntilReady:          getBoolEnv("BAK_VS_WAIT_UNTIL_READY", true),
		VSWaitUntilReadyTimeout:   getEnv("BAK_VS_WAIT_UNTIL_READY_TIMEOUT", "15m"),
		ThresholdSpaceUsedPercent: getIntEnv("BAK_THRESHOLD_SPACE_USED_PERCENTAGE", 90),
		DBSkip:                    getBoolEnv("BAK_DB_SKIP", false),
		DBPostgres:                getBoolEnv("BAK_DB_POSTGRES", false),
		DBPostgresExecResource:    getEnv("BAK_DB_POSTGRES_EXEC_RESOURCE", "deployment/app-base"),
		DBPostgresExecContainer:   getEnv("BAK_DB_POSTGRES_EXEC_CONTAINER", "postgres"),
		DBPostgresDumpFile:        getEnv("BAK_DB_POSTGRES_DUMP_FILE", "/var/lib/postgresql/data/dump.sql.gz"),
		DBPostgresUser:            getEnv("BAK_DB_POSTGRES_USER", "${POSTGRES_USER}"),
		DBPostgresPassword:        getEnv("BAK_DB_POSTGRES_PASSWORD", "${POSTGRES_PASSWORD}"),
		DBPostgresDB:              getEnv("BAK_DB_POSTGRES_DB", "${POSTGRES_DB}"),
		DBMySQL:                   getBoolEnv("BAK_DB_MYSQL", false),
		DBMySQLExecResource:       getEnv("BAK_DB_MYSQL_EXEC_RESOURCE", "deployment/app-base"),
		DBMySQLExecContainer:      getEnv("BAK_DB_MYSQL_EXEC_CONTAINER", "mysql"),
		DBMySQLDumpFile:           getEnv("BAK_DB_MYSQL_DUMP_FILE", "/var/lib/mysql/dump.sql.gz"),
		DBMySQLHost:               getEnv("BAK_DB_MYSQL_HOST", "127.0.0.1"),
		DBMySQLUser:               getEnv("BAK_DB_MYSQL_USER", "root"),
		DBMySQLPassword:           getEnv("BAK_DB_MYSQL_PASSWORD", "${MYSQL_ROOT_PASSWORD}"),
		DBMySQLDB:                 getEnv("BAK_DB_MYSQL_DB", "${MYSQL_DATABASE}"),
		Flock:                     getBoolEnv("BAK_FLOCK", false),
		FlockCount:                getIntEnv("BAK_FLOCK_COUNT", getDefaultFlockCount()),
		FlockDir:                  getEnv("BAK_FLOCK_DIR", "/mnt/host-backup-locks"),
		FlockTimeoutSec:           getIntEnv("BAK_FLOCK_TIMEOUT_SEC", 3600),
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

func generateRandomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 6)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
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

func ensurePVCAvailable(config Config) {
	log.Printf("Checking if PVC '%s' exists in namespace '%s'...", config.PVCName, config.Namespace)
	cmd := exec.Command("kubectl", "get", "pvc", config.PVCName, "-n", config.Namespace)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("PVC '%s' not found in namespace '%s'", config.PVCName, config.Namespace)
	}
}

func ensurePostgresAvailable(config Config) {
	log.Printf("Checking if Postgres is available in namespace '%s'...", config.Namespace)
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--", "psql", "--version")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Postgres not available in namespace '%s': %v", config.Namespace, err)
	}
}

func ensureMySQLAvailable(config Config) {
	log.Printf("Checking if MySQL is available in namespace '%s'...", config.Namespace)
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--", "mysql", "--version")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("MySQL not available in namespace '%s': %v", config.Namespace, err)
	}
}

func ensureFreeSpace(config Config, resource, container, dir string) {
	log.Printf("Checking free space on %s in namespace '%s'...", dir, config.Namespace)
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
}

func backupPostgres(config Config) {
	if config.DryRun {
		log.Println("Skipping Postgres backup - dry run mode is active")
		return
	}
	log.Printf("Backing up Postgres database '%s' in namespace '%s'...", config.DBPostgresDB, config.Namespace)
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--",
		"sh", "-c", fmt.Sprintf("PGPASSWORD=%s pg_dump --username=%s --format=p --clean --if-exists %s | gzip > %s",
			config.DBPostgresPassword, config.DBPostgresUser, config.DBPostgresDB, config.DBPostgresDumpFile))
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Error backing up Postgres: %v", err)
	}
}

func backupMySQL(config Config) {
	if config.DryRun {
		log.Println("Skipping MySQL backup - dry run mode is active")
		return
	}
	log.Printf("Backing up MySQL database '%s' in namespace '%s'...", config.DBMySQLDB, config.Namespace)
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--",
		"sh", "-c", fmt.Sprintf("mysqldump --host=%s --user=%s --password=%s --default-character-set=utf8 --add-locks --set-charset --compact --create-options --add-drop-table --lock-tables %s | gzip > %s",
			config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLPassword, config.DBMySQLDB, config.DBMySQLDumpFile))
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Error backing up MySQL: %v", err)
	}
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
		labels["backup-ns.sh/daily"] = now.Format("2006-01-02")
		labels["backup-ns.sh/weekly"] = now.Format("2006-w02")
		labels["backup-ns.sh/monthly"] = now.Format("2006-01")
	} else if config.LabelVSRetain == "days" {
		deleteAfter := time.Now().AddDate(0, 0, config.LabelVSRetainDays).Format("2006-01-02")
		labels["backup-ns.sh/retain"] = "days"
		labels["backup-ns.sh/retain-days"] = strconv.Itoa(config.LabelVSRetainDays)
		labels["backup-ns.sh/delete-after"] = deleteAfter
	}
	return labels
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
	log.Printf("Creating VolumeSnapshot '%s' in namespace '%s'...", vsName, config.Namespace)
	if config.DryRun {
		log.Println("Skipping VolumeSnapshot creation - dry run mode is active")
		return
	}

	vsJSON, err := json.Marshal(vsObject)
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", config.Namespace)
	cmd.Stdin = bytes.NewReader(vsJSON)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error creating VolumeSnapshot: %v", err)
	}

	if config.VSWaitUntilReady {
		log.Printf("Waiting for VolumeSnapshot '%s' to be ready (timeout: %s)...", vsName, config.VSWaitUntilReadyTimeout)
		cmd = exec.Command("kubectl", "wait", "--for=jsonpath='{.status.readyToUse}'=true", "--timeout", config.VSWaitUntilReadyTimeout, "volumesnapshot/"+vsName, "-n", config.Namespace)
		err = cmd.Run()
		if err != nil {
			log.Printf("Warning: VolumeSnapshot '%s' may not be ready: %v", vsName, err)
		}
	}

	cmd = exec.Command("kubectl", "get", "volumesnapshot/"+vsName, "-n", config.Namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Error getting VolumeSnapshot details: %v", err)
	} else {
		log.Printf("VolumeSnapshot details:\n%s", string(output))
	}
}
