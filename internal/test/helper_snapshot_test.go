package test_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/test"
	"github.com/allaboutapps/backup-ns/internal/test/mocks"
	"github.com/allaboutapps/backup-ns/internal/util"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSnapshot(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "foo",
		B: 1,
		C: true,
	}

	b := "Hello World!"

	test.Snapshoter.Save(t, a, b)
}

func TestSnapshotWithReplacer(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	randID, err := util.GenerateRandomBase64String(20)
	require.NoError(t, err)
	a := struct {
		ID string
		A  string
		B  int
		C  bool
		D  *string
	}{
		ID: randID,
		A:  "foo",
		B:  1,
		C:  true,
	}

	replacer := func(s string) string {
		re, err := regexp.Compile(`ID:.*"(.*)",`)
		require.NoError(t, err)
		return re.ReplaceAllString(s, "ID: <redacted>,")
	}
	test.Snapshoter.Replacer(replacer).Save(t, a)
}

func TestSnapshotShouldFail(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "fo",
		B: 1,
		C: true,
	}

	b := "Hello World!"

	tMock := new(mocks.TestingT)
	tMock.On("Helper").Return()
	tMock.On("Name").Return("TestSnapshotShouldFail")
	tMock.On("Error", mock.Anything).Return()
	test.Snapshoter.Save(tMock, a, b)
	tMock.AssertNotCalled(t, "Fatal")
	tMock.AssertNotCalled(t, "Fatalf")
	tMock.AssertCalled(t, "Error", mock.Anything)
}

func TestSnapshotWithUpdate(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "fo",
		B: 1,
		C: true,
	}

	b := "Hello World!"

	tMock := new(mocks.TestingT)
	tMock.On("Helper").Return()
	tMock.On("Name").Return("TestSnapshotWithUpdate")
	tMock.On("Errorf", mock.Anything, mock.Anything).Return()
	test.Snapshoter.Update(true).Save(tMock, a, b)
	tMock.AssertNotCalled(t, "Error")
	tMock.AssertNotCalled(t, "Fatal")
	tMock.AssertCalled(t, "Errorf", mock.Anything, mock.Anything)
}

func TestSnapshotNotExists(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "foo",
		B: 1,
		C: true,
	}

	b := "Hello World!"

	defer func() {
		os.Remove(filepath.Join(test.DefaultSnapshotDirPathAbs, "TestSnapshotNotExists.golden"))
	}()

	tMock := new(mocks.TestingT)
	tMock.On("Helper").Return()
	tMock.On("Name").Return("TestSnapshotNotExists")
	tMock.On("Fatalf", mock.Anything, mock.Anything).Return()
	tMock.On("Fatal", mock.Anything).Return()
	tMock.On("Error", mock.Anything).Return()
	tMock.On("Errorf", mock.Anything, mock.Anything).Return()
	test.Snapshoter.Save(tMock, a, b)
	tMock.AssertNotCalled(t, "Error")
	tMock.AssertNotCalled(t, "Fatal")
	tMock.AssertCalled(t, "Errorf", mock.Anything, mock.Anything)
}

func TestSnapshotSkipFields(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	randID, err := util.GenerateRandomBase64String(20)
	require.NoError(t, err)
	a := struct {
		ID string
		A  string
		B  int
		C  bool
		D  *string
	}{
		ID: randID,
		A:  "foo",
		B:  1,
		C:  true,
	}

	test.Snapshoter.Skip([]string{"ID"}).Save(t, a)
}

func TestSnapshotSkipPrefixedFields(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}

	a := struct {
		ID            string
		OtherIDStr    string
		OtherIDInt    int
		OtherIDBool   bool
		OtherIDPTR    *string
		OtherIDStruct struct {
			ID string
		}
	}{
		ID:          "foo",
		OtherIDStr:  "id str",
		OtherIDInt:  4,
		OtherIDBool: true,
		OtherIDStruct: struct{ ID string }{
			ID: "foo",
		},
	}

	test.Snapshoter.Skip([]string{"ID"}).Save(t, a)
}

func TestSnapshotSkipMultilineFields(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	randID, err := util.GenerateRandomBase64String(20)
	require.NoError(t, err)
	a := struct {
		ID string
		A  string
		B  int
		C  bool
		D  interface{}
		E  []string
		F  map[string]int
	}{
		ID: randID,
		A:  "foo",
		B:  1,
		C:  true,
		D: struct {
			Foo string
			Bar int
		}{
			Foo: "skip me",
			Bar: 3,
		},
		E: []string{"skip me", "skip me too"},
		F: map[string]int{
			"skip me":       1,
			"skip me too":   2,
			"skip me three": 3,
		},
	}

	test.Snapshoter.Skip([]string{"ID", "D", "E", "F"}).Save(t, a)
}

func TestSnapshotWithLabel(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "foo",
		B: 1,
		C: true,
	}

	b := "Hello World!"

	test.Snapshoter.Label("_A").Save(t, a)
	test.Snapshoter.Label("_B").Save(t, b)
}

func TestSnapshotWithLocation(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}
	a := struct {
		A string
		B int
		C bool
		D *string
	}{
		A: "foo",
		B: 1,
		C: true,
	}

	location := filepath.Join(util.GetProjectRootDir(), "/test/testdata/snapshots")
	test.Snapshoter.Location(location).Save(t, a)
}

func TestSnapshotJSON(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}

	randID, err := util.GenerateRandomBase64String(20)
	require.NoError(t, err)

	details := struct {
		ID string
		A  string
		B  int
		C  bool
		D  interface{}
		E  []string
		F  map[string]int
	}{
		ID: randID,
		A:  "foo",
		B:  1,
		C:  true,
		D: struct {
			Foo string
			Bar int
		}{
			Foo: "skip me",
			Bar: 3,
		},
		E: []string{"skip me", "skip me too"},
		F: map[string]int{
			"skip me":       1,
			"skip me too":   2,
			"skip me three": 3,
		},
	}

	marshaled, err := json.Marshal(details)
	require.NoError(t, err)

	test.Snapshoter.Redact("ID").SaveJSON(t, json.RawMessage(marshaled))
}

func TestSnapshotSaveBytesImage(t *testing.T) {
	if test.UpdateGoldenGlobal {
		t.Skip()
	}

	filepath := filepath.Join(util.GetProjectRootDir(), "/test/testdata", "example.jpg")

	// read file and save bytes
	content, err := os.ReadFile(filepath)
	require.NoError(t, err)

	test.Snapshoter.SaveBytes(t, content, "jpg")
}
