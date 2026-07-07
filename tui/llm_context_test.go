package tui

import (
	"strings"
	"testing"
)

func TestLookupKnownHint_LongestPrefix(t *testing.T) {
	hints := []KnownHint{
		{Path: "/Users/alice/Library/Caches", Name: "App Caches"},
		{Path: "/Users/alice/.npm", Name: "npm cache", CleanNote: "npm cache clean --force"},
	}

	if h := LookupKnownHint("/Users/alice/.npm/_cacache", hints); h == nil || h.Name != "npm cache" {
		t.Fatalf("expected npm cache hint, got %#v", h)
	}
	if h := LookupKnownHint("/Users/alice/Other", hints); h != nil {
		t.Fatalf("expected no hint, got %#v", h)
	}
}

func TestAppendKnownHint(t *testing.T) {
	var sb strings.Builder
	appendKnownHint(&sb, "/Users/alice/.npm", []KnownHint{
		{Path: "/Users/alice/.npm", Name: "npm cache", CleanNote: "npm cache clean --force", Cleanable: true},
	})
	out := sb.String()
	if !strings.Contains(out, "npm cache") || !strings.Contains(out, "npm cache clean") {
		t.Fatalf("unexpected hint block: %q", out)
	}
}
