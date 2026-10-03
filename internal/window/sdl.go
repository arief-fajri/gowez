//go:build (darwin || linux || windows) && (amd64 || arm64)

package window

import (
	"fmt"
	"strings"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
)

// maxEventsPerPump bounds one Pump batch so a flood of input can never
// stall a frame (G-REL-01: no operation may block forever); the
// remainder is drained by the next Pump.
const maxEventsPerPump = 256

// unloader is the (unexported) type returned by binsdl.Load.
type unloader interface{ Unload() }

// sdlWindow implements Window on top of SDL3 loaded through purego —
// no cgo, shared libraries embedded in the binary (DRR-001).
//
// All methods run on the goroutine that called New (the OS main
// thread); there is no internal locking because the contract forbids
// cross-goroutine use.
type sdlWindow struct {
	lib unloader

	title    string
	logicalW int
	logicalH int

	win  *sdl.Window
	rend *sdl.Renderer
	tex  *sdl.Texture
	texW int
	texH int

	inited bool
	closed bool
}

// newPlatformWindow creates the SDL3 window backend. Init and window
// creation must happen on the OS main thread (SDL requirement); calling
// New from another goroutine fails explicitly rather than
// half-initializing.
func newPlatformWindow(opts Options) (Window, error) {
	if opts.Width <= 0 || opts.Height <= 0 {
		return nil, fmt.Errorf("window: invalid size %dx%d", opts.Width, opts.Height)
	}
	lib := binsdl.Load()
	w := &sdlWindow{lib: lib, title: opts.Title, logicalW: opts.Width, logicalH: opts.Height}

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		lib.Unload()
		return nil, fmt.Errorf("window: init video (main thread required): %w", err)
	}
	w.inited = true

	// VSync is best effort only: DRR-001 finding F3 showed Present does
	// not block even with vsync reported on — the app loop owns frame
	// pacing and never depends on this.
	_ = sdl.SetHint(sdl.HINT_RENDER_VSYNC, "1")

	win, rend, err := sdl.CreateWindowAndRenderer(opts.Title, opts.Width, opts.Height, sdl.WINDOW_RESIZABLE)
	if err != nil {
		sdl.Quit()
		lib.Unload()
		return nil, fmt.Errorf("window: create: %w", err)
	}
	w.win, w.rend = win, rend

	_ = rend.SetDrawColor(0, 0, 0, 255)
	_ = rend.SetVSync(1)
	return w, nil
}

func (w *sdlWindow) Title() string { return w.title }

func (w *sdlWindow) SetTitle(title string) {
	w.title = title
	if !w.closed {
		_ = w.win.SetTitle(title)
	}
}

func (w *sdlWindow) Size() (int, int) {
	if !w.closed {
		if pw, ph, err := w.win.Size(); err == nil {
			return int(pw), int(ph)
		}
	}
	return w.logicalW, w.logicalH
}

func (w *sdlWindow) PixelSize() (int, int) {
	if !w.closed {
		if pw, ph, err := w.win.SizeInPixels(); err == nil {
			return int(pw), int(ph)
		}
	}
	return w.logicalW, w.logicalH
}

func (w *sdlWindow) Show() {
	if !w.closed {
		_ = w.win.Show()
	}
}

// Close releases every native resource exactly once; repeated calls are
// no-ops (invariant I12).
func (w *sdlWindow) Close() {
	if w.closed {
		return
	}
	w.closed = true
	if w.tex != nil {
		w.tex.Destroy()
		w.tex = nil
	}
	if w.rend != nil {
		w.rend.Destroy()
		w.rend = nil
	}
	if w.win != nil {
		w.win.Destroy()
		w.win = nil
	}
	if w.inited {
		sdl.Quit()
		w.inited = false
	}
	if w.lib != nil {
		w.lib.Unload()
		w.lib = nil
	}
}

