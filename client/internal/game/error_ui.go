package game

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) DrawError(screen *ebiten.Image) {
	if !g.showError {
		return
	}

	// Laisse le temps de lire : au moins 2,5 s, plus si le message est long
	// (avant, certains messages disparaissaient au bout d'une demi-seconde).
	if g.errorMessage != g.errorToastFor {
		g.errorToastFor = g.errorMessage
		if g.errorDebounce > 0 {
			if need := 150 + 4*len([]rune(g.errorMessage)); g.errorDebounce < need {
				g.errorDebounce = need
			}
		}
	}

	if g.errorDebounce > 0 {
		g.errorDebounce--
		if g.errorDebounce == 0 {
			g.showError = false
			g.errorToastFor = ""
		}
	}

	drawErrorToast(screen, float64(g.screenWidth), float64(g.screenHeight), g.errorMessage)
}

// drawErrorToast : bandeau d'erreur centré, bois sombre avec un liseré rouge.
func drawErrorToast(screen *ebiten.Image, sw, sh float64, msg string) {
	face := bodyFace(15, true)
	w := 560.0
	if w > sw-32 {
		w = sw - 32
	}
	// Le message est affiché en entier (avant : coupé après 4 lignes).
	lines := wrapText(stripUnsupported(msg), face, w-60)
	if len(lines) > 12 {
		lines = lines[:12]
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
