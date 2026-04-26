package game

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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

	// Draw dark overlay
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 200}, false)

	// Draw modal background (red tint)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{50, 10, 10, 240}, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{200, 50, 50, 255}, true)

	// Title
	titleY := y + padding
	ebitenutil.DebugPrintAt(screen, "ENGAGEMENT IMPOSSIBLE", int(x+padding), int(titleY))

	// Error message
	msgY := titleY + 40
	errorText := g.ui.PveEngageError
	if errorText == "" {
		errorText = "Erreur inconnue"
	}

	// Split long messages into multiple lines
	lineHeight := 20.0
	maxWidth := int(w - padding*2)
	charsPerLine := maxWidth / 6 // Approximate
	lines := []string{}
	currentLine := ""
	words := strings.Fields(errorText)
	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word
		if len(testLine) > charsPerLine && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine = testLine
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	for i, line := range lines {
		if i >= 5 {
			// Limit to 5 lines
			break
		}
		ebitenutil.DebugPrintAt(screen, line, int(x+padding), int(msgY+float64(i)*lineHeight))
	}

	// OK button
	btnY := y + h - 50
	btnW := 100.0
	btnH := 30.0
	btnX := x + w/2 - btnW/2
	btnCol := color.RGBA{150, 50, 50, 255}
	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
	vector.StrokeRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "OK", int(btnX)+35, int(btnY)+8)
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowPveEngageError = false
		g.ui.PveEngageError = ""
		g.ui.PveEngageErrorCode = ""
		return true
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
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
