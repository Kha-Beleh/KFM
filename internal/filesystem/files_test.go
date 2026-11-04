package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFile(t *testing.T) {
	t.Run("new file with spaces and Unicode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "новый файл.txt")
		if err := CreateFile(path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || info.Size() != 0 {
			t.Fatalf("expected empty regular file: %v", info)
		}
	})
	t.Run("existing file is preserved", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "existing.txt")
		if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := CreateFile(path); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("expected existence error, got %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "keep" {
			t.Fatalf("existing file changed: %q, %v", got, err)
		}
	})
	t.Run("absent parent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "file.txt")
		if err := CreateFile(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected missing parent error, got %v", err)
		}
	})
	t.Run("directory is preserved", func(t *testing.T) {
		dir := t.TempDir()
		if err := CreateFile(dir); err == nil {
			t.Fatal("expected error for directory")
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("directory changed: %v", err)
		}
	})
	if err := CreateFile(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	entries, err := ListDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("expected empty directory: %v, %v", entries, err)
	}
	path := filepath.Join(dir, "файл с пробелами.txt")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}
	entries, err = ListDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, entry := range entries {
		got[entry.Name()] = entry.IsDir()
	}
	if len(got) != 2 || !got["subdir"] {
		t.Fatalf("unexpected entries: %v", got)
	}
	if isDir, ok := got[filepath.Base(path)]; !ok || isDir {
		t.Fatalf("regular file missing or marked as directory: %v", got)
	}
	if _, err := ListDir(path); err == nil {
		t.Error("expected error when listing a file")
	}
	if _, err := ListDir(filepath.Join(dir, "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected missing directory error, got %v", err)
	}
}

func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "удалить файл.txt")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file still exists: %v", err)
	}
	if err := RemoveFile(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected missing file error on second removal, got %v", err)
	}
	for _, name := range []string{"empty", "nonempty"} {
		t.Run(name+" directory", func(t *testing.T) {
			subdir := filepath.Join(dir, name)
			if err := os.Mkdir(subdir, 0700); err != nil {
				t.Fatal(err)
			}
			if name == "nonempty" {
				if err := os.WriteFile(filepath.Join(subdir, "keep.txt"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := RemoveFile(subdir); err == nil {
				t.Fatal("expected directory removal to be rejected")
			}
			if info, err := os.Stat(subdir); err != nil || !info.IsDir() {
				t.Fatalf("directory was removed: %v", err)
			}
		})
	}
}
