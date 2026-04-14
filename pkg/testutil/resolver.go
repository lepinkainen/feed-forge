package testutil

import (
	"context"
	"net"
)

// StubResolver is a DNS resolver stub for tests. Callers set Lookup to control
// the returned addresses or error.
type StubResolver struct {
	Lookup func(ctx context.Context, host string) ([]net.IPAddr, error)
}

// LookupIPAddr satisfies the resolver interface by delegating to Lookup.
func (s StubResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return s.Lookup(ctx, host)
}
