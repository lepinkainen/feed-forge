package oglaf

import (
	"strings"
	"testing"
)

func TestComicDescriptionEscapesInterpolatedHTML(t *testing.T) {
	html := comicDescription(
		`https://example.com/comic?next=<script>`,
		`https://cdn.example.com/image" onerror="alert(1)`,
		`Title "quoted" <b>tag</b>`,
	)

	expected := []string{
		`href="https://example.com/comic?next=&lt;script&gt;"`,
		`src="https://cdn.example.com/image&#34; onerror=&#34;alert(1)"`,
		`alt="Title &#34;quoted&#34; &lt;b&gt;tag&lt;/b&gt;"`,
	}

	for _, want := range expected {
		if !strings.Contains(html, want) {
			t.Fatalf("comicDescription() missing %q in %s", want, html)
		}
	}
}
