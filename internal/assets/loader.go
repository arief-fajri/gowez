package assets

import (
	"errors"
	"io/fs"
)

// Loader resolves UI bundle assets from a read-only filesystem.
type Loader struct {
	fsys fs.FS
}

// FromFS wraps an fs.FS (embed.FS for packaged builds, os.DirFS in
// development) as the bundle source.
func FromFS(fsys fs.FS) *Loader {
	return &Loader{fsys: fsys}
}

// Load returns one asset by bundle-relative path.
//
// A missing asset is an explicit startup failure with a clear diagnostic
// (Module 2 §2.4: "UI asset missing → startup fails with a clear
// diagnostic"), never a
// silently empty UI.
func (l *Loader) Load(name string) ([]byte, error) {
	if l == nil || l.fsys == nil {
		return nil, errors.New("assets: no filesystem attached")
	}
	return fs.ReadFile(l.fsys, name)
}
