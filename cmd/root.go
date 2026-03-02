package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/skrashevich/godiskanal/llm"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/scanner"
	"github.com/skrashevich/godiskanal/ui"
)

var (
	scanPath      string
	topN          int
	useLLM        bool
	apiKey        string
	apiURL        string
	model         string
	interactive   bool
	minSize       int64
	oneFilesystem bool
	excludePaths  []string
)

var rootCmd = &cobra.Command{
	Use:   "godiskanal [--path PATH]",
	Short: "Анализатор использования диска для macOS",
	Long: `godiskanal — консольная утилита для анализа использования диска на macOS.

Параллельно сканирует файловую систему, показывает топ директорий по размеру,
проверяет известные «пожиратели» места (Xcode, Docker, кэши пакетных менеджеров,
iCloud и др.) и помогает освободить пространство.

При наличии API ключа (--llm) отправляет данные в OpenAI-совместимый LLM
и стримит персонализированные рекомендации прямо в терминал.

Сканирование:
  • Многопоточное — по умолчанию использует все CPU ядра
  • Прогресс показывает текущий каталог, адаптируется под ширину терминала
  • Директории, не ответившие за 3 секунды (iCloud, NFS), пропускаются
  • Ctrl+C прерывает сканирование и выводит частичные результаты

Переменные окружения:
  OPENAI_API_KEY   API ключ (если не указан --api-key)
  OPENAI_BASE_URL  Базовый URL API (если не указан --api-url)`,
	RunE:          run,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	home, _ := os.UserHomeDir()
	rootCmd.Flags().StringVarP(&scanPath, "path", "p", home, "Путь для сканирования")
	rootCmd.Flags().IntVarP(&topN, "top", "n", 20, "Количество топ-директорий")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Интерактивный режим очистки")
	rootCmd.Flags().BoolVarP(&oneFilesystem, "one-filesystem", "x", false, "Не пересекать границы файловых систем (пропускать точки монтирования)")
	rootCmd.Flags().StringArrayVar(&excludePaths, "exclude", nil, "Исключить путь из сканирования (можно повторять: --exclude ~/a --exclude ~/b)")
	rootCmd.Flags().Int64Var(&minSize, "min-size", 100*1024*1024, "Минимальный размер для отображения (байт)")
	rootCmd.Flags().BoolVar(&useLLM, "llm", false, "Включить LLM-анализ с рекомендациями по очистке")
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "API ключ (переопределяет OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&apiURL, "api-url", "", "Базовый URL API (переопределяет OPENAI_BASE_URL; по умолчанию: https://api.openai.com/v1)")
	rootCmd.Flags().StringVar(&model, "model", "gpt-4o-mini", "Модель LLM")
}

func run(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, _ := os.UserHomeDir()
	scanPath = expandHome(scanPath, home)

	fmt.Printf("\033[1mgodiskanal\033[0m — анализатор диска macOS\n")

	// 1. Disk info
	diskInfo, err := macos.GetDiskInfo(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Предупреждение: не удалось получить информацию о диске: %v\n", err)
	} else {
		ui.PrintDiskUsage(diskInfo.Total, diskInfo.Used, diskInfo.Free)
	}

	// 2. Scan
	ui.Header("СКАНИРОВАНИЕ")
	if oneFilesystem {
		fmt.Printf("  \033[2m-x: пропускать точки монтирования\033[0m\n")
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
		return fmt.Errorf("ошибка сканирования: %w", err)
	}

	if ctx.Err() != nil {
		fmt.Printf("\n  \033[33m⚠ Сканирование прервано (Ctrl+C) — результаты неполные\033[0m\n")
	}

	elapsed := time.Since(start).Seconds()
	ui.PrintScanDone(result.FileCount, result.TotalSize, elapsed)

	if result.Errors > 0 {
		fmt.Printf("  \033[33m⚠ Пропущено %d директорий (нет доступа)\033[0m\n", result.Errors)
	}
	if result.Timeouts > 0 {
		fmt.Printf("  \033[33m⚠ Пропущено %d директорий (таймаут — возможно iCloud или сетевой диск)\033[0m\n", result.Timeouts)
	}

	// Build size map for lookups
	sizeMap := make(map[string]int64, len(result.Entries))
	for _, e := range result.Entries {
		sizeMap[e.Path] = e.Size
	}

	// 3. Top directories
	top := result.TopN(topN)
	var maxTopSize int64
	if len(top) > 0 {
		maxTopSize = top[0].Size
	}
	ui.Header(fmt.Sprintf("ТОП-%d ДИРЕКТОРИЙ", len(top)))
	for i, e := range top {
		ui.PrintTopEntry(i+1, displayPath(e.Path, home), e.Size, maxTopSize)
	}

	// 4. Large node_modules
	nmDirs := macos.LargeNodeModules(scanPath, 200*1024*1024, sizeMap)
	if len(nmDirs) > 0 {
		sort.Slice(nmDirs, func(i, j int) bool {
			return nmDirs[i].Size > nmDirs[j].Size
		})
		ui.Header(fmt.Sprintf("NODE_MODULES (%d найдено)", len(nmDirs)))
		maxNM := nmDirs[0].Size
		for i, nm := range nmDirs {
			if i >= 10 {
				break
			}
			ui.PrintTopEntry(i+1, displayPath(nm.Path, home), nm.Size, maxNM)
		}
	}

	// 5. Known macOS locations
	locs := macos.DefaultLocations(home)
	macos.PopulateSizes(locs, sizeMap, scanPath)
	ui.Header("ИЗВЕСТНЫЕ МЕСТА")
	for _, loc := range locs {
		if !loc.Exists {
			continue
		}
		ui.PrintKnownEntry(loc.Cleanable, loc.Name, displayPath(loc.Path, home), loc.Size)
	}

	// Time Machine snapshots
	if count, err := macos.TimeMachineSnapshotCount(); err == nil && count > 0 {
		fmt.Printf("\n  \033[36mℹ Time Machine: %d локальных снимков\033[0m\n", count)
		fmt.Printf("    Удалить: \033[1mtmutil deletelocalsnapshots /\033[0m\n")
	}

	// 6. LLM analysis
	if useLLM {
		key := apiKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		if key == "" {
			return fmt.Errorf("требуется OpenAI API ключ: --api-key или переменная окружения OPENAI_API_KEY")
		}

		baseURL := apiURL
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}

		ui.Header("АНАЛИЗ LLM")
		fmt.Printf("  \033[2mАнализирую с помощью %s...\033[0m\n\n", model)

		prompt := buildLLMPrompt(diskInfo, top, locs, home)
		client := llm.NewClient(key, model, baseURL)
		usage, err := client.StreamAnalysis(prompt, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  \033[31mОшибка LLM: %v\033[0m\n", err)
		}
		if usage != nil {
			printLLMCost(usage, model)
		}
	}

	// 7. Interactive cleanup
	if interactive {
		runInteractiveCleanup(locs, home)
	} else if !useLLM {
		printQuickTips(locs)
	}

	return nil
}

// buildLLMPrompt creates the analysis prompt for the LLM.
func buildLLMPrompt(disk *macos.DiskInfo, top []scanner.Entry, locs []macos.KnownLocation, home string) string {
	var b strings.Builder

	b.WriteString("## Анализ диска macOS\n\n")

	if disk != nil {
		b.WriteString(fmt.Sprintf("**Диск:** %s всего, %s использовано (%.0f%%), %s свободно\n\n",
			ui.FormatSize(disk.Total),
			ui.FormatSize(disk.Used),
			float64(disk.Used)/float64(disk.Total)*100,
			ui.FormatSize(disk.Free),
		))
	}

	b.WriteString("### Топ директорий по размеру:\n")
	for i, e := range top {
		if i >= 15 {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, displayPath(e.Path, home), ui.FormatSize(e.Size)))
	}

	b.WriteString("\n### Известные macOS локации:\n")
	for _, loc := range locs {
		if !loc.Exists || loc.Size <= 0 {
			continue
		}
		cleanable := ""
		if loc.Cleanable {
			cleanable = " [можно очистить]"
		}
		b.WriteString(fmt.Sprintf("- **%s**: %s%s\n  Путь: %s\n",
			loc.Name, ui.FormatSize(loc.Size), cleanable, displayPath(loc.Path, home)))
	}

	b.WriteString("\nДай конкретные рекомендации по освобождению места, отсортированные по эффекту. ")
	b.WriteString("Для каждого пункта укажи ожидаемый объём освобождаемого места и точную команду или действие.")

	return b.String()
}

// runInteractiveCleanup shows an interactive cleanup menu.
func runInteractiveCleanup(locs []macos.KnownLocation, home string) {
	// Build cleanup actions from cleanable locations with significant size
	type action struct {
		name    string
		size    int64
		path    string
		cleanFn func() error
		note    string
	}

	var actions []action
	for _, loc := range locs {
		if !loc.Exists || !loc.Cleanable || loc.Size < 10*1024*1024 {
			continue
		}
		fn := loc.CleanFn
		path := loc.Path
		note := loc.CleanNote
		if fn == nil {
			fn = func() error {
				return removeAllContents(path)
			}
		}
		actions = append(actions, action{
			name:    loc.Name,
			size:    loc.Size,
			path:    path,
			cleanFn: fn,
			note:    note,
		})
	}

	if len(actions) == 0 {
		fmt.Println("\n  Нет подходящих для очистки директорий.")
		return
	}

	// Sort by size desc
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].size > actions[j].size
	})

	ui.Header("ОЧИСТКА")
	totalCleanable := int64(0)
	for i, a := range actions {
		ui.PrintCleanAction(i+1, a.name, a.size)
		if a.note != "" {
			fmt.Printf("       \033[2m%s\033[0m\n", a.note)
		}
		totalCleanable += a.size
	}
	fmt.Printf("\n  Потенциально освободить: \033[1m%s\033[0m\n", ui.FormatSize(totalCleanable))

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nВыберите действия (например: 1,3 или all, q для выхода): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "" {
		fmt.Println("Отмена.")
		return
	}

	var selected []int
	if strings.ToLower(input) == "all" {
		for i := range actions {
			selected = append(selected, i)
		}
	} else {
		for _, part := range strings.Split(input, ",") {
			part = strings.TrimSpace(part)
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(actions) {
				fmt.Printf("  Пропускаю некорректный номер: %q\n", part)
				continue
			}
			selected = append(selected, n-1)
		}
	}

	if len(selected) == 0 {
		fmt.Println("Ничего не выбрано.")
		return
	}

	// Confirm
	fmt.Printf("\nБудет очищено:\n")
	totalSize := int64(0)
	for _, idx := range selected {
		a := actions[idx]
		fmt.Printf("  • %s (%s)\n", a.name, ui.FormatSize(a.size))
		totalSize += a.size
	}
	fmt.Printf("Итого: %s\n", ui.FormatSize(totalSize))
	fmt.Printf("\033[1mПродолжить? [y/N]: \033[0m")

	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "y" && confirm != "да" {
		fmt.Println("Отмена.")
		return
	}

	// Execute
	fmt.Println()
	for _, idx := range selected {
		a := actions[idx]
		fmt.Printf("  Очистка %s... ", a.name)
		if err := a.cleanFn(); err != nil {
			fmt.Printf("\033[31mошибка: %v\033[0m\n", err)
		} else {
			fmt.Printf("\033[32mготово\033[0m\n")
		}
	}
	fmt.Printf("\n  ✓ Очистка завершена\n")
}

