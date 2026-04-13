package hackernews

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.NewDatabase(database.Config{Path: filepath.Join(t.TempDir(), "hackernews.db")})
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestLoadConfigFromLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domains.json")
	content := []byte(`{"category_domains":{"Docs":["docs.example.com"],"News":["news.example.com"]}}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mapper := LoadConfig(path)
	if mapper == nil {
		t.Fatal("LoadConfig() = nil, want mapper")
	}
	if got := mapper.GetCategoryForDomain("docs.example.com"); got != "Docs" {
		t.Fatalf("GetCategoryForDomain() = %q, want %q", got, "Docs")
	}
}

func TestLoadConfigFallsBackToEmbedded(t *testing.T) {
	mapper := LoadConfig("")
	if mapper == nil {
		t.Fatal("LoadConfig(\"\") = nil, want mapper")
	}
	if got := mapper.GetCategoryForDomain("github.com"); got != "GitHub" {
		t.Fatalf("GetCategoryForDomain(github.com) = %q, want %q", got, "GitHub")
	}
}

func TestGetAllCategories(t *testing.T) {
	mapper := NewCategoryMapper(&DomainConfig{CategoryDomains: map[string][]string{
		"Docs": {"docs.example.com"},
		"News": {"news.example.com"},
	}})

	got := mapper.GetAllCategories()
	sort.Strings(got)
	want := []string{"Docs", "News"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllCategories() = %v, want %v", got, want)
	}
}

func TestItemGetters(t *testing.T) {
	createdAt := time.Unix(1700000000, 0).UTC()
	item := &Item{
		ItemTitle:        "Story",
		ItemLink:         "https://example.com/story",
		ItemCommentsLink: "https://news.ycombinator.com/item?id=1",
		Points:           123,
		ItemCommentCount: 45,
		ItemAuthor:       "alice",
		ItemCreatedAt:    createdAt,
		Domain:           "example.com",
		ItemCategories:   []string{"example.com", "Popular 50+"},
	}

	if item.Title() != "Story" || item.Link() != "https://example.com/story" || item.CommentsLink() != "https://news.ycombinator.com/item?id=1" {
		t.Fatalf("basic getters returned unexpected values: %#v", item)
	}
	if item.Author() != "alice" || item.Score() != 123 || item.CommentCount() != 45 {
		t.Fatalf("author/score/count getters returned unexpected values: %#v", item)
	}
	if !item.CreatedAt().Equal(createdAt) {
		t.Fatalf("CreatedAt() = %v, want %v", item.CreatedAt(), createdAt)
	}
	if !reflect.DeepEqual(item.Categories(), []string{"example.com", "Popular 50+"}) {
		t.Fatalf("Categories() = %v", item.Categories())
	}
	if item.ImageURL() != "" || item.Content() != "" {
		t.Fatalf("ImageURL()/Content() should be empty for Hacker News items")
	}
	if item.AuthorURI() != "https://news.ycombinator.com/user?id=alice" {
		t.Fatalf("AuthorURI() = %q", item.AuthorURI())
	}
	if item.ItemDomain() != "example.com" {
		t.Fatalf("ItemDomain() = %q, want %q", item.ItemDomain(), "example.com")
	}
}

func TestDatabaseHelpers(t *testing.T) {
	db := newTestDB(t)

	if err := initializeSchema(db); err != nil {
		t.Fatalf("initializeSchema() error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	items := []Item{
		{
			ItemID:           "1",
			ItemTitle:        "First",
			ItemLink:         "https://example.com/1",
			ItemCommentsLink: "https://news.ycombinator.com/item?id=1",
			Points:           120,
			ItemCommentCount: 10,
			ItemAuthor:       "alice",
			ItemCreatedAt:    now.Add(-time.Hour),
			UpdatedAt:        now,
		},
		{
			ItemID:           "2",
			ItemTitle:        "Second",
			ItemLink:         "https://example.com/2",
			ItemCommentsLink: "https://news.ycombinator.com/item?id=2",
			Points:           40,
			ItemCommentCount: 5,
			ItemAuthor:       "bob",
			ItemCreatedAt:    now,
			UpdatedAt:        now,
		},
	}

	updated := updateStoredItems(db, items)
	if !updated["1"] || !updated["2"] || len(updated) != 2 {
		t.Fatalf("updateStoredItems(insert) = %v, want both items marked updated", updated)
	}

	items[0].Points = 150
	items[0].ItemTitle = "First Updated"
	updated = updateStoredItems(db, items[:1])
	if !updated["1"] || len(updated) != 1 {
		t.Fatalf("updateStoredItems(update) = %v, want item 1 updated", updated)
	}

	got, err := getAllItems(db, 10, 50)
	if err != nil {
		t.Fatalf("getAllItems() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("getAllItems() len = %d, want 1", len(got))
	}
	if got[0].ItemID != "1" || got[0].Points != 150 || got[0].ItemTitle != "First Updated" {
		t.Fatalf("getAllItems() first item = %#v", got[0])
	}
}

func TestPreprocessItems(t *testing.T) {
	mapper := NewCategoryMapper(&DomainConfig{CategoryDomains: map[string][]string{
		"Docs": {"example.com"},
	}})

	items := []Item{{
		ItemTitle: "Show HN: Amazing PDF Viewer",
		ItemLink:  "https://example.com/article.pdf",
		Points:    125,
	}}

	got := preprocessItems(items, 50, mapper)
	if len(got) != 1 {
		t.Fatalf("preprocessItems() len = %d, want 1", len(got))
	}
	if got[0].Domain != "example.com" {
		t.Fatalf("Domain = %q, want %q", got[0].Domain, "example.com")
	}
	wantCategories := []string{"example.com", "Docs", "Show HN", "High Score 100+"}
	for _, want := range wantCategories {
		if !contains(got[0].ItemCategories, want) {
			t.Fatalf("categories = %v, missing %q", got[0].ItemCategories, want)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
