package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) DrawError(screen *ebiten.Image) {
	if !g.showError {
		return
	}

	if g.errorDebounce > 0 {
		g.errorDebounce--
		if g.errorDebounce == 0 {
			g.showError = false
		}
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	w, h := 400.0, 100.0
	x := (sw - w) / 2
	y := (sh - h) / 2

	// Draw Background (Semi-transparent dark red)
	drawButton(screen, float64(x), float64(y), float64(w), float64(h), g.errorMessage, kindFromColor(color.RGBA{150, 0, 0, 200}))

	// Draw Message (Centered)
}
