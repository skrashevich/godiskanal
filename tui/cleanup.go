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

	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/ui"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func cleanTick() tea.Cmd {
	return tea.Every(120*time.Millisecond, func(time.Time) tea.Msg { return cleanTickMsg{} })
}

// ─── Modes ────────────────────────────────────────────────────────────────────

type cleanMode int

const (
	cleanSelect       cleanMode = iota
	cleanDrill                  // browsing directory contents
	cleanDrillConfirm           // confirm drill selection
	cleanDrillRunning           // running drill cleanup
	cleanDrillDone              // drill cleanup complete → returns to cleanSelect
	cleanConfirm                // confirm main-list selection
	cleanRunning                // running main-list cleanup
	cleanDone                   // main-list cleanup complete
)

// ─── Message types ────────────────────────────────────────────────────────────

type cleanTickMsg struct{}

type cleanItemDoneMsg struct {
	idx int
	err error
}

type drillLoadedMsg struct {
	entries []drillEntry
	maxSize int64
}

type drillItemDoneMsg struct {
	idx int
	err error
}

// ─── Entry types ──────────────────────────────────────────────────────────────

type cleanEntry struct {
	loc      macos.KnownLocation
	selected bool
	cleaning bool
	cleaned  bool
	err      error
}

type drillEntry struct {
	name     string
	path     string
	size     int64 // -1 = unknown
	isDir    bool
	selected bool
	cleaning bool
	cleaned  bool
	err      error
}

// ─── CleanupModel ─────────────────────────────────────────────────────────────

type CleanupModel struct {
	// main list
	items   []cleanEntry
	maxSize int64
	cursor  int
	offset  int

	width        int
	height       int
	mode         cleanMode
	spinnerFrame int

	commandOnly []macos.KnownLocation
	sizeMap     map[string]int64

	// drill-down state
	drillParentName string
	drillParentPath string
	drillEntries    []drillEntry
	drillMaxSize    int64
	drillCursor     int
	drillOffset     int
}

