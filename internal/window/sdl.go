//go:build (darwin || linux || windows) && (amd64 || arm64)

package window

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
)

// maxEventsPerPump bounds one Pump batch so a flood of input can never
// stall a frame (G-REL-01: no operation may block forever); the
// remainder is drained by the next Pump.
const maxEventsPerPump = 256

// unloader is the (unexported) type returned by binsdl.Load.
type unloader interface{ Unload() }

// processLib is the embedded SDL3 shared library, mapped exactly once per
// process.
//
// binsdl.Load() writes the library into a *fresh* temp directory and dlopens it,
// so calling it per window extracts a second copy and registers every Objective-C
// class twice. macOS reports that as duplicate classes and warns of "spurious
// casting failures and mysterious crashes" — observed when the integration suite
// opened a second window (2026-10-08).
//
// The library is a process-lifetime dependency: window-scoped resources
// (texture, renderer, window, video init) are still released by Close, which is
// what invariant I12 is about. Unloading the library with the window would leave
// any later window with a closed library and a re-extracted duplicate.
var (
	processLibOnce sync.Once
	processLib     unloader
)

// loadProcessLib maps the embedded library on first use and returns it
// thereafter. It must be called from the OS main thread, like everything else
// that touches SDL.
func loadProcessLib() unloader {
	processLibOnce.Do(func() { processLib = binsdl.Load() })
	return processLib
}

// sdlWindow implements Window on top of SDL3 loaded through purego —
// no cgo, shared libraries embedded in the binary (DRR-001).
//
// All methods run on the goroutine that called New (the OS main
// thread); there is no internal locking because the contract forbids
// cross-goroutine use.
type sdlWindow struct {
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
	lib := loadProcessLib()
	w := &sdlWindow{title: opts.Title, logicalW: opts.Width, logicalH: opts.Height}

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		// The library stays mapped: it is process-scoped, and unloading it
		// here would leave a later window with a closed library.
		_ = lib
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
	// The shared library is deliberately *not* unloaded here: it is mapped once
	// per process and stays available for any later window (see loadProcessLib).
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
			// Motion never carries a press edge: press state is
			// defined only by button down/up events (docs/EVENTS.md).
			return PointerEvent{X: float64(m.X), Y: float64(m.Y)}, true
		}

	case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
		if m := ev.MouseButtonEvent(); m != nil {
			return PointerEvent{X: float64(m.X), Y: float64(m.Y), Press: m.Down, Button: int(m.Button)}, true
		}

	case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
		if k := ev.KeyboardEvent(); k != nil {
			return KeyEvent{Key: keyName(k.Key), Press: k.Down, Modifier: mapModifiers(k.Mod), Repeat: k.Repeat}, true
		}

	case sdl.EVENT_TEXT_INPUT:
		// Committed text (Milestone 5). An empty string is SDL's text-input
		// shutdown request, not a commit, so it is dropped.
		if te := ev.TextInputEvent(); te != nil && te.Text != "" {
			return TextInputEvent{Text: te.Text}, true
		}

	case sdl.EVENT_TEXT_EDITING:
		// IME composition (preedit) plus the composition window extent.
		if te := ev.TextEditingEvent(); te != nil {
			return TextEditingEvent{
				Text:   te.Text,
				Start:  int(te.Start),
				Length: int(te.Length),
			}, true
		}
	}
	return nil, false
}

// mapModifiers folds the SDL modifier mask into the portable
// ModShift|ModCtrl|ModAlt|ModMeta bitfield so the UI layer never sees
// a platform-specific mask.
func mapModifiers(m sdl.Keymod) int {
	var out int
	if m&sdl.KMOD_SHIFT != 0 {
		out |= ModShift
	}
	if m&sdl.KMOD_CTRL != 0 {
		out |= ModCtrl
	}
	if m&sdl.KMOD_ALT != 0 {
		out |= ModAlt
	}
	if m&sdl.KMOD_GUI != 0 {
		out |= ModMeta
	}
	return out
}

// keyName maps a keycode to the portable name used by KeyEvent. The
// table is complete for the documented key set (docs/EVENTS.md §key
// naming); anything outside it is reported as "#<keycode>" so no key
// is ever dropped silently.
func keyName(k sdl.Keycode) string {
	switch k {
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		return "enter"
	case sdl.K_ESCAPE:
		return "escape"
	case sdl.K_BACKSPACE:
		return "backspace"
	case sdl.K_TAB:
		return "tab"
	case sdl.K_SPACE:
		return "space"
	case sdl.K_DELETE:
		return "delete"
	case sdl.K_INSERT:
		return "insert"
	case sdl.K_HOME:
		return "home"
	case sdl.K_END:
		return "end"
	case sdl.K_PAGEUP:
		return "page-up"
	case sdl.K_PAGEDOWN:
		return "page-down"
	case sdl.K_LEFT:
		return "arrow-left"
	case sdl.K_RIGHT:
		return "arrow-right"
	case sdl.K_UP:
		return "arrow-up"
	case sdl.K_DOWN:
		return "arrow-down"
	case sdl.K_CAPSLOCK:
		return "caps-lock"
	case sdl.K_NUMLOCKCLEAR:
		return "num-lock"
	case sdl.K_PRINTSCREEN:
		return "print-screen"
	case sdl.K_SCROLLLOCK:
		return "scroll-lock"
	case sdl.K_PAUSE:
		return "pause"
	case sdl.K_LSHIFT, sdl.K_RSHIFT:
		return "shift"
	case sdl.K_LCTRL, sdl.K_RCTRL:
		return "control"
	case sdl.K_LALT, sdl.K_RALT:
		return "alt"
	case sdl.K_LGUI, sdl.K_RGUI:
		return "meta"
	case sdl.K_APPLICATION:
		return "menu"
	}
	if k >= sdl.K_F1 && k <= sdl.K_F12 {
		return fmt.Sprintf("f%d", k-sdl.K_F1+1)
	}
	if k >= 0x20 && k < 0x7f {
		return strings.ToLower(string(rune(k)))
	}
	return fmt.Sprintf("#%d", uint32(k))
}

// Present uploads one frame and flips it to the screen. The texture is
// (re)created when the frame size changes — the HiDPI resize path.
// StartTextInput begins delivering committed text and IME composition for
// the window (Milestone 5). SDL3 gates both event kinds behind this call, so
// text only arrives while an editable node holds focus.
func (w *sdlWindow) StartTextInput() error {
	if w.closed || w.win == nil {
		return errors.New("window: text input on a closed window")
	}
	if err := w.win.StartTextInput(); err != nil {
		return fmt.Errorf("window: start text input: %w", err)
	}
	return nil
}

// StopTextInput stops text delivery. Safe to call when text input is not
// active, which is why the runtime can call it unconditionally on blur.
func (w *sdlWindow) StopTextInput() error {
	if w.closed || w.win == nil {
		return nil
	}
	if err := w.win.StopTextInput(); err != nil {
		return fmt.Errorf("window: stop text input: %w", err)
	}
	return nil
}

// textInputSupported reports that the SDL backend delivers text input and IME
// composition (EVENT_TEXT_INPUT / EVENT_TEXT_EDITING are mapped in mapEvent).
func (w *sdlWindow) textInputSupported() bool { return true }

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
