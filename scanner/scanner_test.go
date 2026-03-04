package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ─── Unit tests for Result methods ───────────────────────────────────────────

func TestTopN_ExcludesRoot(t *testing.T) {
	r := &Result{
		Root: "/root",
		Entries: []Entry{
			{Path: "/root", Size: 1000},
			{Path: "/root/a", Size: 800},
			{Path: "/root/b", Size: 600},
			{Path: "/root/c", Size: 400},
		},
	}
	top := r.TopN(2)
	if len(top) != 2 {
		t.Fatalf("TopN(2) returned %d entries, want 2", len(top))
	}
	for _, e := range top {
		if e.Path == r.Root {
			t.Error("TopN should not include root")
		}
	}
	if top[0].Path != "/root/a" || top[1].Path != "/root/b" {
		t.Errorf("TopN returned wrong order: %+v", top)
	}
}

func TestTopN_LimitExceedsEntries(t *testing.T) {
	r := &Result{
		Root:    "/root",
		Entries: []Entry{{Path: "/root/a", Size: 100}},
	}
	if top := r.TopN(10); len(top) != 1 {
		t.Errorf("TopN(10) = %d entries, want 1", len(top))
	}
}

func TestTopN_EmptyEntries(t *testing.T) {
	r := &Result{Root: "/root", Entries: nil}
	if top := r.TopN(5); len(top) != 0 {
		t.Errorf("TopN on empty result = %d, want 0", len(top))
	}
}

func TestSizeOf(t *testing.T) {
	r := &Result{
		Root: "/root",
		Entries: []Entry{
			{Path: "/root/a", Size: 500},
			{Path: "/root/b", Size: 200},
		},
	}
	if got := r.SizeOf("/root/a"); got != 500 {
		t.Errorf("SizeOf(/root/a) = %d, want 500", got)
	}
	if got := r.SizeOf("/root/b"); got != 200 {
		t.Errorf("SizeOf(/root/b) = %d, want 200", got)
	}
	if got := r.SizeOf("/missing"); got != -1 {
		t.Errorf("SizeOf(/missing) = %d, want -1", got)
	}
}

func TestSizeOf_CachesMap(t *testing.T) {
	r := &Result{
		Root:    "/root",
		Entries: []Entry{{Path: "/root/x", Size: 42}},
	}
	// Call twice to exercise the cache branch.
	r.SizeOf("/root/x")
	if got := r.SizeOf("/root/x"); got != 42 {
		t.Errorf("SizeOf (cached) = %d, want 42", got)
	}
}

// ─── Integration tests for Scan ──────────────────────────────────────────────

func TestScan_Basic(t *testing.T) {
	tmp := tempDir(t)
	writeFile(t, filepath.Join(tmp, "a.txt"), "hello")
	sub := filepath.Join(tmp, "sub")
	must(t, os.Mkdir(sub, 0755))
	writeFile(t, filepath.Join(sub, "b.txt"), "world")

	result, err := Scan(context.Background(), tmp, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if result.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", result.FileCount)
	}
	// sub directory must appear in entries
	if result.SizeOf(sub) == -1 {
		t.Errorf("expected %q in scan entries", sub)
	}
	// root entry must exist
	found := false
	for _, e := range result.Entries {
		if e.Path == tmp {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("root %q not found in entries", tmp)
	}
}

func TestScan_TopNExcludesRoot(t *testing.T) {
	tmp := tempDir(t)
	sub := filepath.Join(tmp, "sub")
	must(t, os.Mkdir(sub, 0755))
	writeFile(t, filepath.Join(sub, "f.txt"), "data")

	result, err := Scan(context.Background(), tmp, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	for _, e := range result.TopN(100) {
		if e.Path == tmp {
			t.Error("TopN should not include scan root")
		}
	}
}

func TestScan_HardLinkDedup(t *testing.T) {
	// dir1: single file → get baseline TotalSize
	tmp1 := tempDir(t)
	writeFile(t, filepath.Join(tmp1, "a.txt"), "dedup-test-content")
	r1, err := Scan(context.Background(), tmp1, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	singleSize := r1.TotalSize

	// dir2: same file + hard link to it
	tmp2 := tempDir(t)
	original := filepath.Join(tmp2, "a.txt")
	link := filepath.Join(tmp2, "link.txt")
	writeFile(t, original, "dedup-test-content")
	if err := os.Link(original, link); err != nil {
		t.Skip("hard links not supported:", err)
	}

	r2, err := Scan(context.Background(), tmp2, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// Size should not be doubled: the hard link shares the same blocks.
	if r2.TotalSize != singleSize {
		t.Errorf("TotalSize with hard link = %d, want %d (no double-counting)", r2.TotalSize, singleSize)
	}
	// FileCount counts both entries (link is still a "file")
	if r2.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2 (both link and original counted)", r2.FileCount)
	}
}

func TestScan_Exclude(t *testing.T) {
	tmp := tempDir(t)
	included := filepath.Join(tmp, "included")
	excluded := filepath.Join(tmp, "excluded")
	must(t, os.Mkdir(included, 0755))
	must(t, os.Mkdir(excluded, 0755))
	writeFile(t, filepath.Join(included, "a.txt"), "aaa")
	writeFile(t, filepath.Join(excluded, "b.txt"), "bbb")

	result, err := Scan(context.Background(), tmp, Options{Exclude: []string{excluded}}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (excluded dir's file skipped)", result.FileCount)
	}
	for _, e := range result.Entries {
		if e.Path == excluded {
			t.Errorf("excluded path %q should not appear in entries", excluded)
		}
	}
}

func TestScan_ContextCancel(t *testing.T) {
	tmp := tempDir(t)
	writeFile(t, filepath.Join(tmp, "f.txt"), "x")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before scan starts

	result, err := Scan(ctx, tmp, Options{}, nil)
	// Scan returns partial results, not an error, on context cancellation.
	if err != nil {
		t.Fatalf("Scan returned error on cancelled context: %v", err)
	}
	if result == nil {
		t.Fatal("Scan returned nil result")
	}
}

func TestScan_EmptyDir(t *testing.T) {
	tmp := tempDir(t)
	result, err := Scan(context.Background(), tmp, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if result.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0 for empty dir", result.FileCount)
	}
	if result.TotalSize != 0 {
		t.Errorf("TotalSize = %d, want 0 for empty dir", result.TotalSize)
	}
}

func TestScan_SkipsSymlinks(t *testing.T) {
	tmp := tempDir(t)
	target := filepath.Join(tmp, "real.txt")
	link := filepath.Join(tmp, "link.txt")
	writeFile(t, target, "real content")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	result, err := Scan(context.Background(), tmp, Options{}, nil)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// Symlinks are skipped; only the real file is counted.
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (symlink skipped)", result.FileCount)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// tempDir wraps t.TempDir() and resolves symlinks (macOS: /var → /private/var).
// The scanner internally calls filepath.EvalSymlinks(root), so all paths must
// match the resolved form.
func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