// removeAllContents removes the contents of a directory (not the directory itself).
func removeAllContents(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return nil
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

	ui.Header("РЕКОМЕНДАЦИИ")
	total := int64(0)
	for _, loc := range tips {
		fmt.Printf("  • %-26s  %s  —  %s\n",
			loc.Name, ui.FormatSize(loc.Size), loc.CleanNote)
		total += loc.Size
	}
	fmt.Printf("\n  Потенциально освободить: \033[1m%s\033[0m\n", ui.FormatSize(total))
	fmt.Printf("\n  Запустите с \033[1m-i\033[0m для интерактивной очистки")
	fmt.Printf(" или \033[1m--llm\033[0m для анализа с помощью ИИ.\n")
}

// displayPath shortens a path by replacing the home directory with ~.
func displayPath(path, home string) string {
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

// printLLMCost prints token usage and estimated cost after an LLM call.
func printLLMCost(usage *llm.Usage, model string) {
	fmt.Printf("\n  \033[2m— Токены: %d вход + %d выход = %d итого\033[0m",
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)

	if cost, ok := usage.Cost(model); ok {
		if cost < 0.001 {
			fmt.Printf("  \033[2m| стоимость: <$0.001\033[0m")
		} else {
			fmt.Printf("  \033[2m| стоимость: $%.4f\033[0m", cost)
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

