package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// render_helpers.go contains shared rendering helper functions used across multiple UI components.
// This file was extracted from main.go during Phase 1.8 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// drawTextWithBackground : texte sur une pastille sombre (coûts, etc.).
func drawTextWithBackground(screen *ebiten.Image, txt string, x, y int, bgColor color.RGBA) {
	w := textWidth(stripUnsupported(txt), bodyFace(uiTextSize, false))
	fillRoundRect(screen, float64(x)-6, float64(y)-3, w+12, 21, 8, color.RGBA{20, 13, 7, 200})
	uiPrint(screen, txt, x, y)
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

// draw9Slice dessinait un cadre en images ; il dessine maintenant le
// panneau du thème (bois sombre, bord or) pour toutes les fenêtres qui
// l'utilisent.
func draw9Slice(screen *ebiten.Image, g *Game, x, y, width, height, targetCornerSize float64) {
	r := math.Min(16, math.Min(width, height)/2)
	drawThemePanel(screen, x, y, width, height, r)
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

// drawTooltip : bulle d'aide (première ligne en titre doré).
func drawTooltip(screen *ebiten.Image, x, y int, lines []string) {
	if len(lines) == 0 {
		return
	}
	f := bodyFace(13, false)
	fTitle := bodyFace(13, true)
	lineH := 18.0
	w := 0.0
	for i, l := range lines {
		ff := f
		if i == 0 {
			ff = fTitle
		}
		w = math.Max(w, textWidth(stripUnsupported(l), ff))
	}
	bw, bh := w+20, float64(len(lines))*lineH+12
	fx, fy := float64(x), float64(y)
	// Reste dans l'écran.
	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	if fx+bw > sw-4 {
		fx = sw - 4 - bw
	}
	if fy+bh > sh-4 {
		fy = sh - 4 - bh
	}
	fillRoundRect(screen, fx, fy, bw, bh, 10, color.RGBA{20, 13, 7, 235})
	strokeRoundRect(screen, fx, fy, bw, bh, 10, 1.5, themeGoldDim)
	for i, l := range lines {
		ff, clr := f, color.Color(themeText)
		if i == 0 {
			ff, clr = fTitle, themeGold
		}
		drawText(screen, stripUnsupported(l), ff, fx+10, fy+7+float64(i)*lineH, clr, alignLeft)
	}
}
