package cli // модуль консольного интерфейса

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kha-Beleh/KFM/internal/auth"
	"github.com/Kha-Beleh/KFM/internal/filesystem"
	"github.com/Kha-Beleh/KFM/internal/postgres"
	"github.com/Kha-Beleh/KFM/internal/textfile"
)

// главный цикл программы, выводит ">" в Stdout,
//
//	считывает ввод и отправляет его в ExecCmd()
func Run() error {
	store, err := postgres.Open(os.Getenv("KFM_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err = store.Migrate(); err != nil {
		return fmt.Errorf("не удалось подготовить БД: %w", err)
	}
	state := appState{store: store, auth: &auth.Service{Store: store}}
	var shouldExit bool
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), textfile.MaxFileSize+2)
	for {
		fmt.Printf("%s > ", safeText(workDir))
		ok := scanner.Scan()
		if !ok {
			return scanner.Err()
		}
		line := scanner.Text()
		command, args, _ := strings.Cut(line, " ")
		if command == "" {
			continue
		}
		userID := state.user.ID
		path := args
		if command == "login" || command == "register" {
			path = ""
		}
		id, startErr := store.Start(userID, command, path)
		if startErr != nil {
			return fmt.Errorf("не удалось записать начало команды: %w", startErr)
		}
		var files []postgres.File
		if userID != 0 {
			files = commandFiles(command, args, workDir, false)
		}
		workDir, shouldExit, err = ExecCmd(line, workDir, scanner, &state)
		if userID == 0 && state.user.ID != 0 {
			userID = state.user.ID
			if assignErr := store.AssignUser(id, userID); assignErr != nil {
				return assignErr
			}
		}
		if command == "rm" && err == nil {
			for i := range files {
				files[i].Deleted = true
			}
		} else if userID != 0 {
			files = commandFiles(command, args, workDir, true)
		}
		if userID == 0 {
			files = nil
		}
		if finishErr := store.FinishFiles(id, userID, files, err); finishErr != nil {
			return fmt.Errorf("не удалось сохранить результат команды: %w", finishErr)
		}
		if err != nil {
			fmt.Printf("ошибка: %s\n", safeText(err.Error()))
		}
		if shouldExit {
			return nil
		}
	}
}

// выполняет разбор переданной пользователем строки и вызывает нужную команду
func ExecCmd(cmd string, workDir string, scanner *bufio.Scanner, state *appState) (string, bool, error) {
	// отделяем только команду, т.к. у разных команд разное кол-во аргументов
	cmd, arg, _ := strings.Cut(cmd, " ")
	shouldExit := false
	var err error
	if state != nil && state.user.ID == 0 && cmd != "help" && cmd != "login" && cmd != "register" && cmd != "exit" && cmd != "" {
		return workDir, false, fmt.Errorf("сначала выполните login")
	}
	switch cmd {
	case "register", "login":
		err = handlerAuth(cmd, arg, scanner, state)
	case "logout":
		if state == nil {
			err = fmt.Errorf("приложение не подключено к БД")
		} else {
			state.user = postgres.User{}
		}
	case "history":
		err = handlerHistory(state)
	case "help":
		handlerHelp()

	case "ls":
		err = handlerListDir(arg, workDir)

	case "cd":
		workDir, err = handlerChangeDir(arg, workDir)

	case "drives":
		handlerDrivesInfo()

	case "touch":
		err = handlerTouch(arg, workDir)

	case "rm":
		err = handlerRemove(arg, workDir)

	case "cat":
		err = handlerCat(arg, workDir)

	case "write":
		err = handlerWrite(arg, workDir, scanner)
	case "append":
		err = handlerAppend(arg, workDir, scanner)
	case "json-read", "json-write", "xml-read", "xml-write":
		err = handlerStructured(cmd, arg, workDir, scanner)
	case "zip", "unzip":
		err = handlerArchive(cmd, arg, workDir)
	case "pwd":
		fmt.Println(workDir)

	case "exit":
		shouldExit = true

	// пустой ввод игнорируется
	case "":

	default:
		err = fmt.Errorf("команда не распознана")
	}
	return workDir, shouldExit, err
}

