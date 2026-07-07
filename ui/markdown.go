package ui

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const llmIndent = "  "

var (
	reHeader   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reListItem = regexp.MustCompile(`^(\s*)[-*]\s+(.*)$`)
	reBold     = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reCode     = regexp.MustCompile("`([^`]+)`")
)

// RenderMarkdown renders markdown for the full terminal width (CLI output).
func RenderMarkdown(md string) (string, error) {
	w := TermWidth()
	if w < 40 {
		w = 40
	}
	return RenderMarkdownWidth(md, w)
}

// RenderMarkdownWidth renders markdown wrapped to lineWidth columns (e.g. TUI side panel).
func RenderMarkdownWidth(md string, lineWidth int) (string, error) {
	md = strings.TrimSpace(md)
	if md == "" {
		return "", nil
	}

	if lineWidth < 20 {
		lineWidth = 20
	}
	wrap := lineWidth - len(llmIndent)
	if wrap < 16 {
		wrap = 16
	}

	var b strings.Builder
	for _, line := range strings.Split(md, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteByte('\n')
			continue
		}
		rendered := renderMarkdownLine(line)
		for _, wrapped := range wrapRendered(rendered, wrap) {
			b.WriteString(llmIndent)
			b.WriteString(wrapped)
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

func renderMarkdownLine(line string) string {
	if m := reHeader.FindStringSubmatch(line); m != nil {
		level := len(m[1])
		text := formatInline(m[2])
		switch level {
		case 1, 2:
			return bold + text + reset
		default:
			return bold + cyan + text + reset
		}
	}
	if m := reListItem.FindStringSubmatch(line); m != nil {
		prefix := strings.Repeat(" ", len(m[1]))
		return prefix + dim + "• " + reset + formatInline(m[2])
	}
	return formatInline(line)
}

func formatInline(s string) string {
	s = reCode.ReplaceAllStringFunc(s, func(m string) string {
		inner := reCode.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		return dim + inner[1] + reset
	})
	s = reBold.ReplaceAllStringFunc(s, func(m string) string {
		inner := reBold.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		return bold + inner[1] + reset
	})
	return s
}

// wrapRendered word-wraps a line that may contain ANSI sequences.
func wrapRendered(line string, maxWidth int) []string {
	if visibleWidth(line) <= maxWidth {
		return []string{line}
	}

	words := splitVisibleWords(line)
	if len(words) == 0 {
		return []string{line}
	}

	var lines []string
	var cur strings.Builder
	curW := 0

	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
	}

	for _, w := range words {
		ww := visibleWidth(w)
		if ww > maxWidth {
			flush()
			lines = append(lines, w)
			continue
		}
		sep := 0
		if curW > 0 {
			sep = 1
		}
		if curW+sep+ww > maxWidth {
			flush()
			sep = 0
		}
		if sep == 1 {
			cur.WriteByte(' ')
			curW++
		}
		cur.WriteString(w)
		curW += ww
	}
	flush()
	if len(lines) == 0 {
		return []string{line}
	}
	return lines
}

func splitVisibleWords(s string) []string {
	var words []string
	var cur strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			if cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
			}
			inEscape = true
			cur.WriteByte(s[i])
			continue
		}
		if inEscape {
			cur.WriteByte(s[i])
			if s[i] == 'm' {
				inEscape = false
				words = append(words, cur.String())
				cur.Reset()
			}
			continue
		}
		if s[i] == ' ' {
			if cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteByte(s[i])
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

func visibleWidth(s string) int {
	n := 0
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		n++
		i += size - 1
	}
	return n
}
