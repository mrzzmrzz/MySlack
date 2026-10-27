package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// The bundle identifier was io.github.mrzzmrzz.slack-mygo before it became com.mrzz.myslack.
// Keychain items and the cache are kept under it, so the sign-in and the cache
// come along, once; this can go when every install has started once.
const oldIdentifier = "io.github.mrzzmrzz.slack-mygo"

func migrateIdentifier() {
	if _, err := mygo.Secrets.Get(secretKey); errors.Is(err, mygo.ErrSecretNotFound) {
		old := mygo.Secrets.Service(oldIdentifier)
		if data, err := old.Get(secretKey); err == nil && mygo.Secrets.Set(secretKey, data) == nil {
			old.Delete(secretKey)
		}
	}
	dir := defaultCacheDir()
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		os.Rename(filepath.Join(filepath.Dir(dir), oldIdentifier), dir)
	}
}
