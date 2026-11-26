package game

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// DrawPveEngageErrorModal draws the engagement error modal
func (g *Game) DrawPveEngageErrorModal(screen *ebiten.Image) {
	if !g.ui.ShowPveEngageError {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 500.0, 200.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	drawDimBackground(screen, 170)
	drawWindowFrame(screen, x, y, w, h, "Engagement impossible")
	strokeRoundRect(screen, x, y, w, h, 18, 2, themeRed)

	errorText := g.ui.PveEngageError
	if errorText == "" {
		errorText = "Erreur inconnue"
	}
	face := bodyFace(15, false)
	for i, line := range wrapText(errorText, face, w-padding*2) {
		if i >= 4 {
			break
		}
		drawText(screen, line, face, x+padding, y+68+float64(i)*20, themeText, alignLeft)
	}

	btnY := y + h - 50
	btnW := 100.0
	btnH := 30.0
	btnX := x + w/2 - btnW/2
	drawButton(screen, btnX, btnY, btnW, btnH, "Ok", btnPrimary)
}

// UpdatePveEngageErrorModal handles input for the engagement error modal
// Returns true if input was consumed
func (g *Game) UpdatePveEngageErrorModal() bool {
	if !g.ui.ShowPveEngageError {
		return false
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 500.0, 200.0
	x, y := cx-w/2, cy-h/2
	btnY := y + h - 50
	btnW := 100.0
	btnH := 30.0
	btnX := x + w/2 - btnW/2

	// Close on ESC
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowPveEngageError = false
		g.ui.PveEngageError = ""
		g.ui.PveEngageErrorCode = ""
		return true
	}

	// Handle mouse clicks
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// OK button
		if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
			g.ui.ShowPveEngageError = false
			g.ui.PveEngageError = ""
			g.ui.PveEngageErrorCode = ""
			return true
		}

		// Click anywhere on modal (consume input)
		if fmx >= x && fmx <= x+w && fmy >= y && fmy <= y+h {
			return true
		}

		// Click outside modal (close)
		g.ui.ShowPveEngageError = false
		g.ui.PveEngageError = ""
		g.ui.PveEngageErrorCode = ""
		return true
	}

	return true // Always consume input when modal is open
}
