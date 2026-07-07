//go:build darwin

package macos

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ReadAppBundle reads CFBundleIdentifier and display name from an .app bundle.
func ReadAppBundle(appPath string) (AppBundleInfo, error) {
	appPath = filepath.Clean(appPath)
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return AppBundleInfo{}, fmt.Errorf("info.plist: %w", err)
	}

	bundleID, err := defaultsRead(plist, "CFBundleIdentifier")
	if err != nil || bundleID == "" {
		return AppBundleInfo{}, fmt.Errorf("CFBundleIdentifier: %w", err)
	}

	displayName, _ := defaultsRead(plist, "CFBundleDisplayName")
	if displayName == "" {
		displayName, _ = defaultsRead(plist, "CFBundleName")
	}
	if displayName == "" {
		displayName = strings.TrimSuffix(filepath.Base(appPath), ".app")
	}

	return AppBundleInfo{
		AppPath:     appPath,
		BundleID:    bundleID,
		DisplayName: displayName,
	}, nil
}

func defaultsRead(plistPath, key string) (string, error) {
	out, err := exec.Command("defaults", "read", plistPath, key).Output()
	if err != nil {
		return "", err
	}
	// defaults may return ( "value" ) for localized strings
	s := strings.TrimSpace(string(out))
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.Trim(s, "()")
		s = strings.Trim(s, `"`)
	}
	return strings.Trim(s, `"`), nil
}
