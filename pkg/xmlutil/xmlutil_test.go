package xmlutil

import "testing"

type rss struct {
	Items []struct {
		Title string `xml:"title"`
	} `xml:"channel>item"`
}

func TestDecodeHonoursDeclaredCharset(t *testing.T) {
	body := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><rss><channel><item><title>Caf\xe9 &amp; t\xe9</title></item></channel></rss>")
	doc, err := Decode[rss](body)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 1 || doc.Items[0].Title != "Café & té" {
		t.Fatalf("items = %+v", doc.Items)
	}
}

func TestDecodeRejectsInvalidDocuments(t *testing.T) {
	for _, body := range []string{
		`<rss><channel>`,
		`<?xml version="1.0" encoding="unknown-charset"?><rss/>`,
	} {
		if _, err := Decode[rss]([]byte(body)); err == nil {
			t.Errorf("Decode(%q) succeeded, want error", body)
		}
	}
}
