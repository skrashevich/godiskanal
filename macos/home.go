package macos

import (
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
)

// RunningAsRoot reports whether the effective UID is 0.
func RunningAsRoot() bool {
	return os.Getuid() == 0
}

// ResolveTargetHome returns the home directory used for known locations and ~ expansion.
// When running as root, prefers SUDO_USER / LOGNAME and falls back away from /var/root.
func ResolveTargetHome(fallback string) string {
	if !RunningAsRoot() {
		if fallback != "" {
			return fallback
		}
		h, _ := os.UserHomeDir()
		return h
	}

	// sudo often leaves HOME=/var/root; SUDO_USER is the real account.
	for _, name := range []string{
		os.Getenv("SUDO_USER"),
		os.Getenv("USER"),
		os.Getenv("LOGNAME"),
	} {
		if name == "" || name == "root" {
			continue
		}
		if home := homeForUsername(name); home != "" {
			return home
		}
	}

	if fallback != "" && fallback != "/var/root" {
		return fallback
	}

	homes := DiscoverUserHomes()
	if len(homes) == 1 {
		return homes[0]
	}
	if len(homes) > 1 {
		// Prefer the first non-guest account (sorted); caller may use TargetHomes for all.
		return homes[0]
	}

	if fallback != "" {
		return fallback
	}
	h, _ := os.UserHomeDir()
	return h
}

// TargetHomes returns home directories whose known locations should be checked.
// As a normal user this is a single home; as root it includes every /Users account.
func TargetHomes(primary string) []string {
	if !RunningAsRoot() {
		if primary == "" {
			primary, _ = os.UserHomeDir()
		}
		if primary == "" {
			return nil
		}
		return []string{primary}
	}

	seen := make(map[string]bool)
	var homes []string
	add := func(h string) {
		h = filepath.Clean(h)
		if h == "" || h == "/var/root" || seen[h] {
			return
		}
		if _, err := os.Stat(filepath.Join(h, "Library")); err != nil {
			return
		}
		seen[h] = true
		homes = append(homes, h)
	}

	if primary != "" && primary != "/var/root" {
		add(primary)
	}
	for _, h := range DiscoverUserHomes() {
		add(h)
	}

	sort.Strings(homes)
	return homes
}

// DiscoverUserHomes lists macOS user home directories under /Users.
func DiscoverUserHomes() []string {
	entries, err := os.ReadDir("/Users")
	if err != nil {
		return nil
	}
	var homes []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "Shared" || strings.HasPrefix(name, ".") {
			continue
		}
		home := filepath.Join("/Users", name)
		if _, err := os.Stat(filepath.Join(home, "Library")); err != nil {
			continue
		}
		homes = append(homes, home)
	}
	sort.Strings(homes)
	return homes
}

// DefaultScanPath picks a sensible scan root when --path was not set explicitly.
func DefaultScanPath(home string) string {
	if !RunningAsRoot() {
		if home != "" {
			return home
		}
		h, _ := os.UserHomeDir()
		return h
	}
	if p := strings.TrimSpace(os.Getenv("GODISKANAL_SCAN_ROOT")); p != "" {
		return p
	}
	if _, err := os.Stat("/System/Volumes/Data"); err == nil {
		return "/System/Volumes/Data"
	}
	if home != "" && home != "/var/root" {
		return home
	}
	return "/"
}

func homeForUsername(name string) string {
	if u, err := user.Lookup(name); err == nil && u.HomeDir != "" {
		if u.HomeDir != "/var/root" {
			if _, err := os.Stat(filepath.Join(u.HomeDir, "Library")); err == nil {
				return u.HomeDir
			}
		}
	}
	// macOS convention
	p := filepath.Join("/Users", name)
	if _, err := os.Stat(filepath.Join(p, "Library")); err == nil {
		return p
	}
	return ""
}
