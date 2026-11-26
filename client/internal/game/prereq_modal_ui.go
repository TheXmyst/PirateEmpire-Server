package game

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// prereqModalRect : position de la fenêtre des prérequis, adaptée au nombre de lignes.
func (g *Game) prereqModalRect() (x, y, w, h float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	w = 560.0
	h = 190 + float64(len(g.ui.PrereqModalReqs))*prereqRowH
	if g.ui.PrereqModalSubtitle != "" {
		h += 24
	}
	if h < 240 {
		h = 240
	}
	if h > 500 {
		h = 500
	}
	return (sw - w) / 2, (sh - h) / 2, w, h
}

const prereqRowH = 30.0

// prereqListTop : ordonnée du début de la liste.
func (g *Game) prereqListTop(y float64) float64 {
	if g.ui.PrereqModalSubtitle != "" {
		return y + 96
	}
	return y + 70
}

// DrawPrereqModal draws the prerequisites modal
func (g *Game) DrawPrereqModal(screen *ebiten.Image) {
	if !g.ui.ShowPrereqModal {
		return
	}

	x, y, w, h := g.prereqModalRect()
	padding := 22.0

	drawDimBackground(screen, 170)
	drawWindowFrame(screen, x, y, w, h, niceTitle(g.ui.PrereqModalTitle, "Prérequis manquants"))
	drawCloseButton(screen, x+w-30, y+27, 15)

	if g.ui.PrereqModalSubtitle != "" {
		drawText(screen, ellipsize(g.ui.PrereqModalSubtitle, bodyFace(14, false), w-2*padding), bodyFace(14, false), x+padding, y+60, themeMuted, alignLeft)
	}

	listStartY := g.prereqListTop(y)
	listEndY := y + h - 70
	listHeight := listEndY - listStartY
	maxVisibleItems := int(listHeight / prereqRowH)
	if maxVisibleItems < 1 {
		maxVisibleItems = 1
	}

	totalItems := len(g.ui.PrereqModalReqs)
	maxScroll := 0
	if totalItems > maxVisibleItems {
		maxScroll = totalItems - maxVisibleItems
	}
	if g.ui.PrereqModalScroll < 0 {
		g.ui.PrereqModalScroll = 0
	}
	if g.ui.PrereqModalScroll > float64(maxScroll) {
		g.ui.PrereqModalScroll = float64(maxScroll)
	}

	face := bodyFace(14, false)
	if totalItems == 0 {
		drawText(screen, "Aucun prérequis indiqué.", face, x+padding, listStartY+4, themeMuted, alignLeft)
	} else {
		startIdx := int(g.ui.PrereqModalScroll)
		endIdx := startIdx + maxVisibleItems
		if endIdx > totalItems {
			endIdx = totalItems
		}
		rowW := w - 2*padding
		if maxScroll > 0 {
			rowW -= 14
		}
		currentY := listStartY
		for i := startIdx; i < endIdx; i++ {
			req := g.ui.PrereqModalReqs[i]
			drawInset(screen, x+padding, currentY, rowW, prereqRowH-4)
			fillRoundRect(screen, x+padding+12, currentY+(prereqRowH-4)/2-3, 6, 6, 3, themeRed)
			drawText(screen, ellipsize(formatRequirement(req), face, rowW-40), face, x+padding+28, currentY+5, themeText, alignLeft)
			currentY += prereqRowH
		}

		if maxScroll > 0 {
			barX := x + w - padding - 6
			fillRoundRect(screen, barX, listStartY, 6, listHeight, 3, color.NRGBA{255, 255, 255, 20})
			thumbH := float64(maxVisibleItems) / float64(totalItems) * listHeight
			thumbY := listStartY + (g.ui.PrereqModalScroll/float64(maxScroll))*(listHeight-thumbH)
			fillRoundRect(screen, barX, thumbY, 6, thumbH, 3, themeGoldDim)
		}
	}

	btnW, btnH := 140.0, 38.0
	drawButton(screen, x+w/2-btnW/2, y+h-54, btnW, btnH, "Compris", btnPrimary)
}

