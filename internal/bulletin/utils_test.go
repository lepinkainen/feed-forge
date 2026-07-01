package bulletin

import (
	"testing"
	"time"
)

func TestSlotFor(t *testing.T) {
	tests := []struct {
		name string
		hour int
		want string
	}{
		{name: "midnight", hour: 0, want: "Morning"},
		{name: "early morning", hour: 7, want: "Morning"},
		{name: "just before noon", hour: 11, want: "Morning"},
		{name: "noon", hour: 12, want: "Afternoon"},
		{name: "late afternoon", hour: 17, want: "Afternoon"},
		{name: "evening", hour: 18, want: "Evening"},
		{name: "late evening", hour: 23, want: "Evening"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 7, 1, tt.hour, 0, 0, 0, time.UTC)
			if got := slotFor(now); got != tt.want {
				t.Errorf("slotFor(%02d:00) = %q, want %q", tt.hour, got, tt.want)
			}
		})
	}
}

func TestBulletinTitle(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	got := bulletinTitle("Morning", now)
	if got != "Morning Bulletin — Wed, 1 Jul 2026" {
		t.Errorf("bulletinTitle = %q", got)
	}
}

func TestHTTPCacheDBPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "with ext", path: "bulletin.db", want: "bulletin-httpcache.db"},
		{name: "no ext", path: "bulletin", want: "bulletin-httpcache"},
		{name: "double ext", path: "bulletin.db.bak", want: "bulletin.db-httpcache.bak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpcacheDBPath(tt.path); got != tt.want {
				t.Errorf("httpcacheDBPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
