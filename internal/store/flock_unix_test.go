//go:build !windows

package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLockFileBlocksConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock1, err := lockFile(path)
	if err != nil {
		t.Fatalf("first lockFile: %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		lock2, err := lockFile(path)
		if err != nil {
			t.Errorf("second lockFile: %v", err)
			return
		}
		close(acquired)
		_ = lock2.Close()
	}()

	select {
	case <-acquired:
		t.Fatal("second lock acquired while first still held")
	case <-time.After(200 * time.Millisecond):
	}

	_ = lock1.Close()

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock not acquired after first released")
	}
}

func TestLockFileDifferentPathsIndependent(t *testing.T) {
	dir := t.TempDir()

	lock1, err := lockFile(filepath.Join(dir, "a.lock"))
	if err != nil {
		t.Fatalf("lock a: %v", err)
	}
	defer func() { _ = lock1.Close() }()

	lock2, err := lockFile(filepath.Join(dir, "b.lock"))
	if err != nil {
		t.Fatalf("lock b (should not block): %v", err)
	}
	_ = lock2.Close()
}

func TestLockFileReacquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock1, err := lockFile(path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	_ = lock1.Close()

	lock2, err := lockFile(path)
	if err != nil {
		t.Fatalf("second lock after release: %v", err)
	}
	_ = lock2.Close()
}
