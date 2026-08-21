package lemmy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/providers"
)

const testJWT = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

// fixtureItems serves testdata/front.xml and returns the parsed items, mapped
// the same way Provider.FetchItems maps them, without spinning up the
// BaseProvider (which would create real databases).
func fixtureItems(t *testing.T) []*Item {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("testdata", "front.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = io.Copy(w, strings.NewReader(string(body)))
	}))
	defer srv.Close()

	raw, err := fetchFeed(srv.URL, "https://lemmy.world", "Active")
	if err != nil {
		t.Fatalf("fetchFeed: %v", err)
	}

	items := make([]*Item, 0, len(raw))
	for _, entry := range raw {
		items = append(items, parseItem(entry, "lemmy.world"))
	}
	return items
}

func findByTitlePrefix(t *testing.T, items []*Item, prefix string) *Item {
	t.Helper()
	for _, item := range items {
		if strings.HasPrefix(item.Title(), prefix) {
			return item
		}
	}
	t.Fatalf("no fixture item with title prefix %q", prefix)
	return nil
}

func TestFetchItemsAgainstFixture(t *testing.T) {
	items := fixtureItems(t)
	if len(items) != 3 {
		t.Fatalf("parsed item count = %d, want 3", len(items))
	}

	imagePost := findByTitlePrefix(t, items, "Jackpot")
	linkPost := findByTitlePrefix(t, items, "Student Teacher")
	textPost := findByTitlePrefix(t, items, "What is your backup")

	t.Run("image post", func(t *testing.T) {
		if imagePost.Score() != 1253 || imagePost.CommentCount() != 335 {
			t.Errorf("counts = (%d, %d), want (1253, 335)", imagePost.Score(), imagePost.CommentCount())
		}
		// The poster lives on piefed.world, not the configured instance.
		if got, want := imagePost.Author(), "The_Picard_Maneuver@piefed.world"; got != want {
			t.Errorf("Author() = %q, want %q", got, want)
		}
		if got, want := imagePost.AuthorURI(), "https://piefed.world/u/The_Picard_Maneuver"; got != want {
			t.Errorf("AuthorURI() = %q, want %q", got, want)
		}
		if got, want := imagePost.ImageURL(), "https://lemmy.world/pictrs/image/08675766-b08c-483d-8bf2-e680cffaf81b.jpeg"; got != want {
			t.Errorf("ImageURL() = %q, want %q", got, want)
		}
		if got, want := imagePost.Link(), "https://media.piefed.world/posts/vW/3u/vW3u8B3uA5VbHFh.jpg"; got != want {
			t.Errorf("Link() = %q, want %q", got, want)
		}
		if got, want := imagePost.CommentsLink(), "https://lemmy.world/post/50248187"; got != want {
			t.Errorf("CommentsLink() = %q, want %q", got, want)
		}
		if imagePost.Content() != "" {
			t.Errorf("Content() = %q, want empty for an image post", imagePost.Content())
		}
		if got, want := imagePost.Categories(), []string{"memes"}; len(got) != 1 || got[0] != want[0] {
			t.Errorf("Categories() = %v, want %v", got, want)
		}
		if imagePost.CreatedAt().IsZero() {
			t.Error("CreatedAt() is zero; pubDate was not parsed")
		}
		if got, want := imagePost.CreatedAt().UTC().Format("2006-01-02T15:04:05Z"), "2026-08-03T12:54:24Z"; got != want {
			t.Errorf("CreatedAt() = %q, want %q", got, want)
		}
	})

	t.Run("link post keeps its body", func(t *testing.T) {
		if linkPost.Score() != 851 || linkPost.CommentCount() != 385 {
			t.Errorf("counts = (%d, %d), want (851, 385)", linkPost.Score(), linkPost.CommentCount())
		}
		if got, want := linkPost.Link(), "https://www.gadgetreview.com/student-teacher-snapchat"; got != want {
			t.Errorf("Link() = %q, want %q", got, want)
		}
		if linkPost.Link() == linkPost.CommentsLink() {
			t.Error("Link() should differ from CommentsLink() for a link post")
		}
		if !strings.HasPrefix(linkPost.Content(), "<p>Sixty minutes.") {
			t.Errorf("Content() = %q, want the post body with the header stripped", linkPost.Content())
		}
		if strings.Contains(linkPost.Content(), "submitted by") || strings.Contains(linkPost.Content(), "points |") {
			t.Errorf("Content() still contains the Lemmy header: %q", linkPost.Content())
		}
		// <category> wins over the lower-case community anchor in the header.
		if got, want := linkPost.Categories(), "Technology"; len(got) != 1 || got[0] != want {
			t.Errorf("Categories() = %v, want [%q]", got, want)
		}
		// A text/html enclosure is not a thumbnail; media:content is.
		if got, want := linkPost.ImageURL(), "https://www.gadgetreview.com/wp-content/uploads/Snapchats-AI.jpg"; got != want {
			t.Errorf("ImageURL() = %q, want %q", got, want)
		}
	})

	t.Run("text post", func(t *testing.T) {
		if textPost.Score() != 42 || textPost.CommentCount() != 7 {
			t.Errorf("counts = (%d, %d), want (42, 7)", textPost.Score(), textPost.CommentCount())
		}
		if textPost.Link() != textPost.CommentsLink() {
			t.Errorf("Link() = %q should equal CommentsLink() = %q for a text post", textPost.Link(), textPost.CommentsLink())
		}
		// Local account: no "@host" suffix.
		if got, want := textPost.Author(), "localadmin"; got != want {
			t.Errorf("Author() = %q, want %q", got, want)
		}
		if !strings.HasPrefix(textPost.Content(), "<p>I run restic") {
			t.Errorf("Content() = %q, want the post body", textPost.Content())
		}
		if textPost.ImageURL() != "" {
			t.Errorf("ImageURL() = %q, want empty", textPost.ImageURL())
		}
	})
}

