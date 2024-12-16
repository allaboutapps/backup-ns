package lib

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/allaboutapps/backup-ns/internal/util"
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
	Host          string `json:"BAK_DB_POSTGRES_HOST"`
	Port          string `json:"BAK_DB_POSTGRES_PORT"`
	User          string `json:"BAK_DB_POSTGRES_USER"`
	Password      string `json:"-"` // sensitive
	DB            string `json:"BAK_DB_POSTGRES_DB"`
}

type MySQLConfig struct {
	Enabled             bool   `json:"BAK_DB_MYSQL"`
	ExecResource        string `json:"BAK_DB_MYSQL_EXEC_RESOURCE"`
	ExecContainer       string `json:"BAK_DB_MYSQL_EXEC_CONTAINER"`
	DumpFile            string `json:"BAK_DB_MYSQL_DUMP_FILE"`
	Host                string `json:"BAK_DB_MYSQL_HOST"`
	Port                string `json:"BAK_DB_MYSQL_PORT"`
	User                string `json:"BAK_DB_MYSQL_USER"`
	Password            string `json:"-"` // sensitive
	DB                  string `json:"BAK_DB_MYSQL_DB"`
	DefaultCharacterSet string `json:"BAK_DB_MYSQL_DEFAULT_CHARACTER_SET"`
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
		DryRun: util.GetEnvAsBool("BAK_DRY_RUN", false),

		// The target namespace to backup
		Namespace: util.GetEnv("BAK_NAMESPACE", getCurrentNamespaceWithFallback()),

		// The name of the PVC to backup, the vs will also be labeled via the key "backup-ns.sh/pvc"
		PVCName: util.GetEnv("BAK_PVC_NAME", "data"),

		// A random string to make the volume snapshot name unique (apart from the timestamp)
		VSRand: util.GetEnv("BAK_VS_RAND", GenerateRandomStringOrPanic(6)),

		LabelVS: LabelVSConfig{
			// "backup-ns.sh/type" label value of volume snapshot (e.g. "adhoc" or custom backups, "cronjob" for recurring, etc.)
			// This label is not used for any further selections and only for informational purposes.
			Type: util.GetEnv("BAK_LABEL_VS_TYPE", "adhoc"),

			// "backup-ns.sh/pod" label value of volume snapshot (this is used to identify the backup job that created the snapshot)
			Pod: util.GetEnv("BAK_LABEL_VS_POD", ""),

			// "backup-ns.sh/retain" label value. Currently supported values:
			// "daily_weekly_monthly": keep as long as these label keys (key "backup-ns.sh/daily|weekly|monthly") are available on the vs
			// "days": keep the vs for as long as the label value within key "backup-ns.sh/delete-after" says (YYYY-MM-DD)
			Retain: util.GetEnvEnum("BAK_LABEL_VS_RETAIN", "days", []string{"days", "daily_weekly_monthly"}),

			// The number of days to retain the snapshot if BAK_LABEL_VS_RETAIN is set to "days"
			RetainDays: util.GetEnvAsInt("BAK_LABEL_VS_RETAIN_DAYS", 30),
		},

		// The (go template) of the name of the volume snapshot (will be evaluated after having the flock lock, if enabled)
		VSNameTemplate: util.GetEnv("BAK_VS_NAME_TEMPLATE", "{{ .pvcName }}-{{ .timestamp }}-{{ .rand }}"),

		// The name of the volume snapshot class to use, "" means default class
		VSClassName: util.GetEnv("BAK_VS_CLASS_NAME", ""), // the snapshot calls should have "Retain" deletion policy set!

		// If true, the script will wait until the snapshot is actually ready (useable)
		VSWaitUntilReady: util.GetEnvAsBool("BAK_VS_WAIT_UNTIL_READY", true),

		// The timeout to wait for the snapshot to be ready (as go formatted duration spec)
		VSWaitUntilReadyTimeout: util.GetEnv("BAK_VS_WAIT_UNTIL_READY_TIMEOUT", "15m"),

		// The max allowed used space of the disk mounted at the dump dir before the backup fails
		ThresholdSpaceUsedPercent: util.GetEnvAsInt("BAK_THRESHOLD_SPACE_USED_PERCENTAGE", 90),

		// If true, no application-aware backup is performed (no db - useful for testing the snapshot creation only)
		DBSkip: util.GetEnvAsBool("BAK_DB_SKIP", false),

		Postgres: PostgresConfig{
			// If true, a postgresql dump is created before the snapshot
			Enabled: util.GetEnvAsBool("BAK_DB_POSTGRES", false),

			// The k8s resource to exec into to create the dump
			ExecResource: util.GetEnv("BAK_DB_POSTGRES_EXEC_RESOURCE", "deployment/app-base"),

			// The container inside the above resource to exec into to create the dump
			ExecContainer: util.GetEnv("BAK_DB_POSTGRES_EXEC_CONTAINER", "postgres"),

			// The file inside the container to store the dump
			DumpFile: util.GetEnv("BAK_DB_POSTGRES_DUMP_FILE", "/var/lib/postgresql/data/dump.sql.gz"),

			// The postgresql host to use for connecting/creating/restoring the dump
			Host: util.GetEnv("BAK_DB_POSTGRES_HOST", "127.0.0.1"),

			// The postgresql host to use for connecting/creating/restoring the dump
			Port: util.GetEnv("BAK_DB_POSTGRES_PORT", "5432"),

			// The postgresql user to use for connecting/creating the dump (psql and pg_dump must be allowed)
			// Read from inside the *container* by default (${POSTGRES_USER})
			User: util.GetEnv("BAK_DB_POSTGRES_USER", "${POSTGRES_USER}"),

			// The postgresql password to use for connecting/creating the dump
			// Read from inside the *container* by default (${POSTGRES_PASSWORD})
			Password: util.GetEnv("BAK_DB_POSTGRES_PASSWORD", "${POSTGRES_PASSWORD}"),

			// The postgresql database to use for connecting/creating the dump
			// Read from inside the *container* by default (${POSTGRES_DB})
			DB: util.GetEnv("BAK_DB_POSTGRES_DB", "${POSTGRES_DB}"),
		},

		MySQL: MySQLConfig{
			// If true, a mysql dump is created before the snapshot
			Enabled: util.GetEnvAsBool("BAK_DB_MYSQL", false),

			// The k8s resource to exec into to create the dump
			ExecResource: util.GetEnv("BAK_DB_MYSQL_EXEC_RESOURCE", "deployment/app-base"),

			// The container inside the above resource to exec into to create the dump
			ExecContainer: util.GetEnv("BAK_DB_MYSQL_EXEC_CONTAINER", "mysql"),

			// The file inside the container to store the dump
			DumpFile: util.GetEnv("BAK_DB_MYSQL_DUMP_FILE", "/var/lib/mysql/dump.sql.gz"),

			// The mysql host to use for connecting/creating/restoring the dump
			Host: util.GetEnv("BAK_DB_MYSQL_HOST", "127.0.0.1"),

			// The mysql host to use for connecting/creating/restoring the dump
			Port: util.GetEnv("BAK_DB_MYSQL_PORT", "3306"),

			// The mysql user to use for connecting/creating the dump
			User: util.GetEnv("BAK_DB_MYSQL_USER", "root"),

			// The mysql password to use for connecting/creating the dump
			// Read from inside the *container* by default (${MYSQL_ROOT_PASSWORD})
			Password: util.GetEnv("BAK_DB_MYSQL_PASSWORD", "${MYSQL_ROOT_PASSWORD}"),

			// The mysql database to use for connecting/creating the dump
			// Read from inside the *container* by default (${MYSQL_DATABASE})
			DB: util.GetEnv("BAK_DB_MYSQL_DB", "${MYSQL_DATABASE}"),

			// The mysql character set to use for connecting/creating the dump
			// utf8 is by default active for backwards compatibility
			DefaultCharacterSet: util.GetEnv("BAK_DB_MYSQL_DEFAULT_CHARACTER_SET", "utf8"),
		},

		Flock: FlockConfig{
			// If true, flock is used to coordinate concurrent backup script execution, e.g. controlling per k8s node backup script concurrency
			Enabled: util.GetEnvAsBool("BAK_FLOCK", false),

			// The number of concurrent backup scripts allowed to run
			Count: util.GetEnvAsInt("BAK_FLOCK_COUNT", getDefaultFlockCount()),

			// The dir in which we will create file locks to coordinate multiple running backup-ns.sh jobs
			Dir: util.GetEnv("BAK_FLOCK_DIR", "/mnt/host-backup-locks"),

			// The timeout in seconds to wait for the flock lock until we exit 1
			TimeoutSec: util.GetEnvAsInt("BAK_FLOCK_TIMEOUT_SEC", 3600),
		},
	}
}

func getCurrentNamespaceWithFallback() string {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	output, err := cmd.Output()
	if err != nil {
		// log.Printf("Error getting current namespace: %v", err)
		return "default"
	}
	return string(output)
}

func GenerateRandomStringOrPanic(n int) string {

	randString, err := util.GenerateRandomString(n, []util.CharRange{util.CharRangeAlphaLowerCase}, "")
	if err != nil {
		log.Panicf("GenerateRandomString: Failed to generate secure random number: %v", err)
	}

	return randString
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

// Prints the current timezone and the current date and time
func PrintTimeZone() {
	t := time.Now()
	zone, _ := t.Zone()
	log.Println("TimeZone:", zone, "Now:", t.Format(time.DateOnly), t.Format(time.TimeOnly))
}

func PrintConfig(config Config) {
	c, err := json.MarshalIndent(config, "", "  ")

	if err != nil {
		log.Panic("Failed to PrintConfig")
	}

	log.Println("Config:", string(c))
}
