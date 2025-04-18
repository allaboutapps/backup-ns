package lib_test

import (
	"path"
	"sync"
	"testing"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlockShuffleLockFile(t *testing.T) {
	// Test the flockShuffleLockFile function
	for i := 0; i < 100; i++ {
		lockfile := lib.FlockShuffleLockFile("test", 2)

		// t.Log(lockfile)
		assert.True(t, lockfile == "test/1.lock" || lockfile == "test/2.lock")
	}

}

func TestFlockDryRun(t *testing.T) {
	// Test the flock function in dry-run mode
	lockFile := lib.FlockShuffleLockFile("/tmp", 1)
	t.Logf("Using lock_file='%s'...", lockFile)

	// the following should not be possible unless dryrun is active, same lock
	unlock, err := lib.FlockLock(lockFile, 100*time.Millisecond, true)
	require.NoError(t, err)
	unlock2, err := lib.FlockLock(lockFile, 100*time.Millisecond, true)
	require.NoError(t, err)
	require.NoError(t, unlock())
	require.NoError(t, unlock2())
}

func TestFlockDirNotFound(t *testing.T) {
	// Test the flock function with a non-existing directory
	lockFile := lib.FlockShuffleLockFile("/tmp/this-dir-is-non-existing", 1)
	t.Logf("Using lock_file='%s'...", lockFile)
	_, err := lib.FlockLock(lockFile, 1*time.Millisecond, false)
	t.Log(err)
	require.Error(t, err)
}

func TestFlockTimeoutSequence(t *testing.T) {
	// Test the flock function
	lockFile := lib.FlockShuffleLockFile("/tmp", 1)
	t.Logf("Using lock_file='%s'...", lockFile)

	unlock, err := lib.FlockLock(lockFile, 100*time.Millisecond, false)
	require.NoError(t, err)

	go func() {
		// Release the first lock after 1 sec, second now has the lock!
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, unlock())
	}()

	// Try to acquire a second lock...
	unlock2, err := lib.FlockLock(lockFile, 200*time.Millisecond, false)
	require.NoError(t, err)

	go func() {
		// Release the second lock after a few time the third will never get the lock!
		time.Sleep(200 * time.Millisecond)
		require.NoError(t, unlock2())
	}()

	// The third will now fail to aquire the lock!
	_, err = lib.FlockLock(lockFile, 100*time.Millisecond, false)
	require.Error(t, err)
	t.Log(err)

	// wait until the 3rd is released. We should now be able to acquire the lock again without any issues from the timeouted lock!
	unlock4, err := lib.FlockLock(lockFile, 200*time.Millisecond, false)
	require.NoError(t, err)

	require.NoError(t, unlock4())
}

func TestFlockConcurrent(t *testing.T) {
	// spawn 100 goroutines that try to acquire the same lock, each runs for xms, then unlocks again.

	lockFile := lib.FlockShuffleLockFile("/tmp", 1)
	t.Logf("Using lock_file='%s'...", lockFile)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := lib.FlockLock(lockFile, 10000*time.Millisecond, false)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
			require.NoError(t, unlock())
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func TestFlockTimeout(t *testing.T) {
	lockFile := path.Join(t.TempDir(), "flock_test.lock")

	_, err := lib.FlockLock(lockFile, 100*time.Millisecond, false)
	require.NoError(t, err)

	// Try to acquire a second lock...
	_, err = lib.FlockLock(lockFile, 100*time.Millisecond, false)
	require.Error(t, err)

	assert.Equal(t, "Timeout while trying to acquire lock", err.Error())
}
