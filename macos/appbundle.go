package macos

import (
	"os"
	"path/filepath"
	"strings"
)

// AppBundleInfo describes an installed macOS application bundle.
type AppBundleInfo struct {
	AppPath     string
	BundleID    string
	DisplayName string
}

// AppRelatedFile is a user-data path associated with an app bundle.
type AppRelatedFile struct {
	Path string
	Kind string // short label for UI (Caches, Preferences, …)
	Size int64
}

// IsApplicationsBundle reports whether path is a .app under an Applications folder
// (/Applications, ~/Applications, including Utilities subfolders).
func IsApplicationsBundle(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".app") {
		return false
	}
	clean := filepath.Clean(path)
	for dir := filepath.Dir(clean); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "Applications" {
			return true
		}
	}
	return false
}

// DiscoverAppRelatedFiles returns existing Library (and system Library) paths for the app.
func DiscoverAppRelatedFiles(info AppBundleInfo, homes []string) []AppRelatedFile {
	if info.BundleID == "" {
		return nil
	}
	appInSystemApps := strings.HasPrefix(filepath.Clean(info.AppPath), "/Applications/") ||
		strings.HasPrefix(filepath.Clean(info.AppPath), "/System/Volumes/Data/Applications/")

	var out []AppRelatedFile
	seen := make(map[string]bool)

	add := func(path, kind string) {
		path = filepath.Clean(path)
		if path == "" || seen[path] {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		seen[path] = true
		out = append(out, AppRelatedFile{
			Path: path,
			Kind: kind,
			Size: DirSize(path),
		})
	}

	for _, home := range homes {
		if home == "" {
			continue
		}
		lib := filepath.Join(home, "Library")
		bid := info.BundleID
		name := info.DisplayName

		for _, p := range []string{
			filepath.Join(lib, "Caches", bid),
			filepath.Join(lib, "HTTPStorages", bid),
			filepath.Join(lib, "Containers", bid),
			filepath.Join(lib, "WebKit", bid),
			filepath.Join(lib, "Preferences", bid+".plist"),
			filepath.Join(lib, "Saved Application State", bid+".savedState"),
			filepath.Join(lib, "Cookies", bid+".binarycookies"),
			filepath.Join(lib, "Application Scripts", bid),
			filepath.Join(lib, "Services", bid),
		} {
			add(p, kindForPath(p))
		}

		if matches, _ := filepath.Glob(filepath.Join(lib, "Preferences", "ByHost", bid+".plist")); len(matches) > 0 {
			for _, p := range matches {
				add(p, "Preferences")
			}
		} else if matches, _ := filepath.Glob(filepath.Join(lib, "Preferences", "ByHost", bid+".????????-????-????-????-????????????.plist")); len(matches) > 0 {
			for _, p := range matches {
				add(p, "Preferences")
			}
		}

		if name != "" {
			add(filepath.Join(lib, "Application Support", name), "Application Support")
			add(filepath.Join(lib, "Logs", name), "Logs")
			add(filepath.Join(lib, "WebKit", name), "WebKit")
		}

		if len(bid) > 10 {
			if groups, _ := filepath.Glob(filepath.Join(lib, "Group Containers", "*"+bid+"*")); len(groups) > 0 && len(groups) <= 8 {
				for _, p := range groups {
					add(p, "Group Containers")
				}
			}
		}

		// Fuzzy Application Support folder (e.g. vendor folder vs display name).
		supportDir := filepath.Join(lib, "Application Support")
		if entries, err := os.ReadDir(supportDir); err == nil && name != "" {
			lowerName := strings.ToLower(name)
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				n := e.Name()
				ln := strings.ToLower(n)
				if ln == lowerName || strings.HasPrefix(lowerName, ln) || strings.HasPrefix(ln, lowerName) {
					add(filepath.Join(supportDir, n), "Application Support")
				}
			}
		}
	}

	if appInSystemApps {
		for _, p := range []string{
			filepath.Join("/Library/Caches", info.BundleID),
			filepath.Join("/Library/Preferences", info.BundleID+".plist"),
			filepath.Join("/Library/Logs", info.DisplayName),
		} {
			add(p, kindForPath(p))
		}
	}

	return out
}

func kindForPath(p string) string {
	switch filepath.Base(filepath.Dir(p)) {
	case "Caches":
		return "Caches"
	case "Containers":
		return "Containers"
	case "Preferences", "ByHost":
		return "Preferences"
	case "Application Support":
		return "Application Support"
	case "Saved Application State":
		return "Saved State"
	case "HTTPStorages":
		return "HTTP Storage"
	case "Group Containers":
		return "Group Containers"
	case "Logs":
		return "Logs"
	case "WebKit":
		return "WebKit"
	case "Cookies":
		return "Cookies"
	default:
		return "Related"
	}
}

// ExpandAppDeleteTargets adds related Library files for .app bundles under Applications.
// Returns the expanded size map and relatedFrom[path]=appPath for UI labeling.
func ExpandAppDeleteTargets(marked map[string]int64, homes []string, sizeMap map[string]int64) (map[string]int64, map[string]string) {
	out := make(map[string]int64, len(marked))
	relatedFrom := make(map[string]string)
	for p, sz := range marked {
		out[p] = sz
	}

	for appPath := range marked {
		if !IsApplicationsBundle(appPath) {
			continue
		}
		info, err := ReadAppBundle(appPath)
		if err != nil {
			continue
		}
		for _, rel := range DiscoverAppRelatedFiles(info, homes) {
			if _, exists := out[rel.Path]; exists {
				continue
			}
			sz := rel.Size
			if s, ok := lookupSize(rel.Path, sizeMap); ok {
				sz = s
			}
			out[rel.Path] = sz
			relatedFrom[rel.Path] = appPath
		}
	}
	return out, relatedFrom
}
