package macos

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/skrashevich/godiskanal/i18n"
)

// DiskInfo holds filesystem statistics.
type DiskInfo struct {
	Total int64
	Used  int64
	Free  int64
}

// GetDiskInfo returns disk usage stats for the filesystem containing path.
func GetDiskInfo(path string) (*DiskInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}
	bs := int64(stat.Bsize)
	return &DiskInfo{
		Total: int64(stat.Blocks) * bs,
		Free:  int64(stat.Bavail) * bs,
		Used:  int64(stat.Blocks-stat.Bfree) * bs,
	}, nil
}

// KnownLocation describes a macOS-specific directory that often consumes space.
type KnownLocation struct {
	Name        string
	Path        string
	Description string
	Size        int64 // populated by PopulateSizes
	Exists      bool  // populated by PopulateSizes
	Cleanable   bool
	CleanNote   string
	// CleanFn is the cleanup function. If nil and Cleanable=true, RemoveAll(Path) is used.
	CleanFn func() error
	// CommandOnly means the cleanup MUST go through CleanFn (e.g. Docker, iOS simulators).
	// TUI cleanup skips these items and tells the user to run the command manually.
	CommandOnly bool
}

// DefaultLocations returns the list of well-known macOS space consumers.
func DefaultLocations(home string) []KnownLocation {
	locs := []KnownLocation{
		{
			Name:        "App Caches",
			Path:        filepath.Join(home, "Library/Caches"),
			Description: i18n.T("loc.App Caches.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.App Caches.note"),
		},
		{
			Name:        "App Support",
			Path:        filepath.Join(home, "Library/Application Support"),
			Description: i18n.T("loc.App Support.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.App Support.note"),
		},
		{
			Name:        "Xcode DerivedData",
			Path:        filepath.Join(home, "Library/Developer/Xcode/DerivedData"),
			Description: i18n.T("loc.Xcode DerivedData.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Xcode DerivedData.note"),
		},
		{
			Name:        "iOS Simulators",
			Path:        filepath.Join(home, "Library/Developer/CoreSimulator/Devices"),
			Description: i18n.T("loc.iOS Simulators.desc"),
			Cleanable:   true,
			CommandOnly: true,
			CleanNote:   i18n.T("loc.iOS Simulators.note"),
			CleanFn: func() error {
				return exec.Command("xcrun", "simctl", "delete", "unavailable").Run()
			},
		},
		{
			Name:        "iOS Device Support",
			Path:        filepath.Join(home, "Library/Developer/Xcode/iOS DeviceSupport"),
			Description: i18n.T("loc.iOS Device Support.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.iOS Device Support.note"),
		},
		{
			Name:        "iOS Backups",
			Path:        filepath.Join(home, "Library/Application Support/MobileSync/Backup"),
			Description: i18n.T("loc.iOS Backups.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.iOS Backups.note"),
		},
		{
			Name:        "iCloud Drive",
			Path:        filepath.Join(home, "Library/Mobile Documents"),
			Description: i18n.T("loc.iCloud Drive.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.iCloud Drive.note"),
		},
		{
			Name:        "Downloads",
			Path:        filepath.Join(home, "Downloads"),
			Description: i18n.T("loc.Downloads.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Downloads.note"),
		},
		{
			Name:        "Trash",
			Path:        filepath.Join(home, ".Trash"),
			Description: i18n.T("loc.Trash.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Trash.note"),
			CleanFn: func() error {
				return exec.Command("osascript", "-e",
					`tell application "Finder" to empty trash`).Run()
			},
		},
		{
			Name:        "npm cache",
			Path:        filepath.Join(home, ".npm"),
			Description: i18n.T("loc.npm cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.npm cache.note"),
			CleanFn: func() error {
				return exec.Command("npm", "cache", "clean", "--force").Run()
			},
		},
		{
			Name:        "yarn cache",
			Path:        filepath.Join(home, ".yarn/cache"),
			Description: i18n.T("loc.yarn cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.yarn cache.note"),
			CleanFn: func() error {
				return exec.Command("yarn", "cache", "clean").Run()
			},
		},
		{
			Name:        "pnpm store",
			Path:        filepath.Join(home, ".pnpm-store"),
			Description: i18n.T("loc.pnpm store.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.pnpm store.note"),
			CleanFn: func() error {
				return exec.Command("pnpm", "store", "prune").Run()
			},
		},
		{
			Name:        "Go modules",
			Path:        filepath.Join(home, "go/pkg/mod"),
			Description: i18n.T("loc.Go modules.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Go modules.note"),
			CleanFn: func() error {
				return exec.Command("go", "clean", "-modcache").Run()
			},
		},
		{
			Name:        "Gradle cache",
			Path:        filepath.Join(home, ".gradle/caches"),
			Description: i18n.T("loc.Gradle cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Gradle cache.note"),
		},
		{
			Name:        "Maven cache",
			Path:        filepath.Join(home, ".m2/repository"),
			Description: i18n.T("loc.Maven cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Maven cache.note"),
		},
		{
			Name:        "Rust cargo",
			Path:        filepath.Join(home, ".cargo"),
			Description: i18n.T("loc.Rust cargo.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Rust cargo.note"),
		},
		{
			Name:        "pip cache",
			Path:        filepath.Join(home, "Library/Caches/pip"),
			Description: i18n.T("loc.pip cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.pip cache.note"),
			CleanFn: func() error {
				return exec.Command("pip", "cache", "purge").Run()
			},
		},
		// ── Developer tools ──────────────────────────────────────────────────
		{
			Name:        "Go build cache",
			Path:        filepath.Join(home, "Library/Caches/go-build"),
			Description: i18n.T("loc.Go build cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Go build cache.note"),
			CleanFn: func() error {
				return exec.Command("go", "clean", "-cache").Run()
			},
		},
		{
			Name:        "Rust toolchains",
			Path:        filepath.Join(home, ".rustup/toolchains"),
			Description: i18n.T("loc.Rust toolchains.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Rust toolchains.note"),
		},
		{
			Name:        "CocoaPods cache",
			Path:        filepath.Join(home, ".cocoapods"),
			Description: i18n.T("loc.CocoaPods cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.CocoaPods cache.note"),
			CleanFn: func() error {
				return exec.Command("pod", "cache", "clean", "--all").Run()
			},
		},
		{
			Name:        "Node-gyp cache",
			Path:        filepath.Join(home, ".node-gyp"),
			Description: i18n.T("loc.Node-gyp cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Node-gyp cache.note"),
		},
		{
			Name:        "Dart/Flutter pub",
			Path:        filepath.Join(home, ".pub-cache"),
			Description: i18n.T("loc.Dart/Flutter pub.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Dart/Flutter pub.note"),
			CleanFn: func() error {
				return exec.Command("dart", "pub", "cache", "clean").Run()
			},
		},
		{
			Name:        "NuGet packages",
			Path:        filepath.Join(home, ".nuget/packages"),
			Description: i18n.T("loc.NuGet packages.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.NuGet packages.note"),
			CleanFn: func() error {
				return exec.Command("dotnet", "nuget", "locals", "all", "--clear").Run()
			},
		},
		{
			Name:        "PlatformIO",
			Path:        filepath.Join(home, ".platformio"),
			Description: i18n.T("loc.PlatformIO.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.PlatformIO.note"),
			CleanFn: func() error {
				return exec.Command("pio", "system", "prune", "--force").Run()
			},
		},
		{
			Name:        "Bun packages",
			Path:        filepath.Join(home, ".bun/install"),
			Description: i18n.T("loc.Bun packages.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Bun packages.note"),
			CleanFn: func() error {
				return exec.Command("bun", "pm", "cache", "rm").Run()
			},
		},
		// ── AI / ML ──────────────────────────────────────────────────────────
		{
			Name:        "HuggingFace models",
			Path:        filepath.Join(home, ".cache/huggingface"),
			Description: i18n.T("loc.HuggingFace models.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.HuggingFace models.note"),
		},
		{
			Name:        "Whisper models",
			Path:        filepath.Join(home, ".cache/whisper"),
			Description: i18n.T("loc.Whisper models.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Whisper models.note"),
		},
		{
			Name:        "uv cache",
			Path:        filepath.Join(home, ".cache/uv"),
			Description: i18n.T("loc.uv cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.uv cache.note"),
			CleanFn: func() error {
				return exec.Command("uv", "cache", "clean").Run()
			},
		},
		{
			Name:        "Continue AI index",
			Path:        filepath.Join(home, ".continue/index"),
			Description: i18n.T("loc.Continue AI index.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Continue AI index.note"),
		},
		// ── Editors / IDEs ───────────────────────────────────────────────────
		{
			Name:        "VS Code extensions",
			Path:        filepath.Join(home, ".vscode/extensions"),
			Description: i18n.T("loc.VS Code extensions.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.VS Code extensions.note"),
		},
		// ── Python ───────────────────────────────────────────────────────────
		{
			Name:        "Python venv (~/.venv)",
			Path:        filepath.Join(home, ".venv"),
			Description: i18n.T("loc.Python venv (~/.venv).desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Python venv (~/.venv).note"),
		},
		// ── Browsers / Electron ──────────────────────────────────────────────
		{
			Name:        "Puppeteer Chromium",
			Path:        filepath.Join(home, ".cache/puppeteer"),
			Description: i18n.T("loc.Puppeteer Chromium.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Puppeteer Chromium.note"),
		},
		{
			Name:        "Electron cache",
			Path:        filepath.Join(home, ".cache/electron"),
			Description: i18n.T("loc.Electron cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Electron cache.note"),
		},
		// ── Wine ─────────────────────────────────────────────────────────────
		{
			Name:        "Wine",
			Path:        filepath.Join(home, ".wine"),
			Description: i18n.T("loc.Wine.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.Wine.note"),
		},
		// ── Browsers ─────────────────────────────────────────────────────────
		{
			Name:        "Safari cache",
			Path:        filepath.Join(home, "Library/Caches/com.apple.Safari"),
			Description: i18n.T("loc.Safari cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Safari cache.note"),
		},
		{
			Name:        "Chrome cache",
			Path:        filepath.Join(home, "Library/Caches/com.google.Chrome"),
			Description: i18n.T("loc.Chrome cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Chrome cache.note"),
		},
		{
			Name:        "Telegram",
			Path:        filepath.Join(home, "Library/Application Support/Telegram Desktop"),
			Description: i18n.T("loc.Telegram.desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.Telegram.note"),
		},
		{
			Name:        "Telegram (App Store)",
			Path:        filepath.Join(home, "Library/Group Containers/6N38VWS5BX.ru.keepcoder.Telegram"),
			Description: i18n.T("loc.Telegram (App Store).desc"),
			Cleanable:   false,
			CleanNote:   i18n.T("loc.Telegram (App Store).note"),
		},
	}

	// Docker: check different possible paths
	dockerPaths := []string{
		filepath.Join(home, "Library/Containers/com.docker.docker"),
		filepath.Join(home, ".docker"),
	}
	for _, p := range dockerPaths {
		if _, err := os.Stat(p); err == nil {
			locs = append(locs, KnownLocation{
				Name:        "Docker",
				Path:        p,
				Description: i18n.T("loc.Docker.desc"),
				Cleanable:   true,
				CommandOnly: true,
				CleanNote:   i18n.T("loc.Docker.note"),
				CleanFn: func() error {
					return exec.Command("docker", "system", "prune", "-a", "--volumes", "-f").Run()
				},
			})
			break
		}
	}

	// Homebrew cache
	if brewCache := homebrewCachePath(); brewCache != "" {
		locs = append(locs, KnownLocation{
			Name:        "Homebrew cache",
			Path:        brewCache,
			Description: i18n.T("loc.Homebrew cache.desc"),
			Cleanable:   true,
			CleanNote:   i18n.T("loc.Homebrew cache.note"),
			CleanFn: func() error {
				return exec.Command("brew", "cleanup").Run()
			},
		})
	}

	return locs
}

