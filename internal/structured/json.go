package structured

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

const MaxSize = 1000000
const MaxDepth = 64

// разбирает JSON
func DecodeJSON(data []byte) (any, error) {
	if len(data) > MaxSize || !utf8.Valid(data) {
		return nil, fmt.Errorf("JSON должен быть UTF-8 размером до 1 Мбайт")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("некорректный JSON: %w", err)
		}
		if delim, ok := token.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				depth++
			} else {
				depth--
			}
			if depth > MaxDepth {
				return nil, fmt.Errorf("слишком большая вложенность JSON")
			}
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("некорректный JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("ожидается один JSON-документ")
	}
	return value, nil
}

// сериализует JSON
func EncodeJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("не удалось сериализовать JSON: %w", err)
	}
	if _, err = DecodeJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}
