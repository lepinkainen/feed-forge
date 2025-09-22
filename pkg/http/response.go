package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// ReadResponseBody reads and closes HTTP response body
func ReadResponseBody(resp *http.Response) ([]byte, error) {
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()
	return io.ReadAll(resp.Body)
}

// DecodeJSONResponse decodes JSON response into a struct
func DecodeJSONResponse(resp *http.Response, target any) error {
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// CheckStatusCode validates HTTP response status code
func CheckStatusCode(resp *http.Response, expectedCodes ...int) error {
	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			return nil
		}
	}
	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// GetContentType returns the content type of the response
func GetContentType(resp *http.Response) string {
	return resp.Header.Get("Content-Type")
}

// EnsureStatusOK checks if the response status is 200 OK
func EnsureStatusOK(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}
