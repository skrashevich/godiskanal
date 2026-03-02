package macos

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	CleanFn     func() error // nil means use RemoveAll
}

// DefaultLocations returns the list of well-known macOS space consumers.
func DefaultLocations(home string) []KnownLocation {
	locs := []KnownLocation{
		{
			Name:        "App Caches",
			Path:        filepath.Join(home, "Library/Caches"),
			Description: "Кэши приложений",
			Cleanable:   true,
			CleanNote:   "Кэши пересоздадутся автоматически",
		},
		{
			Name:        "App Support",
			Path:        filepath.Join(home, "Library/Application Support"),
			Description: "Данные приложений",
			Cleanable:   false,
			CleanNote:   "Удалять данные конкретных приложений вручную",
		},
		{
			Name:        "Xcode DerivedData",
			Path:        filepath.Join(home, "Library/Developer/Xcode/DerivedData"),
			Description: "Артефакты сборки Xcode",
			Cleanable:   true,
			CleanNote:   "Xcode пересоберёт при необходимости",
		},
		{
			Name:        "iOS Simulators",
			Path:        filepath.Join(home, "Library/Developer/CoreSimulator/Devices"),
			Description: "Образы iOS-симуляторов",
			Cleanable:   true,
			CleanNote:   "xcrun simctl delete unavailable",
			CleanFn: func() error {
				return exec.Command("xcrun", "simctl", "delete", "unavailable").Run()
			},
		},
		{
			Name:        "iOS Device Support",
			Path:        filepath.Join(home, "Library/Developer/Xcode/iOS DeviceSupport"),
			Description: "Отладочные символы устройств",
			Cleanable:   true,
			CleanNote:   "Старые версии можно удалить",
		},
		{
			Name:        "iOS Backups",
			Path:        filepath.Join(home, "Library/Application Support/MobileSync/Backup"),
			Description: "Резервные копии iPhone/iPad",
			Cleanable:   true,
			CleanNote:   "Управляйте через Finder → устройство → Резервные копии",
		},
		{
			Name:        "iCloud Drive",
			Path:        filepath.Join(home, "Library/Mobile Documents"),
			Description: "Локальные копии iCloud Drive",
			Cleanable:   false,
			CleanNote:   "Управляйте через Системные настройки → Apple ID",
		},
		{
			Name:        "Downloads",
			Path:        filepath.Join(home, "Downloads"),
			Description: "Загрузки",
			Cleanable:   true,
			CleanNote:   "Проверьте содержимое вручную",
		},
		{
			Name:        "Trash",
			Path:        filepath.Join(home, ".Trash"),
			Description: "Корзина",
			Cleanable:   true,
			CleanNote:   "Очистить корзину",
			CleanFn: func() error {
				return exec.Command("osascript", "-e",
					`tell application "Finder" to empty trash`).Run()
			},
		},
		{
			Name:        "npm cache",
			Path:        filepath.Join(home, ".npm"),
			Description: "Кэш npm пакетов",
			Cleanable:   true,
			CleanNote:   "npm cache clean --force",
			CleanFn: func() error {
				return exec.Command("npm", "cache", "clean", "--force").Run()
			},
		},
		{
			Name:        "yarn cache",
			Path:        filepath.Join(home, ".yarn/cache"),
			Description: "Кэш Yarn",
			Cleanable:   true,
			CleanNote:   "yarn cache clean",
			CleanFn: func() error {
				return exec.Command("yarn", "cache", "clean").Run()
			},
		},
		{
			Name:        "pnpm store",
			Path:        filepath.Join(home, ".pnpm-store"),
			Description: "Хранилище pnpm",
			Cleanable:   true,
			CleanNote:   "pnpm store prune",
			CleanFn: func() error {
				return exec.Command("pnpm", "store", "prune").Run()
			},
		},
		{
			Name:        "Go modules",
			Path:        filepath.Join(home, "go/pkg/mod"),
			Description: "Кэш Go-модулей",
			Cleanable:   true,
			CleanNote:   "go clean -modcache",
			CleanFn: func() error {
				return exec.Command("go", "clean", "-modcache").Run()
			},
		},
		{
			Name:        "Gradle cache",
			Path:        filepath.Join(home, ".gradle/caches"),
			Description: "Кэш Gradle",
			Cleanable:   true,
			CleanNote:   "Безопасно удалить",
		},
		{
			Name:        "Maven cache",
			Path:        filepath.Join(home, ".m2/repository"),
			Description: "Локальный репозиторий Maven",
			Cleanable:   true,
			CleanNote:   "Безопасно удалить",
		},
		{
			Name:        "Rust cargo",
			Path:        filepath.Join(home, ".cargo"),
			Description: "Кэш Rust/Cargo",
			Cleanable:   true,
			CleanNote:   "cargo cache --autoclean",
		},
		{
			Name:        "pip cache",
			Path:        filepath.Join(home, "Library/Caches/pip"),
			Description: "Кэш Python pip",
			Cleanable:   true,
			CleanNote:   "pip cache purge",
			CleanFn: func() error {
				return exec.Command("pip", "cache", "purge").Run()
			},
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
				Description: "Docker образы и данные",
				Cleanable:   true,
				CleanNote:   "docker system prune -a --volumes",
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
			Description: "Кэш Homebrew",
			Cleanable:   true,
			CleanNote:   "brew cleanup",
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
			// Inside scan root but not in sizeMap → empty directory
			if _, err := os.Stat(path); err == nil {
				locs[i].Exists = true
				locs[i].Size = 0
			}
		} else {
			// Outside scan root: just check existence, don't walk (too slow)
			if _, err := os.Stat(path); err == nil {
				locs[i].Exists = true
				locs[i].Size = -1 // unknown
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