// homebrewCachePath returns the Homebrew cache directory or empty string.
func homebrewCachePath() string {
	out, err := exec.Command("brew", "--cache").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PopulateSizes fills in the Size and Exists fields for each location.
// sizeMap is the result of scanning (path → total size).
// scanRoot is the root directory that was scanned.
// Locations outside scanRoot are shown as existing (if present) but with size=-1.
func PopulateSizes(locs []KnownLocation, sizeMap map[string]int64, scanRoot string) {
	for i := range locs {
		path := locs[i].Path
		if size, ok := sizeMap[path]; ok {
			locs[i].Size = size
			locs[i].Exists = true
		} else if strings.HasPrefix(path, scanRoot+"/") || path == scanRoot {
			if _, err := os.Stat(path); err == nil {
				locs[i].Exists = true
				locs[i].Size = 0
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				locs[i].Exists = true
				locs[i].Size = -1
			}
		}
	}
}

// DirSize calculates the total size of a directory tree.
func DirSize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

// TimeMachineSnapshotCount returns the number of local Time Machine snapshots.
func TimeMachineSnapshotCount() (int, error) {
	out, err := exec.Command("tmutil", "listlocalsnapshots", "/").Output()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

// LargeNodeModules finds node_modules directories larger than minSize bytes.
func LargeNodeModules(root string, minSize int64, sizeMap map[string]int64) []struct {
	Path string
	Size int64
} {
	var results []struct {
		Path string
		Size int64
	}
	for path, size := range sizeMap {
		if filepath.Base(path) == "node_modules" && size >= minSize {
			results = append(results, struct {
				Path string
				Size int64
			}{path, size})
		}
	}
	return results
}
