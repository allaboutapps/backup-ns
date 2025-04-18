package flock_test

import (
	"context"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlockShuffleLockFile(t *testing.T) {
	for i := 0; i < 100; i++ {
		lockfile := flock.ShuffleLockFile("test", 2)

		assert.True(t, lockfile == "test/1.lock" || lockfile == "test/2.lock")
	}

}

func TestFlockDryRun(t *testing.T) {
	lockFile := flock.ShuffleLockFile("/tmp", 1)

	// the following should not be possible unless dryrun is active, same lock
	unlock, err := flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(true)
	require.NoError(t, err)
	unlock2, err := flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(true)
	require.NoError(t, err)
	require.NoError(t, unlock())
	require.NoError(t, unlock2())
}

func TestFlockLockAfterUnlock(t *testing.T) {
	lockFile := flock.ShuffleLockFile("/tmp", 1)

	unlock, err := flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	err = unlock()
	require.NoError(t, err)

	// Try to acquire the lock again
	unlock, err = flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	err = unlock()
	require.NoError(t, err)
}

func TestFlockDirNotFound(t *testing.T) {
	lockFile := flock.ShuffleLockFile("/tmp/this-dir-is-non-existing", 1)
	_, err := flock.New(lockFile).WithTimeout(1 * time.Millisecond).Lock(false)
	require.Error(t, err)
}

func TestFlockTimeoutSequence(t *testing.T) {
	lockFile := flock.ShuffleLockFile("/tmp", 1)

	unlock, err := flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	go func() {
		// Release the first lock after 100 ms, second now has the lock!
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, unlock())
	}()

	// Try to acquire a second lock...
	unlock2, err := flock.New(lockFile).WithTimeout(200 * time.Millisecond).WithRetryInterval(10 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	go func() {
		// Release the second lock after a few time the third will never get the lock!
		time.Sleep(200 * time.Millisecond)
		require.NoError(t, unlock2())
	}()

	// The third will now fail to aquire the lock!
	_, err = flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(false)
	require.Error(t, err)
	assert.ErrorIs(t, err, flock.ErrTimeoutExceeded)

	// wait until the 3rd is released. We should now be able to acquire the lock again without any issues from the timeouted lock!
	unlock4, err := flock.New(lockFile).WithTimeout(200 * time.Millisecond).WithRetryInterval(10 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	require.NoError(t, unlock4())
}

func TestFlockConcurrent(t *testing.T) {
	// spawn 100 goroutines that try to acquire the same lock, each runs for xms, then unlocks again.

	lockFile := flock.ShuffleLockFile("/tmp", 1)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := flock.New(lockFile).WithTimeout(10000 * time.Millisecond).WithRetryInterval(time.Millisecond).Lock(false)
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

	_, err := flock.New(lockFile).WithTimeout(100 * time.Millisecond).Lock(false)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	testFinishedChan := make(chan struct{})
	go func() {
		// Try to acquire a second lock...
		_, err = flock.New(lockFile).WithTimeout(100 * time.Millisecond).WithRetryInterval(time.Millisecond).Lock(false)
		require.Error(t, err)
		assert.ErrorIs(t, err, flock.ErrTimeoutExceeded)

		testFinishedChan <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	case <-testFinishedChan:
		// Test finished successfully
	}
}
