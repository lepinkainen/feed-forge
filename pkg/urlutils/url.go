// Package urlutils provides URL and common helper functions.
package urlutils

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// IsValidURL checks if a URL is syntactically valid.
func IsValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// LookupIPAddrsResolver resolves hostnames to IP addresses.
type LookupIPAddrsResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// IsFetchableURL checks whether a URL is safe for outbound HTTP fetching.
func IsFetchableURL(urlStr string) bool {
	return IsFetchableURLWithContext(context.Background(), net.DefaultResolver, urlStr)
}

// IsFetchableURLWithContext checks whether a URL is safe for outbound HTTP fetching.
func IsFetchableURLWithContext(ctx context.Context, resolver LookupIPAddrsResolver, urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil || u.Host == "" {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return IsFetchableHostname(ctx, resolver, u.Hostname())
}

// IsFetchableHostname checks whether a hostname is safe for outbound HTTP fetching.
func IsFetchableHostname(ctx context.Context, resolver LookupIPAddrsResolver, hostname string) bool {
	if hostname == "" || strings.EqualFold(hostname, "localhost") {
		return false
	}

	if _, err := netip.ParseAddr(hostname); err == nil {
		return false
	}

	if resolver == nil {
		resolver = net.DefaultResolver
	}

	ips, err := resolver.LookupIPAddr(ctx, hostname)
	if err != nil || len(ips) == 0 {
		return false
	}

	for _, resolvedIP := range ips {
		addr, ok := netip.AddrFromSlice(resolvedIP.IP)
		if !ok {
			return false
		}
		if IsBlockedFetchAddr(addr.Unmap()) {
			return false
		}
	}

	return true
}

// IsBlockedFetchAddr reports whether an IP address is disallowed for outbound fetches.
func IsBlockedFetchAddr(ip netip.Addr) bool {
	ip = ip.Unmap()

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	if ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	if ip.Is4() {
		b := ip.As4()
		if b[0] == 100 && b[1]&0xc0 == 64 { // 100.64.0.0/10 shared address space
			return true
		}
		if b[0] == 169 && b[1] == 254 { // IPv4 link-local / metadata route
			return true
		}
		if b[0] == 198 && (b[1] == 18 || b[1] == 19) { // benchmark testing range
			return true
		}
		if b[0] >= 224 { // multicast + reserved
			return true
		}
	}

	return false
}

// ResolveURL resolves a relative URL against a base URL
// If the URL is already absolute, it returns it unchanged
func ResolveURL(baseURL, relativeURL string) (string, error) {
	// Parse the relative URL
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}

	// If it's already absolute, return as-is
	if rel.IsAbs() {
		return relativeURL, nil
	}

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	// Resolve the relative URL against the base
	resolved := base.ResolveReference(rel)
	return resolved.String(), nil
}
