# godiskanal

Консольная утилита для macOS, которая показывает, куда делось место на диске, и помогает его очистить.
Параллельно сканирует файловую систему, проверяет известные «пожирателей» места и опционально даёт
рекомендации через LLM (OpenAI API или любой совместимый провайдер).

## Установка

```bash
go install github.com/skrashevich/godiskanal@latest
```

или локально:

```bash
git clone ...
cd godiskanal
go build -o ~/go/bin/godiskanal .
```

## Быстрый старт

```bash
# Анализ домашней директории
godiskanal

# Не пересекать границы ФС (быстрее, пропускает внешние тома)
godiskanal -x

# С рекомендациями от ИИ
godiskanal --llm

# Интерактивная очистка
godiskanal -i

# Всё вместе
godiskanal --llm -i -x
```

## Флаги

### Сканирование

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-p, --path` | `~` | Путь для сканирования |
| `-n, --top` | `20` | Количество топ-директорий |
| `-x, --one-filesystem` | `false` | Не пересекать границы файловых систем |
| `--exclude PATH` | — | Исключить путь (можно повторять) |
| `--min-size` | `100MB` | Минимальный размер для отображения |

### LLM-анализ

| Флаг | По умолчанию | Описание |
|---|---|---|
| `--llm` | `false` | Включить анализ с рекомендациями |
| `--api-key` | — | API ключ (или `OPENAI_API_KEY`) |
| `--api-url` | — | Базовый URL API (или `OPENAI_BASE_URL`) |
| `--model` | `gpt-4o-mini` | Модель LLM |

### Очистка

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-i, --interactive` | `false` | Интерактивный режим очистки |

## Сканирование

Сканирование многопоточное — по умолчанию используются все CPU ядра. В процессе отображается
текущий каталог; строка прогресса адаптируется под ширину терминала:

```
  ⠸ 1,234,500 файлов | 128.3 GB  ~/Library/Developer/Xcode/DerivedData/App/Build
```

**Ctrl+C** прерывает сканирование и выводит частичные результаты — данные уже собранных
директорий не теряются.

Директории, не ответившие за 3 секунды (незагруженные файлы iCloud, зависшие NFS/SMB-тома),
автоматически пропускаются. После сканирования выводится предупреждение:

```
⚠ Пропущено 5 директорий (таймаут — возможно iCloud или сетевой диск)
```

### Флаг `-x` — не пересекать границы ФС

Полезен при сканировании корня (`/`) или при наличии примонтированных томов:

```bash
godiskanal -x              # только основная ФС
godiskanal --path /  -x   # весь диск, без /Volumes и /System/Volumes
```

### Флаг `--exclude`

```bash
# исключить конкретный каталог
godiskanal --exclude ~/VirtualBoxVMs

# несколько исключений
godiskanal --exclude ~/VMs --exclude /opt/homebrew
```

---

## API ключ и URL

Ключ и базовый URL передаются через флаги или переменные окружения (флаги имеют приоритет):

```bash
# через переменные окружения (рекомендуется)
export OPENAI_API_KEY=sk-...
godiskanal --llm

# через флаги
godiskanal --llm --api-key sk-... --model gpt-4o
```

### Совместимые провайдеры

Флаг `--api-url` (или переменная `OPENAI_BASE_URL`) позволяет использовать любой OpenAI-совместимый API:

```bash
# Ollama (локальные модели)
godiskanal --llm --api-url http://localhost:11434/v1 --model llama3.2

# LM Studio
godiskanal --llm --api-url http://localhost:1234/v1 --model local-model

# Azure OpenAI
godiskanal --llm \
  --api-url https://my-resource.openai.azure.com/openai/deployments/gpt-4o \
  --api-key $AZURE_OPENAI_KEY \
  --model gpt-4o

# через переменные окружения
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_API_KEY=ollama
godiskanal --llm --model llama3.2
```

После завершения стриминга выводится статистика токенов и стоимость запроса:

```
— Токены: 512 вход + 1024 выход = 1536 итого  | стоимость: $0.0007
```

Стоимость рассчитывается автоматически для популярных моделей OpenAI. При использовании
сторонних провайдеров строка стоимости не выводится, если провайдер не возвращает usage-данные.

---

## Сценарии использования

### 1. Базовый анализ

```bash
godiskanal
```

Утилита:
1. Показывает статистику диска (всего / занято / свободно)
2. Параллельно сканирует `~/` с прогресс-индикатором
3. Выводит топ-20 директорий по размеру
4. Находит крупные `node_modules`
5. Проверяет известные «пожирателей» места: Xcode DerivedData, iOS Simulators, Docker, npm/yarn/pnpm/Go/Homebrew кэши, Корзина и др.
6. Сообщает о локальных снимках Time Machine
7. Выводит краткие рекомендации по очистке

### 2. Анализ через LLM

```bash
godiskanal --llm
```

Формирует промпт с данными сканирования и стримит ответ от LLM в терминал.
После завершения выводит потраченные токены и стоимость запроса.

### 3. Интерактивная очистка

```bash
godiskanal -i
```

После сканирования показывает меню с очищаемыми директориями, запрашивает подтверждение
и выполняет очистку.

### 4. Полный сценарий

```bash
godiskanal --llm -i -x
```

Параллельное сканирование без пересечения границ ФС → рекомендации от ИИ → интерактивная очистка.

### 5. Сканирование конкретного пути

```bash
godiskanal --path ~/Projects --llm
godiskanal --path /Users/shared --top 30
```

---

## Что проверяется автоматически

| Локация | Описание |
|---|---|
| `~/Library/Caches` | Кэши всех приложений |
| `~/Library/Developer/Xcode/DerivedData` | Артефакты сборки Xcode |
| `~/Library/Developer/CoreSimulator/Devices` | Образы iOS-симуляторов |
| `~/Library/Developer/Xcode/iOS DeviceSupport` | Символы отладки устройств |
| `~/Library/Application Support/MobileSync/Backup` | Резервные копии iPhone/iPad |
| `~/.Trash` | Корзина |
| `~/Downloads` | Загрузки |
| `~/.npm` | Кэш npm |
| `~/.yarn/cache` | Кэш Yarn |
| `~/.pnpm-store` | Хранилище pnpm |
| `~/go/pkg/mod` | Кэш Go-модулей |
| `~/.gradle/caches` | Кэш Gradle |
| `~/.m2/repository` | Репозиторий Maven |
| `~/.cargo` | Кэш Rust/Cargo |
| `~/Library/Containers/com.docker.docker` | Docker образы и данные |
| Homebrew cache | Определяется через `brew --cache` |
| Time Machine snapshots | Локальные снимки (`tmutil`) |
| `node_modules` | Любые крупные node_modules в дереве |
