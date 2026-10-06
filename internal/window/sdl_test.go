//go:build (darwin || linux || windows) && (amd64 || arm64)

package window

import (
	"strings"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

func TestKeyNames(t *testing.T) {
	cases := map[sdl.Keycode]string{
		sdl.K_RETURN:      "enter",
		sdl.K_KP_ENTER:    "enter",
		sdl.K_ESCAPE:      "escape",
		sdl.K_BACKSPACE:   "backspace",
		sdl.K_TAB:         "tab",
		sdl.K_SPACE:       "space",
		sdl.K_DELETE:      "delete",
		sdl.K_INSERT:      "insert",
		sdl.K_HOME:        "home",
		sdl.K_END:         "end",
		sdl.K_PAGEUP:      "page-up",
		sdl.K_PAGEDOWN:    "page-down",
		sdl.K_LEFT:        "arrow-left",
		sdl.K_RIGHT:       "arrow-right",
		sdl.K_UP:          "arrow-up",
		sdl.K_DOWN:        "arrow-down",
		sdl.K_CAPSLOCK:    "caps-lock",
		sdl.K_F1:          "f1",
		sdl.K_F12:         "f12",
		sdl.K_LSHIFT:      "shift",
		sdl.K_RSHIFT:      "shift",
		sdl.K_LCTRL:       "control",
		sdl.K_RGUI:        "meta",
		sdl.K_LALT:        "alt",
		sdl.K_APPLICATION: "menu",
		'A':               "a",
		'Z':               "z",
		'0':               "0",
	}
	for k, want := range cases {
		if got := keyName(k); got != want {
			t.Errorf("keyName(%#x) = %q, want %q", uint32(k), got, want)
		}
	}
}

// Unknown keys must surface as "#<keycode>" — never dropped silently.
func TestKeyNameUnknownIsExplicit(t *testing.T) {
	got := keyName(sdl.K_MODE)
	if !strings.HasPrefix(got, "#") {
		t.Fatalf("keyName(K_MODE) = %q, want #<keycode> fallback", got)
	}
}

func TestMapModifiers(t *testing.T) {
	cases := []struct {
		in   sdl.Keymod
		want int
	}{
		{sdl.KMOD_NONE, 0},
		{sdl.KMOD_LSHIFT, ModShift},
		{sdl.KMOD_RCTRL, ModCtrl},
		{sdl.KMOD_LALT | sdl.KMOD_RGUI, ModAlt | ModMeta},
		{sdl.KMOD_SHIFT | sdl.KMOD_CTRL, ModShift | ModCtrl},
	}
	for _, c := range cases {
		if got := mapModifiers(c.in); got != c.want {
			t.Errorf("mapModifiers(%#x) = %#x, want %#x", uint16(c.in), got, c.want)
		}
	}
}
