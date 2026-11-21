package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kha-Beleh/KFM/internal/structured"
)

func TestStructuredHandlersUseWorkDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(strings.NewReader("{\"value\":42}\n"))
	if err := handlerStructured("json-write", "data.json 1", dir, scanner); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = structured.DecodeJSON(data); err != nil {
		t.Fatal(err)
	}
	output := captureOutput(t, func() {
		if err := handlerStructured("json-read", path, dir, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "42") {
		t.Fatalf("JSON output: %s", output)
	}
	path = filepath.Join(dir, "data.xml")
	if err = os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	scanner = bufio.NewScanner(strings.NewReader("<!DOCTYPE a><a/>\n"))
	if err = handlerStructured("xml-write", "data.xml 1", dir, scanner); err == nil {
		t.Fatal("accepted DTD")
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatal("invalid XML changed the file")
	}
}
