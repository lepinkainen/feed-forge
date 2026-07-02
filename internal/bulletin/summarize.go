package bulletin

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/lepinkainen/feed-forge/pkg/llm"
)

// excerptLimit caps how much article text per cluster is sent to the model,
// keeping token usage bounded on a busy news day.
const excerptLimit = 1500

// systemPrompt sets the editorial voice. The "Economist news editor" framing
// yields concise, uniform summaries (which also aids downstream dedup).
const systemPrompt = `You are a news editor at The Economist compiling a reader's daily bulletin. ` +
	`Write concise, neutral summaries with a dry, intelligent tone. No emoji, no hype, no filler.`

// promptTemplate is the instruction wrapped around the clustered stories. It
// asks for a self-contained HTML fragment grouped by topic. Rendered with
// text/template; {{.Count}} is the cluster count and {{.Stories}} the rendered
// clusters. A custom PromptFile override may use the same fields (a missing
// field renders empty rather than corrupting the prompt).
const promptTemplate = `Below are {{.Count}} story clusters gathered from multiple news feeds. Each cluster is one story, possibly covered by several sources.

Produce a single HTML fragment (no <html>/<body> wrapper) that:
- Groups stories under <h2> topic headings you choose (e.g. Technology, Business, World, Science).
- Renders each story as a <p>: one or two sentences summarising it, followed by its sources. Render each source using its given name and icon as a bracketed link: [<a href="URL"><img src="ICON" alt="" width="16" height="16" style="vertical-align:middle;margin-right:3px">NAME</a>]. Use only the sources listed for that story; omit the <img> when no icon is given.
- Orders topics by importance, most significant first.
- Omits nothing material but stays terse.

Stories:
{{.Stories}}`

// promptData is the substitution model for the prompt template.
type promptData struct {
	Count   int
	Stories string
}

// Summarizer turns story clusters into a digest via the Anthropic API.
type Summarizer struct {
	client    anthropic.Client
	model     string
	maxTokens int64
	system    string
	template  *template.Template
}

// NewSummarizer constructs a Summarizer from config and the resolved Anthropic
// API key (see llm.Config.ResolveAPIKey), with an optional prompt override from
// PromptFile.
func NewSummarizer(cfg Config, apiKey string) (*Summarizer, error) {
	cfg = cfg.withDefaults()
	if apiKey == "" {
		return nil, fmt.Errorf("no Anthropic API key: set anthropic.api-key in config or the %s env var", llm.APIKeyEnv)
	}

	tmplText := promptTemplate
	if cfg.PromptFile != "" {
		data, err := os.ReadFile(cfg.PromptFile)
		if err != nil {
			return nil, fmt.Errorf("read prompt file: %w", err)
		}
		tmplText = string(data)
	}
	tmpl, err := template.New("prompt").Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}

	return &Summarizer{
		client:    anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     cfg.Model,
		maxTokens: int64(cfg.MaxTokens),
		system:    systemPrompt,
		template:  tmpl,
	}, nil
}

// Summarize sends the clusters to the model in a single call and returns the
// digest HTML fragment.
func (s *Summarizer) Summarize(ctx context.Context, clusters []Cluster) (string, error) {
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters to summarise")
	}

	var promptBuf bytes.Buffer
	if err := s.template.Execute(&promptBuf, promptData{
		Count:   len(clusters),
		Stories: renderClusters(clusters),
	}); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}

	msg, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(s.model),
		MaxTokens: s.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: s.system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(promptBuf.String())),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic messages: %w", err)
	}
	// A truncated or refused response would otherwise be stored as a permanent,
	// malformed digest, so fail loudly instead of persisting it.
	if msg.StopReason == anthropic.StopReasonMaxTokens {
		return "", fmt.Errorf("summary hit max_tokens=%d (raise bulletin max-tokens)", s.maxTokens)
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("model refused to produce a summary")
	}

	var out strings.Builder
	for _, block := range msg.Content {
		out.WriteString(block.Text)
	}
	result := strings.TrimSpace(out.String())
	if result == "" {
		return "", fmt.Errorf("empty summary from model")
	}
	return result, nil
}

// renderClusters formats clusters into the numbered, source-tagged text block
// fed to the model.
func renderClusters(clusters []Cluster) string {
	var b strings.Builder
	for i, c := range clusters {
		rep := c.Rep()
		fmt.Fprintf(&b, "\n[Story %d] %s\n", i+1, rep.Title)
		b.WriteString("Sources:\n")
		for _, it := range c.Items {
			fmt.Fprintf(&b, "  - name: %s | icon: %s | url: %s\n", sourceName(it), faviconURL(it.URL), it.URL)
		}
		fmt.Fprintf(&b, "Excerpt: %s\n", truncate(rep.RawText, excerptLimit))
	}
	return b.String()
}

// sourceName returns the configured publisher name for an item, falling back to
// the article host when the source was configured without a name.
func sourceName(it Item) string {
	if it.FeedName != "" {
		return it.FeedName
	}
	return hostOf(it.URL)
}

// faviconURL builds a DuckDuckGo icon-service URL for the article's host. The
// digest hotlinks this directly, so readers fetch icons from DuckDuckGo rather
// than from each news site.
func faviconURL(articleURL string) string {
	host := hostOf(articleURL)
	if host == "" {
		return ""
	}
	return "https://icons.duckduckgo.com/ip3/" + host + ".ico"
}

// hostOf returns the hostname of a URL, or "" if it can't be parsed.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
