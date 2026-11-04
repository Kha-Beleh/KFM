package structured

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	value, err := DecodeJSON([]byte(`{"name":"текст","number":9007199254740993,"items":[true,null]}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncodeJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]any)["number"].(json.Number).String() != "9007199254740993" {
		t.Fatal("number lost precision")
	}
}

func TestJSONRejectsInvalidData(t *testing.T) {
	for _, input := range []string{"", "{", "{} {}", "[1,]", string([]byte{255}), strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65), strings.Repeat(" ", MaxSize+1)} {
		if _, err := DecodeJSON([]byte(input)); err == nil {
			t.Errorf("accepted invalid JSON of length %d", len(input))
		}
	}
	if _, err := EncodeJSON(make(chan int)); err == nil {
		t.Fatal("accepted unsupported value")
	}
}

func TestXMLRoundTrip(t *testing.T) {
	for _, input := range []string{`<root id="1"><name>текст &amp; данные</name></root>`, `<p>до<b>внутри</b>после</p>`, `<root xmlns="urn:test"><item /></root>`} {
		node, err := DecodeXML([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		data, err := EncodeXML(node)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeXML(data)
		if err != nil || got.Name != node.Name {
			t.Fatalf("XML round trip: %s, %v", data, err)
		}
	}
}

func TestXMLRejectsThreats(t *testing.T) {
	for _, input := range []string{"", "<a>", "<a/><b/>", "text<a/>", `<!DOCTYPE a [<!ENTITY x SYSTEM "file:///secret">]><a>&x;</a>`, `<!DOCTYPE a [<!ENTITY x "&x;&x;">]><a>&x;</a>`, `<a>&unknown;</a>`, `<?run something?><a/>`, strings.Repeat("<a>", 65) + strings.Repeat("</a>", 65)} {
		if _, err := DecodeXML([]byte(input)); err == nil {
			t.Errorf("accepted XML: %.80s", input)
		}
	}
	if _, err := EncodeXML(nil); err == nil {
		t.Fatal("accepted nil tree")
	}
}
