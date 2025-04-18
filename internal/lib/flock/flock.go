package flock

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrTimeoutExceeded = fmt.Errorf("timeout exceeded while trying to acquire lock")

func ShuffleLockFile(dir string, count int) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
	if err != nil {
		log.Panicf("flockShuffleLockFile: Failed to generate secure random number: %v", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%d.lock", n.Int64()+1))
}

var noop = func() error { return nil }

type Flock struct {
	path          string
	timeout       time.Duration
	retryInterval time.Duration
}

func New(path string) *Flock {
	return &Flock{
		path:          path,
		retryInterval: time.Second,
		timeout:       2 * time.Second,
	}
}

func (f *Flock) WithTimeout(timeout time.Duration) *Flock {
	f.timeout = timeout
	return f
}

func (f *Flock) WithRetryInterval(retryInterval time.Duration) *Flock {
	f.retryInterval = retryInterval
	return f
}

func (f *Flock) Lock(dryRun bool) (func() error, error) {
	if dryRun {
		log.Println("Skipping flock - dry run mode is active")
		return noop, nil
	}

	lockFd, err := os.OpenFile(f.path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	ticker := time.NewTicker(f.retryInterval)
	defer ticker.Stop()

	for {
		err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			log.Printf("Got lock on %q", f.path)

			return func() error {
				if err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_UN); err != nil {
					return fmt.Errorf("failed to release lock: %w", err)
				}
				if err := lockFd.Close(); err != nil {
					return fmt.Errorf("failed to close lock file: %w", err)
				}

				log.Printf("Released lock from %q", f.path)
				return nil
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("unexpected error trying to acquire lock: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, ErrTimeoutExceeded
		case <-ticker.C:
			// continue and try again
		}
	}
}

func GetDefaultFlockCount() int {
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
