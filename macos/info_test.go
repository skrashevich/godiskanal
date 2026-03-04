package macos

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── PopulateSizes ────────────────────────────────────────────────────────────

func TestPopulateSizes_FromSizeMap(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	must(t, os.Mkdir(sub, 0755))

	locs := []KnownLocation{{Path: sub}}
	PopulateSizes(locs, map[string]int64{sub: 1234}, tmp)

	if locs[0].Size != 1234 {
		t.Errorf("Size = %d, want 1234", locs[0].Size)
	}
	if !locs[0].Exists {
		t.Error("Exists should be true when path is in sizeMap")
	}
}

func TestPopulateSizes_EmptyDirInsideRoot(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "empty")
	must(t, os.Mkdir(sub, 0755))

	locs := []KnownLocation{{Path: sub}}
	PopulateSizes(locs, map[string]int64{}, tmp)

	if !locs[0].Exists {
		t.Error("Exists should be true for existing dir inside scanRoot")
	}
	if locs[0].Size != 0 {
		t.Errorf("Size = %d, want 0 for empty dir not in sizeMap", locs[0].Size)
	}
}

func TestPopulateSizes_NonExistentInsideRoot(t *testing.T) {
	tmp := t.TempDir()
	ghost := filepath.Join(tmp, "does-not-exist")

	locs := []KnownLocation{{Path: ghost}}
	PopulateSizes(locs, map[string]int64{}, tmp)

	if locs[0].Exists {
		t.Error("Exists should be false for non-existent path")
	}
}

func TestPopulateSizes_OutsideScanRoot(t *testing.T) {
	scanRoot := t.TempDir()
	outside := t.TempDir() // separate temp = outside scanRoot

	locs := []KnownLocation{{Path: outside}}
	PopulateSizes(locs, map[string]int64{}, scanRoot)

	if !locs[0].Exists {
		t.Error("Exists should be true for existing path outside scanRoot")
	}
	if locs[0].Size != -1 {
		t.Errorf("Size = %d, want -1 for path outside scanRoot (size unknown)", locs[0].Size)
	}
}

func TestPopulateSizes_NonExistentOutsideRoot(t *testing.T) {
	scanRoot := t.TempDir()
	ghost := "/tmp/godiskanal-test-ghost-does-not-exist-12345"

	locs := []KnownLocation{{Path: ghost}}
	PopulateSizes(locs, map[string]int64{}, scanRoot)

	if locs[0].Exists {
		t.Error("Exists should be false for non-existent path outside scanRoot")
	}
}

// ─── LargeNodeModules ────────────────────────────────────────────────────────

func TestLargeNodeModules_FiltersByMinSize(t *testing.T) {
	const mb = 1024 * 1024
	sizeMap := map[string]int64{
		"/a/node_modules":     500 * mb, // included
		"/b/node_modules":     300 * mb, // included
		"/c/node_modules":      50 * mb, // below minSize, excluded
		"/d/vendor":           400 * mb, // wrong name, excluded
	}
	results := LargeNodeModules("/", 200*mb, sizeMap)
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if filepath.Base(r.Path) != "node_modules" {
			t.Errorf("path %q is not node_modules", r.Path)
		}
		if r.Size < 200*mb {
			t.Errorf("size %d is below minSize", r.Size)
		}
	}
}

func TestLargeNodeModules_Empty(t *testing.T) {
	results := LargeNodeModules("/", 1, map[string]int64{})
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 for empty sizeMap", len(results))
	}
}

// ─── DefaultLocations ─────────────────────────────────────────────────────────

func TestDefaultLocations_NotEmpty(t *testing.T) {
	locs := DefaultLocations("/Users/testuser")
	if len(locs) == 0 {
		t.Error("DefaultLocations returned empty list")
	}
}

func TestDefaultLocations_AllHaveNames(t *testing.T) {
	locs := DefaultLocations("/Users/testuser")
	for i, loc := range locs {
		if loc.Name == "" {
			t.Errorf("locs[%d] (Path=%q) has empty Name", i, loc.Path)
		}
	}
}

func TestDefaultLocations_AllHavePaths(t *testing.T) {
	locs := DefaultLocations("/Users/testuser")
	for i, loc := range locs {
		if loc.Path == "" {
			t.Errorf("locs[%d] (Name=%q) has empty Path", i, loc.Name)
		}
	}
}

func TestDefaultLocations_CommandOnlyHasCleanFn(t *testing.T) {
	locs := DefaultLocations("/Users/testuser")
	for _, loc := range locs {
		if loc.CommandOnly && loc.CleanFn == nil {
			t.Errorf("loc %q is CommandOnly but has no CleanFn", loc.Name)
		}
	}
}

func TestDefaultLocations_ContainsExpectedPaths(t *testing.T) {
	home := "/Users/testuser"
	locs := DefaultLocations(home)

	wantPaths := []string{
		filepath.Join(home, "Library/Caches"),
		filepath.Join(home, "Library/Developer/Xcode/DerivedData"),
		filepath.Join(home, ".npm"),
		filepath.Join(home, "go/pkg/mod"),
		filepath.Join(home, ".Trash"),
		filepath.Join(home, "Downloads"),
		filepath.Join(home, "Library/Caches/com.apple.Safari"),
		filepath.Join(home, "Library/Caches/com.google.Chrome"),
		filepath.Join(home, "Library/Application Support/Telegram Desktop"),
	}

	pathSet := make(map[string]bool, len(locs))
	for _, loc := range locs {
		pathSet[loc.Path] = true
	}
	for _, want := range wantPaths {
		if !pathSet[want] {
			t.Errorf("DefaultLocations missing expected path %q", want)
		}
	}
}

func TestDefaultLocations_BrowserCachesCleanable(t *testing.T) {
	home := "/Users/testuser"
	locs := DefaultLocations(home)

	cleanable := map[string]bool{}
	for _, loc := range locs {
		cleanable[loc.Path] = loc.Cleanable
	}

	safariCache := filepath.Join(home, "Library/Caches/com.apple.Safari")
	if !cleanable[safariCache] {
		t.Errorf("Safari cache (%q) should be Cleanable=true", safariCache)
	}

	chromeCache := filepath.Join(home, "Library/Caches/com.google.Chrome")
	if !cleanable[chromeCache] {
		t.Errorf("Chrome cache (%q) should be Cleanable=true", chromeCache)
	}
}

func TestDefaultLocations_TelegramNotCleanable(t *testing.T) {
	home := "/Users/testuser"
	locs := DefaultLocations(home)

	telegramPaths := []string{
		filepath.Join(home, "Library/Application Support/Telegram Desktop"),
		filepath.Join(home, "Library/Group Containers/6N38VWS5BX.ru.keepcoder.Telegram"),
	}
	cleanable := map[string]bool{}
	for _, loc := range locs {
		cleanable[loc.Path] = loc.Cleanable
	}
	for _, p := range telegramPaths {
		if cleanable[p] {
			t.Errorf("Telegram path %q should NOT be Cleanable (contains user data)", p)
		}
	}
}

// ─── DirSize ──────────────────────────────────────────────────────────────────

func TestDirSize_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	if got := DirSize(tmp); got != 0 {
		t.Errorf("DirSize(empty) = %d, want 0", got)
	}
}

func TestDirSize_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	must(t, os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello"), 0644))
	must(t, os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("world!"), 0644))

	got := DirSize(tmp)
	if got <= 0 {
		t.Errorf("DirSize with files = %d, want > 0", got)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
