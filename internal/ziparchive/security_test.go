package ziparchive

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type securityEntry struct {
	name         string
	data         []byte
	mode         os.FileMode
	declaredSize uint64
}

// создаёт небольшой архив для проверки атаки
func securityArchive(t *testing.T, entries []securityEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		var output interface{ Write([]byte) (int, error) }
		var err error
		if entry.declaredSize != 0 {
			header.UncompressedSize64 = entry.declaredSize
			header.CompressedSize64 = entry.declaredSize
			output, err = writer.CreateRaw(header)
		} else {
			output, err = writer.CreateHeader(header)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = output.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestSecurityZipSlipCannotChangeOutsideFile(t *testing.T) {
	for _, name := range []string{"../outside.txt", "safe/../../outside.txt", `..\outside.txt`, `C:\outside.txt`, `//server/share/outside.txt`, `file.txt:stream`, `CON.txt`} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			outside := filepath.Join(dir, "outside.txt")
			if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "attack.zip")
			if err := os.WriteFile(archive, securityArchive(t, []securityEntry{{name: name, data: []byte("overwrite")}}), 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(dir, "out")
			if err := ExtractFile(archive, destination); err == nil {
				t.Fatal("accepted ZipSlip or special Windows path")
			}
			got, err := os.ReadFile(outside)
			if err != nil || string(got) != "keep" {
				t.Fatal("changed file outside extraction directory")
			}
			if _, err = os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("invalid archive left an extraction directory")
			}
		})
	}
}

func TestSecurityZIPRejectsSpecialFiles(t *testing.T) {
	for _, mode := range []os.FileMode{os.ModeSymlink | 0777, os.ModeNamedPipe | 0600, os.ModeDevice | 0600} {
		t.Run(mode.String(), func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "special.zip")
			data := securityArchive(t, []securityEntry{{name: "special", data: []byte("../outside"), mode: mode}})
			if err := os.WriteFile(archive, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := ExtractFile(archive, filepath.Join(dir, "out")); err == nil {
				t.Fatal("accepted link or special file")
			}
		})
	}
}

func TestSecurityZIPResourceLimitsBeforeExtraction(t *testing.T) {
	var tooMany []securityEntry
	for i := 0; i < MaxFiles+1; i++ {
		tooMany = append(tooMany, securityEntry{name: fmt.Sprintf("file-%d", i)})
	}
	for _, tc := range []struct {
		name    string
		entries []securityEntry
		message string
	}{
		{"file count", tooMany, "слишком много элементов"},
		{"declared file size", []securityEntry{{name: "large", declaredSize: MaxFileSize + 1}}, "размер распакованных данных"},
		{"declared total size", []securityEntry{{name: "a", declaredSize: 7000000}, {name: "b", declaredSize: 7000000}, {name: "c", declaredSize: 7000000}}, "размер распакованных данных"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "limits.zip")
			data := securityArchive(t, tc.entries)
			if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
				t.Fatalf("invalid test directory: %v", err)
			}
			if err := os.WriteFile(archive, data, 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(dir, "out")
			err := ExtractFile(archive, destination)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected resource rejection, got %v", err)
			}
			if _, err = os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("started extraction before checking limits")
			}
		})
	}
}

func TestSecurityZIPCompressionRatio(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	file, err := writer.Create("zeros.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(make([]byte, 4000000)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, readErr := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if readErr != nil {
		t.Fatal(readErr)
	}
	t.Logf("declared compression: %d / %d", reader.File[0].UncompressedSize64, reader.File[0].CompressedSize64)
	if reader.File[0].UncompressedSize64 <= reader.File[0].CompressedSize64*1000 {
		t.Skip("компрессор не создал коэффициент выше 1000 для этой тестовой строки")
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "ratio.zip")
	if err = os.WriteFile(archive, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	err = ExtractFile(archive, filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "коэффициент") {
		t.Fatalf("compression ratio not rejected: %v", err)
	}
}

func TestSecurityZIPChecksumFailureRemovesPartialFiles(t *testing.T) {
	marker := []byte("UNIQUE_CRC_PAYLOAD_123456789")
	data := securityArchive(t, []securityEntry{{name: "first.txt", data: []byte("first")}, {name: "bad.txt", data: marker}})
	offset := bytes.Index(data, marker)
	if offset < 0 {
		t.Fatal("payload not found in stored archive")
	}
	data[offset] ^= 0xff
	dir := t.TempDir()
	archive := filepath.Join(dir, "corrupt.zip")
	if err := os.WriteFile(archive, data, 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "out")
	if err := ExtractFile(archive, destination); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("expected checksum error, got %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("partial files survived checksum failure")
	}
}

func TestSecurityZIPDoesNotDecodeURLPaths(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "encoded.zip")
	data := securityArchive(t, []securityEntry{{name: "%2e%2e/file.txt", data: []byte("literal")}})
	if err := os.WriteFile(archive, data, 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "out")
	if err := ExtractFile(archive, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); !os.IsNotExist(err) {
		t.Fatal("decoded archive path escaped destination")
	}
	got, err := os.ReadFile(filepath.Join(destination, "%2e%2e", "file.txt"))
	if err != nil || string(got) != "literal" {
		t.Fatal("encoded name was not kept literal")
	}
}