// NewCleanup creates a cleanup TUI model from populated KnownLocations.
// sizeMap is the scanner size cache used for directory sizes in drill-down.
func NewCleanup(locs []macos.KnownLocation, sizeMap map[string]int64) CleanupModel {
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
	if sizeMap == nil {
		sizeMap = map[string]int64{}
	}
	return CleanupModel{
		items:       items,
		commandOnly: cmdOnly,
		maxSize:     maxSize,
		sizeMap:     sizeMap,
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
		if m.mode == cleanRunning || m.mode == cleanDrillRunning {
			m.spinnerFrame++
			return m, cleanTick()
		}
		return m, nil

	// ── main list cleanup events ───────────────────────────────────────────
	case cleanItemDoneMsg:
		m.items[msg.idx].cleaning = false
		m.items[msg.idx].cleaned = true
		m.items[msg.idx].err = msg.err
		for i, it := range m.items {
			if it.selected && !it.cleaned {
				m.items[i].cleaning = true
				return m, cmdCleanEntry(i, it.loc)
			}
		}
		m.mode = cleanDone
		return m, nil

	// ── drill-down events ──────────────────────────────────────────────────
	case drillLoadedMsg:
		m.drillEntries = msg.entries
		m.drillMaxSize = msg.maxSize
		m.drillCursor = 0
		m.drillOffset = 0
		// transition to drill mode only after load completes
		m.mode = cleanDrill
		return m, nil

	case drillItemDoneMsg:
		m.drillEntries[msg.idx].cleaning = false
		m.drillEntries[msg.idx].cleaned = true
		m.drillEntries[msg.idx].err = msg.err
		for i, e := range m.drillEntries {
			if e.selected && !e.cleaned {
				m.drillEntries[i].cleaning = true
				return m, cmdCleanDrillEntry(i, e.path)
			}
		}
		m.mode = cleanDrillDone
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m CleanupModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ── drill confirm ──────────────────────────────────────────────────────
	if m.mode == cleanDrillConfirm {
		switch key {
		case "y", "Y":
			m.mode = cleanDrillRunning
			for i, e := range m.drillEntries {
				if e.selected {
					m.drillEntries[i].cleaning = true
					return m, tea.Batch(cmdCleanDrillEntry(i, e.path), cleanTick())
				}
			}
			m.mode = cleanDrillDone
		default:
			m.mode = cleanDrill
		}
		return m, nil
	}

	// ── drill running: block all except hard quit ──────────────────────────
	if m.mode == cleanDrillRunning {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// ── drill done: any key → back to main list ────────────────────────────
	if m.mode == cleanDrillDone {
		m.mode = cleanSelect
		m.drillEntries = nil
		return m, nil
	}

	// ── drill select ───────────────────────────────────────────────────────
	if m.mode == cleanDrill {
		return m.handleDrillKey(key)
	}

	// ── main confirm ───────────────────────────────────────────────────────
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
			m.mode = cleanDone
		default:
			m.mode = cleanSelect
		}
		return m, nil
	}

	// ── main running: block all except hard quit ───────────────────────────
	if m.mode == cleanRunning {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// ── main done: any key exits ───────────────────────────────────────────
	if m.mode == cleanDone {
		return m, tea.Quit
	}

	// ── main select mode ───────────────────────────────────────────────────
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

	case "enter":
		// Drill into the item under cursor (if it has a path to browse)
		if len(m.items) > 0 {
			it := m.items[m.cursor]
			if it.loc.Path != "" {
				m.drillParentName = it.loc.Name
				m.drillParentPath = it.loc.Path
				// Show loading immediately; drillLoadedMsg will transition to cleanDrill
				m.mode = cleanDrill
				m.drillEntries = nil
				return m, cmdLoadDrillEntries(it.loc.Path, m.sizeMap)
			}
		}

	case "c", "C":
		// Confirm bulk cleanup of selected items
		for _, it := range m.items {
			if it.selected {
				m.mode = cleanConfirm
				break
			}
		}
	}

	return m, nil
}

func (m CleanupModel) handleDrillKey(key string) (tea.Model, tea.Cmd) {
	listH := m.drillListHeight()

	switch key {
	case "q", "esc", "backspace", "left", "h":
		m.mode = cleanSelect
		m.drillEntries = nil
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.drillCursor > 0 {
			m.drillCursor--
			if m.drillCursor < m.drillOffset {
				m.drillOffset = m.drillCursor
			}
		}

	case "down", "j":
		if m.drillCursor < len(m.drillEntries)-1 {
			m.drillCursor++
			if m.drillCursor >= m.drillOffset+listH {
				m.drillOffset++
			}
		}

	case "pgup":
		m.drillCursor = max(0, m.drillCursor-listH)
		m.drillOffset = max(0, m.drillOffset-listH)

	case "pgdown":
		if n := len(m.drillEntries); n > 0 {
			m.drillCursor = min(n-1, m.drillCursor+listH)
			if m.drillCursor >= m.drillOffset+listH {
				m.drillOffset = m.drillCursor - listH + 1
			}
		}

	case "home", "g":
		m.drillCursor, m.drillOffset = 0, 0

	case "end", "G":
		if n := len(m.drillEntries); n > 0 {
			m.drillCursor = n - 1
			m.drillOffset = max(0, n-listH)
		}

	case " ":
		if len(m.drillEntries) > 0 {
			m.drillEntries[m.drillCursor].selected = !m.drillEntries[m.drillCursor].selected
			if m.drillCursor < len(m.drillEntries)-1 {
				m.drillCursor++
				if m.drillCursor >= m.drillOffset+listH {
					m.drillOffset++
				}
			}
		}

	case "a", "A":
		allOn := true
		for _, e := range m.drillEntries {
			if !e.selected {
				allOn = false
				break
			}
		}
		for i := range m.drillEntries {
			m.drillEntries[i].selected = !allOn
		}

	case "enter", "c", "C", "d", "D":
		for _, e := range m.drillEntries {
			if e.selected {
				m.mode = cleanDrillConfirm
				break
			}
		}
	}

	return m, nil
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m CleanupModel) View() string {
	switch m.mode {
	case cleanDrill:
		if m.drillEntries == nil {
			// Still loading
			hdr := m.renderDrillHeader()
			sep := styleSep.Render(strings.Repeat("─", m.width))
			spin := spinFrames[m.spinnerFrame%len(spinFrames)]
			return hdr + "\n" + spin + i18n.T("cleanup.loading") + "\n" + sep
		}
		return m.renderDrillHeader() + "\n" + m.renderDrillList() + m.renderDrillFooter()

	case cleanDrillConfirm:
		return m.renderDrillHeader() + "\n" + m.renderDrillConfirm()

	case cleanDrillRunning:
		return m.renderDrillHeader() + "\n" + m.renderDrillRunning()

	case cleanDrillDone:
		return m.renderDrillHeader() + "\n" + m.renderDrillDone()

	case cleanConfirm:
		hdr := m.renderHeader()
		return hdr + "\n" + m.renderConfirm()

	case cleanRunning:
		hdr := m.renderHeader()
		return hdr + "\n" + m.renderRunning()

	case cleanDone:
		hdr := m.renderHeader()
		return hdr + "\n" + m.renderDone()
	}

	hdr := m.renderHeader()
	return hdr + "\n" + m.renderList() + m.renderFooter()
}

