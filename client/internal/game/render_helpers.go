package game

import (
	"fmt"
	"image/color"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// render_helpers.go contains shared rendering helper functions used across multiple UI components.
// This file was extracted from main.go during Phase 1.8 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// drawTextWithBackground draws text with an opaque background to prevent ghosting.
// This ensures old text is cleared before new text is drawn, preventing visual artifacts.
func drawTextWithBackground(screen *ebiten.Image, text string, x, y int, bgColor color.RGBA) {
	// Estimate text width (approximate: ~6 pixels per character, with some margin)
	textWidth := len(text) * 6
	if textWidth < 60 {
		textWidth = 60 // Minimum width for short text
	}
	textHeight := 14

	// Draw opaque background rectangle to clear any previous text
	vector.DrawFilledRect(screen, float32(x-2), float32(y-2), float32(textWidth+4), float32(textHeight+4), bgColor, true)

	// Draw text on top
	ebitenutil.DebugPrintAt(screen, text, x, y)
}

// buildCostString builds a cost string in a fixed order to prevent flickering.
// Resources are always displayed in the same order: Wood, Stone, Gold, Rum.
func buildCostString(costs map[domain.ResourceType]float64, markMissing bool, hasResources map[domain.ResourceType]float64) string {
	// Fixed order to ensure deterministic string building
	order := []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum}

	// Resource name translations
	names := map[domain.ResourceType]string{
		domain.Wood:  "Bois",
		domain.Stone: "Pierre",
		domain.Gold:  "Or",
		domain.Rum:   "Rhum",
	}

	costStr := ""
	for _, res := range order {
		amt, ok := costs[res]
		if !ok || amt == 0 {
			continue
		}

		if markMissing && hasResources != nil {
			has := hasResources[res]
			if has < amt {
				costStr += fmt.Sprintf("%s:%.0f(X) ", names[res], amt)
			} else {
				costStr += fmt.Sprintf("%s:%.0f ", names[res], amt)
			}
		} else {
			costStr += fmt.Sprintf("%s:%.0f ", names[res], amt)
		}
	}
	return costStr
}

// draw9Slice draws a 9-slice UI element using the game's slice assets.
// This creates a scalable UI panel with corners, edges, and center.
func draw9Slice(screen *ebiten.Image, g *Game, x, y, width, height, targetCornerSize float64) {
	if g.sliceTopLeft == nil {
		return
	}
	bounds := g.sliceTopLeft.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	scale := 1.0
	if targetCornerSize > 0 {
		scale = targetCornerSize / float64(sw)
	}

	cornerW := float64(sw) * scale
	cornerH := float64(sh) * scale

	drawImgScaled(screen, g.sliceTopLeft, x, y, cornerW, cornerH)
	drawImgScaled(screen, g.sliceTopRight, x+width-cornerW, y, cornerW, cornerH)
	drawImgScaled(screen, g.sliceBotLeft, x, y+height-cornerH, cornerW, cornerH)
	drawImgScaled(screen, g.sliceBotRight, x+width-cornerW, y+height-cornerH, cornerW, cornerH)

	drawImgScaled(screen, g.sliceTopCenter, x+cornerW, y, width-2*cornerW, cornerH)
	drawImgScaled(screen, g.sliceBotCenter, x+cornerW, y+height-cornerH, width-2*cornerW, cornerH)
	drawImgScaled(screen, g.sliceMidLeft, x, y+cornerH, cornerW, height-2*cornerH)
	drawImgScaled(screen, g.sliceMidRight, x+width-cornerW, y+cornerH, cornerW, height-2*cornerH)
	drawImgScaled(screen, g.sliceCenter, x+cornerW, y+cornerH, width-2*cornerW, height-2*cornerH)
}

// drawImgScaled draws an image scaled to the specified width and height.
func drawImgScaled(screen *ebiten.Image, img *ebiten.Image, x, y, w, h float64) {
	if img == nil || w <= 0 || h <= 0 {
		return
	}
	bounds := img.Bounds()
	iw, ih := bounds.Dx(), bounds.Dy()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(w/float64(iw), h/float64(ih))
	op.GeoM.Translate(x, y)
	screen.DrawImage(img, op)
}

// drawTooltip draws a tooltip box with multiple lines of text.
func drawTooltip(screen *ebiten.Image, x, y int, lines []string) {
	if len(lines) == 0 {
		return
	}
	var textHeight = 14
	var boxH = len(lines)*textHeight + 10
	var boxW = 0
	for _, l := range lines {
		if len(l)*7 > boxW {
			boxW = len(l) * 7
		}
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(boxW+10), float32(boxH), color.RGBA{0, 0, 0, 200}, true)
	for i, l := range lines {
		ebitenutil.DebugPrintAt(screen, l, x+5, y+5+i*textHeight)
	}
}
