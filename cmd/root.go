package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/llm"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/scanner"
	"github.com/skrashevich/godiskanal/tui"
	"github.com/skrashevich/godiskanal/ui"
	"github.com/skrashevich/godiskanal/version"
)

var (
	scanPath      string
	topN          int
	useLLM        bool
	apiKey        string
	apiURL        string
	model         string
	interactive   bool
	browse        bool
	minSize       int64
	oneFilesystem bool
	excludePaths  []string
	showVersion   bool
)

func Execute() {
	home, _ := os.UserHomeDir()

	rootCmd := &cobra.Command{
		Use:           "godiskanal [--path PATH]",
		Short:         i18n.T("cmd.short"),
		Long:          i18n.T("cmd.long"),
		RunE:          run,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			if showVersion {
				fmt.Printf("godiskanal %s\n", version.Version)
				os.Exit(0)
			}
		},
	}

	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, i18n.T("flag.version"))
	rootCmd.Flags().StringVarP(&scanPath, "path", "p", home, i18n.T("flag.path"))
	rootCmd.Flags().IntVarP(&topN, "top", "n", 20, i18n.T("flag.top"))
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, i18n.T("flag.interactive"))
	rootCmd.Flags().BoolVarP(&oneFilesystem, "one-filesystem", "x", false, i18n.T("flag.one_filesystem"))
	rootCmd.Flags().StringArrayVar(&excludePaths, "exclude", nil, i18n.T("flag.exclude"))
	rootCmd.Flags().Int64Var(&minSize, "min-size", 100*1024*1024, i18n.T("flag.min_size"))
	rootCmd.Flags().BoolVarP(&browse, "browse", "b", false, i18n.T("flag.browse"))
	rootCmd.Flags().BoolVar(&useLLM, "llm", false, i18n.T("flag.llm"))
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", i18n.T("flag.api_key"))
	rootCmd.Flags().StringVar(&apiURL, "api-url", "", i18n.T("flag.api_url"))
	rootCmd.Flags().StringVar(&model, "model", "gpt-4o-mini", i18n.T("flag.model"))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, i18n.Tf("err.generic", err))
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rawHome, _ := os.UserHomeDir()
	home := macos.ResolveTargetHome(rawHome)
	if !cmd.Flags().Changed("path") {
		scanPath = macos.DefaultScanPath(home)
	}
	scanPath = expandHome(scanPath, home)

	fmt.Printf("\033[1m%s\033[0m\n", i18n.T("app.title"))
	targetHomes := macos.TargetHomes(home)
	if macos.RunningAsRoot() {
		if rawHome == "/var/root" && home != rawHome {
			fmt.Println(i18n.Tf("root.home_hint", home))
		} else if len(targetHomes) > 1 {
			fmt.Println(i18n.Tf("root.multi_user", len(targetHomes)))
		}
		if !cmd.Flags().Changed("path") {
			fmt.Println(i18n.Tf("root.scan_default", scanPath))
		}
	}

	// 1. Disk info
	diskInfo, err := macos.GetDiskInfo(scanPath)
	if err != nil {
		fmt.Fprint(os.Stderr, i18n.Tf("err.disk_info", err))
	} else {
		ui.PrintDiskUsage(diskInfo.Total, diskInfo.Used, diskInfo.Free)
	}

	// 2. Scan
	ui.Header(i18n.T("scan.header"))
	if oneFilesystem {
		fmt.Println(i18n.T("scan.one_fs"))
	}
	start := time.Now()
	spinFrame := 0

	opts := scanner.Options{
		OneFilesystem: oneFilesystem,
		Exclude:       buildExcludes(excludePaths, home),
	}

	result, err := scanner.Scan(ctx, scanPath, opts, func(files, bytes int64, currentDir string) {
		ui.PrintScanProgress(spinFrame, files, bytes, currentDir)
		spinFrame++
	})
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.scan"), err)
	}

	if ctx.Err() != nil {
		fmt.Println(i18n.T("scan.interrupted"))
	}

	elapsed := time.Since(start).Seconds()
	ui.PrintScanDone(result.FileCount, result.TotalSize, elapsed)

	if result.Errors > 0 {
		fmt.Println(i18n.Tf("scan.perm_errors", result.Errors))
	}
	if result.Timeouts > 0 {
		fmt.Println(i18n.Tf("scan.timeouts", result.Timeouts))
	}

	// Build size map for lookups
	sizeMap := make(map[string]int64, len(result.Entries)+1)
	sizeMap[scanPath] = result.TotalSize
	for _, e := range result.Entries {
		sizeMap[e.Path] = e.Size
	}

	// 3. TUI browser (if --browse)
	if browse {
		var llmClient *llm.Client
		key := apiKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		if key != "" {
			baseURL := apiURL
			if baseURL == "" {
				baseURL = os.Getenv("OPENAI_BASE_URL")
			}
			llmClient = llm.NewClient(key, model, baseURL)
		}
		var llmHints []tui.KnownHint
		if llmClient != nil {
			llmHints = tui.KnownHintsFromLocs(prepareKnownLocations(targetHomes, sizeMap, scanPath, minSize))
		}
		m := tui.New(scanPath, sizeMap, llmClient, llmHints, targetHomes)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("err.browser"), err)
		}
		return nil
	}

	// 4. Top directories
	top := result.TopN(topN)
	var maxTopSize int64
	if len(top) > 0 {
		maxTopSize = top[0].Size
	}
	ui.Header(i18n.Tf("top.header", len(top)))
	for i, e := range top {
		ui.PrintTopEntry(i+1, displayPath(e.Path, home), e.Size, maxTopSize)
	}

	// 4. Large node_modules
	nmDirs := macos.LargeNodeModules(scanPath, 200*1024*1024, sizeMap)
	if len(nmDirs) > 0 {
		sort.Slice(nmDirs, func(i, j int) bool {
			return nmDirs[i].Size > nmDirs[j].Size
		})
		ui.Header(i18n.Tf("node_modules.header", len(nmDirs)))
		maxNM := nmDirs[0].Size
		for i, nm := range nmDirs {
			if i >= 10 {
				break
			}
			ui.PrintTopEntry(i+1, displayPath(nm.Path, home), nm.Size, maxNM)
		}
	}

	// 5. Known macOS locations
	displayLocs := prepareKnownLocations(targetHomes, sizeMap, scanPath, minSize)
	ui.Header(i18n.Tf("known.header", len(displayLocs)))
	for _, loc := range displayLocs {
		ui.PrintKnownEntry(loc.Cleanable, loc.Name, displayPathForHomes(loc.Path, targetHomes), loc.Size)
	}

	// Time Machine snapshots
	tmSnapshots := 0
	if count, err := macos.TimeMachineSnapshotCount(); err == nil && count > 0 {
		tmSnapshots = count
		fmt.Println(i18n.Tf("tm.info", count))
		fmt.Println(i18n.T("tm.delete"))
	}

	// 6. LLM analysis
	if useLLM {
		key := apiKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		if key == "" {
			return fmt.Errorf("%s", i18n.T("err.api_key"))
		}

		baseURL := apiURL
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		baseURL = llm.ResolveBaseURL(baseURL)

		ui.Header(i18n.T("llm.header"))
		fmt.Println(i18n.Tf("llm.model", model))
		fmt.Println(i18n.Tf("llm.provider", llm.ProviderLabel(baseURL), baseURL))
		fmt.Println(i18n.T("llm.analyzing") + "\n")

		prompt := buildLLMPrompt(llmPromptInput{
			ScanPath:      scanPath,
			Homes:         targetHomes,
			RunningAsRoot: macos.RunningAsRoot(),
			Disk:          diskInfo,
			Top:           top,
			Known:         displayLocs,
			NodeModules:   nmDirs,
			TMSnapshots:   tmSnapshots,
		})
		client := llm.NewClient(key, model, baseURL)
		var llmBuf strings.Builder
		usage, err := client.StreamAnalysis(prompt, &llmBuf)
		if err != nil {
			fmt.Fprintln(os.Stderr, i18n.Tf("llm.error", err))
		} else if llmBuf.Len() > 0 {
			if rendered, rerr := ui.RenderMarkdown(llmBuf.String()); rerr != nil {
				fmt.Print(llmBuf.String())
				if !strings.HasSuffix(llmBuf.String(), "\n") {
					fmt.Println()
				}
			} else {
				fmt.Print(rendered)
			}
		}
		if usage != nil {
			printLLMCost(usage, model)
		}
	}

	// 7. Interactive cleanup (TUI)
	if interactive {
		m := tui.NewCleanup(displayLocs, sizeMap)
		if !m.HasItems() {
			fmt.Println(i18n.T("cleanup.no_items"))
		} else {
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("err.cleanup_tui"), err)
			}
		}
	} else if !useLLM {
		printQuickTips(displayLocs)
	}

	return nil
}

