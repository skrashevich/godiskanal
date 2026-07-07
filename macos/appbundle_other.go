//go:build !darwin

package macos

import "errors"

// ReadAppBundle is only supported on macOS.
func ReadAppBundle(appPath string) (AppBundleInfo, error) {
	return AppBundleInfo{}, errors.New("app bundle metadata requires macOS")
}
