package main

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mrzzmrzz/MySlack/slack"
)

// The cache keeps what the app fetched, so that it shows again at once and
// is not fetched again: avatars and thumbnails in images/, files previewed
// in files/. It lives where macOS expects caches, in ~/Library/Caches, which
// the system may empty when the disk runs low; the app trims it to
// cacheLimit, the files used least recently first, and the settings show
// and clear it.

const cacheLimit = 500 << 20

// defaultCacheDir is the app's folder in ~/Library/Caches.
func defaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "io.github.mrzzmrzz.slack-mygo")
}

// imagePath is where the image at url is kept, by a hash of the URL.
func (a *app) imagePath(url string) string {
	sum := sha1.Sum([]byte(url))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(a.cacheDir, "images", name[:2], name)
}

// filePath is where a file is kept to preview it, apart from the user's
// Downloads.
func (a *app) filePath(f slack.File) string {
	return filepath.Join(a.cacheDir, "files", safeName(f.ID), safeName(f.Name))
}

// cached reads a file of the cache, marking it used.
func cached(path string) ([]byte, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	os.Chtimes(path, now, now)
	return data, true
}

// keep writes data to the cache at path, whole or not at all.
func keep(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	part := path + ".part"
	if os.WriteFile(part, data, 0o644) == nil {
		os.Rename(part, path)
	}
}

type cacheEntry struct {
	path string
	size int64
	used time.Time
}

func (a *app) cacheEntries() (entries []cacheEntry, total int64) {
	filepath.WalkDir(a.cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			entries = append(entries, cacheEntry{path, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	return entries, total
}

// cacheSize is how much the cache takes on disk.
func (a *app) cacheSize() int64 {
	_, total := a.cacheEntries()
	return total
}

// trimCache deletes the files used least recently until the cache is
// within limit.
func (a *app) trimCache(limit int64) {
	entries, total := a.cacheEntries()
	if total <= limit {
		return
	}
	slices.SortFunc(entries, func(x, y cacheEntry) int { return x.used.Compare(y.used) })
	for _, e := range entries {
		if total <= limit {
			break
		}
		if os.Remove(e.path) == nil {
			total -= e.size
		}
	}
}

// clearCache deletes the whole cache; what it held is fetched again when
// needed.
func (a *app) clearCache() {
	if err := os.RemoveAll(a.cacheDir); err != nil {
		log.Printf("cache: %v", err)
	}
}