// prepareKnownLocations loads, sizes, and filters known macOS/dev paths for display and LLM hints.
func prepareKnownLocations(targetHomes []string, sizeMap map[string]int64, scanPath string, minSize int64) []macos.KnownLocation {
	locs := macos.AllKnownLocations(targetHomes)
	if macos.RunningAsRoot() {
		locs = append(locs, macos.SystemLocations()...)
	}
	enrichDisk := macos.RunningAsRoot() || len(targetHomes) > 1
	macos.PopulateSizesEnriched(locs, sizeMap, scanPath, enrichDisk, 24)
	macos.SortKnownBySize(locs)
	return macos.FilterKnownForDisplay(locs, minSize)
}

// printQuickTips shows a brief summary of actionable items.
func printQuickTips(locs []macos.KnownLocation) {
	var tips []macos.KnownLocation
	for _, loc := range locs {
		if loc.Exists && loc.Cleanable && loc.Size > 100*1024*1024 {
			tips = append(tips, loc)
		}
	}
	if len(tips) == 0 {
		return
	}

	sort.Slice(tips, func(i, j int) bool {
		return tips[i].Size > tips[j].Size
	})

	ui.Header(i18n.T("tips.header"))
	total := int64(0)
	for _, loc := range tips {
		fmt.Printf("  • %-26s  %s  —  %s\n",
			loc.Name, ui.FormatSize(loc.Size), loc.CleanNote)
		total += loc.Size
	}
	fmt.Print(i18n.Tf("tips.potential", ui.FormatSize(total)))
	fmt.Print(i18n.T("tips.run"))
}

