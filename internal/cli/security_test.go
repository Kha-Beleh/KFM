package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kha-Beleh/KFM/internal/auth"
	"github.com/Kha-Beleh/KFM/internal/postgres"
)

func TestSecurityCommandSeparatorsAreNotExecuted(t *testing.T) {
	for _, argument := range []string{"normal.txt; touch injected.txt", "normal.txt && touch injected.txt", "normal.txt & echo test > injected.txt"} {
		t.Run(argument, func(t *testing.T) {
			dir := t.TempDir()
			_, exit, _ := ExecCmd("touch "+argument, dir, nil, nil)
			if exit {
				t.Fatal("filename unexpectedly terminated command loop")
			}
			for _, name := range []string{"normal.txt", "injected.txt"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("interpreted shell syntax: %s", name)
				}
			}
		})
	}
}

func TestSecurityTerminalEscapeSequencesAreNotPrinted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "controls.txt")
	payload := "safe\x1b[2J\x1b]52;c;AAAA\x07\x00\n"
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	output := captureOutput(t, func() {
		if err := handlerCat(path, dir); err != nil {
			t.Fatal(err)
		}
	})
	for _, control := range []rune{'\x1b', '\x07', 0} {
		if strings.ContainsRune(output, control) {
			t.Errorf("printed terminal control U+%04X", control)
		}
	}
	if !strings.Contains(output, "safe") || !strings.Contains(output, `\u001b`) {
		t.Fatal("lost text or did not show escaped controls")
	}
}

func TestSecurityWriteValidationPreservesOriginalFile(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		count       int
	}{
		{"incomplete", "one\n", 2},
		{"invalid UTF-8", string([]byte{255}) + "\n", 1},
		{"long line", strings.Repeat("x", 10001) + "\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "keep.txt")
			if err := os.WriteFile(path, []byte("keep original"), 0600); err != nil {
				t.Fatal(err)
			}
			args := "keep.txt 1"
			if tc.count == 2 {
				args = "keep.txt 2"
			}
			if err := handlerWrite(args, dir, bufio.NewScanner(strings.NewReader(tc.input))); err == nil {
				t.Fatal("accepted invalid input")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "keep original" {
				t.Fatal("validation failure changed file")
			}
		})
	}
}

func TestSecuritySQLInjectionAgainstPostgreSQL(t *testing.T) {
	store, _ := testDatabase(t)
	user, err := store.CreateUser("existing-user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{`' OR '1'='1' --`, `x'; DROP TABLE users; --`} {
		if _, err = store.GetUser(payload); err == nil {
			t.Fatal("injection returned an existing user")
		}
		id, err := store.Start(user.ID, "write", payload)
		if err != nil {
			t.Fatal(err)
		}
		if err = store.Finish(id, user.ID, &postgres.File{Path: payload, Size: 1}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = store.DB.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatalf("users table affected: %d %v", count, err)
	}
	if err = store.DB.QueryRow("SELECT count(*) FROM files").Scan(&count); err != nil || count != 2 {
		t.Fatalf("files table affected: %d %v", count, err)
	}
}

func TestSecurityWrongAuditUserRollsBackMetadata(t *testing.T) {
	store, _ := testDatabase(t)
	one, err := store.CreateUser("first", "hash")
	if err != nil {
		t.Fatal(err)
	}
	two, err := store.CreateUser("second", "hash")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Start(one.ID, "write", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Finish(id, two.ID, &postgres.File{Path: "note.txt", Size: 42}, nil); err == nil {
		t.Fatal("accepted wrong command author")
	}
	var count int
	if err = store.DB.QueryRow("SELECT count(*) FROM files").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed transaction left metadata")
	}
	if err = store.Finish(id, one.ID, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityLoginErrorsAndAttemptLimit(t *testing.T) {
	store, _ := testDatabase(t)
	service := auth.Service{Store: store}
	if _, err := service.Register("student", "student password 123"); err != nil {
		t.Fatal(err)
	}
	_, missing := service.Login("missing", "wrong password")
	_, wrong := service.Login("student", "wrong password")
	if missing == nil || wrong == nil || missing.Error() != wrong.Error() {
		t.Fatal("login discloses whether account exists")
	}
	for i := 0; i < 3; i++ {
		if _, err := service.Login("student", "wrong password"); err == nil {
			t.Fatal("accepted wrong password")
		}
	}
	if _, err := service.Login("student", "student password 123"); err == nil {
		t.Fatal("attempt limit did not block login")
	}
}

func TestSecurityConcurrentAuditCompletion(t *testing.T) {
	store, _ := testDatabase(t)
	user, err := store.CreateUser("student", "hash")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Start(user.ID, "write", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		size int64
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := int64(1); i <= 2; i++ {
		go func(size int64) {
			<-start
			results <- result{size, store.Finish(id, user.ID, &postgres.File{Path: "note.txt", Size: size}, nil)}
		}(i)
	}
	close(start)
	successes := 0
	var winner int64
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err == nil {
			successes++
			winner = got.size
		}
	}
	if successes != 1 {
		t.Fatalf("completed one operation %d times", successes)
	}
	var size int64
	if err = store.DB.QueryRow("SELECT size FROM files WHERE owner_id=$1 AND location=$2", user.ID, "note.txt").Scan(&size); err != nil || size != winner {
		t.Fatalf("losing transaction changed metadata: size=%d winner=%d err=%v", size, winner, err)
	}
}

func TestSecurityPasswordsAreNotStoredInAudit(t *testing.T) {
	store, address := testDatabase(t)
	t.Setenv("KFM_DATABASE_URL", address)
	password := "private password 123"
	script := "register student\n" + password + "\nlogin student\n" + password + "\nhistory\nexit\n"
	path := filepath.Join(t.TempDir(), "input.txt")
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
	if strings.Contains(output, password) {
		t.Fatal("password disclosed in output")
	}
	user, err := store.GetUser("student")
	if err != nil || user.PasswordHash == password || !auth.CheckPassword(user.PasswordHash, password) {
		t.Fatalf("password not stored as a usable hash: %v", err)
	}
	rows, err := store.DB.Query("SELECT operation_type,path,error FROM operations")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var command, path, message string
		if err = rows.Scan(&command, &path, &message); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(command+path+message, password) || strings.Contains(command+path+message, user.PasswordHash) {
			t.Fatal("password or hash written into audit")
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}
