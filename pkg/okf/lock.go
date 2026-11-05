package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// bundleWriteLockFile is the dot-prefixed lock file created inside a bundle root
	// to serialize concurrent local Read-Modify-Write transactions across processes.
	// Dot-prefixed files are ignored by LoadBundle, Validate, and vault sync scanners.
	bundleWriteLockFile = ".okf-write.lock"

	defaultLockTimeout  = 10 * time.Second
	defaultStaleLockAge = 30 * time.Second
)

// ErrConcurrentModification is returned by SaveConcept when an existing concept
// loaded via LoadBundle was modified on disk by another writer before SaveConcept
// acquired the bundle lock (Optimistic Concurrency Check / CAS failure).
var ErrConcurrentModification = errors.New("concurrent modification detected")

// ErrConceptNotFound is returned by UpdateConcept when the target concept ID
// does not exist in the loaded bundle.
var ErrConceptNotFound = errors.New("concept not found")

func hashConceptContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// WithBundleLock acquires an exclusive cross-process write lock for the bundle
// at bundleDir, executes fn, and releases the lock.
func WithBundleLock(bundleDir string, fn func() error) error {
	return withBundleLockConfig(bundleDir, defaultLockTimeout, defaultStaleLockAge, fn)
}

func withBundleLockConfig(bundleDir string, timeout, staleAge time.Duration, fn func() error) error {
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bundle directory for lock: %w", err)
	}

	lockPath, err := ensureWithinRoot(bundleDir, bundleWriteLockFile)
	if err != nil {
		return fmt.Errorf("unsafe bundle lock path: %w", err)
	}

	releaseFileLock, err := acquireFileLock(lockPath, timeout, staleAge)
	if err != nil {
		return err
	}
	defer releaseFileLock()

	return fn()
}

func acquireFileLock(lockPath string, timeout, staleAge time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	backoff := 500 * time.Microsecond
	const maxBackoff = 15 * time.Millisecond

	for {
		// #nosec G304 -- lockPath is confined to bundle root via ensureWithinRoot
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			meta := fmt.Sprintf("pid=%d\nacquired_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			_, _ = f.WriteString(meta)
			_ = f.Close()

			return func() {
				_ = os.Remove(lockPath)
			}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to acquire bundle write lock %q: %w", lockPath, err)
		}

		if staleAge > 0 && isLockStale(lockPath, staleAge) {
			_ = os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for bundle write lock %q after %s", lockPath, timeout)
		}

		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func isLockStale(lockPath string, staleAge time.Duration) bool {
	// #nosec G304 -- lockPath is confined to bundle root via ensureWithinRoot
	if data, err := os.ReadFile(lockPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if after, ok := strings.CutPrefix(strings.TrimSpace(line), "acquired_at="); ok {
				if ts, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(after)); parseErr == nil {
					return time.Since(ts) > staleAge
				}
			}
		}
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > staleAge
}
