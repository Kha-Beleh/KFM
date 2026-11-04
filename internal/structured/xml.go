package structured

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type XMLNode struct {
	Name    xml.Name
	Attrs   []xml.Attr
	Content []XMLPart
}

type XMLPart struct {
	Text    string
	Element *XMLNode
}

// разбирает XML без DTD и внешних сущностей
func DecodeXML(data []byte) (*XMLNode, error) {
	if len(data) > MaxSize || !utf8.Valid(data) {
		return nil, fmt.Errorf("XML должен быть UTF-8 размером до 1 Мбайт")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *XMLNode
	var stack []*XMLNode
	count := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("некорректный XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			count++
			if len(stack) >= MaxDepth || count > 10000 {
				return nil, fmt.Errorf("превышена сложность XML")
			}
			node := &XMLNode{Name: token.Name}
			for _, attr := range token.Attr {
				if attr.Name.Local != "xmlns" && attr.Name.Space != "xmlns" {
					node.Attrs = append(node.Attrs, attr)
				}
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("ожидается один корневой элемент XML")
				}
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.Content = append(parent.Content, XMLPart{Element: node})
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("лишний закрывающий элемент XML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(token)) != "" {
					return nil, fmt.Errorf("текст за пределами корневого элемента")
				}
			} else {
				node := stack[len(stack)-1]
				node.Content = append(node.Content, XMLPart{Text: string(token)})
			}
		case xml.Directive:
			return nil, fmt.Errorf("DTD и директивы XML запрещены")
		case xml.ProcInst:
			if token.Target != "xml" || root != nil {
				return nil, fmt.Errorf("инструкции обработки XML запрещены")
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("незавершённый XML-документ")
	}
	return root, nil
}

// сериализует дерево XML
func EncodeXML(node *XMLNode) ([]byte, error) {
	var data bytes.Buffer
	encoder := xml.NewEncoder(&data)
	if err := writeXML(encoder, node, 0); err != nil {
		return nil, err
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	if _, err := DecodeXML(data.Bytes()); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// записывает элемент XML
func writeXML(encoder *xml.Encoder, node *XMLNode, depth int) error {
	if node == nil || depth >= MaxDepth {
		return fmt.Errorf("некорректное дерево XML")
	}
	start := xml.StartElement{Name: node.Name, Attr: node.Attrs}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	for _, part := range node.Content {
		if part.Element != nil {
			if err := writeXML(encoder, part.Element, depth+1); err != nil {
				return err
			}
		} else if err := encoder.EncodeToken(xml.CharData(part.Text)); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}
