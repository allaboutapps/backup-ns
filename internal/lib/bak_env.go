package lib

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Config holds all the configuration options
type Config struct {
	DryRun                    bool   `json:"BAK_DRY_RUN"`
	Namespace                 string `json:"BAK_NAMESPACE"`
	PVCName                   string `json:"BAK_PVC_NAME"`
	VSRand                    string `json:"BAK_VS_RAND"`
	LabelVS                   LabelVSConfig
	VSNameTemplate            string `json:"BAK_VS_NAME_TEMPLATE"`
	VSClassName               string `json:"BAK_VS_CLASS_NAME"`
	VSWaitUntilReady          bool   `json:"BAK_VS_WAIT_UNTIL_READY"`
	VSWaitUntilReadyTimeout   string `json:"BAK_VS_WAIT_UNTIL_READY_TIMEOUT"`
	ThresholdSpaceUsedPercent int    `json:"BAK_THRESHOLD_SPACE_USED_PERCENTAGE"`
	DBSkip                    bool   `json:"BAK_DB_SKIP"`
	Postgres                  PostgresConfig
	MySQL                     MySQLConfig
	Flock                     FlockConfig
}

type LabelVSConfig struct {
	Type       string `json:"BAK_LABEL_VS_TYPE"`
	Pod        string `json:"BAK_LABEL_VS_POD"`
	Retain     string `json:"BAK_LABEL_VS_RETAIN"`
	RetainDays int    `json:"BAK_LABEL_VS_RETAIN_DAYS"`
}

type PostgresConfig struct {
	Enabled       bool   `json:"BAK_DB_POSTGRES"`
	ExecResource  string `json:"BAK_DB_POSTGRES_EXEC_RESOURCE"`
	ExecContainer string `json:"BAK_DB_POSTGRES_EXEC_CONTAINER"`
	DumpFile      string `json:"BAK_DB_POSTGRES_DUMP_FILE"`
	User          string `json:"BAK_DB_POSTGRES_USER"`
	Password      string `json:"-"` // sensitive
	DB            string `json:"BAK_DB_POSTGRES_DB"`
}

type MySQLConfig struct {
	Enabled       bool   `json:"BAK_DB_MYSQL"`
	ExecResource  string `json:"BAK_DB_MYSQL_EXEC_RESOURCE"`
	ExecContainer string `json:"BAK_DB_MYSQL_EXEC_CONTAINER"`
	DumpFile      string `json:"BAK_DB_MYSQL_DUMP_FILE"`
	Host          string `json:"BAK_DB_MYSQL_HOST"`
	User          string `json:"BAK_DB_MYSQL_USER"`
	Password      string `json:"-"` // sensitive
	DB            string `json:"BAK_DB_MYSQL_DB"`
}

type FlockConfig struct {
	Enabled    bool   `json:"BAK_FLOCK"`
	Count      int    `json:"BAK_FLOCK_COUNT"`
	Dir        string `json:"BAK_FLOCK_DIR"`
	TimeoutSec int    `json:"BAK_FLOCK_TIMEOUT_SEC"`
}

