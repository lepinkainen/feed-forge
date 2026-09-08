// Package xmlutil provides charset-aware XML decoding shared by every feed
// parser. It applies to Atom and RSS documents alike.
package xmlutil

import (
	"bytes"
	"encoding/xml"

	"golang.org/x/net/html/charset"
)

// Decode reads an XML document using its declared charset. A bare
// xml.Unmarshal has no CharsetReader and fails on any document that declares
// a non-UTF-8 encoding such as ISO-8859-1.
func Decode[T any](body []byte) (*T, error) {
	var doc T
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
