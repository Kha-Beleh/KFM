package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Tests using standard input/output run sequentially; do not add t.Parallel.
func captureOutput(t *testing.T, action func()) string {
	t.Helper()
	output, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = output
	defer func() {
		os.Stdout = previous
		output.Close()
	}()
	action()
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestExecCmdControlCommands(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		command       string
		exit, wantErr bool
	}{
		{"", false, false},
		{"help", false, false},
		{"exit", true, false},
		{"unknown-command", false, true},
		{"touch", false, true},
		{"rm", false, true},
		{"cat", false, true},
		{"write", false, true},
	} {
		t.Run(tc.command, func(t *testing.T) {
			captureOutput(t, func() {
				gotDir, exit, err := ExecCmd(tc.command, dir, nil, nil)
				if gotDir != dir || exit != tc.exit || (err != nil) != tc.wantErr {
					t.Errorf("unexpected result: dir=%q exit=%v err=%v", gotDir, exit, err)
				}
			})
		})
	}
	output := captureOutput(t, handlerHelp)
	for _, command := range []string{"help", "ls", "cd", "drives", "touch", "rm", "cat", "write", "exit"} {
		if !strings.Contains(output, command) {
			t.Errorf("help does not mention %q", command)
		}
	}
}

func TestExecCmdCRUD(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		name := "relative path"
		if absolute {
			name = "absolute path"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			arg := "файл с пробелами.txt"
			path := filepath.Join(dir, arg)
			if absolute {
				arg = path
			}
			scanner := bufio.NewScanner(strings.NewReader("первая\n\nпоследняя\n"))
			captureOutput(t, func() {
				for _, command := range []string{"touch   " + arg, "write " + arg + " 3"} {
					gotDir, exit, err := ExecCmd(command, dir, scanner, nil)
					if err != nil || exit || gotDir != dir {
						t.Fatalf("%q: dir=%q exit=%v err=%v", command, gotDir, exit, err)
					}
				}
			})
			output := captureOutput(t, func() {
				if _, _, err := ExecCmd("cat "+arg, dir, scanner, nil); err != nil {
					t.Fatal(err)
				}
			})
			if output != "первая\n\nпоследняя\n" {
				t.Fatalf("cat output: %q", output)
			}
			if _, _, err := ExecCmd("touch "+arg, dir, scanner, nil); err == nil {
				t.Fatal("touch should reject an existing file")
			}
			if _, _, err := ExecCmd("rm "+arg, dir, scanner, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("rm did not delete file: %v", err)
			}
			for _, command := range []string{"cat " + arg, "rm " + arg} {
				if _, _, err := ExecCmd(command, dir, scanner, nil); err == nil {
					t.Errorf("%q should reject missing file", command)
				}
			}
		})
	}
}

func TestListDirCommand(t *testing.T) {
	dir := t.TempDir()
	output := captureOutput(t, func() {
		if err := handlerListDir("", dir); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "каталог пуст") {
		t.Fatalf("unexpected empty directory output: %q", output)
	}
	subdir := filepath.Join(dir, "sub dir")
	if err := os.Mkdir(subdir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "note.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"sub dir", subdir} {
		output = captureOutput(t, func() {
			if err := handlerListDir(arg, dir); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(output, "note.txt") {
			t.Errorf("listing %q did not show file: %q", arg, output)
		}
	}
}

func TestChangeDir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub dir")
	if err := os.Mkdir(subdir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, arg, want string
		wantErr         bool
	}{
		{"relative directory", "sub dir", subdir, false},
		{"absolute directory", subdir, subdir, false},
		{"current directory", ".", dir, false},
		{"parent directory", "..", filepath.Dir(dir), false},
		{"missing relative path", "missing", dir, true},
		{"missing absolute path", filepath.Join(dir, "missing"), dir, true},
		{"reject relative file", "file.txt", dir, true},
		{"reject absolute file", file, dir, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := handlerChangeDir(tc.arg, dir)
			if (err != nil) != tc.wantErr || filepath.Clean(got) != filepath.Clean(tc.want) {
				t.Errorf("cd %q: got dir=%q err=%v; want dir=%q, error=%v", tc.arg, got, err, tc.want, tc.wantErr)
			}
		})
	}
	if runtime.GOOS == "windows" {
		for _, arg := range []string{"/", "\\"} {
			got, err := handlerChangeDir(arg, dir)
			want := filepath.VolumeName(dir) + string(filepath.Separator)
			if err != nil || filepath.Clean(got) != filepath.Clean(want) {
				t.Errorf("Windows root %q: got %q, %v; want %q", arg, got, err, want)
			}
		}
	}
}

func TestRunReadsCommandsAndTextFromSameInput(t *testing.T) {
	_, address := testDatabase(t)
	t.Setenv("KFM_DATABASE_URL", address)
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "stdin.txt")
	script := "register student\nstudent password 123\nlogin student\nstudent password 123\ncd " + dir + "\ntouch note.txt\nwrite note.txt 2\nhello\nмир\ncat note.txt\nrm note.txt\nexit\n"
	if err := os.WriteFile(inputPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = previous; input.Close() }()
	output := captureOutput(t, func() {
		if err := Run(); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(output, "ошибка:") || !strings.Contains(output, "hello\nмир\n") {
		t.Fatalf("unexpected command loop output: %q", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "note.txt")); !os.IsNotExist(err) {
		t.Errorf("CRUD sequence did not remove file: %v", err)
	}
}
