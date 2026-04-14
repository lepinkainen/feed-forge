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
