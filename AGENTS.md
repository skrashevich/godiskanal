# AGENTS.md

## Cursor Cloud specific instructions

This is a Go CLI tool (disk space analyzer). No external services or databases required.

**Build & Run:**
- `go build -o godiskanal .` — builds the binary
- `./godiskanal --path <dir> --top N` — runs a scan on a given directory

**Testing:**
- `go test ./...` — runs all tests (38 tests across 5 packages)
- `go vet ./...` — static analysis / linting

**Notes:**
- The tool is designed for macOS, but builds and runs on Linux. macOS-specific features (Homebrew cache, Time Machine, iOS Simulators) gracefully degrade on Linux.
- The `--llm` flag requires an `OPENAI_API_KEY` env var or `--api-key` flag; this is optional and not needed for core scanning/browsing/cleanup functionality.
- TUI modes (`-b` for browser, `-i` for interactive cleanup) require a terminal; they use Bubble Tea and will not work in non-TTY contexts.
- No Makefile, Dockerfile, or CI config exists; the standard Go toolchain is the only build dependency.
