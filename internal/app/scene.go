package app

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/text"
)

// helloScene is the Milestone 1 acceptance scene (cmd/gowez-hello): a
// live counter proves the loop runs, centered text proves the shaping
// stack, and a moving block proves rasterization happens every frame.
//
// Layout is computed in framebuffer pixels; scale converts logical
// units to pixels so geometry stays stable across HiDPI displays.
type helloScene struct {
	font   *text.Font
	frames int
}

func newHelloScene() (*helloScene, error) {
	f, err := text.Default()
	if err != nil {
		return nil, err
	}
	return &helloScene{font: f}, nil
}

// Tick advances the scene by one frame.
func (s *helloScene) Tick() { s.frames++ }

// Draw renders the scene for a pw×ph framebuffer backed by a
// logicalW-pixel-wide window. DrawText positions are baselines.
func (s *helloScene) Draw(r render.Renderer, pw, ph, logicalW int) error {
	scale := float64(pw) / float64(logicalW)

	r.DrawRect(0, 0, float64(pw), float64(ph), render.Color{R: 0.09, G: 0.10, B: 0.13, A: 1})

	// Title.
	const titleText = "GoWEZ"
	titleSize := 48 * scale
	title, err := text.Shape(titleText, s.font.WithSize(titleSize))
	if err != nil {
		return fmt.Errorf("shape title: %w", err)
	}
	r.DrawText((float64(pw)-title.Width)/2, float64(ph)*0.34, titleText, render.TextOptions{
		FontSize: titleSize,
		Color:    render.Color{R: 1, G: 1, B: 1, A: 1},
	})

	// Live counter: proves the loop is running and time advances.
	secs := s.frames / 60
	counterText := fmt.Sprintf("frame %06d · %02d:%02d:%02d",
		s.frames, secs/3600, (secs/60)%60, secs%60)
	counterSize := 20 * scale
	counter, err := text.Shape(counterText, s.font.WithSize(counterSize))
	if err != nil {
		return fmt.Errorf("shape counter: %w", err)
	}
	r.DrawText((float64(pw)-counter.Width)/2, float64(ph)*0.52, counterText, render.TextOptions{
		FontSize: counterSize,
		Color:    render.Color{R: 0.75, G: 0.78, B: 0.85, A: 1},
	})

	// Quit hint.
	const hintText = "close the window to quit"
	hintSize := 15 * scale
	hint, err := text.Shape(hintText, s.font.WithSize(hintSize))
	if err != nil {
		return fmt.Errorf("shape hint: %w", err)
	}
	r.DrawText((float64(pw)-hint.Width)/2, float64(ph)*0.64, hintText, render.TextOptions{
		FontSize: hintSize,
		Color:    render.Color{R: 0.45, G: 0.48, B: 0.56, A: 1},
	})

	// Moving block: proves pixels change every frame.
	block := 64 * scale
	travel := float64(pw) - block
	if travel < 1 {
		travel = 1
	}
	x := float64((s.frames * 4) % int(travel))
	r.DrawRect(x, float64(ph)*0.76, block, block, render.Color{R: 0.94, G: 0.35, B: 0.16, A: 1})
	return nil
}