// ── main list header ──────────────────────────────────────────────────────────

func (m CleanupModel) renderHeader() string {
	title := styleHeader.Render(i18n.T("cleanup.header"))

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

// ── main list ─────────────────────────────────────────────────────────────────

func (m CleanupModel) renderList() string {
	var b strings.Builder
	listH := m.listHeight()

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

	if noteVisible {
		note := m.items[m.cursor].loc.CleanNote
		b.WriteString("  " + styleDim.Render("↳ "+truncate(note, m.width-6)) + "\n")
	}
	return b.String()
}

func (m CleanupModel) renderEntry(i int) string {
	it := m.items[i]
	isCursor := i == m.cursor

	cur := "  "
	if isCursor {
		cur = styleBold.Render("► ")
	}

	check := "[ ] "
	if it.selected {
		check = styleMarked.Render("[✓]") + " "
	}

	sz := fmt.Sprintf("%*s", sizeWidth, ui.FormatSize(it.loc.Size))
	sz = colorSize(it.loc.Size, sz)
	bar := makeBar(it.loc.Size, m.maxSize, barWidth)

	name := it.loc.Name
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

// ── main footer ───────────────────────────────────────────────────────────────

func (m CleanupModel) renderFooter() string {
	sep := styleSep.Render(strings.Repeat("─", m.width))
	hints := styleDim.Render(i18n.T("cleanup.footer"))
	return sep + "\n" + hints
}

// ── main confirm ──────────────────────────────────────────────────────────────

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
	b.WriteString(i18n.Tf("cleanup.confirm", len(lines), ui.FormatSize(total)))
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteString(i18n.T("cleanup.confirm_yes"))
	return b.String()
}

// ── main running ──────────────────────────────────────────────────────────────

func (m CleanupModel) renderRunning() string {
	var b strings.Builder
	b.WriteString("\n")
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		icon, status := m.runningItemStatus(it.cleaning, it.cleaned, it.err)
		sz := fmt.Sprintf("%9s", ui.FormatSize(it.loc.Size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(it.loc.Name, 28), status))
	}
	return b.String()
}

// ── main done ─────────────────────────────────────────────────────────────────

