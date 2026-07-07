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
	"github.com/charmbracelet/lipgloss"

	"github.com/skrashevich/godiskanal/i18n"
	"github.com/skrashevich/godiskanal/llm"
	"github.com/skrashevich/godiskanal/macos"
	"github.com/skrashevich/godiskanal/ui"
)

// ─── Layout constants ────────────────────────────────────────────────────────

const (
	barWidth  = 20
	sizeWidth = 9

	minSideWidth = 35 // minimum useful panel width
	maxSideWidth = 52 // maximum panel width
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

// ─── Messages ────────────────────────────────────────────────────────────────

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

// autoDescMsg carries a debounced auto-description from LLM.
type autoDescMsg struct {
	reqID int
	text  string
	err   error
}

// analysisMsg carries a deep content-analysis result from LLM (triggered by 'i').
type analysisMsg struct {
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
	parentSize int64

	// Selection for deletion
	marked        map[string]int64
	markedRelated map[string]string // related Library path → parent .app path
	homes         []string          // user homes for app-related file discovery

	// Pre-computed sizes from scanner (path → bytes)
	sizeCache map[string]int64

	// Dimensions
	width  int
	height int

	// UI state
	mode    viewMode
	loading bool
	status  string

	// LLM side panel
	llmClient          *llm.Client
	llmKnownHints      []KnownHint
	llmPanelText       string
	llmPanelLoading    bool
	llmPanelIsAnalysis bool
	llmDescReqID       int
}

// New creates a new browser model. llmClient may be nil to disable LLM features.
// knownHints enriches LLM prompts when the current path matches a scanner-known location.
func New(root string, sizeCache map[string]int64, llmClient *llm.Client, knownHints []KnownHint, homes []string) Model {
	if len(homes) == 0 {
		homes = macos.TargetHomes(macos.ResolveTargetHome(""))
	}
	m := Model{
		root:          root,
		current:       root,
		marked:        make(map[string]int64),
		markedRelated: make(map[string]string),
		homes:         homes,
		sizeCache:     sizeCache,
		llmClient:     llmClient,
		llmKnownHints: knownHints,
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
		if de.Type()&os.ModeSymlink != 0 {
			continue
		}
		full := filepath.Join(path, de.Name())
		var size int64
		if de.IsDir() {
			size = cache[full]
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
			m.status = i18n.Tf("browser.error", msg.err)
			return m, nil
		}
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
		// Auto-describe the first entry in the side panel
		return m, m.autoDescCmd()

	case deletedMsg:
		m.mode = modeNormal
		m.markedRelated = make(map[string]string)
		if msg.err != nil {
			m.status = i18n.Tf("browser.delete_error", msg.err)
		} else {
			m.status = i18n.Tf("browser.delete_done",
				len(msg.paths), ui.FormatSize(msg.freed))
			m.marked = make(map[string]int64)
			for _, p := range msg.paths {
				delete(m.sizeCache, p)
			}
		}
		m.loading = true
		return m, m.cmdLoad()

	case autoDescMsg:
		// Ignore stale requests (cursor moved while LLM was running)
		if msg.reqID == m.llmDescReqID {
			m.llmPanelLoading = false
			m.llmPanelIsAnalysis = false
			if msg.err != nil {
				m.llmPanelText = "—"
			} else {
				m.llmPanelText = msg.text
			}
		}
		return m, nil

	case analysisMsg:
		m.llmPanelLoading = false
		m.llmPanelIsAnalysis = true
		if msg.err != nil {
			m.llmPanelText = i18n.Tf("browser.error", msg.err)
		} else {
			m.llmPanelText = msg.text
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.mode == modeHelp {
		m.mode = modeNormal
		return m, nil
	}

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
			m.markedRelated = make(map[string]string)
			m.status = i18n.T("browser.delete_cancel")
		}
		return m, nil
	}

	if m.mode == modeDeleting {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Normal mode
	listH := m.listHeight()
	cursorMoved := false

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
			cursorMoved = true
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.offset+listH {
				m.offset++
			}
			cursorMoved = true
		}

	case "pgup":
		old := m.cursor
		m.cursor = max(0, m.cursor-listH)
		m.offset = max(0, m.offset-listH)
		cursorMoved = m.cursor != old

	case "pgdown":
		if n := len(m.entries); n > 0 {
			old := m.cursor
			m.cursor = min(n-1, m.cursor+listH)
			if m.cursor >= m.offset+listH {
				m.offset = m.cursor - listH + 1
			}
			cursorMoved = m.cursor != old
		}

	case "home", "g":
		cursorMoved = m.cursor != 0
		m.cursor = 0
		m.offset = 0

	case "end", "G":
		if n := len(m.entries); n > 0 {
			cursorMoved = m.cursor != n-1
			m.cursor = n - 1
			m.offset = max(0, n-listH)
		}

	case "enter", "right", "l":
		if len(m.entries) > 0 && m.entries[m.cursor].IsDir {
			m.stack = append(m.stack, m.current)
			m.current = m.entries[m.cursor].Path
			m.loading = true
			m.status = ""
			m.llmPanelText = ""
			m.llmPanelLoading = false
			return m, m.cmdLoad()
		}

	case "esc", "backspace", "left", "h":
		if len(m.stack) > 0 {
			m.current = m.stack[len(m.stack)-1]
			m.stack = m.stack[:len(m.stack)-1]
			m.loading = true
			m.status = ""
			m.llmPanelText = ""
			m.llmPanelLoading = false
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
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor >= m.offset+listH {
					m.offset++
				}
				cursorMoved = true
			}
		}

	case "d":
		if len(m.marked) > 0 {
			m.prepareDeleteConfirm()
		} else if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.marked = map[string]int64{e.Path: e.Size}
			m.prepareDeleteConfirm()
		}

	case "D":
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.marked = map[string]int64{e.Path: e.Size}
			m.prepareDeleteConfirm()
		}

	case "i":
		// Deep analysis: list contents and ask for cleanup advice
		if m.llmClient != nil && len(m.entries) > 0 {
			e := m.entries[m.cursor]
			m.llmPanelLoading = true
			m.llmPanelText = ""
			m.llmPanelIsAnalysis = true
			return m, cmdAnalyze(m.llmClient, e.Path, e.Size, e.IsDir, m.sizeCache, m.llmKnownHints)
		}

	case "I":
		// Clear panel, return to auto-desc of current entry
		m.llmPanelText = ""
		m.llmPanelIsAnalysis = false
		return m, m.autoDescCmd()
	}

	// Fire auto-description when cursor has moved
	if cursorMoved {
		return m, m.autoDescCmd()
	}

	return m, nil
}

