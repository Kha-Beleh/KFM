package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kha-Beleh/KFM/internal/auth"
	"github.com/Kha-Beleh/KFM/internal/postgres"
	"golang.org/x/term"
)

type appState struct {
	store *postgres.Store
	auth  *auth.Service
	user  postgres.User
}

// обрабатывает регистрацию и вход
func handlerAuth(command string, args string, scanner *bufio.Scanner, state *appState) error {
	if state == nil {
		return fmt.Errorf("нет подключения к БД")
	}
	if state.user.ID != 0 {
		return fmt.Errorf("сначала выполните logout")
	}
	name := strings.TrimSpace(args)
	if name == "" || strings.ContainsAny(name, " \t") {
		return fmt.Errorf("укажите один логин")
	}
	password, err := readPassword(scanner)
	if err != nil {
		return err
	}
	if command == "register" {
		_, err = state.auth.Register(name, password)
		if err == nil {
			fmt.Println("пользователь зарегистрирован; выполните login")
		}
	} else {
		state.user, err = state.auth.Login(name, password)
	}
	return err
}

// считывает пароль отдельно от команды
func readPassword(scanner *bufio.Scanner) (string, error) {
	fmt.Print("пароль: ")
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		return string(password), err
	}
	if scanner == nil {
		return "", fmt.Errorf("нет источника ввода")
	}
	if !scanner.Scan() {
		if scanner.Err() != nil {
			return "", scanner.Err()
		}
		return "", fmt.Errorf("пароль не введён")
	}
	return scanner.Text(), nil
}

// показывает действия текущего пользователя
func handlerHistory(state *appState) error {
	if state == nil || state.user.ID == 0 {
		return fmt.Errorf("сначала выполните login")
	}
	operations, err := state.store.History(state.user.ID)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		fmt.Printf("%d %s %s %s %s %s\n", operation.ID, operation.Time.Format("2006-01-02 15:04:05"), operation.Type, safeText(operation.Path), operation.Status, safeText(operation.Error))
	}
	return nil
}

// получает сведения о файлах для журнала
func commandFiles(command string, args string, workDir string, after bool) []postgres.File {
	var names []string
	switch command {
	case "touch", "rm", "cat", "json-read", "xml-read":
		names = []string{strings.TrimSpace(args)}
	case "write", "append", "json-write", "xml-write":
		args = strings.TrimSpace(args)
		if separator := strings.LastIndexAny(args, " \t"); separator >= 0 {
			names = []string{strings.TrimSpace(args[:separator])}
		}
	case "zip", "unzip":
		parts, err := splitArguments(args)
		if err == nil && len(parts) == 2 {
			names = parts
		}
	default:
		return nil
	}
	var files []postgres.File
	for _, name := range names {
		if name == "" {
			continue
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(workDir, name)
		}
		info, err := os.Stat(name)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			files = append(files, postgres.File{Path: name, Size: info.Size()})
		}
		if command == "unzip" && after && info.IsDir() {
			filepath.WalkDir(name, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type().IsRegular() {
					if info, err := entry.Info(); err == nil {
						files = append(files, postgres.File{Path: path, Size: info.Size()})
					}
				}
				return nil
			})
		}
	}
	return files
}
