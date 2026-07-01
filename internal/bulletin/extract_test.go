package bulletin

import (
	"strings"
	"testing"
)

// sampleArticleHTML is a minimal but readability-extractable article page. Its
// body comfortably exceeds minExtractedLen so extraction succeeds. Shared with
// the Fetch httptest test.
const sampleArticleHTML = `<!DOCTYPE html>
<html><head><title>Transit Budget Debate</title></head>
<body>
<nav>home about contact</nav>
<article>
<h1>City Council Debates Transit Budget</h1>
<p>The city council convened on Tuesday evening to debate the proposed transit
budget for the coming fiscal year, a plan that would raise fares while expanding
weekend service across three underserved districts. Supporters argued the change
would stabilise the agency's finances; opponents warned that higher fares would
push riders back into cars and undercut the very ridership the expansion aims to
attract. After two hours of public comment the vote was deferred to next week.</p>
</article>
<footer>copyright</footer>
</body></html>`

func TestExtractTextSuccess(t *testing.T) {
	got, err := extractText("https://news.example/transit", []byte(sampleArticleHTML))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if !strings.Contains(got, "city council convened") {
		t.Errorf("extracted text missing article body: %q", got)
	}
	if strings.Contains(got, "copyright") {
		t.Errorf("extracted text leaked chrome/footer: %q", got)
	}
}

func TestExtractTextTooShort(t *testing.T) {
	_, err := extractText("https://news.example/stub", []byte(`<html><body><p>tiny</p></body></html>`))
	if err == nil {
		t.Fatal("expected an error for below-threshold extraction, got nil")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("error = %q, want a too-short error", err)
	}
}
