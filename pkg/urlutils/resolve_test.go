package urlutils

import "testing"

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		relativeURL string
		want        string
		wantErr     bool
	}{
		{
			name:        "relative path",
			baseURL:     "https://example.com/articles/post/",
			relativeURL: "image.jpg",
			want:        "https://example.com/articles/post/image.jpg",
		},
		{
			name:        "root relative path",
			baseURL:     "https://example.com/articles/post/",
			relativeURL: "/static/image.jpg",
			want:        "https://example.com/static/image.jpg",
		},
		{
			name:        "absolute URL unchanged",
			baseURL:     "https://example.com/articles/post/",
			relativeURL: "https://cdn.example.org/image.jpg",
			want:        "https://cdn.example.org/image.jpg",
		},
		{
			name:        "query only reference",
			baseURL:     "https://example.com/articles/post",
			relativeURL: "?page=2",
			want:        "https://example.com/articles/post?page=2",
		},
		{
			name:        "invalid base URL",
			baseURL:     "://bad-base",
			relativeURL: "image.jpg",
			wantErr:     true,
		},
		{
			name:        "invalid relative URL",
			baseURL:     "https://example.com",
			relativeURL: "http://[::1",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveURL(tt.baseURL, tt.relativeURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("ResolveURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