// niceTitle : un titre écrit en majuscules passe en casse normale.
func niceTitle(s, fallback string) string {
	if s == "" {
		return fallback
	}
	if strings.ToUpper(s) != s {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// formatRequirement formats a requirement for display
func formatRequirement(req domain.Requirement) string {
	if req.Message != "" {
		return req.Message
	}

	// Fallback formatting based on kind
	switch req.Kind {
	case "building_level":
		if req.Current == 0 {
			return fmt.Sprintf("%s niveau %d requis (non construit)", req.Name, req.Needed)
		}
		return fmt.Sprintf("%s niveau %d requis (actuel: %d)", req.Name, req.Needed, req.Current)
	case "tech":
		return fmt.Sprintf("Technologie requise: %s (non recherchée)", req.Name)
	case "resource":
		return fmt.Sprintf("%s insuffisant: besoin de %d, avez %d", req.Name, req.Needed, req.Current)
	default:
		return fmt.Sprintf("%s requis", req.Name)
	}
}

// UpdatePrereqModal handles input for the prerequisites modal
// Returns true if input was consumed
func (g *Game) UpdatePrereqModal() bool {
	if !g.ui.ShowPrereqModal {
		return false
	}

	x, y, w, h := g.prereqModalRect()
	listHeight := (y + h - 70) - g.prereqListTop(y)

	// Close on ESC
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.closePrereqModal()
		return true
	}

	// Handle mouse clicks
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// Close button (X)
		if fmx >= x+w-47 && fmx <= x+w-13 && fmy >= y+10 && fmy <= y+44 {
			g.closePrereqModal()
			return true
		}

		// OK button
		btnY := y + h - 54
		btnW := 140.0
		btnH := 38.0
		btnX := x + w/2 - btnW/2
		if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
			g.closePrereqModal()
			return true
		}

		// Click outside modal (close)
		if fmx < x || fmx > x+w || fmy < y || fmy > y+h {
			g.closePrereqModal()
			return true
		}

		// Click inside modal (consume input)
		return true
	}

	// Mouse wheel scrolling
	_, dy := wheelDelta()
	if dy != 0 {
		totalItems := len(g.ui.PrereqModalReqs)
		maxVisibleItems := int(listHeight / prereqRowH)
		maxScroll := 0
		if totalItems > maxVisibleItems {
			maxScroll = totalItems - maxVisibleItems
		}

		if maxScroll > 0 {
			g.ui.PrereqModalScroll -= dy * 3 // Scroll speed
			if g.ui.PrereqModalScroll < 0 {
				g.ui.PrereqModalScroll = 0
			}
			if g.ui.PrereqModalScroll > float64(maxScroll) {
				g.ui.PrereqModalScroll = float64(maxScroll)
			}
			return true
		}
	}

	// Arrow keys for scrolling
	if isKeyJustPressed(ebiten.KeyArrowDown) {
		totalItems := len(g.ui.PrereqModalReqs)
		maxVisibleItems := int(listHeight / prereqRowH)
		maxScroll := 0
		if totalItems > maxVisibleItems {
			maxScroll = totalItems - maxVisibleItems
		}
		if g.ui.PrereqModalScroll < float64(maxScroll) {
			g.ui.PrereqModalScroll++
		}
		return true
	}
	if isKeyJustPressed(ebiten.KeyArrowUp) {
		if g.ui.PrereqModalScroll > 0 {
			g.ui.PrereqModalScroll--
		}
		return true
	}

	return true // Always consume input when modal is open
}

// closePrereqModal closes the prerequisites modal
func (g *Game) closePrereqModal() {
	g.ui.ShowPrereqModal = false
	g.ui.PrereqModalTitle = ""
	g.ui.PrereqModalSubtitle = ""
	g.ui.PrereqModalReqs = nil
	g.ui.PrereqModalScroll = 0
}

// openPrereqModal opens the prerequisites modal with the given requirements
func (g *Game) openPrereqModal(title string, subtitle string, reqs []domain.Requirement) {
	g.ui.ShowPrereqModal = true
	g.ui.PrereqModalTitle = title
	g.ui.PrereqModalSubtitle = subtitle
	g.ui.PrereqModalReqs = reqs
	g.ui.PrereqModalScroll = 0
	fmt.Printf("[PREREQ] modal opened title=%s reqs=%d\n", title, len(reqs))
}
