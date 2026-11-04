package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerWriteReadsBlockAndPreservesNextCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my notes.txt")
	if err := os.WriteFile(path, []byte("previous contents"), 0600); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(strings.NewReader("первая\r\n\r\nпоследняя\r\nexit\r\n"))
	_, exit, err := ExecCmd("write my notes.txt 3", dir, scanner, nil)
	if err != nil || exit {
		t.Fatalf("unexpected command result: %v, %v", exit, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "первая\n\nпоследняя\n" {
		t.Fatalf("unexpected contents: %q, %v", got, err)
	}
	if !scanner.Scan() || scanner.Text() != "exit" {
		t.Fatal("next command was lost or consumed")
	}
}

func TestHandlerWriteRejectsInputWithoutChangingFile(t *testing.T) {
	for _, tc := range []struct{ name, args, input string }{
		{"missing count", "notes.txt", ""},
		{"invalid count", "notes.txt nope", ""},
		{"zero count", "notes.txt 0", ""},
		{"negative count", "notes.txt -1", ""},
		{"incomplete input", "notes.txt 2", "only one\n"},
		{"long line", "notes.txt 2", strings.Repeat("x", 10001) + "\nsecond\nexit\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "notes.txt")
			if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			scanner := bufio.NewScanner(strings.NewReader(tc.input))
			if err := handlerWrite(tc.args, dir, scanner); err == nil {
				t.Fatal("expected error")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "keep" {
				t.Fatalf("file changed: %q, %v", got, err)
			}
			if tc.name == "long line" && (!scanner.Scan() || scanner.Text() != "exit") {
				t.Fatal("input block was not fully consumed")
			}
		})
	}
}
