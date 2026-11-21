package textfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileContentsAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"empty", nil, false},
		{"UTF-8 and blank lines", []byte("Привет\n\nмир\n"), false},
		{"CRLF preserved", []byte("one\r\ntwo\r\n"), false},
		{"larger than old buffer", bytes.Repeat([]byte("a"), 4097), false},
		{"exact file limit", bytes.Repeat([]byte("a"), MaxFileSize), false},
		{"over file limit", bytes.Repeat([]byte("a"), MaxFileSize+1), true},
		{"invalid UTF-8", []byte{0xff, 0xfe}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "текст.txt")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadFile(path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr && !bytes.Equal(got, tc.data) {
				t.Fatalf("contents differ: got %d bytes, want %d", len(got), len(tc.data))
			}
			if tc.wantErr && len(got) != 0 {
				t.Fatal("rejected file returned partial contents")
			}
		})
	}
}

func TestReadFileInvalidPaths(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadFile(filepath.Join(dir, "missing.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected missing file error, got %v", err)
	}
	if _, err := ReadFile(dir); err == nil {
		t.Error("expected directory to be rejected")
	}
	if _, err := ReadFile(""); err == nil {
		t.Error("expected empty path to be rejected")
	}
}

func TestWriteToFileBoundarySizes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines [][]byte
	}{
		{"exact line limit", [][]byte{bytes.Repeat([]byte("a"), MaxLineSize)}},
		// 100 lines of 9999 bytes plus one LF each = 1,000,000 bytes.
		{"exact file limit", bytesToLines(bytes.Repeat([]byte("a"), 9999), 100)},
		{"empty content", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notes.txt")
			if err := os.WriteFile(path, []byte("old contents"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := WriteToFile(path, tc.lines); err != nil {
				t.Fatal(err)
			}
			var want []byte
			for _, line := range tc.lines {
				want = append(want, line...)
				want = append(want, '\n')
			}
			got, err := ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("round trip failed: %d bytes, error %v", len(got), err)
			}
		})
	}
}

func bytesToLines(line []byte, count int) [][]byte {
	lines := make([][]byte, count)
	for i := range lines {
		lines[i] = line
	}
	return lines
}