func handlerWrite(args string, workDir string, scanner *bufio.Scanner) error {
	name, data, err := readWriteInput(args, workDir, scanner)
	if err != nil {
		return err
	}
	return writeData(name, data)
}

func handlerCat(filePath string, workDir string) error {
	filePath = strings.TrimLeft(filePath, " ")
	if filePath == "" {
		err := fmt.Errorf("укажите имя файла или путь к нему")
		return err
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(workDir, filePath)
	}
	readed, err := textfile.ReadFile(filePath)
	if err != nil {
		return err
	}
	fmt.Printf("%s", safeText(string(readed)))
	return nil
}

func handlerRemove(filePath string, workDir string) error {
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

func handlerTouch(filePath string, workDir string) error {
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

func handlerChangeDir(arg string, workDir string) (string, error) {
	var err error
	dirpath := strings.TrimLeft(arg, " ")
	if (dirpath == "/" || dirpath == "\\") && runtime.GOOS == "windows" {
		dirpath = filepath.Join(filepath.VolumeName(workDir), "\\")
	}
	if filepath.IsAbs(dirpath) {
		var info os.FileInfo
		info, err = os.Stat(dirpath)
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("путь не является каталогом")
		}
		if err == nil {
			workDir = dirpath
		}
	} else {
		var info os.FileInfo
		info, err = os.Stat(filepath.Join(workDir, dirpath))
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("путь не является каталогом")
		}
		if err == nil {
			workDir = filepath.Join(workDir, dirpath)
		}
	}
	if err != nil {
		err = fmt.Errorf("директории %q не существует: %w", dirpath, err)
	}
	return workDir, err
}

func handlerDrivesInfo() {
	disks, _ := filesystem.ListDrivesInfo()
	for _, dsk := range disks {
		fmt.Printf("Device: %s \n FileSystem: %s \n MountPoint: %s \n TotalBytes: %d \n Used: %d \n Free: %d \n", dsk.Device, dsk.FStype,
			dsk.MountPoint, dsk.TotalBytes, dsk.Used, dsk.Free)
	}
}

// выводит название элементов среза содержимого директории
func handlerListDir(arg string, workDir string) error {
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
		fmt.Println(safeText(entry.Name()))
	}
	if len(entries) == 0 {
		fmt.Println("каталог пуст")
	}
	return nil
}

func handlerHelp() {
	fmt.Println("Доступные команды:")
	fmt.Println("help - список всех команд с описанием назначения каждой")
	fmt.Println("register <login> - регистрация; пароль вводится отдельно")
	fmt.Println("login <login> - вход для записи действий от имени пользователя")
	fmt.Println("logout - выход из учётной записи")
	fmt.Println("history - последние 100 действий пользователя")
	fmt.Println("pwd - текущий рабочий каталог")
	fmt.Println("ls <path> - вывод содержимого директории, где path - путь к директории")
	fmt.Println("cd <path> - изменение рабочей директории на path, где path - путь к директории")
	fmt.Println("drives - вывести информацию о дисках")
	fmt.Println("touch <path> - создаёт новый пустой файл path; если файл существует, возвращает ошибку, где path - путь к файлу")
	fmt.Println("rm <path> - удаляет файл path, где path - путь к файлу")
	fmt.Println("cat <path> - выводит содержимое файла path, где path - путь к файлу")
	fmt.Println("write <path> <n> - заменяет содержимое существующего файла n строками UTF-8 (строка до 10 Кбайт, файл до 1 Мбайт)")
	fmt.Println("append <path> <n> - добавляет n строк в конец файла")
	fmt.Println("json-read <path>, xml-read <path> - разбор и вывод данных")
	fmt.Println("json-write <path> <n>, xml-write <path> <n> - проверка и сохранение данных")
	fmt.Println("zip <source> <archive>, unzip <archive> <new-directory> - ZIP; пути с пробелами заключайте в кавычки")
	fmt.Println("exit - выход")
}
