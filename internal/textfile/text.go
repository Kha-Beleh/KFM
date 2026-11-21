package textfile // модуль работы с текстовыми файлами
import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

const (
	MaxFileSize = 1000 * 1000 // 1 Мбайт, включая переводы строк
	MaxLineSize = 10 * 1000   // 10 Кбайт без перевода строки
)

// читает файл
func ReadFile(filepath string) ([]byte, error) {
	var err error
	file, err := os.Open(filepath)
	if err != nil {
		err := fmt.Errorf("ошибка при чтении файла: %w", err)
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		err := fmt.Errorf("ошибка при чтении файла: %w", err)
		return nil, err
	}
	if !info.Mode().IsRegular() {
		err = fmt.Errorf("ошибка при чтении файла: файл не является обычным")
		return nil, err
	}
	limited := io.LimitReader(file, MaxFileSize+1)
	readed, err := io.ReadAll(limited)
	if err != nil {
		err := fmt.Errorf("ошибка при чтении файла: %w", err)
		return nil, err
	}
	if len(readed) > MaxFileSize {
		err := fmt.Errorf("ошибка при чтении файла: превышен размер файла в 1 Мбайт")
		return nil, err
	}
	if !utf8.Valid(readed) {
		err := fmt.Errorf("ошибка при чтении файла: файл содержит данные в неподдерживаемой кодировке, ожидается UTF-8")
		return nil, err
	}
	return readed, nil
}

// записывает строки text в файл.
func WriteToFile(filePath string, text [][]byte) (err error) {
	var data bytes.Buffer
	for _, line := range text {
		if len(line) > MaxLineSize {
			return fmt.Errorf("ошибка при записи в файл: строка превышает 10 Кбайт")
		}
		if !utf8.Valid(line) {
			return fmt.Errorf("ошибка при записи в файл: ожидается UTF-8")
		}
		if len(line)+1 > MaxFileSize-data.Len() {
			return fmt.Errorf("ошибка при записи в файл: превышен размер 1 Мбайт")
		}
		data.Write(line)
		data.WriteByte('\n')
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %q для записи: %w", filePath, err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("не удалось закрыть файл %q: %w", filePath, closeErr)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("не удалось проверить файл %q: %w", filePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("запись поддерживается только для обычных файлов: %q", filePath)
	}
	if err = file.Truncate(0); err != nil {
		return fmt.Errorf("не удалось очистить файл %q: %w", filePath, err)
	}
	n, err := file.Write(data.Bytes())
	if err != nil {
		return fmt.Errorf("не удалось записать файл %q: %w", filePath, err)
	}
	if n != data.Len() {
		return fmt.Errorf("не удалось записать файл %q полностью: %w", filePath, io.ErrShortWrite)
	}
	return nil
}