func LoadConfig() Config {
	return Config{
		// If true, no actual dump/backup is performed, just a dry run to check if everything is in place (still exec into the target container)
		DryRun: getBoolEnv("BAK_DRY_RUN", false),

		// The target namespace to backup
		Namespace: getEnv("BAK_NAMESPACE", getCurrentNamespace()),

		// The name of the PVC to backup, the vs will also be labeled via the key "backup-ns.sh/pvc"
		PVCName: getEnv("BAK_PVC_NAME", "data"),

		// A random string to make the volume snapshot name unique (apart from the timestamp)
		VSRand: getEnv("BAK_VS_RAND", generateRandomString(6)),

		LabelVS: LabelVSConfig{
			// "backup-ns.sh/type" label value of volume snapshot (e.g. "adhoc" or custom backups, "cronjob" for recurring, etc.)
			// This label is not used for any further selections and only for informational purposes.
			Type: getEnv("BAK_LABEL_VS_TYPE", "adhoc"),

			// "backup-ns.sh/pod" label value of volume snapshot (this is used to identify the backup job that created the snapshot)
			Pod: getEnv("BAK_LABEL_VS_POD", ""),

			// "backup-ns.sh/retain" label value. Currently supported values:
			// "daily_weekly_monthly": keep as long as these label keys (key "backup-ns.sh/daily|weekly|monthly") are available on the vs
			// "days": keep the vs for as long as the label value within key "backup-ns.sh/delete-after" says (YYYY-MM-DD)
			Retain: getEnv("BAK_LABEL_VS_RETAIN", "daily_weekly_monthly"),

			// The number of days to retain the snapshot if BAK_LABEL_VS_RETAIN is set to "days"
			RetainDays: getIntEnv("BAK_LABEL_VS_RETAIN_DAYS", 30),
		},

		// The (go template) of the name of the volume snapshot (will be evaluated after having the flock lock, if enabled)
		VSNameTemplate: getEnv("BAK_VS_NAME_TEMPLATE", "{{ .pvcName }}-{{ .timestamp }}-{{ .rand }}"),

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

		Postgres: PostgresConfig{
			// If true, a postgresql dump is created before the snapshot
			Enabled: getBoolEnv("BAK_DB_POSTGRES", false),

			// The k8s resource to exec into to create the dump
			ExecResource: getEnv("BAK_DB_POSTGRES_EXEC_RESOURCE", "deployment/app-base"),

			// The container inside the above resource to exec into to create the dump
			ExecContainer: getEnv("BAK_DB_POSTGRES_EXEC_CONTAINER", "postgres"),

			// The file inside the container to store the dump
			DumpFile: getEnv("BAK_DB_POSTGRES_DUMP_FILE", "/var/lib/postgresql/data/dump.sql.gz"),

			// The postgresql user to use for connecting/creating the dump (psql and pg_dump must be allowed)
			User: getEnv("BAK_DB_POSTGRES_USER", "${POSTGRES_USER}"),

			// The postgresql password to use for connecting/creating the dump
			Password: getEnv("BAK_DB_POSTGRES_PASSWORD", "${POSTGRES_PASSWORD}"),

			// The postgresql database to use for connecting/creating the dump
			DB: getEnv("BAK_DB_POSTGRES_DB", "${POSTGRES_DB}"),
		},

		MySQL: MySQLConfig{
			// If true, a mysql dump is created before the snapshot
			Enabled: getBoolEnv("BAK_DB_MYSQL", false),

			// The k8s resource to exec into to create the dump
			ExecResource: getEnv("BAK_DB_MYSQL_EXEC_RESOURCE", "deployment/app-base"),

			// The container inside the above resource to exec into to create the dump
			ExecContainer: getEnv("BAK_DB_MYSQL_EXEC_CONTAINER", "mysql"),

			// The file inside the container to store the dump
			DumpFile: getEnv("BAK_DB_MYSQL_DUMP_FILE", "/var/lib/mysql/dump.sql.gz"),

			// The mysql host to use for connecting/creating the dump
			Host: getEnv("BAK_DB_MYSQL_HOST", "127.0.0.1"),

			// The mysql user to use for connecting/creating the dump
			User: getEnv("BAK_DB_MYSQL_USER", "root"),

			// The mysql password to use for connecting/creating the dump
			Password: getEnv("BAK_DB_MYSQL_PASSWORD", "${MYSQL_ROOT_PASSWORD}"),

			// The mysql database to use for connecting/creating the dump
			DB: getEnv("BAK_DB_MYSQL_DB", "${MYSQL_DATABASE}"),
		},

		Flock: FlockConfig{
			// If true, flock is used to coordinate concurrent backup script execution, e.g. controlling per k8s node backup script concurrency
			Enabled: getBoolEnv("BAK_FLOCK", false),

			// The number of concurrent backup scripts allowed to run
			Count: getIntEnv("BAK_FLOCK_COUNT", getDefaultFlockCount()),

			// The dir in which we will create file locks to coordinate multiple running backup-ns.sh jobs
			Dir: getEnv("BAK_FLOCK_DIR", "/mnt/host-backup-locks"),

			// The timeout in seconds to wait for the flock lock until we exit 1
			TimeoutSec: getIntEnv("BAK_FLOCK_TIMEOUT_SEC", 3600),
		},
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
		// log.Printf("Error getting current namespace: %v", err)
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

// GetBAKEnvVars returns all environment variables starting with "BAK_", excluding secrets
func GetBAKEnvVars() map[string]string {
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			key, value := parts[0], parts[1]
			if strings.HasPrefix(key, "BAK_") && !strings.Contains(key, "PASSWORD") {
				envVars[key] = value
			}
		}
	}
	return envVars
}

func PrintBAKEnvVars() {
	envVars := GetBAKEnvVars()
	for key, value := range envVars {
		log.Printf("%s='%s'\n", key, value)
	}
}

func PrintConfig(config Config) {
	c, err := json.MarshalIndent(config, "", "  ")

	if err != nil {
		log.Fatalf("Failed to printEnv")
	}

	log.Println("PrintConfig", string(c))
}
