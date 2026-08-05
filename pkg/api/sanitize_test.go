package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeURLForLog(t *testing.T) {
	const jwt = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "no secrets passes through unchanged",
			url:  "https://tildes.net/~tech/topics.atom",
			want: "https://tildes.net/~tech/topics.atom",
		},
		{
			name: "query is preserved when nothing is secret",
			url:  "https://lemmy.world/feeds/all.xml?sort=Active",
			want: "https://lemmy.world/feeds/all.xml?sort=Active",
		},
		{
			name: "jwt path segment is redacted",
			url:  "https://lemmy.world/feeds/front.xml/" + jwt,
			want: "https://lemmy.world/feeds/front.xml/REDACTED",
		},
		{
			name: "jwt path segment is redacted with a query present",
			url:  "https://lemmy.world/feeds/front.xml/" + jwt + "?sort=Active",
			want: "https://lemmy.world/feeds/front.xml/REDACTED?sort=Active",
		},
		{
			// Lemmy's real front page route puts the token inside a segment
			// that also carries the ".xml" suffix.
			name: "jwt embedded in a segment with a suffix is redacted",
			url:  "https://lemmy.world/feeds/front/" + jwt + ".xml",
			want: "https://lemmy.world/feeds/front/REDACTED.xml",
		},
		{
			name: "jwt embedded in a segment with a suffix and a query",
			url:  "https://lemmy.world/feeds/front/" + jwt + ".xml?sort=Active",
			want: "https://lemmy.world/feeds/front/REDACTED.xml?sort=Active",
		},
		{
			name: "secret query value is redacted, key is kept",
			url:  "https://example.com/feed?token=abc123&sort=New",
			want: "https://example.com/feed?sort=New&token=REDACTED",
		},
		{
			name: "secret query key match is case-insensitive",
			url:  "https://example.com/feed?JWT=abc123",
			want: "https://example.com/feed?JWT=REDACTED",
		},
		{
			name: "userinfo credentials are redacted",
			url:  "https://user:pass@example.com/feed",
			want: "https://REDACTED@example.com/feed",
		},
		{
			name: "dotted filename is not mistaken for a jwt",
			url:  "https://www.oglaf.com/feeds/rss/index.html",
			want: "https://www.oglaf.com/feeds/rss/index.html",
		},
		{
			name: "empty string stays empty",
			url:  "",
			want: "",
		},
		{
			name: "unparseable url fails closed",
			url:  "https://example.com/\x7f\x00",
			want: "REDACTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeURLForLog(tt.url); got != tt.want {
				t.Errorf("SanitizeURLForLog(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}

	t.Run("token never survives in the output", func(t *testing.T) {
		for _, raw := range []string{
			"https://lemmy.world/feeds/front/" + jwt + ".xml",
			"https://lemmy.world/feeds/front/" + jwt + ".xml?sort=Active",
			"https://lemmy.world/feeds/front.xml/" + jwt,
			"https://lemmy.world/feeds/front.xml?jwt=" + jwt,
		} {
			if got := SanitizeURLForLog(raw); strings.Contains(got, jwt) {
				t.Errorf("SanitizeURLForLog(%q) leaked the token: %q", raw, got)
			}
		}
	})
}

func TestSanitizeTextForLog(t *testing.T) {
	const jwt = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "token in a quoted url is redacted",
			in:   `Get "https://lemmy.world/feeds/front.xml/` + jwt + `?sort=Active": no such host`,
			want: `Get "https://lemmy.world/feeds/front.xml/REDACTED?sort=Active": no such host`,
		},
		{
			name: "host names are not mistaken for tokens",
			in:   `Get "https://media.piefed.world/posts/a.b.jpg": no such host`,
			want: `Get "https://media.piefed.world/posts/a.b.jpg": no such host`,
		},
		{
			name: "text without a token is unchanged",
			in:   "dial tcp: lookup tildes.net: no such host",
			want: "dial tcp: lookup tildes.net: no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeTextForLog(tt.in); got != tt.want {
				t.Errorf("SanitizeTextForLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRedactURLInError(t *testing.T) {
	const jwt = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	rawURL := "https://lemmy.world/feeds/front.xml/" + jwt + "?sort=Active"

	t.Run("nil error stays nil", func(t *testing.T) {
		if got := RedactURLInError(nil, rawURL); got != nil {
			t.Errorf("RedactURLInError(nil, …) = %v, want nil", got)
		}
	})

	t.Run("token is removed from the message", func(t *testing.T) {
		inner := &HTTPError{StatusCode: 401, Message: "GET " + rawURL + " failed"}
		redacted := RedactURLInError(inner, rawURL)

		if strings.Contains(redacted.Error(), jwt) {
			t.Errorf("redacted error still leaks the token: %q", redacted)
		}
		if !strings.Contains(redacted.Error(), "REDACTED") {
			t.Errorf("redacted error should mark the removal: %q", redacted)
		}

		// The wrapped error must stay inspectable, or the retry policy and the
		// transient-upstream demotion in logAPICall stop working.
		code, ok := UpstreamStatusCode(redacted)
		if !ok || code != 401 {
			t.Errorf("UpstreamStatusCode() = (%d, %v), want (401, true)", code, ok)
		}
		if !errors.Is(redacted, inner) {
			t.Error("redacted error should unwrap to the original")
		}
	})

	t.Run("error without a token is returned as-is", func(t *testing.T) {
		inner := errors.New("dial tcp: no such host")
		if got := RedactURLInError(inner, rawURL); got != inner {
			t.Errorf("RedactURLInError() = %v, want the original error unchanged", got)
		}
	})
}