func (m CleanupModel) renderDone() string {
	var b strings.Builder
	b.WriteString("\n")

	var freed int64
	var errs int
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		icon, st := m.runningItemStatus(it.cleaning, it.cleaned, it.err)
		sz := fmt.Sprintf("%9s", ui.FormatSize(it.loc.Size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(it.loc.Name, 28), st))
		if it.err != nil {
			errs++
		} else {
			freed += it.loc.Size
		}
	}

	b.WriteString(i18n.Tf("cleanup.freed", ui.FormatSize(freed)))
	if errs > 0 {
		b.WriteString(i18n.Tf("cleanup.errors", errs))
	}
	if len(m.commandOnly) > 0 {
		b.WriteString(i18n.T("cleanup.manual"))
		for _, loc := range m.commandOnly {
			b.WriteString(fmt.Sprintf("    • %-20s  %s\n",
				loc.Name, styleDim.Render(loc.CleanNote)))
		}
	}
	b.WriteString(i18n.T("cleanup.exit"))
	return b.String()
}

// ── drill header ──────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillHeader() string {
	title := styleHeader.Render(fmt.Sprintf("godiskanal — %s", m.drillParentName))

	var selCount int
	var selSize int64
	for _, e := range m.drillEntries {
		if e.selected {
			selCount++
			if e.size > 0 {
				selSize += e.size
			}
		}
	}
	sel := ""
	if selCount > 0 {
		sel = styleMarked.Render(fmt.Sprintf("  [*%d  %s]", selCount, ui.FormatSize(selSize)))
	}

	pathStr := styleDim.Render(truncate(m.drillParentPath, m.width-2))
	sep := styleSep.Render(strings.Repeat("─", m.width))
	return title + sel + "\n" + "  " + pathStr + "\n" + sep
}

// ── drill list ────────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillList() string {
	var b strings.Builder
	listH := m.drillListHeight()
	end := min(m.drillOffset+listH, len(m.drillEntries))

	for i := m.drillOffset; i < end; i++ {
		b.WriteString(m.renderDrillEntryRow(i))
		b.WriteString("\n")
	}
	for i := end - m.drillOffset; i < listH; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m CleanupModel) renderDrillEntryRow(i int) string {
	e := m.drillEntries[i]
	isCursor := i == m.drillCursor

	cur := "  "
	if isCursor {
		cur = styleBold.Render("► ")
	}

	check := "[ ] "
	if e.selected {
		check = styleMarked.Render("[✓]") + " "
	}

	sz := fmt.Sprintf("%*s", sizeWidth, ui.FormatSize(e.size))
	sz = colorSize(e.size, sz)
	bar := makeBar(e.size, m.drillMaxSize, barWidth)

	name := e.name
	if e.isDir {
		name += "/"
	}
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

// ── drill footer ──────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillFooter() string {
	sep := styleSep.Render(strings.Repeat("─", m.width))
	hints := styleDim.Render(i18n.T("cleanup.drill.footer"))
	return sep + "\n" + hints
}

// ── drill confirm ─────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillConfirm() string {
	var total int64
	var lines []string
	for _, e := range m.drillEntries {
		if !e.selected {
			continue
		}
		sizeStr := ui.FormatSize(e.size)
		if e.size < 0 {
			sizeStr = "   —   "
		} else {
			total += e.size
		}
		lines = append(lines,
			fmt.Sprintf("    • %-28s %s", truncate(e.name, 28), sizeStr))
	}

	var b strings.Builder
	b.WriteString(i18n.Tf("cleanup.drill.confirm", len(lines), ui.FormatSize(total)))
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteString(i18n.T("cleanup.drill.confirm_yes"))
	return b.String()
}

// ── drill running ─────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillRunning() string {
	var b strings.Builder
	b.WriteString("\n")
	for _, e := range m.drillEntries {
		if !e.selected {
			continue
		}
		icon, status := m.runningItemStatus(e.cleaning, e.cleaned, e.err)
		sz := fmt.Sprintf("%9s", ui.FormatSize(e.size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(e.name, 28), status))
	}
	return b.String()
}

