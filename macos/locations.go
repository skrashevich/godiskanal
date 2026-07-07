package macos

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/skrashevich/godiskanal/i18n"
)

// AllKnownLocations returns known space consumers for one or more home directories.
// With multiple homes, location names are prefixed with the account name.
func AllKnownLocations(homes []string) []KnownLocation {
	if len(homes) == 0 {
		return nil
	}
	if len(homes) == 1 {
		return DefaultLocations(homes[0])
	}

	seen := make(map[string]bool)
	var out []KnownLocation
	for _, home := range homes {
		account := filepath.Base(home)
		for _, loc := range DefaultLocations(home) {
			key := loc.Path
			if seen[key] {
				continue
			}
			seen[key] = true
			loc.Name = account + ": " + loc.Name
			out = append(out, loc)
		}
	}
	return out
}

// SystemLocations returns machine-wide directories that are often safe to trim when running as root.
func SystemLocations() []KnownLocation {
	return []KnownLocation{
		{
			Name:        "System Caches",
			Path:        "/Library/Caches",
			Description: i18n.T("loc.System Caches.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.System Caches.note"),
		},
		{
			Name:        "System Logs",
			Path:        "/private/var/log",
			Description: i18n.T("loc.System Logs.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.System Logs.note"),
		},
		{
			Name:        "Temporary files",
			Path:        "/private/tmp",
			Description: i18n.T("loc.Temporary files.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Temporary files.note"),
		},
		{
			Name:        "User temp caches",
			Path:        "/private/var/folders",
			Description: i18n.T("loc.User temp caches.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.User temp caches.note"),
		},
		{
			Name:        "Installer leftovers",
			Path:        "/Library/Updates",
			Description: i18n.T("loc.Installer leftovers.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.Installer leftovers.note"),
		},
	}
}

// SortKnownBySize sorts locations by size descending (unknown sizes last).
func SortKnownBySize(locs []KnownLocation) {
	sort.Slice(locs, func(i, j int) bool {
		si, sj := locs[i].Size, locs[j].Size
		if si < 0 {
			si = 0
		}
		if sj < 0 {
			sj = 0
		}
		if si != sj {
			return si > sj
		}
		return locs[i].Name < locs[j].Name
	})
}

// FilterKnownForDisplay keeps locations worth showing in standard mode.
func FilterKnownForDisplay(locs []KnownLocation, minSize int64) []KnownLocation {
	const minShow = 10 * 1024 * 1024 // 10 MiB when size is known
	out := make([]KnownLocation, 0, len(locs))
	for _, loc := range locs {
		if !loc.Exists {
			continue
		}
		if loc.Size < 0 {
			// Exists but outside scan tree — still useful when cleanable
			if loc.Cleanable {
				out = append(out, loc)
			}
			continue
		}
		if loc.Size >= minShow || (loc.Cleanable && loc.Size > 0) {
			out = append(out, loc)
		} else if !loc.Cleanable && loc.Size >= minSize {
			out = append(out, loc)
		}
	}
	return out
}

// lookupSize finds a directory size in the scan map, including macOS /Users ↔ Data volume aliases.
func lookupSize(path string, sizeMap map[string]int64) (int64, bool) {
	if size, ok := sizeMap[path]; ok {
		return size, true
	}
	clean := filepath.Clean(path)
	if size, ok := sizeMap[clean]; ok {
		return size, true
	}

	var aliases []string
	if strings.HasPrefix(clean, "/Users/") {
		aliases = append(aliases, "/System/Volumes/Data"+clean)
	} else if strings.HasPrefix(clean, "/System/Volumes/Data/Users/") {
		aliases = append(aliases, strings.TrimPrefix(clean, "/System/Volumes/Data"))
	}

	for _, alt := range aliases {
		if size, ok := sizeMap[alt]; ok {
			return size, true
		}
		if size, ok := sizeMap[filepath.Clean(alt)]; ok {
			return size, true
		}
	}
	return 0, false
}
