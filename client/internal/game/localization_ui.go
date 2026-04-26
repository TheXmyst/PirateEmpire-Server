package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawLocalizationOverlay draws the coordinate overlay in localization mode
func (g *Game) DrawLocalizationOverlay(screen *ebiten.Image) {
	if !g.ui.LocalizationMode {
		return
	}

	mx, my := ebiten.CursorPosition()
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
	vector.DrawFilledRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), color.RGBA{0, 0, 0, 200}, true)
	vector.StrokeRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), 2, color.RGBA{100, 100, 200, 255}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "LOCALIZATION MODE", int(boxX)+10, int(boxY)+10)

	// Coordinates
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("WORLD X: %.0f", worldX), int(boxX)+10, int(boxY)+30)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("WORLD Y: %.0f", worldY), int(boxX)+10, int(boxY)+45)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("SCREEN X: %d", mx), int(boxX)+10, int(boxY)+60)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("SCREEN Y: %d", my), int(boxX)+10, int(boxY)+75)

	// Hint
	ebitenutil.DebugPrintAt(screen, "Press F1 or ESC to return", int(boxX)+10, int(boxY)+95)
}