// autoDescCmd increments the debounce counter and returns a command that
// fetches a brief LLM description of the entry under the cursor.
func (m *Model) autoDescCmd() tea.Cmd {
	if m.llmClient == nil || len(m.entries) == 0 || !m.hasSidePanel() {
		return nil
	}
	m.llmDescReqID++
	m.llmPanelLoading = true
	m.llmPanelText = ""
	m.llmPanelIsAnalysis = false
	return cmdAutoDesc(m.llmClient, m.entries[m.cursor], m.llmDescReqID, m.entries, m.current, m.sizeCache, m.llmKnownHints)
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

	header := m.renderHeader()
	footer := m.renderFooter()

	var content string
	if m.loading {
		content = i18n.T("browser.loading")
	} else if m.hasSidePanel() {
		sideW := m.sidePanelWidth()
		lw := m.listW()
		listH := m.listHeight()
		leftLines := strings.Split(m.renderListFixed(lw, listH), "\n")
		rightLines := m.renderPanelLines(sideW, listH)
		content = mergeColumns(leftLines, lw, rightLines) + "\n"
	} else {
		content = m.renderList()
	}

	return header + "\n" + content + footer
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

// ── file list ─────────────────────────────────────────────────────────────────

func (m Model) renderList() string {
	return m.renderListFixed(m.width, m.listHeight())
}

func (m Model) renderListFixed(w, h int) string {
	var b strings.Builder
	end := min(m.offset+h, len(m.entries))
	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderEntry(i, w))
		b.WriteString("\n")
	}
	// Fill empty rows so the column has fixed height
	for i := end - m.offset; i < h; i++ {
		b.WriteString("\n")
	}
	// Remove trailing newline (mergeColumns adds its own)
	s := b.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

