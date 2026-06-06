package opengraph

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/urlutils"
)

// isProxiableRedditURL checks if a URL is from a reddit domain worth proxying for OG data.
// Only reddit.com pages (galleries, posts) have useful metadata.
// Media hosts (i.redd.it, v.redd.it) just return generic "Reddit" titles.
func isProxiableRedditURL(targetURL string) bool {
	host, err := hostnameFromURL(targetURL)
	if err != nil {
		return false
	}

	return host == "reddit.com" || strings.HasSuffix(host, ".reddit.com")
}

func (f *Fetcher) isBlockedURL(targetURL string) bool {
	host, err := hostnameFromURL(targetURL)
	if err != nil {
		return false
	}

	blockedDomains := []string{
		"x.com",
		"twitter.com",
		"facebook.com",
		"instagram.com",
		"linkedin.com",
		"i.redd.it",
		"v.redd.it",
	}

	for _, domain := range blockedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}

	// Reddit page URLs are blocked unless we have a proxy configured.
	if isProxiableRedditURL(targetURL) && f.proxy == nil {
		return true
	}

	return false
}

func hostnameFromURL(targetURL string) (string, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	return strings.ToLower(parsedURL.Hostname()), nil
}

func allowedDialHosts(proxy *ProxyConfig) map[string]struct{} {
	if proxy == nil || proxy.URL == "" {
		return nil
	}

	host, err := hostnameFromURL(proxy.URL)
	if err != nil || host == "" {
		return nil
	}

	return map[string]struct{}{host: {}}
}

func newSafeFetchTransport(resolver urlutils.LookupIPAddrsResolver, allowedHosts map[string]struct{}, baseDialer *net.Dialer) *http.Transport {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if baseDialer == nil {
		baseDialer = &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = safeDialContext(resolver, allowedHosts, baseDialer)
	return transport
}

func safeDialContext(resolver urlutils.LookupIPAddrsResolver, allowedHosts map[string]struct{}, baseDialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if baseDialer == nil {
		baseDialer = &net.Dialer{}
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return safeDial(ctx, resolver, allowedHosts, baseDialer, network, addr)
	}
}

func safeDial(ctx context.Context, resolver urlutils.LookupIPAddrsResolver, allowedHosts map[string]struct{}, baseDialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = strings.ToLower(host)
	if _, ok := allowedHosts[host]; ok {
		return baseDialer.DialContext(ctx, network, addr)
	}
	if _, parseErr := netip.ParseAddr(host); parseErr == nil {
		return nil, fmt.Errorf("refusing direct IP fetch for host %q", host)
	}

	ipAddrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil || len(ipAddrs) == 0 {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}

	return dialFirstAllowedIP(ctx, baseDialer, network, host, port, ipAddrs)
}

func dialFirstAllowedIP(ctx context.Context, baseDialer *net.Dialer, network, host, port string, ipAddrs []net.IPAddr) (net.Conn, error) {
	var lastErr error
	for _, ipAddr := range ipAddrs {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if !ok {
			lastErr = fmt.Errorf("invalid resolved IP for host %q", host)
			continue
		}
		addr = addr.Unmap()
		if urlutils.IsBlockedFetchAddr(addr) {
			lastErr = fmt.Errorf("resolved disallowed IP %s for host %q", addr, host)
			continue
		}
		conn, dialErr := baseDialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no allowed IP addresses for host %q", host)
	}
	return nil, lastErr
}
