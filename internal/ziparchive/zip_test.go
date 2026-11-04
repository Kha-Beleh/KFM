package ziparchive

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestZIPRoundTrip(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = root.Mkdir("source", 0700); err != nil {
		t.Fatal(err)
	}
	if err = root.WriteFile("source/текст.txt", []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = root.Mkdir("source/empty", 0700); err != nil {
		t.Fatal(err)
	}
	if err = Create(root, "source", "data.zip"); err != nil {
		t.Fatal(err)
	}
	if err = Create(root, "source", "data.zip"); err == nil {
		t.Fatal("overwrote archive")
	}
	if err = Extract(root, "data.zip", "out"); err != nil {
		t.Fatal(err)
	}
	data, err := root.ReadFile("out/source/текст.txt")
	if err != nil || string(data) != "hello" {
		t.Fatalf("round trip: %q %v", data, err)
	}
	if err = Extract(root, "data.zip", "out"); err == nil {
		t.Fatal("overwrote destination")
	}
}

func TestZIPRejectsDangerousNames(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "C:/windows", `..\escape`, "file:stream", "NUL.txt", "dir/../escape", "file.", "file ", "dir//file"} {
		t.Run(name, func(t *testing.T) {
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			makeArchive(t, root, []string{name}, []byte("data"), false)
			if err = Extract(root, "test.zip", "out"); err == nil {
				t.Fatal("accepted unsafe archive")
			}
			if _, err = root.Stat("out"); !os.IsNotExist(err) {
				t.Fatal("created destination for invalid archive")
			}
		})
	}
}

func TestZIPRejectsLinksDuplicatesAndBombs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		data  []byte
		link  bool
	}{
		{"link", []string{"link"}, []byte("../outside"), true},
		{"duplicate", []string{"file", "FILE"}, []byte("x"), false},
		{"count", makeNames(MaxFiles + 1), nil, false},
		{"size", []string{"large"}, bytes.Repeat([]byte("x"), MaxFileSize+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			makeArchive(t, root, tc.names, tc.data, tc.link)
			if err = Extract(root, "test.zip", "out"); err == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}

func TestZIPRejectsCorruptArchive(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = os.WriteFile(filepath.Join(dir, "test.zip"), []byte("not zip"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Extract(root, "test.zip", "out"); err == nil {
		t.Fatal("accepted corrupt archive")
	}
}

func TestZIPUsesIndependentSystemPaths(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.txt")
	archive := filepath.Join(t.TempDir(), "archive.zip")
	destination := filepath.Join(t.TempDir(), "unpacked")
	if err := os.WriteFile(source, []byte("system paths"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(source, archive); err != nil {
		t.Fatal(err)
	}
	if err := ExtractFile(archive, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "source.txt"))
	if err != nil || string(data) != "system paths" {
		t.Fatalf("contents: %q %v", data, err)
	}
}

func makeArchive(t *testing.T, root *os.Root, names []string, data []byte, link bool) {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if link {
			header.SetMode(os.ModeSymlink | 0777)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("test.zip", buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func makeNames(count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = "file" + string(rune(0x1000+i))
	}
	return names
}
