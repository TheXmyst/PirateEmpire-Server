package game

import (
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

	drawErrorToast(screen, float64(g.screenWidth), float64(g.screenHeight), g.errorMessage)
}

// drawErrorToast : bandeau d'erreur centré, bois sombre avec un liseré rouge.
func drawErrorToast(screen *ebiten.Image, sw, sh float64, msg string) {
	face := bodyFace(15, true)
	w := 440.0
	lines := wrapText(stripUnsupported(msg), face, w-60)
	if len(lines) > 4 {
		lines = lines[:4]
	}
	h := 36 + float64(len(lines))*20
	x, y := (sw-w)/2, (sh-h)/2
	drawThemePanel(screen, x, y, w, h, 14)
	strokeRoundRect(screen, x, y, w, h, 14, 2, themeRed)
	fillRoundRect(screen, x+20, y+h/2-5, 10, 10, 5, themeRed)
	for i, l := range lines {
		drawText(screen, l, face, x+42, y+18+float64(i)*20-2, themeText, alignLeft)
	}
}
