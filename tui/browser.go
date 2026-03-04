package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skrashevich/godiskanal/llm"
	"github.com/skrashevich/godiskanal/ui"
)

// ─── Layout constants ────────────────────────────────────────────────────────

const (
	barWidth  = 20
	sizeWidth = 9
)

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().Bold(true)
	styleSep    = lipgloss.NewStyle().Faint(true)
	styleBold   = lipgloss.NewStyle().Bold(true)
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleMarked = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	styleLLM    = lipgloss.NewStyle().Foreground(lipgloss.Color("147"))

	sizeStyleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	sizeStyleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	sizeStyleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	sizeStyleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Faint(true)
)

// ─── Types ───────────────────────────────────────────────────────────────────

type viewMode int

const (
	modeNormal viewMode = iota
	modeConfirm
	modeDeleting
	modeHelp
)

// DirEntry is a single filesystem entry shown in the browser.
type DirEntry struct {
	Name  string
	Path  string
	Size  int64
	IsDir bool
}

// ─── Messages ─────────────────────────────────────────────────────────────────

type loadedMsg struct {
	path    string
	entries []DirEntry
	err     error
}

type deletedMsg struct {
	paths []string
	freed int64
	err   error
}

type llmResultMsg struct {
	text string
	err  error
}

// ─── Model ───────────────────────────────────────────────────────────────────

// Model is the bubbletea model for the disk browser.
type Model struct {
	// Navigation
	root    string
	current string
	stack   []string

	// Content
	entries    []DirEntry
	cursor     int
	offset     int
	parentSize int64 // total size of current dir (for relative bars)

	// Selection for deletion
	marked map[string]int64

	// Pre-computed sizes from scanner (path → bytes)
	sizeCache map[string]int64

	// Dimensions
	width  int
	height int

	// UI state
	mode    viewMode
	loading bool
	status  string

	// LLM panel
	llmClient  *llm.Client
	llmText    string
	llmLoading bool
	showLLM    bool
}

// New creates a new browser model. llmClient may be nil to disable LLM features.
func New(root string, sizeCache map[string]int64, llmClient *llm.Client) Model {
	m := Model{
		root:      root,
		current:   root,
		marked:    make(map[string]int64),
		sizeCache: sizeCache,
		llmClient: llmClient,
		width:     80,
		height:    24,
		loading:   true,
	}
	if s, ok := sizeCache[root]; ok {
		m.parentSize = s
	}
	return m
}

// ─── Init ─────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return m.cmdLoad()
}

func (m Model) cmdLoad() tea.Cmd {
	path := m.current
	cache := m.sizeCache
	return func() tea.Msg {
		entries, err := readDirEntries(path, cache)
		return loadedMsg{path: path, entries: entries, err: err}
	}
}

