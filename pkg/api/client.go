package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	httputil "github.com/lepinkainen/feed-forge/pkg/http"
)

// GetAndDecode performs an HTTP GET request and decodes the JSON response.
func GetAndDecode(client *http.Client, url string, target any, headers map[string]string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform GET request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if err := httputil.EnsureStatusOK(res); err != nil {
		return fmt.Errorf("http status error: %w", err)
	}

	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode json response: %w", err)
	}

	slog.Debug("Successfully fetched and decoded", "url", url)
	return nil
}
