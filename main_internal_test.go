package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlockShuffleLockFile(t *testing.T) {
	// Test the flockShuffleLockFile function
	for i := 0; i < 100; i++ {
		lockfile := flockShuffleLockFile("test", 2)

		// t.Log(lockfile)
		assert.True(t, lockfile == "test/1.lock" || lockfile == "test/2.lock")
	}

}
