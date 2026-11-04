package cli

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kha-Beleh/KFM/internal/postgres"
)

// открывает отдельную схему тестовой PostgreSQL
func testDatabase(t *testing.T) (*postgres.Store, string) {
	t.Helper()
	address := os.Getenv("KFM_TEST_DATABASE_URL")
	if address == "" {
		t.Skip("задайте KFM_TEST_DATABASE_URL")
	}
	admin, err := postgres.Open(address)
	if err != nil {
		t.Fatal(err)
	}
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "kfm_test_" + hex.EncodeToString(random[:])
	if _, err = admin.DB.Exec("CREATE SCHEMA " + name); err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	query := uri.Query()
	query.Set("search_path", name)
	uri.RawQuery = query.Encode()
	store, err := postgres.Open(uri.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close(); admin.DB.Exec("DROP SCHEMA " + name + " CASCADE"); admin.Close() })
	if err = store.Migrate(); err != nil {
		t.Fatal(err)
	}
	return store, uri.String()
}

func TestUsersShareSystemPathsAndHaveSeparateAudit(t *testing.T) {
	store, address := testDatabase(t)
	t.Setenv("KFM_DATABASE_URL", address)
	dir := t.TempDir()
	unusedHome := filepath.Join(t.TempDir(), "must-not-be-created")
	t.Setenv("KFM_HOME", unusedHome)
	path := filepath.Join(t.TempDir(), "input.txt")
	script := strings.Join([]string{
		"touch forbidden.txt",
		"register first", "first password 123",
		"login first", "first password 123",
		"cd " + dir,
		"touch shared.txt", "write shared.txt 1", "shared contents",
		"logout",
		"register second", "second password 123",
		"login second", "second password 123",
		"cat " + filepath.Join(dir, "shared.txt"),
		"rm shared.txt", "exit",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = old; input.Close() }()
	output := captureOutput(t, func() {
		if err := Run(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "shared contents") || strings.Count(output, "ошибка:") != 1 {
		t.Fatalf("unexpected output: %s", output)
	}
	if strings.Contains(output, "password 123") {
		t.Fatal("password printed")
	}
	if _, err = os.Stat(unusedHome); !os.IsNotExist(err) {
		t.Fatal("created a personal directory")
	}
	if _, err = os.Stat(filepath.Join(dir, "shared.txt")); !os.IsNotExist(err) {
		t.Fatal("second user could not remove shared file")
	}
	for _, check := range []struct{ user, command string }{{"first", "write"}, {"second", "cat"}, {"second", "rm"}} {
		var count int
		err = store.DB.QueryRow(`SELECT count(*) FROM operations o JOIN users u ON u.id=o.user_id WHERE u.username=$1 AND o.operation_type=$2 AND o.status='success'`, check.user, check.command).Scan(&count)
		if err != nil || count != 1 {
			t.Fatalf("wrong audit user for %s: %d %v", check.command, count, err)
		}
	}
}
