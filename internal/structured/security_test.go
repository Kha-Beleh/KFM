package structured

import (
	"bytes"
	"encoding/xml"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSecurityXMLDoesNotReadExternalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	secret := "KFM_TEST_SECRET_NOT_TO_DISCLOSE"
	if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}).String()
	payload := `<!DOCTYPE root [<!ENTITY external SYSTEM "` + uri + `">]><root>&external;</root>`
	value, err := DecodeXML([]byte(payload))
	if err == nil || value != nil {
		t.Fatal("XML accepted a local external entity")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("XML error disclosed file contents")
	}
}

func TestSecurityXMLDoesNotMakeNetworkRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte("external-secret"))
	}))
	defer server.Close()
	for _, payload := range []string{
		`<!DOCTYPE root [<!ENTITY external SYSTEM "` + server.URL + `">]><root>&external;</root>`,
		`<!DOCTYPE root SYSTEM "` + server.URL + `"><root/>`,
		`<!DOCTYPE root [<!ENTITY % external SYSTEM "` + server.URL + `">%external;]><root/>`,
	} {
		if _, err := DecodeXML([]byte(payload)); err == nil {
			t.Error("accepted external XML declaration")
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("XML made %d external requests", requests.Load())
	}
}

func TestSecurityXMLRejectsEntityExpansion(t *testing.T) {
	payload := `<!DOCTYPE root [
<!ENTITY a "1234567890">
<!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">
<!ENTITY c "&b;&b;&b;&b;&b;&b;&b;&b;&b;&b;">
<!ENTITY d "&c;&c;&c;&c;&c;&c;&c;&c;&c;&c;">
]><root>&d;</root>`
	if value, err := DecodeXML([]byte(payload)); err == nil || value != nil {
		t.Fatal("accepted entity expansion")
	}
}

func TestSecurityDocumentComplexityLimits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		decode func([]byte) error
		data   []byte
	}{
		{"JSON size", func(b []byte) error { _, e := DecodeJSON(b); return e }, bytes.Repeat([]byte(" "), MaxSize+1)},
		{"XML size", func(b []byte) error { _, e := DecodeXML(b); return e }, bytes.Repeat([]byte(" "), MaxSize+1)},
		{"JSON depth", func(b []byte) error { _, e := DecodeJSON(b); return e }, []byte(strings.Repeat("[", MaxDepth+1) + "0" + strings.Repeat("]", MaxDepth+1))},
		{"XML depth", func(b []byte) error { _, e := DecodeXML(b); return e }, []byte(strings.Repeat("<a>", MaxDepth+1) + strings.Repeat("</a>", MaxDepth+1))},
		{"XML element count", func(b []byte) error { _, e := DecodeXML(b); return e }, []byte("<root>" + strings.Repeat("<a/>", 10000) + "</root>")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.decode(tc.data); err == nil {
				t.Fatal("resource limit not enforced")
			}
		})
	}
	if _, err := DecodeJSON([]byte(strings.Repeat("[", MaxDepth) + "0" + strings.Repeat("]", MaxDepth))); err != nil {
		t.Fatalf("rejected allowed JSON depth: %v", err)
	}
	if _, err := DecodeXML([]byte("<root>" + strings.Repeat("<a/>", 9999) + "</root>")); err != nil {
		t.Fatalf("rejected allowed XML element count: %v", err)
	}
}

func TestSecuritySerializationRejectsCyclesAndInvalidNumbers(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, value := range []any{cycle, math.NaN(), math.Inf(1)} {
		if _, err := EncodeJSON(value); err == nil {
			t.Fatal("accepted unsafe or unsupported JSON value")
		}
	}
	node := &XMLNode{Name: xml.Name{Local: "root"}}
	node.Content = []XMLPart{{Element: node}}
	if _, err := EncodeXML(node); err == nil {
		t.Fatal("accepted cyclic XML tree")
	}
}

func TestSecurityXMLTextCannotInjectElements(t *testing.T) {
	node := &XMLNode{Name: xml.Name{Local: "root"}, Content: []XMLPart{{Text: `</root><injected>&data</injected><root>`}}}
	data, err := EncodeXML(node)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeXML(data)
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, part := range decoded.Content {
		if part.Element != nil {
			t.Fatal("plain text became an XML element")
		}
		text.WriteString(part.Text)
	}
	if text.String() != node.Content[0].Text {
		t.Fatal("escaped text did not survive round trip")
	}
}
