package i18n

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	lang         string
	translations map[string]string
)

// Init detects the system locale and loads the appropriate translation table.
// Must be called once at program startup before any T() calls.
func Init() {
	lang = detectLang()
	switch lang {
	case "ru":
		translations = translationsRU
	default:
		translations = translationsEN
	}
}

// Lang returns the current language code ("ru" or "en").
func Lang() string { return lang }

// T returns the translated string for the given key.
// If the key is not found, the key itself is returned.
func T(key string) string {
	if s, ok := translations[key]; ok {
		return s
	}
	return key
}

// Tf returns a formatted translated string (fmt.Sprintf with the translated template).
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

func detectLang() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		val := os.Getenv(env)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(val), "ru") {
			return "ru"
		}
		return "en"
	}
	// macOS: read system locale via defaults
	if out, err := exec.Command("defaults", "read", "NSGlobalDomain", "AppleLocale").Output(); err == nil {
		locale := strings.TrimSpace(string(out))
		if strings.HasPrefix(strings.ToLower(locale), "ru") {
			return "ru"
		}
	}
	return "en"
}
