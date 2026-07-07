package ui

import (
	"strings"
	"testing"
)

const sampleLLMMarkdown = `### 1. Удалить локальные снимки Time Machine (~неизвестный выигрыш)
- **Зачем:** Локальные снимки могут занимать значительное место.
- **Риск:** низкий
- **Действие:** ` + "`tmutil deletelocalsnapshots /`" + `

**Дальше:** Используйте godiskanal -i.`

func TestRenderMarkdown(t *testing.T) {
	out, err := RenderMarkdown(sampleLLMMarkdown)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if strings.Contains(out, "### ") {
		t.Errorf("raw markdown heading leaked:\n%s", out)
	}
	if strings.Contains(out, "**Зачем:**") {
		t.Errorf("raw bold markers leaked:\n%s", out)
	}
	if !strings.Contains(out, "Зачем:") {
		t.Error("expected formatted label Зачем:")
	}
	if !strings.Contains(out, "tmutil deletelocalsnapshots") {
		t.Error("expected inline code content")
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, llmIndent) {
			t.Errorf("line missing indent: %q", line)
		}
	}
}

func TestRenderMarkdownWidth_narrow(t *testing.T) {
	out, err := RenderMarkdownWidth(sampleLLMMarkdown, 40)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected output")
	}
	// Should wrap to multiple lines in a narrow panel.
	if strings.Count(out, "\n") < 3 {
		t.Errorf("expected wrapped lines in narrow width, got %d newlines", strings.Count(out, "\n"))
	}
}

func TestRenderMarkdown_empty(t *testing.T) {
	out, err := RenderMarkdown("  \n  ")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
}

func TestFormatInline_boldAndCode(t *testing.T) {
	got := formatInline("run `brew cleanup` for **cache**")
	if strings.Contains(got, "**") || strings.Contains(got, "`") {
		t.Fatalf("markers not stripped: %q", got)
	}
	if !strings.Contains(got, "brew cleanup") || !strings.Contains(got, "cache") {
		t.Fatalf("content lost: %q", got)
	}
}