// TestParseItemCounts covers the counts line directly, including the signed
// score a downvoted post carries. Without the sign, "-3 points" parses as +3
// and passes a positive --min-score.
func TestParseItemCounts(t *testing.T) {
	const header = `submitted by <a href="https://lemmy.world/u/someone">someone</a> to <a href="https://lemmy.world/c/test">test</a><br>`

	tests := []struct {
		name         string
		description  string
		wantScore    int
		wantComments int
		wantContent  string
	}{
		{
			name:         "positive score",
			description:  header + `42 points | <a href="https://lemmy.world/post/1">7 comments</a><br><p>body</p>`,
			wantScore:    42,
			wantComments: 7,
			wantContent:  "<p>body</p>",
		},
		{
			name:         "negative score",
			description:  header + `-3 points | <a href="https://lemmy.world/post/1">7 comments</a><br><p>body</p>`,
			wantScore:    -3,
			wantComments: 7,
			wantContent:  "<p>body</p>",
		},
		{
			name:         "zero score",
			description:  header + `0 points | <a href="https://lemmy.world/post/1">0 comments</a><br><p>body</p>`,
			wantScore:    0,
			wantComments: 0,
			wantContent:  "<p>body</p>",
		},
		{
			name:         "counts line missing",
			description:  header + `<p>body</p>`,
			wantScore:    0,
			wantComments: 0,
			wantContent:  "<p>body</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := parseItem(rssItem{
				Title:       "Test",
				Guid:        "https://lemmy.world/post/1",
				Description: tt.description,
				Creator:     "https://lemmy.world/u/someone",
			}, "lemmy.world")

			if item.Score() != tt.wantScore {
				t.Errorf("Score() = %d, want %d", item.Score(), tt.wantScore)
			}
			if item.CommentCount() != tt.wantComments {
				t.Errorf("CommentCount() = %d, want %d", item.CommentCount(), tt.wantComments)
			}
			if item.Content() != tt.wantContent {
				t.Errorf("Content() = %q, want %q", item.Content(), tt.wantContent)
			}
			if strings.Contains(item.Content(), "points") || strings.Contains(item.Content(), "submitted by") {
				t.Errorf("Content() still contains the header: %q", item.Content())
			}
		})
	}
}

// TestDownvotedPostIsFiltered is the behavioral consequence of parsing the
// score as signed: a downvoted post falls below any threshold of 0 or more.
// Dropping the sign would parse "-3 points" as +3 and let the post through.
func TestDownvotedPostIsFiltered(t *testing.T) {
	item := parseItem(rssItem{
		Title:       "Unpopular",
		Guid:        "https://lemmy.world/post/1",
		Description: `submitted by <a href="https://lemmy.world/u/x">x</a> to <a href="https://lemmy.world/c/test">test</a><br>-3 points | <a href="https://lemmy.world/post/1">1 comments</a><br>`,
	}, "lemmy.world")

	if item.Score() != -3 {
		t.Fatalf("Score() = %d, want -3", item.Score())
	}
	for _, minScore := range []int{0, 1, 25} {
		if !(&Provider{MinScore: minScore}).skip(item) {
			t.Errorf("a post at %d points should be dropped by MinScore %d", item.Score(), minScore)
		}
	}
	// A negative threshold still lets it through, which is how a reader opts
	// into seeing downvoted posts.
	if (&Provider{MinScore: -10}).skip(item) {
		t.Error("MinScore -10 should keep a post at -3 points")
	}
}