func readDirEntries(path string, cache map[string]int64) ([]DirEntry, error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]DirEntry, 0, len(des))
	for _, de := range des {
		// Skip symlinks — sizes not tracked
		if de.Type()&os.ModeSymlink != 0 {
			continue
		}
		full := filepath.Join(path, de.Name())
		var size int64
		if de.IsDir() {
			size = cache[full] // 0 if not scanned
		} else {
			if info, err2 := de.Info(); err2 == nil {
				size = info.Size()
			}
		}
		entries = append(entries, DirEntry{
			Name:  de.Name(),
			Path:  full,
			Size:  size,
			IsDir: de.IsDir(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})
	return entries, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Ошибка: %v", msg.err)
		} else {
			m.entries = msg.entries
			m.cursor = 0
			m.offset = 0
			if s, ok := m.sizeCache[m.current]; ok {
				m.parentSize = s
			} else {
				var total int64
				for _, e := range msg.entries {
					total += e.Size
				}
				m.parentSize = total
			}
		}
		return m, nil

	case deletedMsg:
		m.mode = modeNormal
		if msg.err != nil {
			m.status = fmt.Sprintf("Ошибка удаления: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("✓ Удалено %d, освобождено %s",
				len(msg.paths), ui.FormatSize(msg.freed))
			m.marked = make(map[string]int64)
			for _, p := range msg.paths {
				delete(m.sizeCache, p)
			}
		}
		m.loading = true
		return m, m.cmdLoad()

	case llmResultMsg:
		m.llmLoading = false
		if msg.err != nil {
			m.llmText = fmt.Sprintf("Ошибка: %v", msg.err)
		} else {
			m.llmText = msg.text
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Help overlay: any key closes
	if m.mode == modeHelp {
		m.mode = modeNormal
		return m, nil
	}

	// Confirm deletion dialog
	if m.mode == modeConfirm {
		switch key {
		case "y", "Y":
			paths := make([]string, 0, len(m.marked))
			sizes := make(map[string]int64, len(m.marked))
			for p, s := range m.marked {
				paths = append(paths, p)
				sizes[p] = s
			}
			m.mode = modeDeleting
			return m, cmdDelete(paths, sizes)
		default:
			m.mode = modeNormal
			m.status = "Удаление отменено"
		}
		return m, nil
	}

	// While deleting — ignore all keys except quit
	if m.mode == modeDeleting {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Normal mode
	listH := m.listHeight()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.mode = modeHelp

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.offset+listH {
				m.offset++
			}
		}

	case "pgup":
		m.cursor = max(0, m.cursor-listH)
		m.offset = max(0, m.offset-listH)

	case "pgdown":
		if n := len(m.entries); n > 0 {
			m.cursor = min(n-1, m.cursor+listH)
			if m.cursor >= m.offset+listH {
				m.offset = m.cursor - listH + 1
			}
		}

	case "home", "g":
		m.cursor = 0
		m.offset = 0

	case "end", "G":
		if n := len(m.entries); n > 0 {
			m.cursor = n - 1
			m.offset = max(0, n-listH)
		}

	case "enter", "right", "l":
		if len(m.entries) > 0 && m.entries[m.cursor].IsDir {
			m.stack = append(m.stack, m.current)
			m.current = m.entries[m.cursor].Path
			m.loading = true
			m.status = ""
			m.showLLM = false
			m.llmText = ""
			return m, m.cmdLoad()
		}

	case "esc", "backspace", "left", "h":
		if len(m.stack) > 0 {
			m.current = m.stack[len(m.stack)-1]
			m.stack = m.stack[:len(m.stack)-1]
			m.loading = true
			m.status = ""
			m.showLLM = false
			m.llmText = ""
			return m, m.cmdLoad()
		}

	case " ":
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			if _, ok := m.marked[e.Path]; ok {
				delete(m.marked, e.Path)
			} else {
				m.marked[e.Path] = e.Size
			}
			// Advance cursor
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor >= m.offset+listH {
					m.offset++
				}
			}
		}

	case "d":
		if len(m.marked) > 0 {
			m.mode = modeConfirm
		} else if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.marked = map[string]int64{e.Path: e.Size}
			m.mode = modeConfirm
		}

	case "D":
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.marked = map[string]int64{e.Path: e.Size}
			m.mode = modeConfirm
		}

	case "i":
		if m.llmClient != nil && len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.showLLM = true
			m.llmLoading = true
			m.llmText = ""
			return m, cmdDescribe(m.llmClient, e.Path, e.Size, e.IsDir, m.entries)
		}

	case "I":
		m.showLLM = false
		m.llmText = ""
	}

	return m, nil
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.mode == modeHelp {
		return m.renderHelp()
	}
	if m.mode == modeConfirm {
		return m.renderHeader() + "\n" + m.renderConfirm()
	}
	if m.mode == modeDeleting {
		return m.renderHeader() + "\n" + m.renderDeleting()
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	if m.loading {
		b.WriteString("\n  Загрузка...\n")
	} else {
		b.WriteString(m.renderList())
	}

	b.WriteString(m.renderFooter())
	return b.String()
}

func (m Model) renderHeader() string {
	path := m.current
	maxPathLen := m.width - 30
	if maxPathLen > 10 && utf8.RuneCountInString(path) > maxPathLen {
		runes := []rune(path)
		path = "…" + string(runes[len(runes)-maxPathLen+1:])
	}

	title := styleHeader.Render("godiskanal — " + path)

	markedStr := ""
	if len(m.marked) > 0 {
		var total int64
		for _, s := range m.marked {
			total += s
		}
		markedStr = styleMarked.Render(
			fmt.Sprintf("  [*%d  %s]", len(m.marked), ui.FormatSize(total)),
		)
	}

	sep := styleSep.Render(strings.Repeat("─", m.width))
	return title + markedStr + "\n" + sep
}