// displayPath shortens a path by replacing the home directory with ~.
func displayPath(path, home string) string {
	return displayPathForHomes(path, []string{home})
}

// displayPathForHomes replaces any known home prefix with ~user or ~.
func displayPathForHomes(path string, homes []string) string {
	best := ""
	for _, home := range homes {
		if home == "" {
			continue
		}
		if path == home && len(home) > len(best) {
			best = home
		}
		if strings.HasPrefix(path, home+"/") && len(home) > len(best) {
			best = home
		}
	}
	if best == "" {
		return path
	}
	if path == best {
		if filepath.Base(best) == "root" || best == "/var/root" {
			return best
		}
		return "~"
	}
	rel := path[len(best):]
	if len(homes) > 1 {
		return "~" + filepath.Base(best) + rel
	}
	return "~" + rel
}

// printLLMCost prints token usage and estimated cost after an LLM call.
func printLLMCost(usage *llm.Usage, model string) {
	fmt.Print(i18n.Tf("llm.tokens",
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens))

	if cost, ok := usage.Cost(model); ok {
		if cost < 0.001 {
			fmt.Print(i18n.T("llm.cost_low"))
		} else {
			fmt.Print(i18n.Tf("llm.cost", cost))
		}
	}
	fmt.Println()
}

// buildExcludes expands ~ in user-supplied exclude paths.
func buildExcludes(paths []string, home string) []string {
	if len(paths) == 0 {
		return nil
	}
	result := make([]string, len(paths))
	for i, p := range paths {
		result[i] = expandHome(p, home)
	}
	return result
}

// expandHome expands ~ in a path to the actual home directory.
func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return filepath.Clean(path)
}
