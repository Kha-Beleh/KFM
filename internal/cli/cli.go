package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Kha-Beleh/KFM/internal/filesystem"
)

func Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		ok := scanner.Scan()
		if !ok {
			return scanner.Err()
		}
		shouldExit, err := ExecCmd(scanner.Text())
		if err != nil {
			fmt.Printf("ошибка: %v\n", err)
		}
		if shouldExit {
			return nil
		}
	}
}

func ExecCmd(cmd string) (bool, error) {
	cmd, arg, _ := strings.Cut(cmd, " ")
	shouldExit := false
	var err error
	switch cmd {
	case "help":
		PrintHelp()

	case "ls":
		var entries []os.DirEntry
		arg = strings.TrimLeft(arg, " ")
		if arg == "" {
			arg = "."
		}
		entries, err = filesystem.ListDir(arg)
		if err == nil {
			PrintDir(entries)
		}

	case "exit":
		shouldExit = true

	case "":

	default:
		err = fmt.Errorf("команда не распознана")
	}
	return shouldExit, err
}

func PrintDir(entries []os.DirEntry) {
	if len(entries) == 0 {
		fmt.Println("каталог пуст")
		return
	}
	for _, entry := range entries {
		fmt.Println(entry.Name())
	}
}

func PrintHelp() {
	fmt.Println("Доступные команды:")
	fmt.Println("help - список всех команд с описанием назначения каждой")
	fmt.Println("ls path - вывод содержимого директории, где path - путь к директории")
	fmt.Println("exit - выход")
}
