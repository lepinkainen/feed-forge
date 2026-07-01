package bulletin

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

const (
	bulletinTemplate = "bulletin-atom.tmpl"
	bulletinPageTmpl = "bulletin-page.html.tmpl"
	// LatestPageName is the stable filename for the most recent bulletin HTML page.
	LatestPageName = "bulletin-latest.html"
)

// atomData is the view model for the bulletin Atom template.
type atomData struct {
	FeedTitle string
	Subtitle  string
	FeedID    string
	SelfLink  string
	Updated   string
	Generator string
	Entries   []atomEntry
}

type atomEntry struct {
	ID        string
	Title     string
	Link      string
	Updated   string
	Published string
	Content   string // trusted HTML fragment from the model, emitted in CDATA
}

// slotFor derives a human bulletin slot label from the hour of day when one is
// not supplied explicitly.
func slotFor(t time.Time) string {
	switch h := t.Hour(); {
	case h < 12:
		return "Morning"
	case h < 18:
		return "Afternoon"
	default:
		return "Evening"
	}
}

// PublishOptions carries the inputs for Publish.
type PublishOptions struct {
	Config      Config
	DBPath      string
	Outfile     string // Atom feed output path
	HTMLDir     string // when non-empty, also export the digest as an HTML page here
	FeedBaseURL string // self/alternate link for the Atom feed
	Slot        string // bulletin slot label; derived from time of day when empty
	APIKey      string // resolved Anthropic API key (see llm.Config.ResolveAPIKey)
}

// feedEntryLimit is how many recent bulletins the Atom feed carries, so a reader
// that misses a publish can still catch up.
const feedEntryLimit = 20

// Publish summarises any unpublished items into a new bulletin (the only step
// that calls the model), then always (re-)renders the Atom feed and HTML page
// from the stored bulletins. Running it with no unpublished items therefore
// rebuilds the outputs from existing data without any LLM call.
func Publish(opts PublishOptions) error {
	cfg := opts.Config.withDefaults()
	ctx := context.Background()

	store, err := NewStore(opts.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	newRow, err := createBulletinIfPending(ctx, store, cfg, opts.Slot, opts.HTMLDir, opts.APIKey)
	if err != nil {
		return err
	}

	bulletins, err := store.LatestBulletins(ctx, feedEntryLimit)
	if err != nil {
		return err
	}
	if len(bulletins) == 0 {
		slog.Info("bulletin: no bulletins to render")
		return nil
	}

	if err := writeAtom(opts.Outfile, opts.FeedBaseURL, bulletins); err != nil {
		return err
	}
	if opts.HTMLDir != "" {
		if err := writeLatestHTML(opts.HTMLDir, bulletins[0]); err != nil {
			return err
		}
	}

	slog.Info("bulletin: rendered feed", "entries", len(bulletins), "new", newRow != nil, "outfile", opts.Outfile)
	return nil
}

// createBulletinIfPending summarises unpublished items into a new stored
// bulletin and returns it, or nil when there is nothing pending. When htmlDir is
// set, the dated archive page is written before the row is committed, so the
// commit that marks the items consumed never happens without its archive.
func createBulletinIfPending(ctx context.Context, store *Store, cfg Config, slot, htmlDir, apiKey string) (*Row, error) {
	items, err := store.UnpublishedItems(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		slog.Info("bulletin: no new items; rebuilding outputs from stored bulletins")
		return nil, nil
	}

	clusters := clusterItems(items, cfg.SimhashThreshold)
	slog.Info("bulletin: clustered", "items", len(items), "clusters", len(clusters))

	summarizer, err := NewSummarizer(cfg, apiKey)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if slot == "" {
		slot = slotFor(now)
	}
	digest, err := summarizer.Summarize(ctx, clusters)
	if err != nil {
		return nil, err
	}

	row := Row{
		PublishedAt: now,
		Slot:        slot,
		Title:       bulletinTitle(slot, now),
		Content:     digest,
	}

	// Write the durable archive page first: CreateBulletin is what marks these
	// items published, so if the write fails we return before committing and the
	// items stay pending for the next run. A crash after the write merely
	// rewrites the same dated file next time.
	if htmlDir != "" {
		if derr := writeDatedHTML(htmlDir, row); derr != nil {
			return nil, derr
		}
	}

	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	id, err := store.CreateBulletin(ctx, row, ids)
	if err != nil {
		return nil, err
	}
	row.ID = id
	slog.Info("bulletin: published", "id", id, "slot", slot, "items", len(ids))
	return &row, nil
}

// bulletinTitle builds the display title for a bulletin.
func bulletinTitle(slot string, t time.Time) string {
	return fmt.Sprintf("%s Bulletin — %s", slot, t.Format("Mon, 2 Jan 2006"))
}

// cdataSafe neutralises the only sequence that can terminate a CDATA section, so
// a model-authored digest containing a literal "]]>" cannot break out and
// corrupt the surrounding Atom XML. The split closes and immediately reopens the
// CDATA around the ">" so the rendered text is unchanged.
func cdataSafe(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}

// writeAtom renders the feed carrying the given bulletins (newest first) as one
// Atom entry each.
func writeAtom(outfile, feedBaseURL string, bulletins []Row) error {
	tmplContent, err := feed.ReadTemplateContent(bulletinTemplate)
	if err != nil {
		return fmt.Errorf("read bulletin template: %w", err)
	}
	tmpl, err := template.New("bulletin").Funcs(feed.TemplateFuncs()).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("parse bulletin template: %w", err)
	}

	entries := make([]atomEntry, len(bulletins))
	for i, b := range bulletins {
		stamp := b.PublishedAt.Format(time.RFC3339)
		entries[i] = atomEntry{
			ID:        fmt.Sprintf("urn:feed-forge:bulletin:%d", b.ID),
			Title:     b.Title,
			Link:      feedBaseURL,
			Updated:   stamp,
			Published: stamp,
			Content:   cdataSafe(b.Content),
		}
	}

	data := atomData{
		FeedTitle: "Feed Forge Bulletin",
		Subtitle:  "Aggregated, de-duplicated news digests",
		FeedID:    "urn:feed-forge:bulletin",
		SelfLink:  feedBaseURL,
		Updated:   entries[0].Updated,
		Generator: "feed-forge",
		Entries:   entries,
	}

	if derr := filesystem.EnsureDirectoryExists(outfile); derr != nil {
		return derr
	}
	// #nosec G304 -- output path is an explicit CLI/config input, intentionally written to disk.
	f, err := os.Create(outfile)
	if err != nil {
		return fmt.Errorf("create outfile: %w", err)
	}
	defer func() { _ = f.Close() }()

	if terr := tmpl.Execute(f, data); terr != nil {
		return fmt.Errorf("execute bulletin template: %w", terr)
	}
	return nil
}

