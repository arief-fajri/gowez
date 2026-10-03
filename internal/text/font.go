package text

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	otfont "github.com/go-text/typesetting/font"
	"golang.org/x/image/font/gofont/goregular"
)

// DefaultSizePx is the nominal size (logical pixels) of a Font before
// it is specialized with WithSize.
const DefaultSizePx = 16

// Font is a loaded font face ready for shaping.
//
// A Font is immutable: WithSize returns a copy sharing the same parsed
// face, so one parse can serve any number of sizes.
type Font struct {
	// Family is the resolved family name.
	Family string
	// Size is the nominal size in logical pixels.
	Size float64

	face *otfont.Face
}

var (
	defaultOnce sync.Once
	defaultFace *otfont.Face
	defaultErr  error
)

// Default returns the embedded Go Regular face at DefaultSizePx.
//
// The embedded bytes are parsed once per process; subsequent calls share
// the parsed face. The parse result is deterministic, so Default is safe
// to use from tests and golden runs.
func Default() (*Font, error) {
	defaultOnce.Do(func() {
		defaultFace, defaultErr = otfont.ParseTTF(bytes.NewReader(goregular.TTF))
	})
	if defaultErr != nil {
		return nil, fmt.Errorf("text: embedded font: %w", defaultErr)
	}
	return &Font{Family: "Go Regular", Size: DefaultSizePx, face: defaultFace}, nil
}

// Load reads a font face from disk at DefaultSizePx.
//
// An unreadable or unparseable file is an explicit error; the caller must
// see the failure, never a silently substituted face (Hard rule 6).
func Load(path string) (*Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("text: load %q: %w", path, err)
	}
	f, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("text: load %q: %w", path, err)
	}
	base := filepath.Base(path)
	f.Family = strings.TrimSuffix(base, filepath.Ext(base))
	return f, nil
}

// Parse parses TTF/OTF bytes into a Font at DefaultSizePx.
func Parse(data []byte) (*Font, error) {
	if len(data) == 0 {
		return nil, errors.New("text: empty font data")
	}
	face, err := otfont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("text: parse: %w", err)
	}
	return &Font{Size: DefaultSizePx, face: face}, nil
}

// WithSize returns a copy of f with a different nominal size in logical
// pixels. Non-positive sizes fall back to DefaultSizePx.
func (f *Font) WithSize(sizePx float64) *Font {
	if sizePx <= 0 {
		sizePx = DefaultSizePx
	}
	c := *f
	c.Size = sizePx
	return &c
}