func TestBuildFrontFeedURL(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		sort     string
		want     string
	}{
		{
			// The route is /feeds/{type}/{name}.xml, so the JWT is the name.
			// The documented "/feeds/front.xml/{jwt}" form 404s.
			name:     "plain instance",
			instance: "https://lemmy.world",
			sort:     "Active",
			want:     "https://lemmy.world/feeds/front/" + testJWT + ".xml?sort=Active",
		},
		{
			name:     "trailing slash is trimmed",
			instance: "https://lemmy.world/",
			sort:     "TopDay",
			want:     "https://lemmy.world/feeds/front/" + testJWT + ".xml?sort=TopDay",
		},
		{
			name:     "empty sort omits the query",
			instance: "https://sopuli.xyz",
			sort:     "",
			want:     "https://sopuli.xyz/feeds/front/" + testJWT + ".xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFrontFeedURL(tt.instance, testJWT, tt.sort); got != tt.want {
				t.Errorf("buildFrontFeedURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSkipFilters(t *testing.T) {
	items := fixtureItems(t)

	tests := []struct {
		name      string
		provider  *Provider
		wantKept  int
		wantTitle string
	}{
		{
			name:     "no filtering keeps everything",
			provider: &Provider{},
			wantKept: 3,
		},
		{
			name:     "min score drops the text post",
			provider: &Provider{MinScore: 100},
			wantKept: 2,
		},
		{
			name:     "min comments drops the text post",
			provider: &Provider{MinComments: 100},
			wantKept: 2,
		},
		{
			name:      "ignore list drops a community",
			provider:  &Provider{IgnoreCommunities: normalizeCommunitySet([]string{"memes"})},
			wantKept:  2,
			wantTitle: "Jackpot",
		},
		{
			name:      "ignore list matches a differently-cased category",
			provider:  &Provider{IgnoreCommunities: normalizeCommunitySet([]string{"technology"})},
			wantKept:  2,
			wantTitle: "Student Teacher",
		},
		{
			name:      "ignore list accepts the !community@instance form",
			provider:  &Provider{IgnoreCommunities: normalizeCommunitySet([]string{"!selfhosted@lemmy.world"})},
			wantKept:  2,
			wantTitle: "What is your backup",
		},
		{
			name:     "thresholds can drop everything",
			provider: &Provider{MinScore: 10000},
			wantKept: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kept := make([]*Item, 0, len(items))
			for _, item := range items {
				if !tt.provider.skip(item) {
					kept = append(kept, item)
				}
			}
			if len(kept) != tt.wantKept {
				t.Fatalf("kept %d items, want %d", len(kept), tt.wantKept)
			}
			if tt.wantTitle == "" {
				return
			}
			for _, item := range kept {
				if strings.HasPrefix(item.Title(), tt.wantTitle) {
					t.Errorf("item %q should have been filtered out", item.Title())
				}
			}
		})
	}
}

func TestNormalizeCommunity(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"memes", "memes"},
		{"Memes", "memes"},
		{"  Technology  ", "technology"},
		{"!selfhosted", "selfhosted"},
		{"!selfhosted@lemmy.world", "selfhosted"},
		{"c/selfhosted", "selfhosted"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := normalizeCommunity(tt.in); got != tt.want {
			t.Errorf("normalizeCommunity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveJWT(t *testing.T) {
	t.Run("environment wins over config", func(t *testing.T) {
		t.Setenv(JWTEnv, "from-env")
		if got := resolveJWT("from-config"); got != "from-env" {
			t.Errorf("resolveJWT() = %q, want %q", got, "from-env")
		}
	})

	t.Run("config is the fallback", func(t *testing.T) {
		t.Setenv(JWTEnv, "")
		if got := resolveJWT("from-config"); got != "from-config" {
			t.Errorf("resolveJWT() = %q, want %q", got, "from-config")
		}
	})

	t.Run("whitespace is trimmed", func(t *testing.T) {
		t.Setenv(JWTEnv, "  padded  ")
		if got := resolveJWT(""); got != "padded" {
			t.Errorf("resolveJWT() = %q, want %q", got, "padded")
		}
	})
}

func TestMissingJWTErrorLeaksNothing(t *testing.T) {
	t.Setenv(JWTEnv, "")

	_, err := NewLemmyProvider(&Config{Instance: "https://lemmy.world"})
	if err == nil {
		t.Fatal("NewLemmyProvider should fail without a JWT")
	}
	if !strings.Contains(err.Error(), JWTEnv) {
		t.Errorf("error should name %s, got %q", JWTEnv, err)
	}
	if strings.Contains(err.Error(), "front.xml") {
		t.Errorf("error must not contain the feed URL, got %q", err)
	}
}

func TestRegistryRegistration(t *testing.T) {
	info, err := providers.DefaultRegistry.Get("lemmy")
	if err != nil {
		t.Fatalf("registry lookup: %v", err)
	}
	if info == nil {
		t.Fatal("registry entry is nil")
	}
	if info.Name != "lemmy" || info.Preview == nil || info.Preview.TemplateName != "lemmy-atom" {
		t.Errorf("unexpected registry metadata: %+v", info)
	}

	cfg, ok := info.ConfigFactory().(*Config)
	if !ok {
		t.Fatal("ConfigFactory did not return *lemmy.Config")
	}
	if cfg.Instance != defaultInstance || cfg.Sort != defaultSort {
		t.Errorf("config defaults = (%q, %q), want (%q, %q)", cfg.Instance, cfg.Sort, defaultInstance, defaultSort)
	}

	// The feed ID must never be the authenticated URL: it is rendered into the
	// Atom output and into the generated index.html and feeds.opml.
	if strings.Contains(info.Preview.Config.ID, "front.xml") {
		t.Errorf("preview feed ID must not reference front.xml, got %q", info.Preview.Config.ID)
	}

	if _, err := factory("not a config"); err == nil {
		t.Error("factory should reject wrong config type")
	}
}
