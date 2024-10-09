package lib

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func FlockShuffleLockFile(dir string, count int) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
	if err != nil {
		log.Fatalf("flockShuffleLockFile: Failed to generate secure random number: %v", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%d.lock", n.Int64()+1))
}

func FlockLock(lockFile string, timeoutSec int, dryRun bool) func() {
	if dryRun {
		log.Println("Skipping flock - dry run mode is active")
		return func() {}
	}

	lockFd, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Failed to open lock file: %v", err)
	}

	_, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	err = syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX)
	if err != nil {
		log.Fatalf("Failed to acquire lock: %v", err)
	}

	log.Printf("Got lock on '%s'!", lockFile)

	return func() {
		err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_UN)
		if err != nil {
			log.Printf("Warning: Failed to release lock: %v", err)
		}
		lockFd.Close()
		log.Printf("Released lock from '%s'", lockFile)
	}
}
