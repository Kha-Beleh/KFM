package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Kha-Beleh/KFM/internal/structured"
	"github.com/Kha-Beleh/KFM/internal/textfile"
	"github.com/Kha-Beleh/KFM/internal/ziparchive"
)

// получает путь и блок вводимых строк
func readWriteInput(args string, workDir string, scanner *bufio.Scanner) (string, []byte, error) {
	args = strings.TrimSpace(args)
	separator := strings.LastIndexAny(args, " \t")
	if separator < 0 {
		return "", nil, fmt.Errorf("укажите путь и количество строк")
	}
	name := strings.TrimSpace(args[:separator])
	count, err := strconv.Atoi(args[separator+1:])
	if name == "" || err != nil || count < 1 || count > textfile.MaxFileSize {
		return "", nil, fmt.Errorf("неверный путь или количество строк")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(workDir, name)
	}
	if scanner == nil {
		return "", nil, fmt.Errorf("нет источника ввода")
	}
	fmt.Printf("Введите %d строк; запись выполняется после проверки ввода:\n", count)
	var data bytes.Buffer
	var inputErr error
	for i := 0; i < count; i++ {
		if !scanner.Scan() {
			if scanner.Err() != nil {
				return "", nil, scanner.Err()
			}
			return "", nil, fmt.Errorf("ввод закончился раньше; файл не изменён")
		}
		if inputErr != nil {
			continue
		}
		line := scanner.Bytes()
		if len(line) > textfile.MaxLineSize || len(line)+1 > textfile.MaxFileSize-data.Len() {
			inputErr = fmt.Errorf("превышен размер текста")
			continue
		}
		if !utf8.Valid(line) {
			inputErr = fmt.Errorf("ожидается UTF-8")
			continue
		}
		data.Write(line)
		data.WriteByte('\n')
	}
	if inputErr != nil {
		return "", nil, inputErr
	}
	return name, data.Bytes(), nil
}

// записывает проверенные текстовые данные
func writeData(name string, data []byte) error {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return textfile.WriteToFile(name, lines)
}

// добавляет текст в файл
func handlerAppend(args string, workDir string, scanner *bufio.Scanner) error {
	name, data, err := readWriteInput(args, workDir, scanner)
	if err != nil {
		return err
	}
	old, err := textfile.ReadFile(name)
	if err != nil {
		return err
	}
	return writeData(name, append(old, data...))
}

// разбирает и сериализует JSON или XML
func handlerStructured(command string, args string, workDir string, scanner *bufio.Scanner) error {
	var name string
	var data []byte
	var err error
	write := strings.HasSuffix(command, "-write")
	if write {
		name, data, err = readWriteInput(args, workDir, scanner)
	} else {
		name = strings.TrimSpace(args)
		if name == "" {
			return fmt.Errorf("укажите путь к файлу")
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(workDir, name)
		}
		data, err = textfile.ReadFile(name)
	}
	if err != nil {
		return err
	}
	if strings.HasPrefix(command, "json") {
		var value any
		value, err = structured.DecodeJSON(data)
		if err == nil {
			data, err = structured.EncodeJSON(value)
		}
	} else {
		var value *structured.XMLNode
		value, err = structured.DecodeXML(data)
		if err == nil {
			data, err = structured.EncodeXML(value)
		}
	}
	if err != nil {
		return err
	}
	if write {
		return writeData(name, data)
	}
	fmt.Println(safeText(string(data)))
	return nil
}

// разбирает два пути с поддержкой кавычек
func splitArguments(value string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	active := false
	for _, char := range value {
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
		} else if char == '"' || char == '\'' {
			quote = char
			active = true
		} else if unicode.IsSpace(char) {
			if active {
				args = append(args, current.String())
				current.Reset()
				active = false
			}
		} else {
			current.WriteRune(char)
			active = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("не закрыты кавычки")
	}
	if active {
		args = append(args, current.String())
	}
	return args, nil
}

// создаёт или извлекает ZIP по системным путям
func handlerArchive(command string, args string, workDir string) error {
	paths, err := splitArguments(args)
	if err != nil {
		return err
	}
	if len(paths) != 2 || paths[0] == "" || paths[1] == "" {
		return fmt.Errorf("укажите исходный путь и путь назначения")
	}
	for i := range paths {
		if !filepath.IsAbs(paths[i]) {
			paths[i] = filepath.Join(workDir, paths[i])
		}
	}
	if command == "zip" {
		return ziparchive.CreateFile(paths[0], paths[1])
	}
	return ziparchive.ExtractFile(paths[0], paths[1])
}

// экранирует управляющие символы терминала
func safeText(value string) string {
	var out strings.Builder
	for _, char := range value {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			fmt.Fprintf(&out, "\\u%04x", char)
		} else {
			out.WriteRune(char)
		}
	}
	return out.String()
}
