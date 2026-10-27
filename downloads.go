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

	"github.com/mrzzmrzz/Slack-MyGo/slack"
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
		return fmt.Sprintf("%d 字节", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.2f GB", float64(n)/1e9)
}
