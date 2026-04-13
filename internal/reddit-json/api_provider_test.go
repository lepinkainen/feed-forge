package redditjson

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

func redditListingJSON(posts ...string) string {
	return `{"data":{"children":[` + strings.Join(posts, ",") + `]}}`
}

func redditPostJSON(title, url, permalink string, score, comments int, author, subreddit string, created float64) string {
	return fmt.Sprintf(`{"data":{"title":%q,"url":%q,"permalink":%q,"created_utc":%.0f,"score":%d,"num_comments":%d,"author":%q,"subreddit":%q}}`, title, url, permalink, created, score, comments, author, subreddit)
}

func TestNewRedditAPIAndFetchRedditHomepage(t *testing.T) {
	var gotSecret, gotFeedID, gotUser, gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSecret = r.Header.Get("X-Proxy-Secret")
		gotFeedID = r.Header.Get("X-Feed-ID")
		gotUser = r.Header.Get("X-Feed-User")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(redditListingJSON(
			redditPostJSON("hello", "https://www.reddit.com/r/golang/comments/1", "/r/golang/comments/1", 100, 20, "alice", "golang", 1700000000),
		)))
	}))
	defer server.Close()

	api := NewRedditAPI(server.URL, "secret", "feed123", "alice")
	if api == nil || api.feedURL != server.URL || api.client == nil {
		t.Fatalf("NewRedditAPI() = %#v", api)
	}

	posts, err := api.FetchRedditHomepage()
	if err != nil {
		t.Fatalf("FetchRedditHomepage() error = %v", err)
	}
	if len(posts) != 1 || posts[0].Data.Title != "hello" {
		t.Fatalf("FetchRedditHomepage() = %#v", posts)
	}
	if gotSecret != "secret" || gotFeedID != "feed123" || gotUser != "alice" {
		t.Fatalf("proxy headers = (%q, %q, %q)", gotSecret, gotFeedID, gotUser)
	}
	if !strings.Contains(gotUA, "feed-forge/1.0") {
		t.Fatalf("User-Agent = %q, want reddit client UA", gotUA)
	}
}

func TestFetchRedditHomepage_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	api := NewRedditAPI(server.URL, "", "", "")
	posts, err := api.FetchRedditHomepage()
	if err == nil || posts != nil {
		t.Fatalf("FetchRedditHomepage() = (%#v, %v), want error", posts, err)
	}
}

func TestRedditPostAdditionalMethods(t *testing.T) {
	post := &RedditPost{}
	post.Data.Title = "Hello"
	post.Data.URL = "https://example.com/post"
	post.Data.Permalink = "/r/golang/comments/abc123/hello/"
	post.Data.CreatedUTC = 1700000000
	post.Data.Score = 99
	post.Data.NumComments = 12
	post.Data.Author = "alice"
	post.Data.Subreddit = "golang"

	if post.Title() != "Hello" || post.Link() != "https://example.com/post" || post.Author() != "alice" {
		t.Fatalf("basic getters returned unexpected values: %#v", post)
	}
	if post.Score() != 99 || post.CommentCount() != 12 {
		t.Fatalf("score/comment getters returned unexpected values: %#v", post)
	}
	if got := post.CreatedAt(); got.Unix() != 1700000000 {
		t.Fatalf("CreatedAt() = %v, want unix 1700000000", got)
	}

	empty := &RedditPost{}
	if got := empty.Categories(); len(got) != 0 {
		t.Fatalf("Categories(empty) = %v, want empty", got)
	}
	if got := empty.Content(); got != "" {
		t.Fatalf("Content(empty) = %q, want empty", got)
	}
	empty.Data.SelfTextHTML = "null"
	if got := empty.Content(); got != "" {
		t.Fatalf("Content(null) = %q, want empty", got)
	}
}

func TestRedditProviderFetchItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(redditListingJSON(
			redditPostJSON("keep one", "https://www.reddit.com/r/golang/comments/1", "/r/golang/comments/1", 100, 20, "alice", "golang", 1700000000),
			redditPostJSON("keep two", "https://www.reddit.com/r/golang/comments/2", "/r/golang/comments/2", 80, 15, "bob", "golang", 1700000001),
			redditPostJSON("drop", "https://www.reddit.com/r/golang/comments/3", "/r/golang/comments/3", 10, 1, "carol", "golang", 1700000002),
		)))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	filesystem.SetCacheDir(cacheDir)
	t.Cleanup(func() { filesystem.SetCacheDir("") })

	providerAny, err := NewRedditProvider(50, 10, "feed123", "alice", server.URL, "", "")
	if err != nil {
		t.Fatalf("NewRedditProvider() error = %v", err)
	}
	provider := providerAny.(*RedditProvider)
	defer func() { _ = provider.Close() }()

	items, err := provider.FetchItems(1)
	if err != nil {
		t.Fatalf("FetchItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(FetchItems()) = %d, want 1", len(items))
	}
	if items[0].Title() != "keep one" {
		t.Fatalf("FetchItems()[0].Title() = %q, want keep one", items[0].Title())
	}
}

func TestRedditProviderGenerateFeed(t *testing.T) {
	oldOverride := feed.GetTemplateOverrideFS()
	oldFallback := feed.GetTemplateFallbackFS()
	feed.SetTemplateOverrideFS(fstest.MapFS{
		"reddit-atom.tmpl": &fstest.MapFile{Data: []byte(`<feed>{{range .Items}}<entry><title>{{.Title}}</title><author>{{.Author}}</author><summary>{{.Summary}}</summary></entry>{{end}}</feed>`)},
	})
	feed.SetTemplateFallbackFS(fstest.MapFS{})
	t.Cleanup(func() {
		feed.SetTemplateOverrideFS(oldOverride)
		feed.SetTemplateFallbackFS(oldFallback)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(redditListingJSON(
			redditPostJSON("Generated post", "https://www.reddit.com/r/golang/comments/1", "/r/golang/comments/1", 100, 20, "alice", "golang", float64(time.Now().Unix())),
		)))
	}))
	defer server.Close()

	filesystem.SetCacheDir(t.TempDir())
	t.Cleanup(func() { filesystem.SetCacheDir("") })

	providerAny, err := NewRedditProvider(50, 10, "feed123", "alice", server.URL, "", "")
	if err != nil {
		t.Fatalf("NewRedditProvider() error = %v", err)
	}
	provider := providerAny.(*RedditProvider)
	defer func() { _ = provider.Close() }()

	outfile := filepath.Join(t.TempDir(), "feeds", "reddit.xml")
	if err := provider.GenerateFeed(outfile); err != nil {
		t.Fatalf("GenerateFeed() error = %v", err)
	}

	content, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	body := string(content)
	for _, want := range []string{"Generated post", "alice", "Score: 100 | Comments: 20"} {
		if !strings.Contains(body, want) {
			t.Fatalf("generated feed missing %q:\n%s", want, body)
		}
	}
}
