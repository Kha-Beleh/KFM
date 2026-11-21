package filesystem // модуль работы с файловой системой

import (
	"fmt"
	"os"
)

// предоставляет срез содержимого директории по переданному пути, если возможно
func ListDir(filePath string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(filePath)
	if err == nil {
		return entries, nil
	}
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("директории %q не существует: %w", filePath, err)
	}
	return nil, fmt.Errorf("не удалось прочитать каталог %q: %w", filePath, err)
}

// создает пустой файл
func CreateFile(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		err = fmt.Errorf("не удалось создать файл %q: %w", filePath, err)
		return err
	}
	err = file.Close()
	return err
}

// удаляет файл
func RemoveFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		err = fmt.Errorf("не удалось удалить файл %q: %w", filePath, err)
		return err
	}
	if info.IsDir() {
		err = fmt.Errorf("rm удаляет только файлы")
		return err
	}
	err = os.Remove(filePath)
	if err != nil {
		err = fmt.Errorf("не удалось удалить файл %q: %w", filePath, err)
		return err
	}
	return nil
}