func (m Model) renderList() string {
	var b strings.Builder
	listH := m.listHeight()
	end := min(m.offset+listH, len(m.entries))

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderEntry(i))
		b.WriteString("\n")
	}
	// Fill empty rows
	for i := end - m.offset; i < listH; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderEntry(i int) string {
	e := m.entries[i]
	selected := i == m.cursor
	_, marked := m.marked[e.Path]

	// Cursor (2 chars)
	cursor := "  "
	if selected {
		cursor = styleBold.Render("► ")
	}

	// Size (sizeWidth chars, right-aligned)
	sizeStr := fmt.Sprintf("%*s", sizeWidth, ui.FormatSize(e.Size))
	sizeStr = colorSize(e.Size, sizeStr)

	// Bar (barWidth chars)
	bar := makeBar(e.Size, m.parentSize, barWidth)

	// Mark + type indicator (3 chars)
	mark := " "
	if marked {
		mark = styleMarked.Render("*")
	}
	dirInd := "  "
	if e.IsDir {
		dirInd = "▸ "
	}

	// Name (remaining width)
	name := e.Name
	if e.IsDir {
		name += "/"
	}

	// cursor(2) + size(sizeWidth) + sp(1) + bar(barWidth) + sp(2) + mark(1) + dirInd(2)
	fixedCols := 2 + sizeWidth + 1 + barWidth + 2 + 1 + 2
	nameW := m.width - fixedCols
	if nameW < 4 {
		nameW = 4
	}
	if utf8.RuneCountInString(name) > nameW {
		runes := []rune(name)
		name = string(runes[:nameW-1]) + "…"
	}
	if selected {
		name = styleBold.Render(name)
	}

	return fmt.Sprintf("%s%s %s  %s%s%s",
		cursor, sizeStr, bar, mark, dirInd, name)
}

func (m Model) renderFooter() string {
	sep := styleSep.Render(strings.Repeat("─", m.width))

	hintParts := "↑↓/jk nav  Enter/→ open  ←/Esc back  Space mark  d delete"
	if m.llmClient != nil {
		hintParts += "  i explain"
	}
	hintParts += "  q quit  ? help"
	hints := styleDim.Render(hintParts)

	var sb strings.Builder
	sb.WriteString(sep + "\n" + hints)

	if m.status != "" {
		sb.WriteString("\n" + m.status)
	}

	if m.showLLM {
		sb.WriteString("\n" + styleSep.Render(strings.Repeat("─", m.width)))
		if m.llmLoading {
			sb.WriteString("\n" + styleLLM.Render("  ⟳ LLM думает..."))
		} else if m.llmText != "" {
			sb.WriteString("\n" + styleLLM.Render(wrapText(m.llmText, m.width-4, "  ")))
		}
	}

	return sb.String()
}

func (m Model) renderDeleting() string {
	var total int64
	paths := make([]string, 0, len(m.marked))
	for p, s := range m.marked {
		total += s
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  ⟳ Удаление %d элемент(ов) (%s)...\n\n",
		len(m.marked), ui.FormatSize(total)))
	for _, p := range paths {
		b.WriteString(fmt.Sprintf("    %s\n", p))
	}
	b.WriteString("\n  Пожалуйста, подождите...\n")
	return b.String()
}

func (m Model) renderConfirm() string {
	var total int64
	paths := make([]string, 0, len(m.marked))
	for p, s := range m.marked {
		total += s
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  Удалить %d элемент(ов) (%s)?\n\n",
		len(m.marked), ui.FormatSize(total)))
	for _, p := range paths {
		b.WriteString(fmt.Sprintf("    %s  (%s)\n", p, ui.FormatSize(m.marked[p])))
	}
	b.WriteString("\n  [y] Да, удалить  [любая другая] Отмена\n")
	return b.String()
}

