package textfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteToFileReplacesContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("long previous contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteToFile(path, [][]byte{[]byte("текст"), {}, []byte("end")}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "текст\n\nend\n" {
		t.Fatalf("unexpected contents: %q", got)
	}
}

func TestWriteToFileInvalidInputPreservesFile(t *testing.T) {
	fullLine := bytes.Repeat([]byte("a"), MaxLineSize)
	var tooLarge [][]byte
	for i := 0; i < 100; i++ {
		tooLarge = append(tooLarge, fullLine)
	}
	for name, lines := range map[string][][]byte{
		"long line":     {bytes.Repeat([]byte("a"), MaxLineSize+1)},
		"invalid UTF-8": {{0xff}},
		"total size":    tooLarge,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notes.txt")
			if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := WriteToFile(path, lines); err == nil {
				t.Fatal("expected input error")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "keep" {
				t.Fatalf("file changed: %q, %v", got, err)
			}
		})
	}
}

func TestWriteToFileRequiresExistingRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.txt")
	if err := WriteToFile(path, [][]byte{[]byte("text")}); err == nil {
		t.Fatal("expected error for absent file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected file status: %v", err)
	}
	if err := WriteToFile(dir, [][]byte{[]byte("text")}); err == nil {
		t.Fatal("expected error for directory")
	}
}
