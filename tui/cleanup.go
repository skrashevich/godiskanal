package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/ui"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type cleanTickMsg struct{}

func cleanTick() tea.Cmd {
	return tea.Every(120*time.Millisecond, func(time.Time) tea.Msg { return cleanTickMsg{} })
}

// ─── Cleanup-specific types ───────────────────────────────────────────────────

type cleanMode int

const (
	cleanSelect   cleanMode = iota // browsing + marking items
	cleanConfirm                   // "are you sure?" dialog
	cleanRunning                   // executing cleanup actions
	cleanDone                      // finished, showing results
)

type cleanEntry struct {
	loc      macos.KnownLocation
	selected bool
	cleaning bool // currently running
	cleaned  bool
	err      error
}

type cleanItemDoneMsg struct {
	idx int
	err error
}

// ─── CleanupModel ─────────────────────────────────────────────────────────────

// CleanupModel is the bubbletea model for the interactive cleanup selector.
type CleanupModel struct {
	items   []cleanEntry
	maxSize int64

	cursor       int
	offset       int
	width        int
	height       int
	mode         cleanMode
	spinnerFrame int

	// CommandOnly items excluded from TUI — listed separately for info
	commandOnly []macos.KnownLocation
}

// NewCleanup creates a cleanup TUI model from populated KnownLocations.
// Items with Cleanable=false or that don't exist are excluded.
// CommandOnly items (Docker, iOS sims) are listed separately for info.
func NewCleanup(locs []macos.KnownLocation) CleanupModel {
	var items []cleanEntry
	var cmdOnly []macos.KnownLocation
	var maxSize int64
	for _, loc := range locs {
		if !loc.Cleanable || !loc.Exists {
			continue
		}
		if loc.CommandOnly {
			cmdOnly = append(cmdOnly, loc)
			continue
		}
		items = append(items, cleanEntry{loc: loc})
		if loc.Size > maxSize {
			maxSize = loc.Size
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].loc.Size > items[j].loc.Size
	})
	return CleanupModel{
		items:       items,
		commandOnly: cmdOnly,
		maxSize:     maxSize,
		width:       80,
		height:      24,
	}
}

// HasItems reports whether there is anything to clean.
func (m CleanupModel) HasItems() bool { return len(m.items) > 0 }

// ─── Init / Update ────────────────────────────────────────────────────────────

func (m CleanupModel) Init() tea.Cmd { return nil }

func (m CleanupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case cleanTickMsg:
		if m.mode == cleanRunning {
			m.spinnerFrame++
			return m, cleanTick()
		}
		return m, nil

	case cleanItemDoneMsg:
		m.items[msg.idx].cleaning = false
		m.items[msg.idx].cleaned = true
		m.items[msg.idx].err = msg.err
		// Launch next pending item
		for i, it := range m.items {
			if it.selected && !it.cleaned {
				m.items[i].cleaning = true
				return m, cmdCleanEntry(i, it.loc)
			}
		}
		m.mode = cleanDone
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m CleanupModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ── confirm dialog ────────────────────────────────────────────────────────
	if m.mode == cleanConfirm {
		switch key {
		case "y", "Y":
			m.mode = cleanRunning
			for i, it := range m.items {
				if it.selected {
					m.items[i].cleaning = true
					return m, tea.Batch(cmdCleanEntry(i, it.loc), cleanTick())
				}
			}
			m.mode = cleanDone // nothing selected somehow
		default:
			m.mode = cleanSelect
		}
		return m, nil
	}

	// ── running: block all except hard quit ───────────────────────────────────
	if m.mode == cleanRunning {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// ── done: any key exits ───────────────────────────────────────────────────
	if m.mode == cleanDone {
		return m, tea.Quit
	}

	// ── select mode ───────────────────────────────────────────────────────────
	listH := m.listHeight()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			if m.cursor >= m.offset+listH {
				m.offset++
			}
		}

	case "pgup":
		m.cursor = max(0, m.cursor-listH)
		m.offset = max(0, m.offset-listH)

	case "pgdown":
		if n := len(m.items); n > 0 {
			m.cursor = min(n-1, m.cursor+listH)
			if m.cursor >= m.offset+listH {
				m.offset = m.cursor - listH + 1
			}
		}

	case "home", "g":
		m.cursor, m.offset = 0, 0

	case "end", "G":
		if n := len(m.items); n > 0 {
			m.cursor = n - 1
			m.offset = max(0, n-listH)
		}

	case " ":
		if len(m.items) > 0 {
			m.items[m.cursor].selected = !m.items[m.cursor].selected
			// Advance cursor
			if m.cursor < len(m.items)-1 {
				m.cursor++
				if m.cursor >= m.offset+listH {
					m.offset++
				}
			}
		}

	case "a", "A":
		allOn := true
		for _, it := range m.items {
			if !it.selected {
				allOn = false
				break
			}
		}
		for i := range m.items {
			m.items[i].selected = !allOn
		}

	case "enter", "c", "C":
		for _, it := range m.items {
			if it.selected {
				m.mode = cleanConfirm
				break
			}
		}
	}

	return m, nil
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m CleanupModel) View() string {
	hdr := m.renderHeader()
	switch m.mode {
	case cleanConfirm:
		return hdr + "\n" + m.renderConfirm()
	case cleanRunning:
		return hdr + "\n" + m.renderRunning()
	case cleanDone:
		return hdr + "\n" + m.renderDone()
	}
	return hdr + "\n" + m.renderList() + m.renderFooter()
}

