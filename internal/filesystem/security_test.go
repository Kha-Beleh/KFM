package filesystem

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSecurityConcurrentCreateDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-file.txt")
	start := make(chan struct{})
	var group sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			if CreateFile(path) == nil {
				successes.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected one successful creator, got %d", successes.Load())
	}
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if CreateFile(path) == nil {
		t.Fatal("overwrote existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatal("existing contents changed")
	}
}

func TestSecurityCreateRejectsExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if CreateFile(link) == nil {
		t.Fatal("created through an existing symlink")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep" {
		t.Fatal("symlink target was overwritten")
	}
}
