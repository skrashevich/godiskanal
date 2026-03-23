package i18n

var translationsEN = map[string]string{
	// ── Application ──────────────────────────────────────────────────────────
	"app.title": "godiskanal — macOS disk analyzer",

	// ── CLI ──────────────────────────────────────────────────────────────────
	"cmd.short": "macOS disk usage analyzer",
	"cmd.long": `godiskanal is a command-line tool for analyzing disk usage on macOS.

It scans the filesystem in parallel, shows top directories by size,
checks well-known space hogs (Xcode, Docker, package manager caches,
iCloud, etc.) and helps free up space.

With an API key (--llm), it sends data to an OpenAI-compatible LLM
and streams personalized recommendations directly to the terminal.

Scanning:
  • Multi-threaded — uses all CPU cores by default
  • Progress shows current directory, adapts to terminal width
  • Directories not responding within 3 seconds (iCloud, NFS) are skipped
  • Ctrl+C interrupts scanning and outputs partial results

Disk browser (--browse / -b):
  • ncdu-like TUI with directory tree navigation
  • Displays file and folder sizes with visual bars
  • Space — mark for deletion, d — delete marked
  • i — get LLM explanation (if API key is provided)

Interactive cleanup (-i):
  • TUI interface for selecting directories to clean via Space
  • Shows sizes and progress bars, sorted by volume
  • Asks for confirmation before deletion
  • Docker and iOS Simulators require running a command manually

Environment variables:
  OPENAI_API_KEY   API key (if --api-key is not specified)
  OPENAI_BASE_URL  Base API URL (if --api-url is not specified)`,

	// ── Flags ────────────────────────────────────────────────────────────────
	"flag.path":           "Path to scan",
	"flag.top":            "Number of top directories",
	"flag.interactive":    "Interactive cleanup mode",
	"flag.one_filesystem": "Do not cross filesystem boundaries (skip mount points)",
	"flag.exclude":        "Exclude path from scanning (can repeat: --exclude ~/a --exclude ~/b)",
	"flag.min_size":       "Minimum size to display (bytes)",
	"flag.browse":         "Interactive disk browser (TUI) with navigation and deletion",
	"flag.llm":            "Enable LLM analysis with cleanup recommendations",
	"flag.api_key":        "API key (overrides OPENAI_API_KEY)",
	"flag.api_url":        "Base API URL (overrides OPENAI_BASE_URL; default: https://api.openai.com/v1)",
	"flag.model":          "LLM model",

	// ── Errors ───────────────────────────────────────────────────────────────
	"err.generic":     "Error: %v\n",
	"err.disk_info":   "  Warning: could not get disk info: %v\n",
	"err.scan":        "scan error",
	"err.browser":     "browser error",
	"err.cleanup_tui": "cleanup TUI error",
	"err.api_key":     "OpenAI API key required: --api-key flag or OPENAI_API_KEY environment variable",

	// ── Scanning ─────────────────────────────────────────────────────────────
	"scan.header":      "SCANNING",
	"scan.one_fs":      "  \033[2m-x: skip mount points\033[0m",
	"scan.interrupted": "\n  \033[33m⚠ Scan interrupted (Ctrl+C) — results incomplete\033[0m",
	"scan.perm_errors": "  \033[33m⚠ Skipped %d directories (no access)\033[0m",
	"scan.timeouts":    "  \033[33m⚠ Skipped %d directories (timeout — possibly iCloud or network drive)\033[0m",
	"scan.progress":    "  %s %s files | %s  ",
	"scan.done":        "\r  ✓ Scanned %s files | %s | %.1f s",

	// ── Disk ─────────────────────────────────────────────────────────────────
	"disk.header": "DISK",
	"disk.total":  "Total:",
	"disk.used":   "Used:",
	"disk.free":   "Free:",

	// ── Top directories ──────────────────────────────────────────────────────
	"top.header":          "TOP-%d DIRECTORIES",
	"node_modules.header": "NODE_MODULES (%d found)",
	"known.header":        "KNOWN LOCATIONS",

	// ── Time Machine ─────────────────────────────────────────────────────────
	"tm.info":   "\n  \033[36mℹ Time Machine: %d local snapshots\033[0m",
	"tm.delete": "    Delete: \033[1mtmutil deletelocalsnapshots /\033[0m",

	// ── LLM ──────────────────────────────────────────────────────────────────
	"llm.header":    "LLM ANALYSIS",
	"llm.analyzing": "  \033[2mAnalyzing with %s...\033[0m",
	"llm.error":     "\n  \033[31mLLM Error: %v\033[0m",
	"llm.tokens":    "\n  \033[2m— Tokens: %d in + %d out = %d total\033[0m",
	"llm.cost_low":  "  \033[2m| cost: <$0.001\033[0m",
	"llm.cost":      "  \033[2m| cost: $%.4f\033[0m",

	// ── LLM prompts ─────────────────────────────────────────────────────────
	"llm.prompt.header":    "## macOS Disk Analysis\n\n",
	"llm.prompt.disk":      "**Disk:** %s total, %s used (%.0f%%), %s free\n\n",
	"llm.prompt.top_dirs":  "### Top directories by size:\n",
	"llm.prompt.known":     "\n### Known macOS locations:\n",
	"llm.prompt.cleanable": " [cleanable]",
	"llm.prompt.path":      "  Path: %s\n",
	"llm.prompt.request":   "\nProvide specific recommendations for freeing space, sorted by impact. For each item, specify the expected amount of freed space and the exact command or action.",

	"llm.system.describe": "You are a macOS expert. Briefly (2-4 sentences) explain what this path\n(directory or file) is and whether it is safe to delete to free disk space.\nRespond in English. If the path is clearly system-critical, warn about it.",
	"llm.system.analyze": "You are a macOS expert helping users free up disk space.\nAnalyze the disk usage data and provide specific, actionable recommendations.\nPrioritize recommendations by the potential amount of freed space.\nUse markdown: headings, bold text, lists. Respond in English.\nBe specific: provide exact commands and paths for cleanup.",
	"llm.empty_response": "empty response from API",

	// ── Tips ─────────────────────────────────────────────────────────────────
	"tips.header":    "RECOMMENDATIONS",
	"tips.potential": "\n  Potentially free: \033[1m%s\033[0m",
	"tips.run":       "\n  Run with \033[1m-i\033[0m for interactive cleanup or \033[1m--llm\033[0m for AI-powered analysis.\n",

	// ── Cleanup no items ─────────────────────────────────────────────────────
	"cleanup.no_items": "\n  No suitable directories for cleanup.",

	// ── Browser TUI ──────────────────────────────────────────────────────────
	"browser.loading":       "\n  Loading...\n",
	"browser.error":         "Error: %v",
	"browser.delete_error":  "Delete error: %v",
	"browser.delete_done":   "✓ Deleted %d, freed %s",
	"browser.delete_cancel": "Deletion cancelled",

	"browser.panel.analysis":    "  ● Content analysis",
	"browser.panel.description": "  ○ Description",
	"browser.panel.thinking":    "  ⟳ Thinking...",
	"browser.panel.hint":        "  i: content analysis",

	"browser.footer.full":  "↑↓/jk nav.  Enter/→ enter  ←/Esc back  Space mark  d delete  i analyze  q quit  ? help",
	"browser.footer.base":  "↑↓/jk nav.  Enter/→ enter  ←/Esc back  Space mark  d delete",
	"browser.footer.desc":  "  i describe",
	"browser.footer.quit":  "  q quit  ? help",

	"browser.confirm":      "\n  Delete %d item(s) (%s)?\n\n",
	"browser.confirm_yes":  "\n  [y] Yes, delete  [any other] Cancel\n",
	"browser.deleting":     "\n  ⟳ Deleting %d item(s) (%s)...\n\n",
	"browser.please_wait":  "\n  Please wait...\n",

	"browser.help": `
  Disk browser controls
  ────────────────────────────────────────
  ↑ / k          Up
  ↓ / j          Down
  Enter / → / l  Open directory
  Esc / ← / h    Back
  PgUp / PgDn    Page scroll
  g / Home       Go to top
  G / End        Go to bottom
  Space           Mark for deletion
  d               Delete marked (or current)
  D               Delete current item
`,
	"browser.help.llm": `  i               Detailed content analysis
  I               Return to auto-description
`,
	"browser.help.quit":   "  q               Quit",
	"browser.help.return": "\n  Press any key to return",

	// ── Browser LLM prompts ─────────────────────────────────────────────────
	"browser.llm.file":      "file",
	"browser.llm.dir":       "directory",
	"browser.llm.auto":      "Path: %s\nType: %s\nSize: %s\n\nBriefly explain what this is and whether it's safe to delete.",
	"browser.llm.analyze":   "Analysis: %s\nType: %s\nSize: %s\n",
	"browser.llm.contents":  "\nContents (top 15 by size):\n",
	"browser.llm.questions": "\nAnswer in English:\n1. What is this and what is it for?\n2. What can be safely deleted and how much space will be freed?\n3. What are the risks of deletion?\n4. Specific commands or paths for cleanup.",

	// ── Cleanup TUI ──────────────────────────────────────────────────────────
	"cleanup.header":       "godiskanal — Interactive cleanup",
	"cleanup.loading":      " Loading contents...",
	"cleanup.footer":       "↑↓/jk nav.  Space select  a all/reset  Enter details  c clean  q quit",
	"cleanup.confirm":      "\n  Clean %d item(s) (%s)?\n\n",
	"cleanup.confirm_yes":  "\n  [y] Yes, clean  [any other key] Cancel\n",
	"cleanup.freed":        "\n  Freed: %s",
	"cleanup.errors":       "  (%d errors)",
	"cleanup.manual":       "\n\n  Require manual cleanup (run command):\n",
	"cleanup.exit":         "\n  Press any key to exit\n",
	"cleanup.status.error":    "error: ",
	"cleanup.status.done":     "done",
	"cleanup.status.cleaning": "cleaning...",
	"cleanup.status.waiting":  "waiting",

	"cleanup.drill.footer":      "↑↓/jk nav.  Space select  a all/reset  Enter/c delete  Esc back",
	"cleanup.drill.confirm":     "\n  Delete %d item(s) (%s)?\n\n",
	"cleanup.drill.confirm_yes": "\n  [y] Yes, delete  [any other key] Cancel\n",
	"cleanup.drill.freed":       "\n  Freed: ~%s",
	"cleanup.drill.back":        "\n\n  Press any key to return to list\n",

	// ── Known locations ──────────────────────────────────────────────────────
	"loc.App Caches.desc":           "Application caches",
	"loc.App Caches.note":           "Caches are recreated automatically",
	"loc.App Support.desc":          "Application data",
	"loc.App Support.note":          "Delete specific application data manually",
	"loc.Xcode DerivedData.desc":    "Xcode build artifacts",
	"loc.Xcode DerivedData.note":    "Xcode will rebuild when needed",
	"loc.iOS Simulators.desc":       "iOS simulator images",
	"loc.iOS Simulators.note":       "xcrun simctl delete unavailable",
	"loc.iOS Device Support.desc":   "Device debug symbols",
	"loc.iOS Device Support.note":   "Old versions can be removed",
	"loc.iOS Backups.desc":          "iPhone/iPad backups",
	"loc.iOS Backups.note":          "Manage via Finder → device → Backups",
	"loc.iCloud Drive.desc":         "Local copies of iCloud Drive",
	"loc.iCloud Drive.note":         "Manage via System Settings → Apple ID",
	"loc.Downloads.desc":            "Downloads",
	"loc.Downloads.note":            "Review contents manually",
	"loc.Trash.desc":                "Trash",
	"loc.Trash.note":                "Empty the trash",
	"loc.npm cache.desc":            "npm package cache",
	"loc.npm cache.note":            "npm cache clean --force",
	"loc.yarn cache.desc":           "Yarn cache",
	"loc.yarn cache.note":           "yarn cache clean",
	"loc.pnpm store.desc":           "pnpm store",
	"loc.pnpm store.note":           "pnpm store prune",
	"loc.Go modules.desc":           "Go module cache",
	"loc.Go modules.note":           "go clean -modcache",
	"loc.Gradle cache.desc":         "Gradle cache",
	"loc.Gradle cache.note":         "Safe to delete",
	"loc.Maven cache.desc":          "Local Maven repository",
	"loc.Maven cache.note":          "Safe to delete",
	"loc.Rust cargo.desc":           "Rust/Cargo cache",
	"loc.Rust cargo.note":           "cargo cache --autoclean",
	"loc.pip cache.desc":            "Python pip cache",
	"loc.pip cache.note":            "pip cache purge",
	"loc.Go build cache.desc":       "Go build cache",
	"loc.Go build cache.note":       "go clean -cache",
	"loc.Rust toolchains.desc":      "Installed Rust versions (rustup)",
	"loc.Rust toolchains.note":      "rustup toolchain list; rustup toolchain remove <version>",
	"loc.CocoaPods cache.desc":      "CocoaPods cache (iOS/macOS dependencies)",
	"loc.CocoaPods cache.note":      "pod cache clean --all",
	"loc.Node-gyp cache.desc":       "Native Node.js module cache",
	"loc.Node-gyp cache.note":       "Safe to delete, will be recreated",
	"loc.Dart/Flutter pub.desc":     "Dart/Flutter package cache",
	"loc.Dart/Flutter pub.note":     "dart pub cache clean",
	"loc.NuGet packages.desc":       ".NET / NuGet package cache",
	"loc.NuGet packages.note":       "dotnet nuget locals all --clear",
	"loc.PlatformIO.desc":           "PlatformIO toolchains and libraries (embedded dev)",
	"loc.PlatformIO.note":           "pio system prune",
	"loc.Bun packages.desc":         "Bun cache and global packages",
	"loc.Bun packages.note":         "bun pm cache rm",
	"loc.HuggingFace models.desc":   "Local HuggingFace models",
	"loc.HuggingFace models.note":   "huggingface-cli delete-cache or delete models manually",
	"loc.Whisper models.desc":       "OpenAI Whisper models (speech recognition)",
	"loc.Whisper models.note":       "Will be re-downloaded on next run",
	"loc.uv cache.desc":             "uv package manager cache (Python)",
	"loc.uv cache.note":             "uv cache clean",
	"loc.Continue AI index.desc":    "Continue AI code search index",
	"loc.Continue AI index.note":    "Recreated automatically when VS Code opens",
	"loc.VS Code extensions.desc":   "Visual Studio Code extensions",
	"loc.VS Code extensions.note":   "Remove unused extensions in VS Code",
	"loc.Python venv (~/.venv).desc": "Python virtual environment",
	"loc.Python venv (~/.venv).note": "Delete if not needed, recreate with python -m venv",
	"loc.Puppeteer Chromium.desc":   "Chromium for Puppeteer (browser testing)",
	"loc.Puppeteer Chromium.note":   "Will be re-downloaded on next Puppeteer use",
	"loc.Electron cache.desc":       "Electron SDK cache",
	"loc.Electron cache.note":       "Will be re-downloaded when building Electron apps",
	"loc.Wine.desc":                 "Wine data (running Windows applications)",
	"loc.Wine.note":                 "Contains Windows application data, delete with caution",
	"loc.Safari cache.desc":         "Safari disk cache",
	"loc.Safari cache.note":         "Cache is recreated automatically when browsing",
	"loc.Chrome cache.desc":         "Google Chrome disk cache",
	"loc.Chrome cache.note":         "Cache is recreated automatically",
	"loc.Telegram.desc":             "Telegram Desktop data (including media cache)",
	"loc.Telegram.note":             "Clear media cache via Settings → Privacy → Storage",
	"loc.Telegram (App Store).desc": "Telegram from Mac App Store data",
	"loc.Telegram (App Store).note": "Clear media cache via Settings → Privacy → Storage",
	"loc.Firefox cache.desc":            "Firefox disk cache",
	"loc.Firefox cache.note":            "Cache is recreated automatically",
	"loc.Firefox cache (alt).desc":      "Firefox disk cache (alternate location)",
	"loc.Firefox cache (alt).note":      "Cache is recreated automatically",
	"loc.Slack.desc":                    "Slack data (including cache and logs)",
	"loc.Slack.note":                    "Clear cache via Slack → Help → Reset App",
	"loc.Discord.desc":                  "Discord data (cache and local storage)",
	"loc.Discord.note":                  "Clear cache via Discord Settings → Advanced",
	"loc.Spotify cache.desc":            "Spotify streaming cache",
	"loc.Spotify cache.note":            "Cache is recreated automatically when streaming",
	"loc.Zoom.desc":                     "Zoom meetings data and cache",
	"loc.Zoom.note":                     "Remove old recordings manually",
	"loc.Steam.desc":                    "Steam client and game data",
	"loc.Steam.note":                    "Manage games via Steam → Library",
	"loc.Xcode Archives.desc":          "Xcode app archives (.xcarchive)",
	"loc.Xcode Archives.note":          "Old archives can be removed via Xcode → Organizer",
	"loc.Xcode Previews.desc":          "SwiftUI preview build artifacts",
	"loc.Xcode Previews.note":          "Safe to delete, rebuilt on next preview",
	"loc.Android SDK.desc":             "Android SDK, emulators, and build tools",
	"loc.Android SDK.note":             "Manage via Android Studio → SDK Manager",
	"loc.JetBrains caches.desc":        "IntelliJ/WebStorm/GoLand/PyCharm caches",
	"loc.JetBrains caches.note":        "Safe to delete, IDEs will rebuild caches",
	"loc.Composer cache.desc":           "PHP Composer package cache",
	"loc.Composer cache.note":           "composer clear-cache",
	"loc.Ruby gems.desc":                "Ruby gems (global installations)",
	"loc.Ruby gems.note":                "gem cleanup (removes old versions)",
	"loc.Cypress cache.desc":            "Cypress test runner binaries",
	"loc.Cypress cache.note":            "npx cypress cache clear",
	"loc.Playwright browsers.desc":      "Playwright browser binaries (Chromium, Firefox, WebKit)",
	"loc.Playwright browsers.note":      "Will be re-downloaded on next npx playwright install",
	"loc.Ollama models.desc":            "Local LLM models for Ollama",
	"loc.Ollama models.note":            "ollama rm <model> to remove specific models",
	"loc.PyTorch Hub.desc":              "PyTorch Hub cached models and checkpoints",
	"loc.PyTorch Hub.note":              "Will be re-downloaded when needed",
	"loc.Poetry cache.desc":             "Python Poetry package cache",
	"loc.Poetry cache.note":             "poetry cache clear --all .",
	"loc.Application logs.desc":         "Application log files",
	"loc.Application logs.note":         "Old logs can be safely removed",
	"loc.Saved Application State.desc":  "Saved window positions and states for apps",
	"loc.Saved Application State.note":  "Safe to delete, apps will recreate on next launch",
	"loc.Mail Downloads.desc":           "Apple Mail downloaded attachments",
	"loc.Mail Downloads.note":           "Copies of email attachments, safe to clear",
	"loc.Cursor extensions.desc":        "Cursor IDE extensions",
	"loc.Cursor extensions.note":        "Remove unused extensions in Cursor",
	"loc.Conda.desc":                    "Conda/Anaconda package cache (pkgs/)",
	"loc.Conda.note":                    "conda clean --all",
	"loc.Docker.desc":               "Docker images and data",
	"loc.Docker.note":               "docker system prune -a --volumes",
	"loc.Homebrew cache.desc":       "Homebrew cache",
	"loc.Homebrew cache.note":       "brew cleanup",
}