// htmlPageData is the view model for the standalone bulletin HTML page.
type htmlPageData struct {
	Title     string
	Slot      string
	Date      string
	Generated string
	Content   string // trusted HTML fragment from the model
}

// renderHTMLPage renders a bulletin as a standalone HTML document.
func renderHTMLPage(b Row) ([]byte, error) {
	tmplContent, err := feed.ReadTemplateContent(bulletinPageTmpl)
	if err != nil {
		return nil, fmt.Errorf("read bulletin page template: %w", err)
	}
	tmpl, err := template.New("bulletin-page").Funcs(feed.TemplateFuncs()).Parse(tmplContent)
	if err != nil {
		return nil, fmt.Errorf("parse bulletin page template: %w", err)
	}

	data := htmlPageData{
		Title:     b.Title,
		Slot:      b.Slot,
		Date:      b.PublishedAt.Format("Monday, 2 January 2006"),
		Generated: b.PublishedAt.Format(time.RFC1123),
		Content:   b.Content,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute bulletin page template: %w", err)
	}
	return buf.Bytes(), nil
}

// writeHTMLFile renders a bulletin and writes it to path.
func writeHTMLFile(path string, b Row) error {
	page, err := renderHTMLPage(b)
	if err != nil {
		return err
	}
	if err := filesystem.EnsureDirectoryExists(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, page, 0o600); err != nil {
		return fmt.Errorf("write bulletin page %s: %w", path, err)
	}
	return nil
}

// writeDatedHTML writes a bulletin's dated archive page (written once, when the
// bulletin is first created).
func writeDatedHTML(htmlDir string, b Row) error {
	name := fmt.Sprintf("bulletin-%s-%s.html", b.PublishedAt.Format("2006-01-02"), strings.ToLower(b.Slot))
	path := filepath.Join(htmlDir, name)
	if err := writeHTMLFile(path, b); err != nil {
		return err
	}
	slog.Info("bulletin: wrote HTML archive", "path", path)
	return nil
}

// writeLatestHTML (re-)writes the stable bulletin-latest.html from the most
// recent bulletin.
func writeLatestHTML(htmlDir string, b Row) error {
	path := filepath.Join(htmlDir, LatestPageName)
	if err := writeHTMLFile(path, b); err != nil {
		return err
	}
	slog.Info("bulletin: wrote HTML latest", "path", path)
	return nil
}

// SummarizeDryRun clusters and summarises the current unpublished items and
// returns the digest without writing any feed or touching the database. Backs
// the bulletin-summarize debug command for prompt/model iteration.
func SummarizeDryRun(cfg Config, dbPath, apiKey string) (string, error) {
	cfg = cfg.withDefaults()

	ctx := context.Background()

	store, err := NewStore(dbPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = store.Close() }()

	items, err := store.UnpublishedItems(ctx)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no unpublished items to summarise")
	}

	clusters := clusterItems(items, cfg.SimhashThreshold)
	slog.Info("bulletin: dry-run clustered", "items", len(items), "clusters", len(clusters))

	summarizer, err := NewSummarizer(cfg, apiKey)
	if err != nil {
		return "", err
	}
	return summarizer.Summarize(ctx, clusters)
}
