package textfile // модуль работы с текстовыми файлами
import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// читает файл
func ReadFile(filepath string) ([]byte, error) {
	const maxSize = 1000 * 1000
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
	limited := io.LimitReader(file, maxSize+1)
	readed, err := io.ReadAll(limited)
	if err != nil {
		err := fmt.Errorf("ошибка при чтении файла: %w", err)
		return nil, err
	}
	if len(readed) > maxSize {
		err := fmt.Errorf("ошибка при чтении файла: превышен размер файла в 1 Мбайт")
		return nil, err
	}
	if !utf8.Valid(readed) {
		err := fmt.Errorf("ошибка при чтении файла: файл содержит данные в неподдерживаемой кодировке, ожидается UTF-8")
		return nil, err
	}
	return readed, nil
}
