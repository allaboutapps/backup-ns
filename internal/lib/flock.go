package lib

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func FlockShuffleLockFile(dir string, count int) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
	if err != nil {
		log.Panicf("flockShuffleLockFile: Failed to generate secure random number: %v", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%d.lock", n.Int64()+1))
}

var noop = func() error { return nil }

func FlockLock(lockFile string, timeout time.Duration, dryRun bool) (func() error, error) {
	if dryRun {
		log.Println("Skipping flock - dry run mode is active")
		return noop, nil
	}

	lockFd, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return noop, fmt.Errorf("Failed to open lock file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lockChan := make(chan error, 1)
	mu := &sync.Mutex{}

	go func() {
		mu.Lock()
		defer mu.Unlock()
		lockChan <- syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX)
	}()

	select {
	case <-ctx.Done():
		mu.Lock()
		defer mu.Unlock()
		lockFd.Close()
		return noop, fmt.Errorf("Timeout while trying to acquire lock: %w", err)
	case err := <-lockChan:
		if err != nil {
			mu.Lock()
			defer mu.Unlock()
			lockFd.Close()
			return noop, fmt.Errorf("Failed to acquire lock: %w", err)
		}
	}

	log.Printf("Got lock on '%s'!", lockFile)

	return func() error {
		mu.Lock()
		defer mu.Unlock()
		if err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_UN); err != nil {
			return fmt.Errorf("Failed to release lock: %w", err)
		}
		lockFd.Close()
		log.Printf("Released lock from '%s'", lockFile)
		return nil
	}, nil
}

func getDefaultFlockCount() int {
	cmd := exec.Command("nproc", "--all")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting nproc: %v", err)
		return 2
	}
	nproc, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		log.Printf("Error parsing nproc: %v", err)
		return 2
	}
	if nproc < 2 {
		return 1
	}
	return nproc / 2
}
