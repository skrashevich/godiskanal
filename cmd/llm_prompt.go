package cmd

import (
	"fmt"
	"strings"

	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/scanner"
	"github.com/skrashevich/godiskanal/ui"
)

const (
	llmMaxTopDirs       = 15
	llmMaxKnownLocs     = 30
	llmMaxNodeModules   = 10
	llmMinKnownForPrompt = 1 * 1024 * 1024 // 1 MiB
)

// llmPromptInput is the scan context passed to the startup --llm analysis.
type llmPromptInput struct {
	ScanPath      string
	Homes         []string
	RunningAsRoot bool
	Disk          *macos.DiskInfo
	Top           []scanner.Entry
	Known         []macos.KnownLocation
	NodeModules   []struct {
		Path string
		Size int64
	}
	TMSnapshots int // 0 = none or unavailable
}

// buildLLMPrompt creates the user message for startup disk analysis (--llm).
func buildLLMPrompt(in llmPromptInput) string {
	var b strings.Builder

	b.WriteString(i18n.T("llm.prompt.header"))
	b.WriteString(i18n.Tf("llm.prompt.scan", in.ScanPath))
	if in.RunningAsRoot {
		b.WriteString(i18n.T("llm.prompt.root"))
	}

	if in.Disk != nil {
		b.WriteString(i18n.Tf("llm.prompt.disk",
			ui.FormatSize(in.Disk.Total),
			ui.FormatSize(in.Disk.Used),
			float64(in.Disk.Used)/float64(in.Disk.Total)*100,
			ui.FormatSize(in.Disk.Free),
		))
	}

	b.WriteString(i18n.T("llm.prompt.top_dirs"))
	for i, e := range in.Top {
		if i >= llmMaxTopDirs {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1,
			displayPathForHomes(e.Path, in.Homes), ui.FormatSize(e.Size)))
	}

	if len(in.NodeModules) > 0 {
		b.WriteString(i18n.T("llm.prompt.node_modules"))
		for i, nm := range in.NodeModules {
			if i >= llmMaxNodeModules {
				break
			}
			b.WriteString(i18n.Tf("llm.prompt.node_item",
				displayPathForHomes(nm.Path, in.Homes), ui.FormatSize(nm.Size)))
		}
	}

	if in.TMSnapshots > 0 {
		b.WriteString(i18n.Tf("llm.prompt.tm", in.TMSnapshots))
	}

	b.WriteString(i18n.T("llm.prompt.known"))
	shown := 0
	for _, loc := range in.Known {
		if !loc.Exists {
			continue
		}
		if loc.Size > 0 && loc.Size < llmMinKnownForPrompt && !loc.Cleanable {
			continue
		}
		if loc.Size <= 0 && !loc.Cleanable {
			continue
		}

		flags := ""
		if loc.Cleanable {
			flags += i18n.T("llm.prompt.cleanable")
		}
		if loc.CommandOnly {
			flags += i18n.T("llm.prompt.manual")
		}

		sizeStr := ui.FormatSize(loc.Size)
		if loc.Size < 0 {
			sizeStr = i18n.T("llm.prompt.size_unknown")
		}

		b.WriteString(fmt.Sprintf("- **%s**: %s%s\n", loc.Name, sizeStr, flags))
		b.WriteString(i18n.Tf("llm.prompt.path", displayPathForHomes(loc.Path, in.Homes)))
		if loc.Description != "" {
			b.WriteString(i18n.Tf("llm.prompt.desc", loc.Description))
		}
		if loc.CleanNote != "" {
			b.WriteString(i18n.Tf("llm.prompt.suggested", loc.CleanNote))
		}

		shown++
		if shown >= llmMaxKnownLocs {
			break
		}
	}

	b.WriteString(i18n.T("llm.prompt.request"))
	return b.String()
}
