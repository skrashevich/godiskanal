package cmd

import (
	"strings"
	"testing"

	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/scanner"
)

func TestBuildLLMPrompt_IncludesContextAndHints(t *testing.T) {
	i18n.Init()

	prompt := buildLLMPrompt(llmPromptInput{
		ScanPath:      "/Users/alice",
		Homes:         []string{"/Users/alice"},
		RunningAsRoot: true,
		Disk: &macos.DiskInfo{
			Total: 500 * 1024 * 1024 * 1024,
			Used:  400 * 1024 * 1024 * 1024,
			Free:  100 * 1024 * 1024 * 1024,
		},
		Top: []scanner.Entry{
			{Path: "/Users/alice/Projects/big", Size: 12 * 1024 * 1024 * 1024},
		},
		Known: []macos.KnownLocation{
			{
				Name:        "npm cache",
				Path:        "/Users/alice/.npm",
				Description: "npm download cache",
				Size:        2 * 1024 * 1024 * 1024,
				Exists:      true,
				Cleanable:   true,
				CleanNote:   "npm cache clean --force",
			},
			{
				Name:      "Docker",
				Path:      "/Users/alice/Library/Containers/com.docker.docker",
				Size:      8 * 1024 * 1024 * 1024,
				Exists:    true,
				Cleanable: true,
				CleanNote: "docker system prune -a --volumes",
				CommandOnly: true,
			},
		},
		NodeModules: []struct {
			Path string
			Size int64
		}{
			{Path: "/Users/alice/app/node_modules", Size: 500 * 1024 * 1024},
		},
		TMSnapshots: 3,
	})

	checks := []string{
		"/Users/alice",
		"running as root",
		"~/Projects/big",
		"node_modules",
		"Time Machine",
		"tmutil deletelocalsnapshots",
		"npm cache",
		"npm cache clean --force",
		"cleanable in godiskanal -i",
		"run suggested command",
		"docker system prune",
		"Your task",
		"godiskanal -i",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

func TestBuildLLMPrompt_SkipsTinyNonCleanable(t *testing.T) {
	i18n.Init()

	prompt := buildLLMPrompt(llmPromptInput{
		ScanPath: "/Users/bob",
		Homes:    []string{"/Users/bob"},
		Known: []macos.KnownLocation{
			{Name: "tiny", Path: "/Users/bob/tiny", Size: 100, Exists: true, Cleanable: false},
			{Name: "big", Path: "/Users/bob/big", Size: 200 * 1024 * 1024, Exists: true, Cleanable: false},
		},
	})

	if strings.Contains(prompt, "**tiny**") {
		t.Error("expected tiny non-cleanable location to be omitted")
	}
	if !strings.Contains(prompt, "**big**") {
		t.Error("expected large location to be included")
	}
}
