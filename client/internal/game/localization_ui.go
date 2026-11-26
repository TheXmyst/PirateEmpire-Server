package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawLocalizationOverlay draws the coordinate overlay in localization mode
func (g *Game) DrawLocalizationOverlay(screen *ebiten.Image) {
	if !g.ui.LocalizationMode {
		return
	}

	mx, my := cursorPosition()
	screenW, screenH := float64(g.screenWidth), float64(g.screenHeight)

	// Convert screen coords to world coords
	worldX := (float64(mx)-screenW/2)/g.camZoom + g.camX
	worldY := (float64(my)-screenH/2)/g.camZoom + g.camY

	// Draw overlay box (top-left corner)
	boxX := 10.0
	boxY := 10.0
	boxW := 250.0
	boxH := 100.0

	// Background
	drawButton(screen, float64(boxX), float64(boxY), float64(boxW), float64(boxH), "Localization mode", kindFromColor(color.RGBA{0, 0, 0, 200}))

	// Title

	// Coordinates
	uiPrint(screen, fmt.Sprintf("WORLD X: %.0f", worldX), int(boxX)+10, int(boxY)+30)
	uiPrint(screen, fmt.Sprintf("WORLD Y: %.0f", worldY), int(boxX)+10, int(boxY)+45)
	uiPrint(screen, fmt.Sprintf("SCREEN X: %d", mx), int(boxX)+10, int(boxY)+60)
	uiPrint(screen, fmt.Sprintf("SCREEN Y: %d", my), int(boxX)+10, int(boxY)+75)

	// Hint
	uiPrint(screen, "Press F1 or ESC to return", int(boxX)+10, int(boxY)+95)
}