func (m Model) renderHelp() string {
	return `
  Управление браузером диска
  ────────────────────────────────────────
  ↑ / k          Вверх
  ↓ / j          Вниз
  Enter / → / l  Открыть директорию
  Esc / ← / h    Назад
  PgUp / PgDn    Листать страницами
  g / Home       В начало
  G / End        В конец
  Space           Отметить для удаления
  d               Удалить отмеченные (или текущий)
  D               Удалить текущий элемент
  i               Объяснить через LLM
  I               Закрыть панель LLM
  q               Выйти

  Нажмите любую клавишу для возврата`
}

func (m Model) listHeight() int {
	h := m.height - 4 // header(2 lines) + footer(2 lines)
	if m.status != "" {
		h--
	}
	if m.showLLM {
		// separator + response (up to 3 lines)
		h -= 2
		if m.llmText != "" {
			h -= countWrappedLines(m.llmText, m.width-4)
		}
	}
	if h < 2 {
		h = 2
	}
	return h
}

// ─── Async commands ───────────────────────────────────────────────────────────

func cmdDelete(paths []string, sizes map[string]int64) tea.Cmd {
	return func() tea.Msg {
		var freed int64
		var deleted []string
		var lastErr error
		for _, p := range paths {
			if err := os.RemoveAll(p); err != nil {
				lastErr = err
			} else {
				freed += sizes[p]
				deleted = append(deleted, p)
			}
		}
		return deletedMsg{paths: deleted, freed: freed, err: lastErr}
	}
}

func cmdDescribe(client *llm.Client, path string, size int64, isDir bool, siblings []DirEntry) tea.Cmd {
	return func() tea.Msg {
		kind := "файл"
		if isDir {
			kind = "директория"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Путь: %s\nТип: %s\nРазмер: %s\n",
			path, kind, ui.FormatSize(size)))
		// Include top siblings for context
		if len(siblings) > 0 {
			sb.WriteString("\nТоп элементов в родительской директории:\n")
			for i, s := range siblings {
				if i >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("  %s — %s\n", s.Name, ui.FormatSize(s.Size)))
			}
		}
		sb.WriteString("\nЧто это? Стоит ли удалить для экономии места?")
		text, err := client.Describe(sb.String())
		return llmResultMsg{text: text, err: err}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func makeBar(size, total int64, width int) string {
	if total <= 0 {
		return strings.Repeat("░", width)
	}
	n := int(float64(size) / float64(total) * float64(width))
	if n > width {
		n = width
	}
	if n < 0 {
		n = 0
	}
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

func colorSize(size int64, s string) string {
	switch {
	case size >= 10<<30: // ≥10 GB
		return sizeStyleRed.Render(s)
	case size >= 1<<30: // ≥1 GB
		return sizeStyleYellow.Render(s)
	case size >= 50<<20: // ≥50 MB
		return sizeStyleCyan.Render(s)
	default:
		return sizeStyleGreen.Render(s)
	}
}

// wrapText wraps text at maxWidth, prefixing each line with indent.
func wrapText(text string, maxWidth int, indent string) string {
	if maxWidth <= 0 {
		return indent + text
	}
	indentW := utf8.RuneCountInString(indent)
	var b strings.Builder
	lines := strings.Split(text, "\n")
	for li, line := range lines {
		words := strings.Fields(line)
		col := 0
		for _, w := range words {
			wl := utf8.RuneCountInString(w)
			if col == 0 {
				b.WriteString(indent)
				b.WriteString(w)
				col = indentW + wl
			} else if col+1+wl <= maxWidth {
				b.WriteByte(' ')
				b.WriteString(w)
				col += 1 + wl
			} else {
				b.WriteString("\n" + indent)
				b.WriteString(w)
				col = indentW + wl
			}
		}
		if li < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func countWrappedLines(text string, maxWidth int) int {
	if maxWidth <= 0 {
		return 1
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		words := strings.Fields(line)
		col := 0
		for _, w := range words {
			wl := utf8.RuneCountInString(w)
			if col == 0 {
				col = wl
				count++
			} else if col+1+wl <= maxWidth {
				col += 1 + wl
			} else {
				col = wl
				count++
			}
		}
		if len(words) == 0 {
			count++
		}
	}
	return max(count, 1)
}
