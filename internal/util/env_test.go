package util_test

import (
	"os"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_STRING"
	res := util.GetEnv(testVarKey, "noVal")
	assert.Equal(t, "noVal", res)

	t.Setenv(testVarKey, "string")
	defer os.Unsetenv(testVarKey)
	res = util.GetEnv(testVarKey, "noVal")
	assert.Equal(t, "string", res)
}

func TestGetEnvEnum(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_ENUM"

	panicFunc := func() {
		_ = util.GetEnvEnum(testVarKey, "smtp", []string{"mock", "foo"})
	}
	assert.Panics(t, panicFunc)

	res := util.GetEnvEnum(testVarKey, "smtp", []string{"mock", "smtp"})
	assert.Equal(t, "smtp", res)

	t.Setenv(testVarKey, "mock")
	defer os.Unsetenv(testVarKey)
	res = util.GetEnvEnum(testVarKey, "smtp", []string{"mock", "smtp"})
	assert.Equal(t, "mock", res)

	t.Setenv(testVarKey, "foo")
	res = util.GetEnvEnum(testVarKey, "smtp", []string{"mock", "smtp"})
	assert.Equal(t, "smtp", res)
}

func TestGetEnvAsInt(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_INT"
	res := util.GetEnvAsInt(testVarKey, 1)
	assert.Equal(t, 1, res)

	t.Setenv(testVarKey, "2")
	defer os.Unsetenv(testVarKey)
	res = util.GetEnvAsInt(testVarKey, 1)
	assert.Equal(t, 2, res)

	t.Setenv(testVarKey, "3x")
	res = util.GetEnvAsInt(testVarKey, 1)
	assert.Equal(t, 1, res)
}

func TestGetEnvAsBool(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_BOOL"
	res := util.GetEnvAsBool(testVarKey, true)
	assert.Equal(t, true, res)

	t.Setenv(testVarKey, "f")
	defer os.Unsetenv(testVarKey)
	res = util.GetEnvAsBool(testVarKey, true)
	assert.Equal(t, false, res)

	t.Setenv(testVarKey, "0")
	res = util.GetEnvAsBool(testVarKey, true)
	assert.Equal(t, false, res)

	t.Setenv(testVarKey, "false")
	res = util.GetEnvAsBool(testVarKey, true)
	assert.Equal(t, false, res)

	t.Setenv(testVarKey, "3x")
	res = util.GetEnvAsBool(testVarKey, true)
	assert.Equal(t, true, res)
}

func TestGetEnvAsStringArr(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_STRING_ARR"
	testVal := []string{"a", "b", "c"}
	res := util.GetEnvAsStringArr(testVarKey, testVal)
	assert.Equal(t, testVal, res)

	t.Setenv(testVarKey, "1,2")
	defer os.Unsetenv(testVarKey)
	res = util.GetEnvAsStringArr(testVarKey, testVal)
	assert.Equal(t, []string{"1", "2"}, res)

	t.Setenv(testVarKey, "")
	res = util.GetEnvAsStringArr(testVarKey, testVal)
	assert.Equal(t, testVal, res)

	t.Setenv(testVarKey, "a, b, c")
	res = util.GetEnvAsStringArr(testVarKey, testVal)
	assert.Equal(t, []string{"a", " b", " c"}, res)

	t.Setenv(testVarKey, "a|b|c")
	res = util.GetEnvAsStringArr(testVarKey, testVal, "|")
	assert.Equal(t, []string{"a", "b", "c"}, res)

	t.Setenv(testVarKey, "a,b,c")
	res = util.GetEnvAsStringArr(testVarKey, testVal, "|")
	assert.Equal(t, []string{"a,b,c"}, res)

	t.Setenv(testVarKey, "a||b||c")
	res = util.GetEnvAsStringArr(testVarKey, testVal, "||")
	assert.Equal(t, []string{"a", "b", "c"}, res)
}

func TestGetEnvAsStringArrTrimmed(t *testing.T) {
	testVarKey := "TEST_ONLY_FOR_UNIT_TEST_STRING_ARR_TRIMMED"
	testVal := []string{"a", "b", "c"}

	t.Setenv(testVarKey, "a, b, c")
	defer os.Unsetenv(testVarKey)
	res := util.GetEnvAsStringArrTrimmed(testVarKey, testVal)
	assert.Equal(t, []string{"a", "b", "c"}, res)

	t.Setenv(testVarKey, "a,   b,c    ")
	res = util.GetEnvAsStringArrTrimmed(testVarKey, testVal)
	assert.Equal(t, []string{"a", "b", "c"}, res)

	t.Setenv(testVarKey, "  a || b  || c  ")
	res = util.GetEnvAsStringArrTrimmed(testVarKey, testVal, "||")
	assert.Equal(t, []string{"a", "b", "c"}, res)
}
