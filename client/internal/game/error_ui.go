package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{150, 0, 0, 200}, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.White, true)

	// Draw Message (Centered)
	ebitenutil.DebugPrintAt(screen, g.errorMessage, int(x)+20, int(y)+40)
}
