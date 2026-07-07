package macos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsApplicationsBundle(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/Applications/Safari.app", true},
		{"/Users/me/Applications/Foo.app", true},
		{"/Applications/Utilities/Terminal.app", true},
		{"/tmp/Foo.app", false},
		{"/Applications/readme.txt", false},
	}
	for _, tc := range tests {
		if got := IsApplicationsBundle(tc.path); got != tc.want {
			t.Errorf("IsApplicationsBundle(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestDiscoverAppRelatedFiles(t *testing.T) {
	home := t.TempDir()
	lib := filepath.Join(home, "Library")
	bundleID := "com.example.testapp"
	displayName := "TestApp"

	mk := func(rel string) string {
		p := filepath.Join(lib, rel)
		if strings.HasSuffix(p, ".plist") {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("<?xml version=\"1.0\"?><plist/>"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(p, "data.bin"), []byte("hello"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}

	cachePath := mk(filepath.Join("Caches", bundleID))
	prefsPath := mk(filepath.Join("Preferences", bundleID+".plist"))
	_ = mk(filepath.Join("Application Support", displayName))

	info := AppBundleInfo{
		AppPath:     "/Applications/TestApp.app",
		BundleID:    bundleID,
		DisplayName: displayName,
	}
	related := DiscoverAppRelatedFiles(info, []string{home})

	found := map[string]bool{}
	for _, r := range related {
		found[r.Path] = true
	}
	for _, want := range []string{cachePath, prefsPath} {
		if !found[want] {
			t.Errorf("missing related path %s; got %v", want, related)
		}
	}
}

func TestExpandAppDeleteTargets(t *testing.T) {
	home := t.TempDir()
	lib := filepath.Join(home, "Library", "Caches", "com.expand.app")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "x"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := filepath.Join(home, "Applications", "Expand.app")
	plist := filepath.Join(app, "Contents", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		t.Fatal(err)
	}
	plistXML := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.expand.app</string>
<key>CFBundleName</key><string>Expand</string>
</dict></plist>`
	if err := os.WriteFile(plist, []byte(plistXML), 0o644); err != nil {
		t.Fatal(err)
	}

	marked := map[string]int64{app: 1000}
	expanded, relatedFrom := ExpandAppDeleteTargets(marked, []string{home}, nil)

	if len(expanded) < 2 {
		t.Fatalf("expected app + related paths, got %d", len(expanded))
	}
	if relatedFrom[lib] != app {
		t.Fatalf("expected lib related to app, got %v", relatedFrom)
	}
}
