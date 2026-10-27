package filesystem

import (
	"fmt"
	"os"
)

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
