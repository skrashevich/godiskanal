package tui

import (
	"path/filepath"
	"strings"

	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/macos"
)

// KnownHint is scanner metadata attached to a known macOS/dev path for LLM prompts.
type KnownHint struct {
	Path        string
	Name        string
	Description string
	CleanNote   string
	Cleanable   bool
	CommandOnly bool
}

// KnownHintsFromLocs converts populated known locations into browser hints.
func KnownHintsFromLocs(locs []macos.KnownLocation) []KnownHint {
	out := make([]KnownHint, 0, len(locs))
	for _, loc := range locs {
		if !loc.Exists || loc.Path == "" {
			continue
		}
		out = append(out, KnownHint{
			Path:        filepath.Clean(loc.Path),
			Name:        loc.Name,
			Description: loc.Description,
			CleanNote:   loc.CleanNote,
			Cleanable:   loc.Cleanable,
			CommandOnly: loc.CommandOnly,
		})
	}
	return out
}

// LookupKnownHint returns the best matching hint for path (exact or longest parent prefix).
func LookupKnownHint(path string, hints []KnownHint) *KnownHint {
	path = filepath.Clean(path)
	var best *KnownHint
	bestLen := -1
	for i := range hints {
		h := &hints[i]
		hp := h.Path
		if path == hp {
			if len(hp) > bestLen {
				best = h
				bestLen = len(hp)
			}
			continue
		}
		if strings.HasPrefix(path, hp+string(filepath.Separator)) && len(hp) > bestLen {
			best = h
			bestLen = len(hp)
		}
	}
	return best
}

func appendKnownHint(sb *strings.Builder, path string, hints []KnownHint) {
	h := LookupKnownHint(path, hints)
	if h == nil {
		return
	}
	sb.WriteString(i18n.Tf("browser.llm.known_match", h.Name))
	if h.Description != "" {
		sb.WriteString(i18n.Tf("browser.llm.known_desc", h.Description))
	}
	if h.CleanNote != "" {
		sb.WriteString(i18n.Tf("browser.llm.known_suggested", h.CleanNote))
	}
	if h.CommandOnly {
		sb.WriteString(i18n.T("browser.llm.known_manual"))
	} else if h.Cleanable {
		sb.WriteString(i18n.T("browser.llm.known_cleanable"))
	}
}