// ── header ───────────────────────────────────────────────────────────────────

func (m CleanupModel) renderHeader() string {
	title := styleHeader.Render("godiskanal — Интерактивная очистка")

	var selCount int
	var selSize int64
	for _, it := range m.items {
		if it.selected {
			selCount++
			selSize += it.loc.Size
		}
	}
	sel := ""
	if selCount > 0 {
		sel = styleMarked.Render(fmt.Sprintf("  [*%d  %s]", selCount, ui.FormatSize(selSize)))
	}

	sep := styleSep.Render(strings.Repeat("─", m.width))
	return title + sel + "\n" + sep
}

// ── select list ───────────────────────────────────────────────────────────────

func (m CleanupModel) renderList() string {
	var b strings.Builder
	listH := m.listHeight()

	// Reserve 1 line for CleanNote of the current item (if any)
	noteVisible := m.cursor >= 0 && m.cursor < len(m.items) &&
		m.items[m.cursor].loc.CleanNote != ""
	rowH := listH
	if noteVisible {
		rowH = listH - 1
	}
	if rowH < 1 {
		rowH = 1
	}
	end := min(m.offset+rowH, len(m.items))

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderEntry(i))
		b.WriteString("\n")
	}
	for i := end - m.offset; i < rowH; i++ {
		b.WriteString("\n")
	}

	// CleanNote line
	if noteVisible {
		note := m.items[m.cursor].loc.CleanNote
		noteStr := "  " + styleDim.Render("↳ "+truncate(note, m.width-6))
		b.WriteString(noteStr + "\n")
	}

	return b.String()
}

func (m CleanupModel) renderEntry(i int) string {
	it := m.items[i]
	isCursor := i == m.cursor

	// Cursor (2 chars)
	cur := "  "
	if isCursor {
		cur = styleBold.Render("► ")
	}

	// Checkbox (4 chars including trailing space)
	check := "[ ] "
	if it.selected {
		check = styleMarked.Render("[✓]") + " "
	}

	// Size (sizeWidth, right-aligned)
	sz := fmt.Sprintf("%*s", sizeWidth, ui.FormatSize(it.loc.Size))
	sz = colorSize(it.loc.Size, sz)

	// Bar
	bar := makeBar(it.loc.Size, m.maxSize, barWidth)

	// Name (remaining width)
	name := it.loc.Name
	// cur(2) + check(4) + size(9) + sp(1) + bar(20) + sp(2)
	fixedCols := 2 + 4 + sizeWidth + 1 + barWidth + 2
	nameW := m.width - fixedCols
	if nameW < 4 {
		nameW = 4
	}
	name = truncate(name, nameW)
	if isCursor {
		name = styleBold.Render(name)
	}

	return fmt.Sprintf("%s%s%s %s  %s", cur, check, sz, bar, name)
}

