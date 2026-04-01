package redditjson

import "testing"

func TestConstructFeedURL(t *testing.T) {
	if got := constructFeedURL("feed123", "alice", ""); got != "https://www.reddit.com/.json?feed=feed123&user=alice" {
		t.Fatalf("constructFeedURL() = %q", got)
	}
	if got := constructFeedURL("feed123", "alice", "https://proxy.example/fetch"); got != "https://proxy.example/fetch" {
		t.Fatalf("constructFeedURL(proxy) = %q", got)
	}
}

func TestFilterPosts(t *testing.T) {
	posts := []RedditPost{
		{Data: struct {
			Title        string       `json:"title"`
			URL          string       `json:"url"`
			Permalink    string       `json:"permalink"`
			CreatedUTC   float64      `json:"created_utc"`
			Score        int          `json:"score"`
			NumComments  int          `json:"num_comments"`
			Author       string       `json:"author"`
			Subreddit    string       `json:"subreddit"`
			SelfText     string       `json:"selftext"`
			SelfTextHTML string       `json:"selftext_html"`
			Thumbnail    string       `json:"thumbnail"`
			Preview      *PreviewData `json:"preview,omitempty"`
		}{Title: "keep", Score: 100, NumComments: 20}},
		{Data: struct {
			Title        string       `json:"title"`
			URL          string       `json:"url"`
			Permalink    string       `json:"permalink"`
			CreatedUTC   float64      `json:"created_utc"`
			Score        int          `json:"score"`
			NumComments  int          `json:"num_comments"`
			Author       string       `json:"author"`
			Subreddit    string       `json:"subreddit"`
			SelfText     string       `json:"selftext"`
			SelfTextHTML string       `json:"selftext_html"`
			Thumbnail    string       `json:"thumbnail"`
			Preview      *PreviewData `json:"preview,omitempty"`
		}{Title: "drop score", Score: 10, NumComments: 20}},
		{Data: struct {
			Title        string       `json:"title"`
			URL          string       `json:"url"`
			Permalink    string       `json:"permalink"`
			CreatedUTC   float64      `json:"created_utc"`
			Score        int          `json:"score"`
			NumComments  int          `json:"num_comments"`
			Author       string       `json:"author"`
			Subreddit    string       `json:"subreddit"`
			SelfText     string       `json:"selftext"`
			SelfTextHTML string       `json:"selftext_html"`
			Thumbnail    string       `json:"thumbnail"`
			Preview      *PreviewData `json:"preview,omitempty"`
		}{Title: "drop comments", Score: 100, NumComments: 1}},
	}

	got := FilterPosts(posts, 50, 10)
	if len(got) != 1 || got[0].Data.Title != "keep" {
		t.Fatalf("FilterPosts() = %#v", got)
	}
}

func TestRedditPostMethods(t *testing.T) {
	post := RedditPost{}
	post.Data.Title = "Hello"
	post.Data.URL = "https://example.com/post"
	post.Data.Permalink = "/r/golang/comments/abc123/hello/"
	post.Data.CreatedUTC = 1700000000
	post.Data.Score = 99
	post.Data.NumComments = 12
	post.Data.Author = "alice"
	post.Data.Subreddit = "golang"
	post.Data.SelfTextHTML = "<!-- SC_OFF --><p>Fish &amp;amp; Chips</p><!-- SC_ON -->"
	post.Data.Thumbnail = "https://example.com/thumb.jpg"
	post.Data.Preview = &PreviewData{Images: []PreviewImage{{Source: ImageSource{URL: "https://example.com/preview.jpg"}}}}

	if got := post.CommentsLink(); got != "https://www.reddit.com/r/golang/comments/abc123/hello/" {
		t.Fatalf("CommentsLink() = %q", got)
	}
	if got := post.Categories(); len(got) != 1 || got[0] != "r/golang" {
		t.Fatalf("Categories() = %v", got)
	}
	if got := post.ImageURL(); got != "https://example.com/preview.jpg" {
		t.Fatalf("ImageURL() = %q", got)
	}
	if got := post.Content(); got != "<p>Fish & Chips</p>" {
		t.Fatalf("Content() = %q", got)
	}
	if got := post.AuthorURI(); got != "https://www.reddit.com/user/alice" {
		t.Fatalf("AuthorURI() = %q", got)
	}
	if got := post.Subreddit(); got != "golang" {
		t.Fatalf("Subreddit() = %q", got)
	}
}

func TestRedditPostImageURL_FallbackToThumbnail(t *testing.T) {
	post := RedditPost{}
	post.Data.Thumbnail = "https://example.com/thumb.jpg"
	if got := post.ImageURL(); got != "https://example.com/thumb.jpg" {
		t.Fatalf("ImageURL() = %q", got)
	}

	post.Data.Thumbnail = "self"
	if got := post.ImageURL(); got != "" {
		t.Fatalf("ImageURL() invalid thumbnail = %q", got)
	}
}
