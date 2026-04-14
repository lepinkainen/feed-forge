package feissarimokat

import (
	"strings"
	"testing"
)

func TestRenderImageTagEscapesAttributes(t *testing.T) {
	tag := renderImageTag(
		`https://static.feissarimokat.com/img/test.jpg" onerror="alert(1)`,
		`Title "quoted" <b>tag</b>`,
	)

	expected := []string{
		`src="https://static.feissarimokat.com/img/test.jpg&#34; onerror=&#34;alert(1)"`,
		`alt="Title &#34;quoted&#34; &lt;b&gt;tag&lt;/b&gt;"`,
	}

	for _, want := range expected {
		if !strings.Contains(tag, want) {
			t.Fatalf("renderImageTag() missing %q in %s", want, tag)
		}
	}
}
