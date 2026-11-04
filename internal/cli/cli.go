package cli // модуль консольного интерфейса

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Kha-Beleh/KFM/internal/filesystem"
	"github.com/Kha-Beleh/KFM/internal/textfile"
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
	scanner.Buffer(make([]byte, 4096), textfile.MaxFileSize+2)
	for {
		fmt.Printf("%s > ", workDir)
		ok := scanner.Scan()
		if !ok {
			return scanner.Err()
		}
		workDir, shouldExit, err = ExecCmd(scanner.Text(), workDir, scanner)
		if err != nil {
			fmt.Printf("ошибка: %v\n", err)
		}
		if shouldExit {
			return nil
		}
	}
}

// выполняет разбор переданной пользователем строки и вызывает нужную команду
func ExecCmd(cmd string, workDir string, scanner *bufio.Scanner) (string, bool, error) {
	// отделяем только команду, т.к. у разных команд разное кол-во аргументов
	cmd, arg, _ := strings.Cut(cmd, " ")
	shouldExit := false
	var err error
	switch cmd {
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
	args = strings.TrimSpace(args)
	separator := strings.LastIndexAny(args, " \t")
	if separator < 0 {
		return fmt.Errorf("использование: write <путь> <количество строк>")
	}
	filePath := strings.TrimSpace(args[:separator])
	if filePath == "" {
		return fmt.Errorf("укажите имя файла или путь к нему")
	}
	nStrings, err := strconv.Atoi(args[separator+1:])
	if err != nil {
		return fmt.Errorf("не удалось определить количество строк: %w", err)
	}
	if nStrings < 1 || nStrings > textfile.MaxFileSize {
		return fmt.Errorf("количество строк должно быть от 1 до %d", textfile.MaxFileSize)
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(workDir, filePath)
	}
	if scanner == nil {
		return fmt.Errorf("не задан источник ввода текста")
	}

	fmt.Printf("Содержимое файла %q будет заменено после успешного ввода. Введите %d строк:\n", filePath, nStrings)
	var lines [][]byte
	var inputErr error
	totalSize := 0
	for i := 0; i < nStrings; i++ {
		if !scanner.Scan() {
			if err = scanner.Err(); err != nil {
				return fmt.Errorf("не удалось прочитать текст: %w", err)
			}
			return fmt.Errorf("ввод завершён: получено %d из %d строк; файл не изменён", i, nStrings)
		}
		// После ошибки пропускаем оставшиеся строки блока, не выполняя их как команды
		if inputErr != nil {
			continue
		}
		line := scanner.Bytes()
		switch {
		case len(line) > textfile.MaxLineSize:
			inputErr = fmt.Errorf("строка %d превышает 10 Кбайт; файл не изменён", i+1)
		case !utf8.Valid(line):
			inputErr = fmt.Errorf("строка %d не соответствует UTF-8; файл не изменён", i+1)
		case len(line)+1 > textfile.MaxFileSize-totalSize:
			inputErr = fmt.Errorf("текст превышает 1 Мбайт; файл не изменён")
		default:
			// Scanner переиспользует буфер, поэтому сохраняем копию строки.
			lines = append(lines, []byte(scanner.Text()))
			totalSize += len(line) + 1
		}
	}
	if inputErr != nil {
		return inputErr
	}
	return textfile.WriteToFile(filePath, lines)
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
	fmt.Printf("%s", readed)
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
		fmt.Println(entry.Name())
	}
	if len(entries) == 0 {
		fmt.Println("каталог пуст")
	}
	return nil
}

func handlerHelp() {
	fmt.Println("Доступные команды:")
	fmt.Println("help - список всех команд с описанием назначения каждой")
	fmt.Println("ls <path> - вывод содержимого директории, где path - путь к директории")
	fmt.Println("cd <path> - изменение рабочей директории на path, где path - путь к директории")
	fmt.Println("drives - вывести информацию о дисках")
	fmt.Println("touch <path> - создаёт новый пустой файл path; если файл существует, возвращает ошибку, где path - путь к файлу")
	fmt.Println("rm <path> - удаляет файл path, где path - путь к файлу")
	fmt.Println("cat <path> - выводит содержимое файла path, где path - путь к файлу")
	fmt.Println("write <path> <n> - заменяет содержимое существующего файла n строками UTF-8 (строка до 10 Кбайт, файл до 1 Мбайт)")
	fmt.Println("exit - выход")
}
