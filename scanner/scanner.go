package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Entry represents a directory with its total size.
type Entry struct {
	Path string
	Size int64
}

// Result holds the results of a disk scan.
type Result struct {
	Root      string
	Entries   []Entry // sorted by size desc
	FileCount int64
	TotalSize int64
	Errors    int
	Timeouts  int // directories skipped due to readdir timeout (e.g. undownloaded iCloud files)
	sizeMap   map[string]int64
}

// Options controls scanner behaviour.
type Options struct {
	// OneFilesystem skips directories that are on a different filesystem (mount points).
	OneFilesystem bool
	// Exclude is a list of absolute paths to skip entirely.
	Exclude []string
	// Workers is the number of parallel directory readers (0 = runtime.NumCPU()).
	Workers int
	// DirTimeout is the max time to wait for a single directory read (0 = 3s default).
	// Directories that exceed this limit are skipped and counted in Result.Timeouts.
	DirTimeout time.Duration
}

// Scan walks the directory tree rooted at root and calculates sizes.
// progressFn is called periodically with (files scanned, bytes counted, current directory).
// The scan is cancelled when ctx is done.
func Scan(ctx context.Context, root string, opts Options, progressFn func(files, bytes int64, currentDir string)) (*Result, error) {
	root = filepath.Clean(root)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	var rootDev uint64
	if opts.OneFilesystem {
		var st syscall.Stat_t
		if err := syscall.Lstat(root, &st); err == nil {
			rootDev = uint64(st.Dev)
		}
	}

	excludeSet := make(map[string]bool, len(opts.Exclude))
	for _, p := range opts.Exclude {
		excludeSet[filepath.Clean(p)] = true
	}

	numWorkers := opts.Workers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	// I/O-bound: more goroutines than CPU cores helps on SSD/NVMe
	if numWorkers < 4 {
		numWorkers = 4
	}
	sem := make(chan struct{}, numWorkers)

	dirTimeout := opts.DirTimeout
	if dirTimeout <= 0 {
		dirTimeout = 3 * time.Second
	}

	type inodeKey struct {
		dev uint64
		ino uint64
	}
	var seenFileInodes sync.Map // inodeKey → struct{}{} (file hard-link dedup)
	var seenDirInodes  sync.Map // inodeKey → struct{}{} (directory dedup incl. firmlinks)

	var (
		mu           sync.Mutex
		dirSizes     = make(map[string]int64)
		errCount     int
		timeoutCount int
	)
	var fileCount, totalBytes atomic.Int64
	var wg sync.WaitGroup

	var processDir func(dir string)
	processDir = func(dir string) {
		defer wg.Done()
		defer func() { <-sem }()

		if ctx.Err() != nil {
			return
		}

		entries, err := readDirCtx(ctx, dir, dirTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if isTimeout(err) {
				mu.Lock()
				timeoutCount++
				mu.Unlock()
				return
			}
			if !os.IsPermission(err) {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
			return
		}

		var localSize int64
		for _, entry := range entries {
			entryPath := filepath.Join(dir, entry.Name())

			if entry.IsDir() {
				if excludeSet[entryPath] {
					continue
				}
				// Lstat every subdirectory to:
				// 1. Deduplicate by inode — APFS firmlinks (/Users, /Applications, etc.)
				//    are directory hardlinks that share an inode with their target under
				//    /System/Volumes/Data/. Without this, each firmlink is scanned twice.
				// 2. Enforce OneFilesystem (skip different-device mount points).
				var st syscall.Stat_t
				if err := syscall.Lstat(entryPath, &st); err == nil {
					key := inodeKey{dev: uint64(st.Dev), ino: uint64(st.Ino)}
					if _, loaded := seenDirInodes.LoadOrStore(key, struct{}{}); loaded {
						continue // already queued via another path (firmlink or bind mount)
					}
					if opts.OneFilesystem && uint64(st.Dev) != rootDev {
						continue
					}
				}
				mu.Lock()
				dirSizes[entryPath] = 0
				mu.Unlock()

				wg.Add(1)
				go func(p string) {
					sem <- struct{}{} // acquire slot before doing work
					processDir(p)
				}(entryPath)
				continue
			}

			if entry.Type()&fs.ModeSymlink != 0 {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Use actual allocated blocks (stat.Blocks * 512) instead of logical size.
			// This correctly handles sparse files (Docker disk images, VM disks) and
			// not-yet-downloaded iCloud stubs, where logical size >> physical usage.
			// Also deduplicates hard links by inode.
			var size int64
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if stat.Nlink > 1 {
					key := inodeKey{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}
					if _, loaded := seenFileInodes.LoadOrStore(key, struct{}{}); loaded {
						fileCount.Add(1)
						continue
					}
				}
				size = int64(stat.Blocks) * 512
			} else {
				size = info.Size()
			}
			localSize += size

			n := fileCount.Add(1)
			b := totalBytes.Add(size)
			if progressFn != nil && n%2000 == 0 {
				progressFn(n, b, dir)
			}
		}

		mu.Lock()
		dirSizes[dir] += localSize
		mu.Unlock()
	}

	// Register root and kick off; seed seenDirInodes so alternate paths to root are skipped.
	var rootSt syscall.Stat_t
	if syscall.Lstat(root, &rootSt) == nil {
		seenDirInodes.Store(inodeKey{dev: uint64(rootSt.Dev), ino: uint64(rootSt.Ino)}, struct{}{})
	}
	mu.Lock()
	dirSizes[root] = 0
	mu.Unlock()

	sem <- struct{}{}
	wg.Add(1)
	go processDir(root)

	wg.Wait()

	if ctx.Err() != nil {
		// partial results — fall through to aggregation
	}

	// Collect all directories sorted deepest-first (by path length desc)
	mu.Lock()
	dirs := make([]string, 0, len(dirSizes))
	for d := range dirSizes {
		dirs = append(dirs, d)
	}
	mu.Unlock()

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	// Propagate child sizes to parents (deepest first)
	mu.Lock()
	totalSizes := make(map[string]int64, len(dirSizes))
	for k, v := range dirSizes {
		totalSizes[k] = v
	}
	mu.Unlock()

	for _, dir := range dirs {
		if dir == root {
			continue
		}
		parent := filepath.Dir(dir)
		if _, ok := totalSizes[parent]; ok {
			totalSizes[parent] += totalSizes[dir]
		}
	}

	entries := make([]Entry, 0, len(totalSizes))
	for path, size := range totalSizes {
		entries = append(entries, Entry{Path: path, Size: size})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &Result{
		Root:      root,
		Entries:   entries,
		FileCount: fileCount.Load(),
		TotalSize: totalBytes.Load(),
		Errors:    errCount,
		Timeouts:  timeoutCount,
	}, nil
}

// SizeOf returns the total size of a directory, or -1 if not found.
func (r *Result) SizeOf(path string) int64 {
	if r.sizeMap == nil {
		r.sizeMap = make(map[string]int64, len(r.Entries))
		for _, e := range r.Entries {
			r.sizeMap[e.Path] = e.Size
		}
	}
	size, ok := r.sizeMap[path]
	if !ok {
		return -1
	}
	return size
}

// errDirTimeout is returned when readDirCtx exceeds the per-directory timeout.
var errDirTimeout = errors.New("directory read timeout")

func isTimeout(err error) bool { return err == errDirTimeout }

// readDirCtx reads a directory with context cancellation and a per-call timeout.
// os.ReadDir is a blocking syscall that cannot be interrupted directly,
// so we run it in a goroutine and select on completion, ctx, and timeout.
// The background goroutine may linger until the syscall finishes, which is acceptable.
func readDirCtx(ctx context.Context, dir string, timeout time.Duration) ([]os.DirEntry, error) {
	type result struct {
		entries []os.DirEntry
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		entries, err := os.ReadDir(dir)
		ch <- result{entries, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errDirTimeout
	case r := <-ch:
		return r.entries, r.err
	}
}

// TopN returns the top N entries (excluding the root itself).
func (r *Result) TopN(n int) []Entry {
	var top []Entry
	for _, e := range r.Entries {
		if e.Path == r.Root {
			continue
		}
		top = append(top, e)
		if len(top) >= n {
			break
		}
	}
	return top
}
