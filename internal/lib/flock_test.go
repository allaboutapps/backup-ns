package lib_test

import (
	"testing"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/assert"
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
	unlock := lib.FlockLock(lockFile, 100*time.Millisecond, true)
	unlock2 := lib.FlockLock(lockFile, 100*time.Millisecond, true)
	unlock()
	unlock2()
}

// func TestFlockDirNotFound(t *testing.T) {
// 	defer func() {
// 		t.Log("Recover!")
// 		if r := recover(); r != nil {
// 			t.Fatalf("Expected error: %v", r)
// 		}
// 		t.Log("Recovered from expected dir not found.")
// 	}()

// 	// Test the flock function with a non-existing directory
// 	lockFile := lib.FlockShuffleLockFile("/tmp/this-dir-is-non-existing", 1)
// 	t.Logf("Using lock_file='%s'...", lockFile)
// 	lib.FlockLock(lockFile, 100*time.Millisecond, false)
// }

func TestFlock(t *testing.T) {
	// Test the flock function
	lockFile := lib.FlockShuffleLockFile("/tmp", 1)
	t.Logf("Using lock_file='%s'...", lockFile)

	unlock := lib.FlockLock(lockFile, 100*time.Millisecond, false)

	go func() {
		// Release the first lock after 1 sec, second now has the lock!
		time.Sleep(100 * time.Millisecond)
		unlock()
	}()

	// Try to acquire a second lock...
	unlock2 := lib.FlockLock(lockFile, 100*time.Millisecond, false)

	go func() {
		// Release the second lock after 2 sec, the third will never get the lock!
		time.Sleep(200 * time.Millisecond)
		unlock2()
	}()

	// The third will now fail to aquire the lock!
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Expected error: %v", r)
			}
			t.Log("Recovered from failed to aquire third lock panic")
		}()
		lib.FlockLock(lockFile, 100*time.Millisecond, false)
	}()
}
