package urlutils

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{name: "valid http URL", url: "http://example.com", expected: true},
		{name: "valid https URL", url: "https://example.com", expected: true},
		{name: "valid ftp URL", url: "ftp://files.example.com/file.txt", expected: true},
		{name: "empty string", url: "", expected: false},
		{name: "domain without scheme", url: "example.com", expected: false},
		{name: "scheme without host", url: "https://", expected: false},
		{name: "invalid scheme still syntactically valid", url: "invalid://example.com", expected: true},
		{name: "malformed URL", url: "ht tp://example.com", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := IsValidURL(tt.url); result != tt.expected {
				t.Errorf("IsValidURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsFetchableURLSyntax(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{name: "https host", url: "https://example.com/page", expected: true},
		{name: "http host", url: "http://example.com/image.jpg", expected: true},
		{name: "host with port", url: "https://example.com:8443/page", expected: true},

		{name: "relative path", url: "/x.jpg", expected: false},
		{name: "bare domain, no scheme", url: "example.com/x.jpg", expected: false},
		{name: "file scheme", url: "file:///tmp/x.jpg", expected: false},
		{name: "ftp scheme", url: "ftp://files.example.com/x.jpg", expected: false},
		{name: "localhost", url: "http://localhost/x.jpg", expected: false},
		{name: "localhost, mixed case", url: "http://LocalHost/x.jpg", expected: false},
		{name: "loopback literal", url: "http://127.0.0.1/x.jpg", expected: false},
		{name: "private literal", url: "http://10.0.0.1/x.jpg", expected: false},
		{name: "public literal IP is still rejected", url: "http://93.184.216.34/x.jpg", expected: false},
		{name: "ipv6 literal", url: "http://[::1]/x.jpg", expected: false},
		{name: "malformed", url: "://bad.jpg", expected: false},
		{name: "empty", url: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := IsFetchableURLSyntax(tt.url); result != tt.expected {
				t.Errorf("IsFetchableURLSyntax(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}

	t.Run("says nothing about resolution", func(t *testing.T) {
		// A hostname that resolves to a blocked address passes the syntax check
		// and is rejected only by IsFetchableURLWithContext.
		const internal = "http://internal.example/page"
		if !IsFetchableURLSyntax(internal) {
			t.Fatalf("IsFetchableURLSyntax(%q) = false, want true", internal)
		}
		privateResolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("10.1.2.3")}}, nil
		}}
		if IsFetchableURLWithContext(context.Background(), privateResolver, internal) {
			t.Errorf("IsFetchableURLWithContext(%q) = true, want false", internal)
		}
	})
}

func TestIsFetchableURLWithContext(t *testing.T) {
	publicResolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}}
	privateResolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.5")}}, nil
	}}
	errorResolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("dns failed")
	}}

	tests := []struct {
		name     string
		url      string
		resolver LookupIPAddrsResolver
		expected bool
	}{
		{name: "valid public hostname", url: "http://example.com", resolver: publicResolver, expected: true},
		{name: "valid https hostname", url: "https://example.com/path?q=1", resolver: publicResolver, expected: true},
		{name: "rejects ftp", url: "ftp://files.example.com/file.txt", resolver: publicResolver, expected: false},
		{name: "rejects custom scheme", url: "gopher://example.com", resolver: publicResolver, expected: false},
		{name: "rejects malformed", url: "://bad", resolver: publicResolver, expected: false},
		{name: "rejects localhost", url: "http://localhost:8080", resolver: publicResolver, expected: false},
		{name: "rejects loopback IPv4", url: "http://127.0.0.1:8080", resolver: publicResolver, expected: false},
		{name: "rejects private IPv4", url: "http://192.168.1.10", resolver: publicResolver, expected: false},
		{name: "rejects link local IPv4", url: "http://169.254.169.254/latest/meta-data", resolver: publicResolver, expected: false},
		{name: "rejects ipv6 loopback", url: "http://[::1]/", resolver: publicResolver, expected: false},
		{name: "rejects public numeric ipv4 too", url: "http://8.8.8.8", resolver: publicResolver, expected: false},
		{name: "rejects public numeric ipv6 too", url: "https://[2606:4700:4700::1111]/", resolver: publicResolver, expected: false},
		{name: "rejects hostname resolving private ip", url: "https://private.example", resolver: privateResolver, expected: false},
		{name: "rejects hostname on dns failure", url: "https://missing.example", resolver: errorResolver, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := IsFetchableURLWithContext(context.Background(), tt.resolver, tt.url); result != tt.expected {
				t.Errorf("IsFetchableURLWithContext(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}
