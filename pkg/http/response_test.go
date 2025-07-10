package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{
			name:        "application/json",
			contentType: "application/json",
			expected:    "application/json",
		},
		{
			name:        "text/html with charset",
			contentType: "text/html; charset=utf-8",
			expected:    "text/html; charset=utf-8",
		},
		{
			name:        "text/plain",
			contentType: "text/plain",
			expected:    "text/plain",
		},
		{
			name:        "empty content type",
			contentType: "",
			expected:    "",
		},
		{
			name:        "application/xml",
			contentType: "application/xml",
			expected:    "application/xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			resp.Header.Set("Content-Type", tt.contentType)

			result := GetContentType(resp)
			if result != tt.expected {
				t.Errorf("GetContentType() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestEnsureStatusOK(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		status      string
		expectError bool
	}{
		{
			name:        "200 OK",
			statusCode:  http.StatusOK,
			status:      "200 OK",
			expectError: false,
		},
		{
			name:        "201 Created",
			statusCode:  http.StatusCreated,
			status:      "201 Created",
			expectError: true,
		},
		{
			name:        "400 Bad Request",
			statusCode:  http.StatusBadRequest,
			status:      "400 Bad Request",
			expectError: true,
		},
		{
			name:        "404 Not Found",
			statusCode:  http.StatusNotFound,
			status:      "404 Not Found",
			expectError: true,
		},
		{
			name:        "500 Internal Server Error",
			statusCode:  http.StatusInternalServerError,
			status:      "500 Internal Server Error",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     tt.status,
			}

			err := EnsureStatusOK(resp)
			if (err != nil) != tt.expectError {
				t.Errorf("EnsureStatusOK() error = %v, expectError = %v", err, tt.expectError)
			}

			if err != nil && !strings.Contains(err.Error(), "unexpected status code") {
				t.Errorf("EnsureStatusOK() error should contain 'unexpected status code', got: %v", err)
			}
		})
	}
}

func TestCheckStatusCode(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedCodes []int
		expectError   bool
	}{
		{
			name:          "single expected code - match",
			statusCode:    http.StatusOK,
			expectedCodes: []int{http.StatusOK},
			expectError:   false,
		},
		{
			name:          "single expected code - no match",
			statusCode:    http.StatusNotFound,
			expectedCodes: []int{http.StatusOK},
			expectError:   true,
		},
		{
			name:          "multiple expected codes - first match",
			statusCode:    http.StatusOK,
			expectedCodes: []int{http.StatusOK, http.StatusCreated, http.StatusAccepted},
			expectError:   false,
		},
		{
			name:          "multiple expected codes - middle match",
			statusCode:    http.StatusCreated,
			expectedCodes: []int{http.StatusOK, http.StatusCreated, http.StatusAccepted},
			expectError:   false,
		},
		{
			name:          "multiple expected codes - last match",
			statusCode:    http.StatusAccepted,
			expectedCodes: []int{http.StatusOK, http.StatusCreated, http.StatusAccepted},
			expectError:   false,
		},
		{
			name:          "multiple expected codes - no match",
			statusCode:    http.StatusNotFound,
			expectedCodes: []int{http.StatusOK, http.StatusCreated, http.StatusAccepted},
			expectError:   true,
		},
		{
			name:          "empty expected codes",
			statusCode:    http.StatusOK,
			expectedCodes: []int{},
			expectError:   true,
		},
		{
			name:          "nil expected codes",
			statusCode:    http.StatusOK,
			expectedCodes: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
			}

			err := CheckStatusCode(resp, tt.expectedCodes...)
			if (err != nil) != tt.expectError {
				t.Errorf("CheckStatusCode() error = %v, expectError = %v", err, tt.expectError)
			}

			if err != nil && !strings.Contains(err.Error(), "unexpected status code") {
				t.Errorf("CheckStatusCode() error should contain 'unexpected status code', got: %v", err)
			}
		})
	}
}

func TestReadResponseBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "simple text",
			body:     "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "empty body",
			body:     "",
			expected: "",
		},
		{
			name:     "JSON content",
			body:     `{"message": "hello", "status": "ok"}`,
			expected: `{"message": "hello", "status": "ok"}`,
		},
		{
			name:     "multiline content",
			body:     "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "unicode content",
			body:     "Hello 世界",
			expected: "Hello 世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(strings.NewReader(tt.body)),
			}

			result, err := ReadResponseBody(resp)
			if err != nil {
				t.Errorf("ReadResponseBody() error = %v", err)
				return
			}

			if string(result) != tt.expected {
				t.Errorf("ReadResponseBody() = %q, expected %q", string(result), tt.expected)
			}
		})
	}
}

func TestDecodeJSONResponse(t *testing.T) {
	type TestStruct struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Count   int    `json:"count"`
	}

	tests := []struct {
		name        string
		statusCode  int
		body        string
		expectError bool
		expected    TestStruct
	}{
		{
			name:        "valid JSON with 200 OK",
			statusCode:  http.StatusOK,
			body:        `{"message": "hello", "status": "ok", "count": 42}`,
			expectError: false,
			expected: TestStruct{
				Message: "hello",
				Status:  "ok",
				Count:   42,
			},
		},
		{
			name:        "non-200 status code",
			statusCode:  http.StatusBadRequest,
			body:        `{"error": "bad request"}`,
			expectError: true,
			expected:    TestStruct{},
		},
		{
			name:        "invalid JSON with 200 OK",
			statusCode:  http.StatusOK,
			body:        `{"message": "hello", "status": }`, // Invalid JSON
			expectError: true,
			expected:    TestStruct{},
		},
		{
			name:        "empty JSON object",
			statusCode:  http.StatusOK,
			body:        `{}`,
			expectError: false,
			expected: TestStruct{
				Message: "",
				Status:  "",
				Count:   0,
			},
		},
		{
			name:        "non-JSON content",
			statusCode:  http.StatusOK,
			body:        "This is not JSON",
			expectError: true,
			expected:    TestStruct{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			var result TestStruct
			err := DecodeJSONResponse(resp, &result)

			if (err != nil) != tt.expectError {
				t.Errorf("DecodeJSONResponse() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if !tt.expectError && result != tt.expected {
				t.Errorf("DecodeJSONResponse() result = %+v, expected %+v", result, tt.expected)
			}
		})
	}
}

// trackingReadCloser is a custom ReadCloser to track if Close() was called
type trackingReadCloser struct {
	*bytes.Reader
	closed bool
}

func (trc *trackingReadCloser) Close() error {
	trc.closed = true
	return nil
}

// Test that response body is properly closed
func TestResponseBodyClosure(t *testing.T) {

	body := "test content"
	tracker := &trackingReadCloser{
		Reader: bytes.NewReader([]byte(body)),
		closed: false,
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       tracker,
	}

	// Test ReadResponseBody
	_, err := ReadResponseBody(resp)
	if err != nil {
		t.Errorf("ReadResponseBody() error = %v", err)
	}

	if !tracker.closed {
		t.Error("ReadResponseBody() should close the response body")
	}

	// Reset for next test
	tracker.closed = false
	tracker.Reader = bytes.NewReader([]byte(`{"test": "data"}`))
	resp.Body = tracker

	// Test DecodeJSONResponse
	var target map[string]interface{}
	err = DecodeJSONResponse(resp, &target)
	if err != nil {
		t.Errorf("DecodeJSONResponse() error = %v", err)
	}

	if !tracker.closed {
		t.Error("DecodeJSONResponse() should close the response body")
	}
}
