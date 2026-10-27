package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Kha-Beleh/KFM/internal/filesystem"
)

// главный цикл программы, выводит ">" в Stdout,
//
//	считывает ввод и отправляет его в ExecCmd()
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

// выполняет разбор переданной пользователем строки и вызывает нужную команду
func ExecCmd(cmd string) (bool, error) {
	// отделяем только команду, т.к. у разных команд разное кол-во аргументов
	cmd, arg, _ := strings.Cut(cmd, " ")
	shouldExit := false
	var err error
	switch cmd {
	case "help":
		HandlerHelp()

	case "ls":
		dirpath := strings.TrimLeft(arg, " ")
		if dirpath == "" {
			dirpath = "."
		}
		err = HandlerListDir(dirpath)

	case "exit":
		shouldExit = true

	// пустой ввод игнорируется
	case "":

	default:
		err = fmt.Errorf("команда не распознана")
	}
	return shouldExit, err
}

// выводит название элементов среза содержимого директории
func HandlerListDir(dirpath string) error {
	entries, err := filesystem.ListDir(dirpath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		fmt.Println(entry.Name())
	}
	if len(entries) == 0 {
		fmt.Println("каталог пуст")
	}
	return nil
}

func HandlerHelp() {
	fmt.Println("Доступные команды:")
	fmt.Println("help - список всех команд с описанием назначения каждой")
	fmt.Println("ls path - вывод содержимого директории, где path - путь к директории")
	fmt.Println("exit - выход")
}
