package macos

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTargetHome_NotRoot(t *testing.T) {
	if RunningAsRoot() {
		t.Skip("requires non-root")
	}
	home, _ := os.UserHomeDir()
	got := ResolveTargetHome(home)
	if got != home {
		t.Errorf("ResolveTargetHome = %q, want %q", got, home)
	}
}

func TestDiscoverUserHomes_SkipsShared(t *testing.T) {
	if _, err := os.Stat("/Users"); err != nil {
		t.Skip("no /Users on this system")
	}
	homes := DiscoverUserHomes()
	for _, h := range homes {
		if filepath.Base(h) == "Shared" {
			t.Errorf("DiscoverUserHomes should not include Shared, got %q", h)
		}
	}
}

func TestDefaultScanPath_NonRoot(t *testing.T) {
	if RunningAsRoot() {
		t.Skip("requires non-root")
	}
	home := "/Users/example"
	if got := DefaultScanPath(home); got != home {
		t.Errorf("DefaultScanPath = %q, want %q", got, home)
	}
}

func TestLookupSize_DataVolumeAlias(t *testing.T) {
	sizeMap := map[string]int64{
		"/System/Volumes/Data/Users/alice/Library/Caches": 999,
	}
	size, ok := lookupSize("/Users/alice/Library/Caches", sizeMap)
	if !ok || size != 999 {
		t.Errorf("lookupSize = (%d, %v), want (999, true)", size, ok)
	}
}

func TestAllKnownLocations_MultiHomePrefixes(t *testing.T) {
	locs := AllKnownLocations([]string{"/Users/a", "/Users/b"})
	if len(locs) < 2 {
		t.Fatalf("expected many locations, got %d", len(locs))
	}
	var prefixed int
	for _, loc := range locs {
		if loc.Name == "a: App Caches" || loc.Name == "b: App Caches" {
			prefixed++
		}
	}
	if prefixed < 2 {
		t.Error("expected account-prefixed names for multi-home")
	}
}
