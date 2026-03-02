package ui

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// ANSI escape codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// FormatSize converts bytes to a human-readable string.
func FormatSize(bytes int64) string {
	if bytes < 0 {
		return "   —   "
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func sizeColor(bytes int64) string {
	gb := float64(bytes) / (1 << 30)
	switch {
	case gb >= 10:
		return red
	case gb >= 1:
		return yellow
	case gb >= 0.05:
		return cyan
	default:
		return green
	}
}

func sizeBar(size, maxSize int64, width int) string {
	if maxSize <= 0 || size <= 0 {
		return strings.Repeat("░", width)
	}
	ratio := float64(size) / float64(maxSize)
	filled := int(math.Round(ratio * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Header prints a bold section header.
func Header(title string) {
	fmt.Printf("\n%s%s%s\n", bold, title, reset)
	fmt.Println(strings.Repeat("─", 60))
}

// PrintDiskUsage shows disk total/used/free.
func PrintDiskUsage(total, used, free int64) {
	Header("ДИСК")
	fmt.Printf("  Всего:        %s%s%s\n", bold, FormatSize(total), reset)

	usedPct := 0.0
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}
	color := green
	if usedPct > 85 {
		color = red
	} else if usedPct > 60 {
		color = yellow
	}
	barStr := sizeBar(used, total, 28)
	fmt.Printf("  Использовано: %s%s  %s  %.0f%%%s\n",
		color, FormatSize(used), barStr, usedPct, reset)
	fmt.Printf("  Свободно:     %s%s%s\n", green, FormatSize(free), reset)
}

// PrintTopEntry prints a single entry in the top directories list.
func PrintTopEntry(rank int, displayPath string, size, maxSize int64) {
	color := sizeColor(size)
	barStr := sizeBar(size, maxSize, 20)
	fmt.Printf("  %s%2d.%s %s%-9s%s  %s%s%s  %s\n",
		dim, rank, reset,
		color, FormatSize(size), reset,
		color, barStr, reset,
		displayPath,
	)
}

// PrintKnownEntry prints a known macOS location entry.
func PrintKnownEntry(cleanable bool, name, displayPath string, size int64) {
	icon := " "
	color := ""
	sizeStr := FormatSize(size)
	if size < 0 {
		sizeStr = "   —   "
	} else if cleanable && size > 50*1024*1024 { // >50MB
		icon = "!"
		color = sizeColor(size)
	}
	fmt.Printf("  [%s] %s%-9s%s  %-26s  %s\n",
		icon, color, sizeStr, reset, name, displayPath)
}

// PrintCleanAction prints a cleanup action in the interactive menu.
func PrintCleanAction(n int, name string, size int64) {
	color := sizeColor(size)
	fmt.Printf("  %s%2d.%s  %s%-9s%s  %s\n",
		dim, n, reset,
		color, FormatSize(size), reset,
		name,
	)
}

// SpinnerFrame returns the current spinner character.
func SpinnerFrame(n int) string {
	return spinnerFrames[n%len(spinnerFrames)]
}

// PrintScanProgress prints the scanning progress line (overwrites previous).
// The current directory is truncated to fit the terminal width dynamically.
func PrintScanProgress(frame int, files, bytes int64, currentDir string) {
	prefix := fmt.Sprintf("  %s %s файлов | %s  ",
		SpinnerFrame(frame),
		FormatComma(files),
		FormatSize(bytes),
	)
	pathWidth := termWidth() - utf8.RuneCountInString(prefix)
	if pathWidth < 10 {
		pathWidth = 10
	}
	truncated := truncatePath(currentDir, pathWidth)
	// \033[K erases from cursor to end of line — no padding needed
	fmt.Printf("\r%s%s%s%s\033[K", prefix, dim, truncated, reset)
}

// truncatePath shortens a path to fit within width columns by keeping the tail.
func truncatePath(path string, width int) string {
	runes := []rune(path)
	if len(runes) <= width {
		return path
	}
	return "…" + string(runes[len(runes)-(width-1):])
}

// termWidth returns the current terminal column width, defaulting to 80.
func termWidth() int {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	); errno == 0 && ws.Col > 0 {
		return int(ws.Col)
	}
	return 80
}

// PrintScanDone prints the final scan result line.
func PrintScanDone(files, bytes int64, elapsed float64) {
	fmt.Printf("\r  ✓ Просканировано %s файлов | %s | %.1f с%s\n",
		FormatComma(files),
		FormatSize(bytes),
		elapsed,
		strings.Repeat(" ", 10),
	)
}

// FormatComma formats a number with comma separators.
func FormatComma(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
