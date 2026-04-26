package game

import (
	"fmt"
	"image/color"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawPrereqModal draws the prerequisites modal
func (g *Game) DrawPrereqModal(screen *ebiten.Image) {
	if !g.ui.ShowPrereqModal {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 600.0, 500.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	// Draw dark overlay
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 200}, false)

	// Draw modal background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 25, 35, 250}, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{218, 165, 32, 255}, true)

	// Title
	titleY := y + padding
	title := g.ui.PrereqModalTitle
	if title == "" {
		title = "PRÉREQUIS MANQUANTS"
	}
	ebitenutil.DebugPrintAt(screen, title, int(x+padding), int(titleY))

	// Subtitle (optional)
	if g.ui.PrereqModalSubtitle != "" {
		subtitleY := titleY + 25
		ebitenutil.DebugPrintAt(screen, g.ui.PrereqModalSubtitle, int(x+padding), int(subtitleY))
	}

	// Separator
	separatorY := titleY + 50
	if g.ui.PrereqModalSubtitle != "" {
		separatorY = titleY + 70
	}
	vector.StrokeLine(screen, float32(x+padding), float32(separatorY), float32(x+w-padding), float32(separatorY), 1, color.Gray{100}, true)

	// Requirements list (scrollable area)
	listStartY := separatorY + 15
	listEndY := y + h - 80 // Leave space for OK button
	listHeight := listEndY - listStartY
	maxVisibleItems := int(listHeight / 25) // ~25px per item

	// Calculate scroll bounds
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

	// Draw requirements
	currentY := listStartY
	startIdx := int(g.ui.PrereqModalScroll)
	endIdx := startIdx + maxVisibleItems
	if endIdx > totalItems {
		endIdx = totalItems
	}

	if totalItems == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun prérequis spécifié", int(x+padding), int(currentY))
	} else {
		for i := startIdx; i < endIdx; i++ {
			if i >= len(g.ui.PrereqModalReqs) {
				break
			}
			req := g.ui.PrereqModalReqs[i]
			reqText := formatRequirement(req)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("• %s", reqText), int(x+padding+10), int(currentY))
			currentY += 25
		}

		// Scroll indicator
		if totalItems > maxVisibleItems {
			scrollBarX := x + w - padding - 10
			scrollBarY := listStartY
			scrollBarH := listHeight
			scrollBarW := 5.0

			// Background
			vector.DrawFilledRect(screen, float32(scrollBarX), float32(scrollBarY), float32(scrollBarW), float32(scrollBarH), color.Gray{60}, true)

			// Thumb
			thumbHeight := float64(maxVisibleItems) / float64(totalItems) * scrollBarH
			thumbY := scrollBarY + (g.ui.PrereqModalScroll/float64(maxScroll))*scrollBarH
			if maxScroll == 0 {
				thumbY = scrollBarY
			}
			vector.DrawFilledRect(screen, float32(scrollBarX), float32(thumbY), float32(scrollBarW), float32(thumbHeight), color.Gray{150}, true)
		}
	}

	// OK button
	btnY := y + h - 50
	btnW := 120.0
	btnH := 35.0
	btnX := x + w/2 - btnW/2
	btnCol := color.RGBA{0, 150, 0, 255}
	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
	vector.StrokeRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "OK", int(btnX)+45, int(btnY)+10)

	// Close button (X) in top right
	closeBtnSize := 25.0
	closeBtnX := x + w - padding - closeBtnSize
	closeBtnY := y + padding
	vector.DrawFilledRect(screen, float32(closeBtnX), float32(closeBtnY), float32(closeBtnSize), float32(closeBtnSize), color.RGBA{100, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(closeBtnX), float32(closeBtnY), float32(closeBtnSize), float32(closeBtnSize), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeBtnX)+8, int(closeBtnY)+5)
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

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 600.0, 500.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.closePrereqModal()
		return true
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// Close button (X)
		closeBtnSize := 25.0
		closeBtnX := x + w - padding - closeBtnSize
		closeBtnY := y + padding
		if fmx >= closeBtnX && fmx <= closeBtnX+closeBtnSize && fmy >= closeBtnY && fmy <= closeBtnY+closeBtnSize {
			g.closePrereqModal()
			return true
		}

		// OK button
		btnY := y + h - 50
		btnW := 120.0
		btnH := 35.0
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
	_, dy := ebiten.Wheel()
	if dy != 0 {
		totalItems := len(g.ui.PrereqModalReqs)
		listHeight := (y + h - 80) - (y + 90) // Approximate list height
		maxVisibleItems := int(listHeight / 25)
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
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		totalItems := len(g.ui.PrereqModalReqs)
		listHeight := (y + h - 80) - (y + 90)
		maxVisibleItems := int(listHeight / 25)
		maxScroll := 0
		if totalItems > maxVisibleItems {
			maxScroll = totalItems - maxVisibleItems
		}
		if g.ui.PrereqModalScroll < float64(maxScroll) {
			g.ui.PrereqModalScroll++
		}
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
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
