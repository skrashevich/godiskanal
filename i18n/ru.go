package i18n

var translationsRU = map[string]string{
	// ── Application ──────────────────────────────────────────────────────────
	"app.title": "godiskanal — анализатор диска macOS",

	// ── CLI ──────────────────────────────────────────────────────────────────
	"cmd.short": "Анализатор использования диска для macOS",
	"cmd.long": `godiskanal — консольная утилита для анализа использования диска на macOS.

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

Браузер диска (--browse / -b):
  • ncdu-подобный TUI с навигацией по дереву директорий
  • Отображает размеры файлов и папок с визуальными барами
  • Пробел — отметить для удаления, d — удалить отмеченное
  • i — получить объяснение от LLM (если передан API ключ)

Интерактивная очистка (-i):
  • TUI-интерфейс с выбором очищаемых директорий через пробел
  • Показывает размеры и прогресс-бары, сортирует по объёму
  • Запрашивает подтверждение перед удалением
  • Docker и iOS Simulators требуют ручного запуска команды

Переменные окружения:
  OPENAI_API_KEY   API ключ (если не указан --api-key)
  OPENAI_BASE_URL  Базовый URL API (если не указан --api-url)`,

	// ── Flags ────────────────────────────────────────────────────────────────
	"flag.path":           "Путь для сканирования",
	"flag.top":            "Количество топ-директорий",
	"flag.interactive":    "Интерактивный режим очистки",
	"flag.one_filesystem": "Не пересекать границы файловых систем (пропускать точки монтирования)",
	"flag.exclude":        "Исключить путь из сканирования (можно повторять: --exclude ~/a --exclude ~/b)",
	"flag.min_size":       "Минимальный размер для отображения (байт)",
	"flag.browse":         "Интерактивный браузер диска (TUI) с навигацией и удалением",
	"flag.llm":            "Включить LLM-анализ с рекомендациями по очистке",
	"flag.api_key":        "API ключ (переопределяет OPENAI_API_KEY)",
	"flag.api_url":        "Базовый URL API (переопределяет OPENAI_BASE_URL; по умолчанию: https://api.openai.com/v1)",
	"flag.model":          "Модель LLM",

	// ── Errors ───────────────────────────────────────────────────────────────
	"err.generic":     "Ошибка: %v\n",
	"err.disk_info":   "  Предупреждение: не удалось получить информацию о диске: %v\n",
	"err.scan":        "ошибка сканирования",
	"err.browser":     "ошибка браузера",
	"err.cleanup_tui": "ошибка TUI очистки",
	"err.api_key":     "требуется OpenAI API ключ: --api-key или переменная окружения OPENAI_API_KEY",

	// ── Scanning ─────────────────────────────────────────────────────────────
	"scan.header":      "СКАНИРОВАНИЕ",
	"scan.one_fs":      "  \033[2m-x: пропускать точки монтирования\033[0m",
	"scan.interrupted": "\n  \033[33m⚠ Сканирование прервано (Ctrl+C) — результаты неполные\033[0m",
	"scan.perm_errors": "  \033[33m⚠ Пропущено %d директорий (нет доступа)\033[0m",
	"scan.timeouts":    "  \033[33m⚠ Пропущено %d директорий (таймаут — возможно iCloud или сетевой диск)\033[0m",
	"scan.progress":    "  %s %s файлов | %s  ",
	"scan.done":        "\r  ✓ Просканировано %s файлов | %s | %.1f с",

	// ── Disk ─────────────────────────────────────────────────────────────────
	"disk.header": "ДИСК",
	"disk.total":  "Всего:",
	"disk.used":   "Использовано:",
	"disk.free":   "Свободно:",

	// ── Top directories ──────────────────────────────────────────────────────
	"top.header":          "ТОП-%d ДИРЕКТОРИЙ",
	"node_modules.header": "NODE_MODULES (%d найдено)",
	"known.header":        "ИЗВЕСТНЫЕ МЕСТА",

	// ── Time Machine ─────────────────────────────────────────────────────────
	"tm.info":   "\n  \033[36mℹ Time Machine: %d локальных снимков\033[0m",
	"tm.delete": "    Удалить: \033[1mtmutil deletelocalsnapshots /\033[0m",

	// ── LLM ──────────────────────────────────────────────────────────────────
	"llm.header":    "АНАЛИЗ LLM",
	"llm.analyzing": "  \033[2mАнализирую с помощью %s...\033[0m",
	"llm.error":     "\n  \033[31mОшибка LLM: %v\033[0m",
	"llm.tokens":    "\n  \033[2m— Токены: %d вход + %d выход = %d итого\033[0m",
	"llm.cost_low":  "  \033[2m| стоимость: <$0.001\033[0m",
	"llm.cost":      "  \033[2m| стоимость: $%.4f\033[0m",

	// ── LLM prompts ─────────────────────────────────────────────────────────
	"llm.prompt.header":    "## Анализ диска macOS\n\n",
	"llm.prompt.disk":      "**Диск:** %s всего, %s использовано (%.0f%%), %s свободно\n\n",
	"llm.prompt.top_dirs":  "### Топ директорий по размеру:\n",
	"llm.prompt.known":     "\n### Известные macOS локации:\n",
	"llm.prompt.cleanable": " [можно очистить]",
	"llm.prompt.path":      "  Путь: %s\n",
	"llm.prompt.request":   "\nДай конкретные рекомендации по освобождению места, отсортированные по эффекту. Для каждого пункта укажи ожидаемый объём освобождаемого места и точную команду или действие.",

	"llm.system.describe": "Ты эксперт по macOS и очистке диска. Дай краткий (2-4 предложения) ПРАКТИЧЕСКИЙ совет.\nНе объясняй что это такое абстрактно — пользователь видит путь и размер.\nВместо этого скажи конкретно: можно ли удалить, что произойдёт после удаления,\nи если есть более правильный способ очистки (команда, настройки приложения) — укажи его.\nОтвечай на русском языке.",
	"llm.system.analyze": "Ты эксперт по macOS, помогающий пользователям освободить место на диске.\nАнализируй данные об использовании диска и давай конкретные, actionable рекомендации.\nПриоритизируй рекомендации по потенциальному объёму освобождаемого места.\nИспользуй markdown: заголовки, жирный текст, списки. Отвечай на русском языке.\nБудь конкретен: указывай точные команды и пути для очистки.",
	"llm.empty_response": "пустой ответ от API",

	// ── Tips ─────────────────────────────────────────────────────────────────
	"tips.header":   "РЕКОМЕНДАЦИИ",
	"tips.potential": "\n  Потенциально освободить: \033[1m%s\033[0m",
	"tips.run":       "\n  Запустите с \033[1m-i\033[0m для интерактивной очистки или \033[1m--llm\033[0m для анализа с помощью ИИ.\n",

	// ── Cleanup no items ─────────────────────────────────────────────────────
	"cleanup.no_items": "\n  Нет подходящих для очистки директорий.",

	// ── Browser TUI ──────────────────────────────────────────────────────────
	"browser.loading":       "\n  Загрузка...\n",
	"browser.error":         "Ошибка: %v",
	"browser.delete_error":  "Ошибка удаления: %v",
	"browser.delete_done":   "✓ Удалено %d, освобождено %s",
	"browser.delete_cancel": "Удаление отменено",

	"browser.panel.analysis":    "  ● Анализ содержимого",
	"browser.panel.description": "  ○ Описание",
	"browser.panel.thinking":    "  ⟳ Думаю...",
	"browser.panel.hint":        "  i: анализ содержимого",

	"browser.footer.full":  "↑↓/jk нав.  Enter/→ войти  ←/Esc назад  Space отметить  d удалить  i анализ  q выйти  ? помощь",
	"browser.footer.base":  "↑↓/jk нав.  Enter/→ войти  ←/Esc назад  Space отметить  d удалить",
	"browser.footer.desc":  "  i описание",
	"browser.footer.quit":  "  q выйти  ? помощь",

	"browser.confirm":      "\n  Удалить %d элемент(ов) (%s)?\n\n",
	"browser.confirm_yes":  "\n  [y] Да, удалить  [любая другая] Отмена\n",
	"browser.deleting":     "\n  ⟳ Удаление %d элемент(ов) (%s)...\n\n",
	"browser.please_wait":  "\n  Пожалуйста, подождите...\n",

	"browser.help": `
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
`,
	"browser.help.llm": `  i               Детальный анализ содержимого
  I               Вернуться к автодескрипции
`,
	"browser.help.quit":    "  q               Выйти",
	"browser.help.return":  "\n  Нажмите любую клавишу для возврата",

	// ── Browser LLM prompts ─────────────────────────────────────────────────
	"browser.llm.file":     "файл",
	"browser.llm.dir":      "директория",
	"browser.llm.auto_header": "Выбранный элемент: %s / %s\nТип: %s | Размер: %s\n",
	"browser.llm.auto_siblings": "\nСодержимое текущей директории (топ по размеру):\n",
	"browser.llm.auto_children": "\nСодержимое выбранной папки (топ по размеру):\n",
	"browser.llm.auto_ask": "\nДай практический совет в 2-3 предложения:\n— Можно ли удалить (целиком или частично) и что произойдёт?\n— Если есть правильный способ очистки (команда, меню приложения) — укажи.\n— Если внутри есть что-то особенно крупное, на что обратить внимание?",
	"browser.llm.analyze":  "Анализ: %s\nТип: %s\nРазмер: %s\n",
	"browser.llm.contents": "\nСодержимое (топ 15 по размеру):\n",
	"browser.llm.questions": "\nОтветь на русском:\n1. Что это и зачем нужно?\n2. Что можно безопасно удалить и сколько места освободится?\n3. Какой риск при удалении?\n4. Конкретные команды или пути для очистки.",

	// ── Cleanup TUI ──────────────────────────────────────────────────────────
	"cleanup.header":       "godiskanal — Интерактивная очистка",
	"cleanup.loading":      " Загрузка содержимого...",
	"cleanup.footer":       "↑↓/jk нав.  Space выбрать  a все/сброс  Enter детали  c очистить  q выйти",
	"cleanup.confirm":      "\n  Очистить %d элемент(ов) (%s)?\n\n",
	"cleanup.confirm_yes":  "\n  [y] Да, очистить  [любая другая клавиша] Отмена\n",
	"cleanup.freed":        "\n  Освобождено: %s",
	"cleanup.errors":       "  (%d ошибок)",
	"cleanup.manual":       "\n\n  Требуют ручной очистки (запустите команду):\n",
	"cleanup.exit":         "\n  Нажмите любую клавишу для выхода\n",
	"cleanup.status.error":    "ошибка: ",
	"cleanup.status.done":     "готово",
	"cleanup.status.cleaning": "очищаю...",
	"cleanup.status.waiting":  "ожидание",

	"cleanup.drill.footer":      "↑↓/jk нав.  Space выбрать  a все/сброс  Enter/c удалить  Esc назад",
	"cleanup.drill.confirm":     "\n  Удалить %d элемент(ов) (%s)?\n\n",
	"cleanup.drill.confirm_yes": "\n  [y] Да, удалить  [любая другая клавиша] Отмена\n",
	"cleanup.drill.freed":       "\n  Освобождено: ~%s",
	"cleanup.drill.back":        "\n\n  Нажмите любую клавишу для возврата к списку\n",

	// ── Known locations ──────────────────────────────────────────────────────
	"loc.App Caches.desc":           "Кэши приложений",
	"loc.App Caches.note":           "Кэши пересоздадутся автоматически",
	"loc.App Support.desc":          "Данные приложений",
	"loc.App Support.note":          "Удалять данные конкретных приложений вручную",
	"loc.Xcode DerivedData.desc":    "Артефакты сборки Xcode",
	"loc.Xcode DerivedData.note":    "Xcode пересоберёт при необходимости",
	"loc.iOS Simulators.desc":       "Образы iOS-симуляторов",
	"loc.iOS Simulators.note":       "xcrun simctl delete unavailable",
	"loc.iOS Device Support.desc":   "Отладочные символы устройств",
	"loc.iOS Device Support.note":   "Старые версии можно удалить",
	"loc.iOS Backups.desc":          "Резервные копии iPhone/iPad",
	"loc.iOS Backups.note":          "Управляйте через Finder → устройство → Резервные копии",
	"loc.iCloud Drive.desc":         "Локальные копии iCloud Drive",
	"loc.iCloud Drive.note":         "Управляйте через Системные настройки → Apple ID",
	"loc.Downloads.desc":            "Загрузки",
	"loc.Downloads.note":            "Проверьте содержимое вручную",
	"loc.Trash.desc":                "Корзина",
	"loc.Trash.note":                "Очистить корзину",
	"loc.npm cache.desc":            "Кэш npm пакетов",
	"loc.npm cache.note":            "npm cache clean --force",
	"loc.yarn cache.desc":           "Кэш Yarn",
	"loc.yarn cache.note":           "yarn cache clean",
	"loc.pnpm store.desc":           "Хранилище pnpm",
	"loc.pnpm store.note":           "pnpm store prune",
	"loc.Go modules.desc":           "Кэш Go-модулей",
	"loc.Go modules.note":           "go clean -modcache",
	"loc.Gradle cache.desc":         "Кэш Gradle",
	"loc.Gradle cache.note":         "Безопасно удалить",
	"loc.Maven cache.desc":          "Локальный репозиторий Maven",
	"loc.Maven cache.note":          "Безопасно удалить",
	"loc.Rust cargo.desc":           "Кэш Rust/Cargo",
	"loc.Rust cargo.note":           "cargo cache --autoclean",
	"loc.pip cache.desc":            "Кэш Python pip",
	"loc.pip cache.note":            "pip cache purge",
	"loc.Go build cache.desc":       "Кэш сборки Go",
	"loc.Go build cache.note":       "go clean -cache",
	"loc.Rust toolchains.desc":      "Установленные версии Rust (rustup)",
	"loc.Rust toolchains.note":      "rustup toolchain list; rustup toolchain remove <version>",
	"loc.CocoaPods cache.desc":      "Кэш CocoaPods (iOS/macOS зависимости)",
	"loc.CocoaPods cache.note":      "pod cache clean --all",
	"loc.Node-gyp cache.desc":       "Кэш нативных Node.js модулей",
	"loc.Node-gyp cache.note":       "Безопасно удалить, пересоздаётся",
	"loc.Dart/Flutter pub.desc":     "Кэш пакетов Dart/Flutter",
	"loc.Dart/Flutter pub.note":     "dart pub cache clean",
	"loc.NuGet packages.desc":       "Кэш пакетов .NET / NuGet",
	"loc.NuGet packages.note":       "dotnet nuget locals all --clear",
	"loc.PlatformIO.desc":           "Тулчейны и библиотеки PlatformIO (embedded dev)",
	"loc.PlatformIO.note":           "pio system prune",
	"loc.Bun packages.desc":         "Кэш и глобальные пакеты Bun",
	"loc.Bun packages.note":         "bun pm cache rm",
	"loc.HuggingFace models.desc":   "Локальные модели HuggingFace",
	"loc.HuggingFace models.note":   "huggingface-cli delete-cache или удалите модели вручную",
	"loc.Whisper models.desc":       "Модели OpenAI Whisper (распознавание речи)",
	"loc.Whisper models.note":       "Перескачаются при следующем запуске",
	"loc.uv cache.desc":             "Кэш пакетного менеджера uv (Python)",
	"loc.uv cache.note":             "uv cache clean",
	"loc.Continue AI index.desc":    "Поисковый индекс кода Continue AI",
	"loc.Continue AI index.note":    "Пересоздаётся автоматически при открытии VS Code",
	"loc.VS Code extensions.desc":   "Расширения Visual Studio Code",
	"loc.VS Code extensions.note":   "Удалите ненужные расширения в VS Code",
	"loc.Python venv (~/.venv).desc": "Виртуальное окружение Python",
	"loc.Python venv (~/.venv).note": "Удалите если окружение не нужно, пересоздаётся через python -m venv",
	"loc.Puppeteer Chromium.desc":   "Chromium для Puppeteer (браузерное тестирование)",
	"loc.Puppeteer Chromium.note":   "Перескачается при следующем использовании Puppeteer",
	"loc.Electron cache.desc":       "Кэш Electron SDK",
	"loc.Electron cache.note":       "Перескачается при сборке Electron-приложений",
	"loc.Wine.desc":                 "Данные Wine (запуск Windows-приложений)",
	"loc.Wine.note":                 "Содержит данные Windows-приложений, удаляйте осторожно",
	"loc.Safari cache.desc":         "Дисковый кэш Safari",
	"loc.Safari cache.note":         "Кэш пересоздаётся автоматически при посещении сайтов",
	"loc.Chrome cache.desc":         "Дисковый кэш Google Chrome",
	"loc.Chrome cache.note":         "Кэш пересоздаётся автоматически",
	"loc.Telegram.desc":             "Данные Telegram Desktop (включая медиакэш)",
	"loc.Telegram.note":             "Очистите медиакэш через Настройки → Конфиденциальность → Хранилище",
	"loc.Telegram (App Store).desc": "Данные Telegram из Mac App Store",
	"loc.Telegram (App Store).note": "Очистите медиакэш через Настройки → Конфиденциальность → Хранилище",
	"loc.Firefox cache.desc":            "Дисковый кэш Firefox",
	"loc.Firefox cache.note":            "Кэш пересоздаётся автоматически",
	"loc.Firefox cache (alt).desc":      "Дисковый кэш Firefox (альтернативное расположение)",
	"loc.Firefox cache (alt).note":      "Кэш пересоздаётся автоматически",
	"loc.Slack.desc":                    "Данные Slack (включая кэш и логи)",
	"loc.Slack.note":                    "Очистите кэш через Slack → Help → Reset App",
	"loc.Discord.desc":                  "Данные Discord (кэш и локальное хранилище)",
	"loc.Discord.note":                  "Очистите кэш через Настройки Discord → Расширенные",
	"loc.Spotify cache.desc":            "Кэш стриминга Spotify",
	"loc.Spotify cache.note":            "Кэш пересоздаётся автоматически при воспроизведении",
	"loc.Zoom.desc":                     "Данные Zoom и кэш",
	"loc.Zoom.note":                     "Удалите старые записи вручную",
	"loc.Steam.desc":                    "Клиент Steam и данные игр",
	"loc.Steam.note":                    "Управляйте играми через Steam → Библиотека",
	"loc.Xcode Archives.desc":          "Архивы приложений Xcode (.xcarchive)",
	"loc.Xcode Archives.note":          "Старые архивы можно удалить через Xcode → Organizer",
	"loc.Xcode Previews.desc":          "Артефакты сборки SwiftUI превью",
	"loc.Xcode Previews.note":          "Безопасно удалить, пересоберутся при следующем превью",
	"loc.Android SDK.desc":             "Android SDK, эмуляторы и инструменты сборки",
	"loc.Android SDK.note":             "Управляйте через Android Studio → SDK Manager",
	"loc.JetBrains caches.desc":        "Кэши IntelliJ/WebStorm/GoLand/PyCharm",
	"loc.JetBrains caches.note":        "Безопасно удалить, IDE пересоздаст кэши",
	"loc.Composer cache.desc":           "Кэш пакетов PHP Composer",
	"loc.Composer cache.note":           "composer clear-cache",
	"loc.Ruby gems.desc":                "Ruby gems (глобальные установки)",
	"loc.Ruby gems.note":                "gem cleanup (удаляет старые версии)",
	"loc.Cypress cache.desc":            "Бинарники тест-раннера Cypress",
	"loc.Cypress cache.note":            "npx cypress cache clear",
	"loc.Playwright browsers.desc":      "Бинарники браузеров Playwright (Chromium, Firefox, WebKit)",
	"loc.Playwright browsers.note":      "Перескачаются при следующем npx playwright install",
	"loc.Ollama models.desc":            "Локальные LLM модели Ollama",
	"loc.Ollama models.note":            "ollama rm <model> для удаления конкретных моделей",
	"loc.PyTorch Hub.desc":              "Кэшированные модели и чекпоинты PyTorch Hub",
	"loc.PyTorch Hub.note":              "Перескачаются при необходимости",
	"loc.Poetry cache.desc":             "Кэш пакетов Python Poetry",
	"loc.Poetry cache.note":             "poetry cache clear --all .",
	"loc.Application logs.desc":         "Логи приложений",
	"loc.Application logs.note":         "Старые логи можно безопасно удалить",
	"loc.Saved Application State.desc":  "Сохранённые позиции окон и состояния приложений",
	"loc.Saved Application State.note":  "Безопасно удалить, приложения пересоздадут при запуске",
	"loc.Mail Downloads.desc":           "Скачанные вложения Apple Mail",
	"loc.Mail Downloads.note":           "Копии вложений из писем, безопасно очистить",
	"loc.Cursor extensions.desc":        "Расширения Cursor IDE",
	"loc.Cursor extensions.note":        "Удалите ненужные расширения в Cursor",
	"loc.Conda.desc":                    "Кэш пакетов Conda/Anaconda (pkgs/)",
	"loc.Conda.note":                    "conda clean --all",
	"loc.Docker.desc":               "Docker образы и данные",
	"loc.Docker.note":               "docker system prune -a --volumes",
	"loc.Homebrew cache.desc":       "Кэш Homebrew",
	"loc.Homebrew cache.note":       "brew cleanup",
}