func (m Model) renderEntry(i int, totalW int) string {
	e := m.entries[i]
	selected := i == m.cursor
	_, marked := m.marked[e.Path]

	cursor := "  "
	if selected {
		cursor = styleBold.Render("► ")
	}

	sizeStr := fmt.Sprintf("%*s", sizeWidth, ui.FormatSize(e.Size))
	sizeStr = colorSize(e.Size, sizeStr)

	bar := makeBar(e.Size, m.parentSize, barWidth)

	mark := " "
	if marked {
		mark = styleMarked.Render("*")
	}
	dirInd := "  "
	if e.IsDir {
		dirInd = "▸ "
	}

	name := e.Name
	if e.IsDir {
		name += "/"
	}
	// cursor(2) + size(sizeWidth) + sp(1) + bar(barWidth) + sp(2) + mark(1) + dirInd(2)
	fixedCols := 2 + sizeWidth + 1 + barWidth + 2 + 1 + 2
	nameW := totalW - fixedCols
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

// ── right panel ───────────────────────────────────────────────────────────────

// renderPanelLines returns a slice of strings (one per line) for the right panel.
// Each line is already styled; display width may exceed panelW due to ANSI codes.
func (m Model) renderPanelLines(panelW, maxH int) []string {
	var lines []string

	if len(m.entries) == 0 {
		return lines
	}
	e := m.entries[m.cursor]

	// ── entry header ──
	icon := "  "
	if e.IsDir {
		icon = "▸ "
	}
	nameRunes := []rune(e.Name)
	if e.IsDir && len(nameRunes) < panelW-3 {
		nameRunes = append(nameRunes, '/')
	}
	maxName := panelW - 4
	if maxName < 4 {
		maxName = 4
	}
	if len(nameRunes) > maxName {
		nameRunes = append(nameRunes[:maxName-1], '…')
	}
	lines = append(lines, styleBold.Render(icon+string(nameRunes)))
	lines = append(lines, styleDim.Render("  "+ui.FormatSize(e.Size)))
	lines = append(lines, styleSep.Render(strings.Repeat("─", panelW)))

	// ── label ──
	if m.llmPanelIsAnalysis {
		lines = append(lines, styleLLM.Render(i18n.T("browser.panel.analysis")))
	} else {
		lines = append(lines, styleDim.Render(i18n.T("browser.panel.description")))
	}

	// ── content ──
	if m.llmPanelLoading {
		lines = append(lines, styleLLM.Render(i18n.T("browser.panel.thinking")))
	} else if m.llmPanelText != "" {
		lines = append(lines, formatLLMPanelLines(m.llmPanelText, panelW)...)
	} else {
		lines = append(lines, styleDim.Render("  —"))
	}

	// ── hint at bottom ──
	if !m.llmPanelIsAnalysis && m.llmClient != nil {
		// Pad with empty lines, then show hint
		for len(lines) < maxH-1 {
			lines = append(lines, "")
		}
		if len(lines) < maxH {
			lines = append(lines, styleDim.Render(i18n.T("browser.panel.hint")))
		}
	}

	// Trim to maxH
	if len(lines) > maxH {
		lines = lines[:maxH]
	}

	return lines
}

// ── footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter() string {
	sep := styleSep.Render(strings.Repeat("─", m.width))

	var hints string
	if m.hasSidePanel() {
		hints = styleDim.Render(i18n.T("browser.footer.full"))
	} else {
		parts := i18n.T("browser.footer.base")
		if m.llmClient != nil {
			parts += i18n.T("browser.footer.desc")
		}
		parts += i18n.T("browser.footer.quit")
		hints = styleDim.Render(parts)
	}

	var sb strings.Builder
	sb.WriteString(sep + "\n" + hints)
	if m.status != "" {
		sb.WriteString("\n" + m.status)
	}
	return sb.String()
}

// ── other modes ───────────────────────────────────────────────────────────────

