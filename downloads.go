package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"github.com/mrzzmrzz/MySlack/slack"

	"github.com/mrzzmrzz/MySlack/locales"
)

// download is a file of the workspace saved, or being saved, to the
// Downloads folder.
type download struct {
	done, total int64
	path        string // where it is, once finished
	err         string
	finished    bool
}

// downloadFile saves a file to the Downloads folder, then opens it when
// open is set, as a picture's click does. A file being saved is left to
// finish; one saved is opened.
func (a *app) downloadFile(f slack.File, open bool) {
	if d := a.downloads[f.ID]; d != nil {
		switch {
		case d.finished:
			a.openFile(d.path)
			return
		case d.err == "":
			return // still coming
		}
	}
	url := f.URLPrivateDownload
	if url == "" || a.client == nil {
		return
	}
	dir, err := mygo.App.Path(mygo.PathDownloads)
	if err != nil {
		a.downloads[f.ID] = &download{err: err.Error()}
		return
	}
	path := freePath(filepath.Join(dir, safeName(f.Name)))
	d := &download{total: f.Size}
	a.downloads[f.ID] = d
	client := a.client
	go func() {
		// Progress redraws the window at most ten times a second.
		var last atomic.Int64
		err := client.Download(context.Background(), url, path, func(done, total int64) {
			if now := time.Now().UnixMilli(); now-last.Load() >= 100 || done == total {
				last.Store(now)
				a.update(func() {
					d.done = done
					if total > 0 {
						d.total = total
					}
				})
			}
		})
		a.update(func() {
			if err != nil {
				d.err = err.Error()
				log.Printf("download %s: %v", f.Name, err)
				return
			}
			d.finished, d.path, d.done = true, path, d.total
			if open {
				a.openFile(path)
			}
		})
	}()
}

func (a *app) openFile(path string) {
	if a.win == nil {
		return
	}
	if err := mygo.Shell.OpenPath(path); err != nil {
		log.Printf("open %s: %v", path, err)
	}
}

func (a *app) revealFile(path string) {
	if a.win != nil {
		mygo.Shell.ShowItemInFolder(path)
	}
}

// safeName keeps a file's name from leaving the Downloads folder.
func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "/", "-"))
	if name == "." || name == ".." || name == "" {
		return "file"
	}
	return name
}

// freePath is path, or "name 2.ext" and on as Finder names copies, so
// that a download replaces nothing.
func freePath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		p := path
		if i > 1 {
			p = fmt.Sprintf("%s %d%s", base, i, ext)
		}
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if _, err := os.Stat(p + ".download"); os.IsNotExist(err) {
				return p
			}
		}
	}
}

// size is a file's size as Finder shows it.
func size(n int64) string {
	switch {
	case n <= 0:
		return ""
	case n < 1000:
		return locales.Bytes(int(n))
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.2f GB", float64(n)/1e9)
}

// preview shows files in Quick Look, the one at index first, as Messages
// does when a picture is clicked; those not yet fetched are fetched to the
// cache first, while their thumbnails show a spinner.
func (a *app) preview(files []slack.File, index int) {
	a.previewing = &pendingPreview{files: files, index: index}
	for _, f := range files {
		path := a.filePath(f)
		if _, err := os.Stat(path); err == nil || a.fetching[f.ID] || a.client == nil || f.URLPrivateDownload == "" {
			continue
		}
		a.fetching[f.ID] = true
		os.MkdirAll(filepath.Dir(path), 0o755)
		client, url := a.client, f.URLPrivateDownload
		go func() {
			err := client.Download(context.Background(), url, path, func(int64, int64) {})
			a.update(func() {
				delete(a.fetching, f.ID)
				if err != nil {
					log.Printf("preview %s: %v", f.Name, err)
				}
				a.showPreview()
			})
		}()
	}
	a.showPreview()
}

type pendingPreview struct {
	files []slack.File
	index int
}

// showPreview opens the preview asked for once its files are fetched.
func (a *app) showPreview() {
	p := a.previewing
	if p == nil {
		return
	}
	var paths []string
	index := 0
	for i, f := range p.files {
		if a.fetching[f.ID] {
			return // still coming
		}
		path := a.filePath(f)
		if _, err := os.Stat(path); err != nil {
			continue // failed: preview the others
		}
		if i == p.index {
			index = len(paths)
		}
		paths = append(paths, path)
	}
	a.previewing = nil
	if len(paths) == 0 || a.win == nil {
		return
	}
	if !quickLook(paths, index) {
		a.openFile(paths[index])
	}
}
