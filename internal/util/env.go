package util

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func GetEnv(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}

func GetEnvEnum(key string, defaultVal string, allowedValues []string) string {
	if !ContainsString(allowedValues, defaultVal) {
		log.Panicf("Default value '%s' is not in the allowed values list.", defaultVal)
	}

	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}

	if !ContainsString(allowedValues, val) {
		log.Printf("Value '%s' is not allowed (key '%s'). Fallback to default value '%s'.\n", val, key, defaultVal)
		return defaultVal
	}

	return val
}

func GetEnvAsInt(key string, defaultVal int) int {
	strVal := GetEnv(key, "")

	if val, err := strconv.Atoi(strVal); err == nil {
		return val
	}

	return defaultVal
}

func GetEnvAsBool(key string, defaultVal bool) bool {
	strVal := GetEnv(key, "")

	if val, err := strconv.ParseBool(strVal); err == nil {
		return val
	}

	return defaultVal
}

// GetEnvAsStringArr reads ENV and returns the values split by separator.
func GetEnvAsStringArr(key string, defaultVal []string, separator ...string) []string {
	strVal := GetEnv(key, "")

	if len(strVal) == 0 {
		return defaultVal
	}

	sep := ","
	if len(separator) >= 1 {
		sep = separator[0]
	}

	return strings.Split(strVal, sep)
}

// GetEnvAsStringArrTrimmed reads ENV and returns the whitespace trimmed values split by separator.
func GetEnvAsStringArrTrimmed(key string, defaultVal []string, separator ...string) []string {
	slc := GetEnvAsStringArr(key, defaultVal, separator...)

	for i := range slc {
		slc[i] = strings.TrimSpace(slc[i])
	}

	return slc
}