// Pump drains the SDL event queue into window events. Non-blocking and
// bounded by maxEventsPerPump.
func (w *sdlWindow) Pump() []Event {
	if w.closed {
		return nil
	}
	var out []Event
	var ev sdl.Event
	for i := 0; i < maxEventsPerPump; i++ {
		if !sdl.PollEvent(&ev) {
			break
		}
		if e, ok := mapEvent(&ev); ok {
			out = append(out, e)
		}
	}
	return out
}

func mapEvent(ev *sdl.Event) (Event, bool) {
	switch ev.Type {
	case sdl.EVENT_QUIT, sdl.EVENT_WINDOW_CLOSE_REQUESTED:
		return CloseEvent{}, true

	case sdl.EVENT_WINDOW_RESIZED:
		if we := ev.WindowEvent(); we != nil {
			return ResizeEvent{Width: int(we.Data1), Height: int(we.Data2)}, true
		}

	case sdl.EVENT_MOUSE_MOTION:
		if m := ev.MouseMotionEvent(); m != nil {
			return PointerEvent{X: float64(m.X), Y: float64(m.Y), Press: m.State != 0}, true
		}

	case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
		if m := ev.MouseButtonEvent(); m != nil {
			return PointerEvent{X: float64(m.X), Y: float64(m.Y), Press: m.Down, Button: int(m.Button)}, true
		}

	case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
		if k := ev.KeyboardEvent(); k != nil {
			return KeyEvent{Key: keyName(k.Key), Press: k.Down, Modifier: int(k.Mod)}, true
		}
	}
	return nil, false
}

// keyName maps a keycode to the portable name used by KeyEvent. The
// common keys are named; anything else is reported as "#<keycode>" so
// no key is ever dropped silently (Milestone 3 completes the table).
func keyName(k sdl.Keycode) string {
	switch k {
	case sdl.K_RETURN:
		return "enter"
	case sdl.K_ESCAPE:
		return "escape"
	case sdl.K_BACKSPACE:
		return "backspace"
	case sdl.K_TAB:
		return "tab"
	case sdl.K_SPACE:
		return "space"
	case sdl.K_LEFT:
		return "arrow-left"
	case sdl.K_RIGHT:
		return "arrow-right"
	case sdl.K_UP:
		return "arrow-up"
	case sdl.K_DOWN:
		return "arrow-down"
	}
	if k >= 0x20 && k < 0x7f {
		return strings.ToLower(string(rune(k)))
	}
	return fmt.Sprintf("#%d", uint32(k))
}

// Present uploads one frame and flips it to the screen. The texture is
// (re)created when the frame size changes — the HiDPI resize path.
func (w *sdlWindow) Present(pixels []byte, width, height int) error {
	if w.closed {
		return fmt.Errorf("window: present after close")
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("window: present invalid size %dx%d", width, height)
	}
	if need := width * height * 4; len(pixels) < need {
		return fmt.Errorf("window: present buffer %d bytes, need %d", len(pixels), need)
	}
	if w.tex == nil || w.texW != width || w.texH != height {
		if w.tex != nil {
			w.tex.Destroy()
			w.tex = nil
		}
		t, err := w.rend.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, width, height)
		if err != nil {
			return fmt.Errorf("window: create texture: %w", err)
		}
		if err := t.SetBlendMode(sdl.BLENDMODE_NONE); err != nil {
			t.Destroy()
			return fmt.Errorf("window: texture blend mode: %w", err)
		}
		w.tex, w.texW, w.texH = t, width, height
	}
	if err := w.tex.Update(nil, pixels, int32(width*4)); err != nil {
		return fmt.Errorf("window: upload frame: %w", err)
	}
	if err := w.rend.Clear(); err != nil {
		return fmt.Errorf("window: clear: %w", err)
	}
	if err := w.rend.RenderTexture(w.tex, nil, nil); err != nil {
		return fmt.Errorf("window: blit: %w", err)
	}
	if err := w.rend.Present(); err != nil {
		return fmt.Errorf("window: present: %w", err)
	}
	return nil
}
