package ziparchive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

const MaxFiles = 1000
const MaxFileSize = 10 * 1000 * 1000
const MaxTotalSize = 20 * 1000 * 1000
const MaxArchiveSize = 32 * 1000 * 1000

// проверяет переносимость пути внутри архива
func validName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		for _, char := range part {
			if unicode.IsControl(char) || strings.ContainsRune(`<>"|?*`, char) {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}

// создаёт ZIP из файла или каталога
func buildArchive(root *os.Root, source string, destination string) ([]byte, error) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	count, total := 0, int64(0)
	err := fs.WalkDir(root.FS(), filepath.ToSlash(source), func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == destination {
			return fmt.Errorf("архив не должен находиться внутри исходного каталога")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("ссылки и специальные файлы запрещены")
		}
		relative, err := filepath.Rel(filepath.Dir(source), filepath.FromSlash(name))
		if err != nil {
			return err
		}
		member := filepath.ToSlash(relative)
		if source == "." {
			member = name
		}
		if member == "." {
			return nil
		}
		if !validName(member) {
			return fmt.Errorf("неподдерживаемое имя %q", member)
		}
		count++
		if count > MaxFiles {
			return fmt.Errorf("слишком много файлов")
		}
		if info.IsDir() {
			_, err = writer.Create(member + "/")
			return err
		}
		if info.Size() > MaxFileSize {
			return fmt.Errorf("файл превышает 10 Мбайт")
		}
		file, err := root.Open(filepath.FromSlash(name))
		if err != nil {
			return err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() {
			return fmt.Errorf("не удалось открыть обычный файл")
		}
		output, err := writer.Create(member)
		if err != nil {
			return err
		}
		n, err := io.Copy(output, io.LimitReader(file, MaxFileSize+1))
		total += n
		if err != nil {
			return err
		}
		if n > MaxFileSize || total > MaxTotalSize {
			return fmt.Errorf("превышен размер данных архива")
		}
		return nil
	})
	if err != nil {
		writer.Close()
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	if data.Len() > MaxArchiveSize {
		return nil, fmt.Errorf("архив слишком большой")
	}
	return data.Bytes(), nil
}

// создаёт архив внутри указанного корня
func Create(root *os.Root, source string, destination string) error {
	data, err := buildArchive(root, source, destination)
	if err != nil {
		return err
	}
	file, err := root.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		root.Remove(destination)
	}
	return err
}

// извлекает ZIP в новый каталог
func Extract(root *os.Root, archivePath string, destination string) (err error) {
	file, err := root.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxArchiveSize {
		return fmt.Errorf("архив должен быть обычным файлом до 32 Мбайт")
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return err
	}
	return extractArchive(root, reader, destination)
}

// проверяет архив и извлекает его в новый каталог
func extractArchive(root *os.Root, reader *zip.Reader, destination string) (err error) {
	if len(reader.File) > MaxFiles {
		return fmt.Errorf("слишком много элементов в архиве")
	}
	seen := map[string]bool{}
	var total uint64
	for _, member := range reader.File {
		if !validName(member.Name) {
			return fmt.Errorf("опасный путь в архиве: %q", member.Name)
		}
		key := strings.ToLower(strings.TrimSuffix(member.Name, "/"))
		if seen[key] {
			return fmt.Errorf("повторяющийся путь в архиве")
		}
		seen[key] = true
		if !member.Mode().IsRegular() && !member.FileInfo().IsDir() {
			return fmt.Errorf("ссылки и специальные файлы запрещены")
		}
		if member.UncompressedSize64 > MaxFileSize || member.UncompressedSize64 > MaxTotalSize-total {
			return fmt.Errorf("превышен размер распакованных данных")
		}
		if member.UncompressedSize64 > 1000000 && member.UncompressedSize64/(member.CompressedSize64+1) > 1000 {
			return fmt.Errorf("подозрительный коэффициент сжатия")
		}
		total += member.UncompressedSize64
	}
	if err = root.Mkdir(destination, 0700); err != nil {
		return fmt.Errorf("нужен новый каталог назначения: %w", err)
	}
	defer func() {
		if err != nil {
			root.RemoveAll(destination)
		}
	}()
	output, err := root.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	written := int64(0)
	for _, member := range reader.File {
		name := filepath.FromSlash(path.Clean(member.Name))
		if member.FileInfo().IsDir() {
			if err = output.MkdirAll(name, 0700); err != nil {
				return err
			}
			continue
		}
		if err = output.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return err
		}
		if err = extractFile(output, member, name, &written); err != nil {
			return err
		}
	}
	return nil
}

// создаёт ZIP по обычным системным путям
func CreateFile(source string, destination string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		relative, relErr := filepath.Rel(source, destination)
		if relErr == nil && filepath.IsLocal(relative) {
			return fmt.Errorf("архив должен находиться вне исходного каталога")
		}
	}
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := buildArchive(root, filepath.Base(source), "")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		os.Remove(destination)
	}
	return writeErr
}

// извлекает ZIP с ограничением только каталогом назначения
func ExtractFile(archivePath string, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxArchiveSize {
		return fmt.Errorf("нужен обычный ZIP-файл до 32 Мбайт")
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer root.Close()
	return extractArchive(root, reader, filepath.Base(destination))
}

// извлекает один файл с проверкой фактического размера
func extractFile(root *os.Root, member *zip.File, name string, total *int64) error {
	source, err := member.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(target, io.LimitReader(source, MaxFileSize+1))
	closeErr := target.Close()
	*total += n
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > MaxFileSize || *total > MaxTotalSize {
		return fmt.Errorf("превышен фактический размер распаковки")
	}
	return nil
}
