package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kha-Beleh/KFM/internal/filesystem"
)

// главный цикл программы, выводит ">" в Stdout,
//
//	считывает ввод и отправляет его в ExecCmd()
func Run() error {
	var shouldExit bool
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s > ", workDir)
		ok := scanner.Scan()
		if !ok {
			return scanner.Err()
		}
		workDir, shouldExit, err = ExecCmd(scanner.Text(), workDir)
		if err != nil {
			fmt.Printf("ошибка: %v\n", err)
		}
		if shouldExit {
			return nil
		}
	}
}

// выполняет разбор переданной пользователем строки и вызывает нужную команду
func ExecCmd(cmd string, workDir string) (string, bool, error) {
	// отделяем только команду, т.к. у разных команд разное кол-во аргументов
	cmd, arg, _ := strings.Cut(cmd, " ")
	shouldExit := false
	var err error
	switch cmd {
	case "help":
		HandlerHelp()

	case "ls":
		err = HandlerListDir(arg, workDir)

	case "cd":
		workDir, err = HandlerChangeDir(arg, workDir)

	case "drives":
		HandlerDrivesInfo()

	case "touch":
		err = HandlerTouch(arg, workDir)

	case "rm":
		err = HandlerRemove(arg, workDir)

	case "exit":
		shouldExit = true

	// пустой ввод игнорируется
	case "":

	default:
		err = fmt.Errorf("команда не распознана")
	}
	return workDir, shouldExit, err
}

func HandlerRemove(filePath string, workDir string) error {
	filePath = strings.TrimLeft(filePath, " ")
	if filePath == "" {
		err := fmt.Errorf("укажите имя файла или путь к нему")
		return err
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(workDir, filePath)
	}
	err := filesystem.RemoveFile(filePath)
	return err
}

func HandlerTouch(filePath string, workDir string) error {
	filePath = strings.TrimLeft(filePath, " ")
	if filePath == "" {
		err := fmt.Errorf("укажите имя файла или путь к нему")
		return err
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(workDir, filePath)
	}
	err := filesystem.CreateFile(filePath)
	return err
}

func HandlerChangeDir(arg string, workDir string) (string, error) {
	var err error
	dirpath := strings.TrimLeft(arg, " ")
	if (dirpath == "/" || dirpath == "\\") && runtime.GOOS == "windows" {
		dirpath = filepath.Join(filepath.VolumeName(workDir), "\\")
	}
	if filepath.IsAbs(dirpath) {
		_, err = os.Stat(dirpath)
		if err == nil {
			workDir = dirpath
		}
	} else {
		_, err = os.Stat(filepath.Join(workDir, dirpath))
		if err == nil {
			workDir = filepath.Join(workDir, dirpath)
		}
	}
	if err != nil {
		err = fmt.Errorf("директории %q не существует: %w", dirpath, err)
	}
	return workDir, err
}

func HandlerDrivesInfo() {
	disks, _ := filesystem.ListDrivesInfo()
	for _, dsk := range disks {
		fmt.Printf("Device: %s \n FileSystem: %s \n MountPoint: %s \n TotalBytes: %d \n Used: %d \n Free: %d \n", dsk.Device, dsk.FStype,
			dsk.MountPoint, dsk.TotalBytes, dsk.Used, dsk.Free)
	}
}

// выводит название элементов среза содержимого директории
func HandlerListDir(arg string, workDir string) error {
	dirpath := strings.TrimLeft(arg, " ")
	if dirpath == "" {
		dirpath = workDir
	} else if !filepath.IsAbs(dirpath) {
		dirpath = filepath.Join(workDir, dirpath)
	}
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
	fmt.Println("ls <path> - вывод содержимого директории, где path - путь к директории")
	fmt.Println("cd <path> - изменение рабочей директории на path, где path - путь к директории")
	fmt.Println("drives - вывести информацию о дисках")
	fmt.Println("touch <path> - создаёт новый пустой файл path; если файл существует, возвращает ошибку, где path - путь к файлу")
	fmt.Println("rm <path> - удаляет файл path, где path - путь к файлу")
	fmt.Println("exit - выход")
}