// ── drill done ────────────────────────────────────────────────────────────────

func (m CleanupModel) renderDrillDone() string {
	var b strings.Builder
	b.WriteString("\n")

	var freed int64
	var errs int
	for _, e := range m.drillEntries {
		if !e.selected {
			continue
		}
		icon, st := m.runningItemStatus(e.cleaning, e.cleaned, e.err)
		sz := fmt.Sprintf("%9s", ui.FormatSize(e.size))
		b.WriteString(fmt.Sprintf("  %s  %s  %-28s  %s\n",
			icon, sz, truncate(e.name, 28), st))
		if e.err != nil {
			errs++
		} else if e.size > 0 {
			freed += e.size
		}
	}

	b.WriteString(i18n.Tf("cleanup.drill.freed", ui.FormatSize(freed)))
	if errs > 0 {
		b.WriteString(i18n.Tf("cleanup.errors", errs))
	}
	b.WriteString(i18n.T("cleanup.drill.back"))
	return b.String()
}

// ── shared status ─────────────────────────────────────────────────────────────

func (m CleanupModel) runningItemStatus(cleaning, cleaned bool, err error) (icon, status string) {
	switch {
	case err != nil:
		return "✗", styleDim.Render(i18n.T("cleanup.status.error") + err.Error())
	case cleaned:
		return "✓", styleDim.Render(i18n.T("cleanup.status.done"))
	case cleaning:
		spin := spinFrames[m.spinnerFrame%len(spinFrames)]
		return spin, styleDim.Render(i18n.T("cleanup.status.cleaning"))
	default:
		return "○", styleDim.Render(i18n.T("cleanup.status.waiting"))
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (m CleanupModel) listHeight() int {
	h := m.height - 4 // header(3 with path) + footer(2)
	if h < 2 {
		h = 2
	}
	return h
}

func (m CleanupModel) drillListHeight() int {
	h := m.height - 5 // drill header(3) + footer(2)
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

// ─── Async commands ───────────────────────────────────────────────────────────

// cmdLoadDrillEntries reads the directory and returns sizes from sizeMap (for dirs)
// or os.Stat (for files). Sorting is by size descending.
func cmdLoadDrillEntries(path string, sizeMap map[string]int64) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(path)
		if err != nil {
			return drillLoadedMsg{}
		}

		var items []drillEntry
		var maxSize int64
		for _, e := range entries {
			if e.Type()&os.ModeSymlink != 0 {
				continue
			}
			entryPath := filepath.Join(path, e.Name())
			info, err := e.Info()
			if err != nil {
				continue
			}

			var size int64
			if e.IsDir() {
				if s, ok := sizeMap[entryPath]; ok {
					size = s
				} else {
					size = -1 // unknown — outside scan root or not yet scanned
				}
			} else {
				size = info.Size()
			}

			items = append(items, drillEntry{
				name:  e.Name(),
				path:  entryPath,
				size:  size,
				isDir: e.IsDir(),
			})
			if size > maxSize {
				maxSize = size
			}
		}

		sort.Slice(items, func(i, j int) bool {
			// unknown sizes (-1) go to the bottom
			si, sj := items[i].size, items[j].size
			if si < 0 && sj >= 0 {
				return false
			}
			if sj < 0 && si >= 0 {
				return true
			}
			if si != sj {
				return si > sj
			}
			return items[i].name < items[j].name
		})

		return drillLoadedMsg{entries: items, maxSize: maxSize}
	}
}

func cmdCleanDrillEntry(idx int, path string) tea.Cmd {
	return func() tea.Msg {
		err := os.RemoveAll(path)
		return drillItemDoneMsg{idx: idx, err: err}
	}
}

func cmdCleanEntry(idx int, loc macos.KnownLocation) tea.Cmd {
	return func() tea.Msg {
		var err error
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
