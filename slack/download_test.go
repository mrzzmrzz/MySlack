package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Download sends the session, reports progress and leaves the file whole.
func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer xoxc-1" || r.Header.Get("Cookie") != "d=xoxd-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("hello, file"))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "a.txt")
	var last int64
	err := New("xoxc-1", "xoxd-1").Download(context.Background(), srv.URL, path, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "hello, file" || last != 11 {
		t.Errorf("file %q, progress %d", data, last)
	}
	if _, err := os.Stat(path + ".download"); !os.IsNotExist(err) {
		t.Error("the partial file is left")
	}
}