func (m Model) renderDeleting() string {
	var total int64
	paths := make([]string, 0, len(m.marked))
	for p, s := range m.marked {
		total += s
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString(i18n.Tf("browser.deleting", len(m.marked), ui.FormatSize(total)))
	for _, p := range paths {
		b.WriteString(fmt.Sprintf("    %s\n", p))
	}
	b.WriteString(i18n.T("browser.please_wait"))
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
	b.WriteString(i18n.Tf("browser.confirm", len(m.marked), ui.FormatSize(total)))
	if len(m.markedRelated) > 0 {
		b.WriteString(i18n.T("browser.confirm.app_related"))
	}
	for _, p := range paths {
		line := fmt.Sprintf("    %s  (%s)", p, ui.FormatSize(m.marked[p]))
		if app, ok := m.markedRelated[p]; ok {
			line += i18n.Tf("browser.confirm.related", filepath.Base(app))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(i18n.T("browser.confirm_yes"))
	return b.String()
}

// prepareDeleteConfirm expands .app deletions with related Library files before confirm UI.
func (m *Model) prepareDeleteConfirm() {
	expanded, related := macos.ExpandAppDeleteTargets(m.marked, m.homes, m.sizeCache)
	m.marked = expanded
	m.markedRelated = related
	m.mode = modeConfirm
}

func (m Model) renderHelp() string {
	llmSection := ""
	if m.llmClient != nil {
		llmSection = i18n.T("browser.help.llm")
	}
	return i18n.T("browser.help") + llmSection + i18n.T("browser.help.quit") + i18n.T("browser.help.return")
}

// ─── Geometry helpers ─────────────────────────────────────────────────────────

func (m Model) hasSidePanel() bool {
	return m.llmClient != nil && m.sidePanelWidth() >= minSideWidth
}

func (m Model) sidePanelWidth() int {
	if m.llmClient == nil {
		return 0
	}
	w := m.width / 3
	if w < minSideWidth {
		return 0
	}
	if w > maxSideWidth {
		return maxSideWidth
	}
	return w
}

func (m Model) listW() int {
	if m.hasSidePanel() {
		return m.width - m.sidePanelWidth() - 1 // 1 for "│"
	}
	return m.width
}

func (m Model) listHeight() int {
	footerLines := 2 // sep + hints
	if m.status != "" {
		footerLines++
	}
	h := m.height - 2 - footerLines // 2 for header (title + sep)
	if h < 2 {
		h = 2
	}
	return h
}

// ─── Column merge ─────────────────────────────────────────────────────────────

// mergeColumns joins left and right lines with a separator column.
// Left lines are padded to leftW display columns.
func mergeColumns(left []string, leftW int, right []string) string {
	maxH := max(len(left), len(right))
	sep := styleSep.Render("│")
	var b strings.Builder
	for i := 0; i < maxH; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		// Pad left to leftW using ANSI-aware width
		visW := lipgloss.Width(l)
		if visW < leftW {
			l += strings.Repeat(" ", leftW-visW)
		}
		b.WriteString(l)
		b.WriteString(sep)
		b.WriteString(r)
		if i < maxH-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
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

// cmdAutoDesc fires after a short debounce delay and returns a brief description.
// It includes directory contents and sibling context for more useful LLM responses.
func cmdAutoDesc(client *llm.Client, entry DirEntry, reqID int, siblings []DirEntry, parentPath string, sizeCache map[string]int64, hints []KnownHint) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(450 * time.Millisecond) // debounce: ignore stale requests
		kind := i18n.T("browser.llm.file")
		if entry.IsDir {
			kind = i18n.T("browser.llm.dir")
		}

		var sb strings.Builder
		sb.WriteString(i18n.Tf("browser.llm.auto_header",
			parentPath, entry.Name, kind, ui.FormatSize(entry.Size)))
		appendKnownHint(&sb, entry.Path, hints)

		// Add sibling context (top entries in the same directory)
		if len(siblings) > 0 {
			sb.WriteString(i18n.T("browser.llm.auto_siblings"))
			for i, s := range siblings {
				if i >= 7 {
					break
				}
				marker := "  "
				if s.Path == entry.Path {
					marker = "► "
				}
				suffix := ""
				if s.IsDir {
					suffix = "/"
				}
				sb.WriteString(fmt.Sprintf("  %s%-35s %s\n",
					marker, s.Name+suffix, ui.FormatSize(s.Size)))
			}
		}

		// Add children for directories
		if entry.IsDir {
			children, err := os.ReadDir(entry.Path)
			if err == nil && len(children) > 0 {
				type item struct {
					name  string
					size  int64
					isDir bool
				}
				var items []item
				for _, c := range children {
					if c.Type()&os.ModeSymlink != 0 {
						continue
					}
					cp := filepath.Join(entry.Path, c.Name())
					var s int64
					if c.IsDir() {
						s = sizeCache[cp]
					} else if info, err2 := c.Info(); err2 == nil {
						s = info.Size()
					}
					items = append(items, item{c.Name(), s, c.IsDir()})
				}
				sort.Slice(items, func(i, j int) bool {
					return items[i].size > items[j].size
				})
				sb.WriteString(i18n.T("browser.llm.auto_children"))
				for i, it := range items {
					if i >= 8 {
						sb.WriteString(i18n.Tf("browser.llm.more_entries", len(items)-8))
						break
					}
					suffix := ""
					if it.isDir {
						suffix = "/"
					}
					sb.WriteString(fmt.Sprintf("  %-35s %s\n",
						it.name+suffix, ui.FormatSize(it.size)))
				}
			}
		}

		sb.WriteString(i18n.T("browser.llm.auto_ask"))

		text, err := client.DescribeAuto(sb.String())
		return autoDescMsg{reqID: reqID, text: text, err: err}
	}
}

// cmdAnalyze reads directory contents and asks LLM for cleanup advice.
func cmdAnalyze(client *llm.Client, path string, size int64, isDir bool, sizeCache map[string]int64, hints []KnownHint) tea.Cmd {
	return func() tea.Msg {
		var sb strings.Builder
		kind := i18n.T("browser.llm.file")
		if isDir {
			kind = i18n.T("browser.llm.dir")
		}
		sb.WriteString(i18n.Tf("browser.llm.analyze",
			path, kind, ui.FormatSize(size)))
		appendKnownHint(&sb, path, hints)

		if isDir {
			entries, err := os.ReadDir(path)
			if err == nil && len(entries) > 0 {
				type item struct {
					name  string
					size  int64
					isDir bool
				}
				var items []item
				for _, e := range entries {
					if e.Type()&os.ModeSymlink != 0 {
						continue
					}
					ep := filepath.Join(path, e.Name())
					var s int64
					if e.IsDir() {
						s = sizeCache[ep]
					} else if info, err2 := e.Info(); err2 == nil {
						s = info.Size()
					}
					items = append(items, item{e.Name(), s, e.IsDir()})
				}
				sort.Slice(items, func(i, j int) bool {
					return items[i].size > items[j].size
				})
				sb.WriteString(i18n.T("browser.llm.contents"))
				for i, it := range items {
					if i >= 15 {
						break
					}
					suffix := ""
					if it.isDir {
						suffix = "/"
					}
					sb.WriteString(fmt.Sprintf("  %-38s %s\n",
						it.name+suffix, ui.FormatSize(it.size)))
				}
			}
		}

		sb.WriteString(i18n.T("browser.llm.questions"))

		text, err := client.DescribeAnalyze(sb.String())
		return analysisMsg{text: text, err: err}
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
	case size >= 10<<30:
		return sizeStyleRed.Render(s)
	case size >= 1<<30:
		return sizeStyleYellow.Render(s)
	case size >= 50<<20:
		return sizeStyleCyan.Render(s)
	default:
		return sizeStyleGreen.Render(s)
	}
}

func formatLLMPanelLines(text string, panelW int) []string {
	formatted, err := ui.RenderMarkdownWidth(text, panelW)
	if err != nil {
		return strings.Split(wrapText(text, panelW-3, "  "), "\n")
	}
	formatted = strings.TrimRight(formatted, "\n")
	if formatted == "" {
		return nil
	}
	return strings.Split(formatted, "\n")
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