// ── footer ────────────────────────────────────────────────────────────────────

func (m CleanupModel) renderFooter() string {
	sep := styleSep.Render(strings.Repeat("─", m.width))
	hints := styleDim.Render(
		"↑↓/jk нав.  Space выбрать  a все/сброс  Enter очистить  q выйти",
	)
	return sep + "\n" + hints
}

// ── confirm ───────────────────────────────────────────────────────────────────

func (m CleanupModel) renderConfirm() string {
	var total int64
	var lines []string
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		total += it.loc.Size
		lines = append(lines,
			fmt.Sprintf("    • %-28s %s", it.loc.Name, ui.FormatSize(it.loc.Size)))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  Очистить %d элемент(ов) (%s)?\n\n",
		len(lines), ui.FormatSize(total)))
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n  [y] Да, очистить  [любая другая клавиша] Отмена\n")
	return b.String()
}

// ── running ───────────────────────────────────────────────────────────────────

func (m CleanupModel) renderRunning() string {
	var b strings.Builder
	b.WriteString("\n")
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		icon, status := m.runningItemStatus(it)
		sz := fmt.Sprintf("%9s", ui.FormatSize(it.loc.Size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(it.loc.Name, 28), status))
	}
	return b.String()
}

func (m CleanupModel) runningItemStatus(it cleanEntry) (icon, status string) {
	switch {
	case it.err != nil:
		return "✗", styleDim.Render("ошибка: "+it.err.Error())
	case it.cleaned:
		return "✓", styleDim.Render("готово")
	case it.cleaning:
		spin := spinFrames[m.spinnerFrame%len(spinFrames)]
		return spin, styleDim.Render("очищаю...")
	default:
		return "○", styleDim.Render("ожидание")
	}
}

// ── done ──────────────────────────────────────────────────────────────────────

func (m CleanupModel) renderDone() string {
	var b strings.Builder
	b.WriteString("\n")

	var freed int64
	var errs int
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		icon, st := m.runningItemStatus(it)
		sz := fmt.Sprintf("%9s", ui.FormatSize(it.loc.Size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(it.loc.Name, 28), st))
		if it.err != nil {
			errs++
		} else {
			freed += it.loc.Size
		}
	}

	b.WriteString(fmt.Sprintf("\n  Освобождено: %s", ui.FormatSize(freed)))
	if errs > 0 {
		b.WriteString(fmt.Sprintf("  (%d ошибок)", errs))
	}
	// Show CommandOnly items that need manual cleanup
	if len(m.commandOnly) > 0 {
		b.WriteString("\n\n  Требуют ручной очистки (запустите команду):\n")
		for _, loc := range m.commandOnly {
			b.WriteString(fmt.Sprintf("    • %-20s  %s\n",
				loc.Name, styleDim.Render(loc.CleanNote)))
		}
	}
	b.WriteString("\n  Нажмите любую клавишу для выхода\n")
	return b.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (m CleanupModel) listHeight() int {
	h := m.height - 4 // header(2) + footer(2)
	if h < 2 {
		h = 2
	}
	return h
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxW {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxW-1]) + "…"
}

func cmdCleanEntry(idx int, loc macos.KnownLocation) tea.Cmd {
	return func() tea.Msg {
		var err error
		// CommandOnly items must use CleanFn (Docker, iOS simulators).
		// All other items use direct path removal — avoids exec PATH issues.
		if loc.CommandOnly && loc.CleanFn != nil {
			err = loc.CleanFn()
		} else if loc.Path != "" {
			err = removeContents(loc.Path)
		} else if loc.CleanFn != nil {
			err = loc.CleanFn()
		}
		return cleanItemDoneMsg{idx: idx, err: err}
	}
}

func removeContents(path string) error {
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
