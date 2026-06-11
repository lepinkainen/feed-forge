package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ProviderResult holds per-provider outcome for a notification.
type ProviderResult struct {
	Name     string
	FeedName string
	Status   string // "generated" | "skipped" | "failed"
	Err      error
	Duration time.Duration
}

// RunResult holds the outcome of a full generate run.
type RunResult struct {
	Providers []ProviderResult
	StartTime time.Time
}

const (
	colorRed = 0xED4245
)

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Footer      *discordFooter `json:"footer,omitempty"`
}

type discordFooter struct {
	Text string `json:"text"`
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

// SendDiscordWebhook posts a failure summary embed to the given Discord webhook URL.
func SendDiscordWebhook(webhookURL string, result RunResult) error {
	var generated, skipped, failed int
	var sb strings.Builder

	for _, p := range result.Providers {
		name := p.FeedName
		if name == "" {
			name = p.Name
		}
		switch p.Status {
		case "generated":
			generated++
			fmt.Fprintf(&sb, "✅ %s · %s\n", name, p.Duration.Round(time.Millisecond))
		case "skipped":
			skipped++
			fmt.Fprintf(&sb, "⏭️ %s (skipped)\n", name)
		default:
			failed++
			errMsg := ""
			if p.Err != nil {
				errMsg = trimError(p.Err.Error())
			}
			fmt.Fprintf(&sb, "❌ **%s** · %s\n", name, errMsg)
		}
	}

	footer := fmt.Sprintf("%d ok · %d skipped · %d failed", generated, skipped, failed)
	ts := result.StartTime.Format("2006-01-02 15:04")

	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title:       fmt.Sprintf("📡 Feed-Forge — %s", ts),
				Description: strings.TrimRight(sb.String(), "\n"),
				Color:       colorRed,
				Footer:      &discordFooter{Text: footer},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body)) //nolint:noctx // no context needed for fire-and-forget webhook
	if err != nil {
		return fmt.Errorf("discord webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord webhook POST: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned %s", resp.Status)
	}

	return nil
}

// trimError returns the first line of err, capped at 200 chars.
func trimError(err string) string {
	if idx := strings.IndexByte(err, '\n'); idx >= 0 {
		err = err[:idx]
	}
	if len(err) > 200 {
		err = err[:200] + "…"
	}
	return err
}
