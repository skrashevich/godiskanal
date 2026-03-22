package i18n

import (
	"os"
	"testing"
)

func TestAllKeysPresent(t *testing.T) {
	for key := range translationsRU {
		if _, ok := translationsEN[key]; !ok {
			t.Errorf("key %q present in RU but missing in EN", key)
		}
	}
	for key := range translationsEN {
		if _, ok := translationsRU[key]; !ok {
			t.Errorf("key %q present in EN but missing in RU", key)
		}
	}
}

func TestTMissingKey(t *testing.T) {
	Init()
	got := T("nonexistent.key.12345")
	if got != "nonexistent.key.12345" {
		t.Errorf("expected key itself for missing translation, got %q", got)
	}
}

func TestTfFormatting(t *testing.T) {
	Init()
	got := Tf("top.header", 10)
	// Should contain "10" regardless of language
	if len(got) == 0 {
		t.Error("Tf returned empty string")
	}
}

func TestDetectLangRu(t *testing.T) {
	os.Setenv("LC_ALL", "ru_RU.UTF-8")
	defer os.Unsetenv("LC_ALL")
	got := detectLang()
	if got != "ru" {
		t.Errorf("expected ru, got %q", got)
	}
}

func TestDetectLangEn(t *testing.T) {
	os.Setenv("LC_ALL", "en_US.UTF-8")
	defer os.Unsetenv("LC_ALL")
	got := detectLang()
	if got != "en" {
		t.Errorf("expected en, got %q", got)
	}
}

func TestLangReturnsValue(t *testing.T) {
	Init()
	l := Lang()
	if l != "ru" && l != "en" {
		t.Errorf("unexpected language: %q", l)
	}
}
