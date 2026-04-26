package game

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawCaptainUI draws the Captain roster/encyclopedia UI
func (g *Game) DrawCaptainUI(screen *ebiten.Image) {
	if !g.ui.ShowCaptainUI {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)

	// Draw dark overlay
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 153}, false)

	// Window dimensions
	cx, cy := sw/2, sh/2
	w, h := 900.0, 600.0
	x, y := cx-w/2, cy-h/2

	// Draw Window Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{10, 15, 25, 240}, true)
	// Border
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{218, 165, 32, 255}, true)

	// Closing button (X)
	padding := 20.0
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - padding
	closeY := y + padding
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), color.RGBA{150, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+8)

	// Tabs
	tabW := 150.0
	tabH := 30.0
	tabY := y + 10 // Top aligned

	// Tab List
	listColor := color.RGBA{40, 40, 60, 255}
	if g.ui.CaptainUITab == "list" || g.ui.CaptainUITab == "" {
		listColor = color.RGBA{80, 80, 150, 255}
	}
	vector.DrawFilledRect(screen, float32(x+padding), float32(tabY), float32(tabW), float32(tabH), listColor, true)
	ebitenutil.DebugPrintAt(screen, "LISTE", int(x+padding+50), int(tabY+8))

	// Tab Upgrade
	upgradeColor := color.RGBA{40, 40, 60, 255}
	if g.ui.CaptainUITab == "upgrade" {
		upgradeColor = color.RGBA{80, 80, 150, 255}
	}
	vector.DrawFilledRect(screen, float32(x+padding+tabW+10), float32(tabY), float32(tabW), float32(tabH), upgradeColor, true)
	ebitenutil.DebugPrintAt(screen, "AMÉLIORER", int(x+padding+tabW+10+30), int(tabY+8))

	// Content bounds
	contentY := tabY + tabH + 10
	contentH := h - (contentY - y) - 20

	// Busy state
	if g.ui.CaptainUIBusy {
		ebitenutil.DebugPrintAt(screen, "Chargement...", int(x+padding), int(contentY+padding))
		return
	}
	// Error display
	if g.ui.CaptainUIError != "" {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Erreur: %s", g.ui.CaptainUIError), int(x+padding), int(contentY+padding))
		return
	}

	if g.ui.CaptainUITab == "upgrade" {
		g.drawCaptainUpgradeContent(screen, x, contentY, w, contentH)
	} else {
		g.drawCaptainListContent(screen, x, contentY, w, contentH)
	}
}

// formatStars formats stars as "★★★☆☆" based on current and max stars
func formatStars(current, max int) string {
	if current < 0 {
		current = 0
	}
	if current > max {
		current = max
	}
	filled := strings.Repeat("★", current)
	empty := strings.Repeat("☆", max-current)
	return filled + empty
}

// UpdateCaptainUI handles input for the Captain UI
func (g *Game) UpdateCaptainUI() bool {
	if !g.ui.ShowCaptainUI {
		return false
	}

	// Recalculate dims (must match Draw)
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 900.0, 600.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowCaptainUI = false
		return true
	}

	// Don't process clicks if busy
	if g.ui.CaptainUIBusy {
		return true
	}

	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Close button (X)
		closeBtnSize := 30.0
		closeX := x + w - closeBtnSize - padding
		closeY := y + padding
		if fmx >= closeX && fmx <= closeX+closeBtnSize && fmy >= closeY && fmy <= closeY+closeBtnSize {
			g.ui.ShowCaptainUI = false
			return true
		}

		// Tabs
		tabW := 150.0
		tabH := 30.0
		tabY := y + 10

		// Tab List Click
		if fmx >= x+padding && fmx <= x+padding+tabW && fmy >= tabY && fmy <= tabY+tabH {
			g.ui.CaptainUITab = "list"
			return true // Consume click
		}

		// Tab Upgrade Click
		if fmx >= x+padding+tabW+10 && fmx <= x+padding+tabW+10+tabW && fmy >= tabY && fmy <= tabY+tabH {
			g.ui.CaptainUITab = "upgrade"
			return true // Consume click
		}
	}

	// Content Logic Delegation
	contentY := y + 10 + 30 + 10 // tabY + tabH + 10
	contentH := h - (contentY - y) - 20

	if g.ui.CaptainUITab == "upgrade" {
		return g.updateCaptainUpgradeTab(x, contentY, w, contentH)
	} else {
		return g.updateCaptainListTab(x, contentY, w, contentH)
	}
}

// loadCaptainsForUI loads captains when UI is opened
func (g *Game) loadCaptainsForUI() {
	if g.ui.CaptainUIBusy {
		return // Already loading
	}

	g.ui.CaptainUIBusy = true
	g.ui.CaptainUIError = ""

	go func() {
		defer func() {
			g.ui.CaptainUIBusy = false
		}()

		captains, err := g.api.GetCaptains()
		if err != nil {
			g.ui.CaptainUIError = err.Error()
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.CaptainUIError = "Session expirée, veuillez vous reconnecter"
			}
			g.Log("Captain UI load error: %v", err)
			return
		}

		g.captains = captains

		// Select first captain if none selected and we have captains
		if g.ui.CaptainUISelectedID == nil && len(captains) > 0 {
			captainID := captains[0].ID
			g.ui.CaptainUISelectedID = &captainID
		}

		g.Log("Captain UI: loaded %d captains", len(captains))
	}()
}

// drawCaptainListContent draws the standard list/details view
func (g *Game) drawCaptainListContent(screen *ebiten.Image, x, y, w, h float64) {
	padding := 20.0
	// Left column (35%) - List
	leftW := w * 0.35
	leftX := x + padding
	leftH := h

	// Right column (65%) - Details
	rightW := w * 0.65
	rightX := leftX + leftW + padding
	rightH := h

	// Draw columns background
	vector.DrawFilledRect(screen, float32(leftX), float32(y), float32(leftW), float32(leftH), color.RGBA{20, 25, 35, 240}, true)
	vector.DrawFilledRect(screen, float32(rightX), float32(y), float32(rightW), float32(rightH), color.RGBA{20, 25, 35, 240}, true)

	// Column separator
	vector.StrokeLine(screen, float32(rightX-padding/2), float32(y), float32(rightX-padding/2), float32(y+rightH), 1, color.Gray{80}, true)

	// Draw captain list (left column)
	if len(g.captains) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun capitaine", int(leftX+padding), int(y+padding))
	} else {
		lineHeight := 50.0
		visibleLines := int(leftH / lineHeight)
		startIndex := int(g.ui.CaptainUIScroll / lineHeight)
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(g.captains) {
			startIndex = len(g.captains)
		}

		currentY := y + padding
		for i := startIndex; i < len(g.captains) && i < startIndex+visibleLines+1; i++ {
			if i < 0 || i >= len(g.captains) {
				continue
			}
			captain := g.captains[i]

			// Highlight selected captain
			isSelected := g.ui.CaptainUISelectedID != nil && captain.ID == *g.ui.CaptainUISelectedID
			itemY := currentY - g.ui.CaptainUIScroll + float64(i-startIndex)*lineHeight

			if itemY < y || itemY+lineHeight > y+leftH {
				continue // Skip if outside visible area
			}

			// Background for selected item
			if isSelected {
				vector.DrawFilledRect(screen, float32(leftX+5), float32(itemY), float32(leftW-10), float32(lineHeight-5), color.RGBA{50, 100, 150, 200}, true)
			}

			// Captain name
			nameY := int(itemY + 8)
			ebitenutil.DebugPrintAt(screen, captain.Name, int(leftX+padding), nameY)

			// Rarity
			rarityText := strings.ToUpper(captain.Rarity)
			ebitenutil.DebugPrintAt(screen, rarityText, int(leftX+padding), nameY+15)

			// Stars
			maxStars := 3
			switch captain.Rarity {
			case "rare":
				maxStars = 4
			case "legendary":
				maxStars = 5
			}
			starsText := formatStars(captain.Stars, maxStars)
			ebitenutil.DebugPrintAt(screen, starsText, int(leftX+padding), nameY+28)
		}
	}

	// Draw captain details (right column)
	if g.ui.CaptainUISelectedID != nil {
		var selectedCaptain *client.Captain
		for i := range g.captains {
			if g.captains[i].ID == *g.ui.CaptainUISelectedID {
				selectedCaptain = &g.captains[i]
				break
			}
		}

		if selectedCaptain != nil {
			detailY := y + padding
			lineSpacing := 20.0

			// Name and Rarity
			ebitenutil.DebugPrintAt(screen, selectedCaptain.Name, int(rightX+padding), int(detailY))
			rarityText := fmt.Sprintf("Rareté: %s", strings.ToUpper(selectedCaptain.Rarity))
			ebitenutil.DebugPrintAt(screen, rarityText, int(rightX+padding), int(detailY+lineSpacing))
			detailY += lineSpacing * 2

			// Stars
			maxStars := 3
			switch selectedCaptain.Rarity {
			case "rare":
				maxStars = 4
			case "legendary":
				maxStars = 5
			}
			starsText := formatStars(selectedCaptain.Stars, maxStars)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Étoiles: %s", starsText), int(rightX+padding), int(detailY))
			detailY += lineSpacing * 2

			// Naval Bonuses (from stars)
			if selectedCaptain.NavalHPBonusPct != 0 || selectedCaptain.NavalSpeedBonusPct != 0 ||
				selectedCaptain.NavalDamageReductionPct != 0 || selectedCaptain.RumConsumptionReductionPct != 0 {
				ebitenutil.DebugPrintAt(screen, "Bonus Navals:", int(rightX+padding), int(detailY))
				detailY += lineSpacing

				bonuses := []string{}
				if selectedCaptain.NavalHPBonusPct != 0 {
					bonuses = append(bonuses, fmt.Sprintf("HP: +%.1f%%", selectedCaptain.NavalHPBonusPct))
				}
				if selectedCaptain.NavalSpeedBonusPct != 0 {
					bonuses = append(bonuses, fmt.Sprintf("Speed: +%.1f%%", selectedCaptain.NavalSpeedBonusPct))
				}
				if selectedCaptain.NavalDamageReductionPct != 0 {
					bonuses = append(bonuses, fmt.Sprintf("DR: +%.1f%%", selectedCaptain.NavalDamageReductionPct))
				}
				if selectedCaptain.RumConsumptionReductionPct != 0 {
					bonuses = append(bonuses, fmt.Sprintf("Rum: -%.1f%%", selectedCaptain.RumConsumptionReductionPct))
				}

				bonusText := strings.Join(bonuses, " ")
				ebitenutil.DebugPrintAt(screen, bonusText, int(rightX+padding+10), int(detailY))
				detailY += lineSpacing * 2
			}

			// Skill/Effect
			ebitenutil.DebugPrintAt(screen, "Compétence:", int(rightX+padding), int(detailY))
			detailY += lineSpacing
			if selectedCaptain.SkillID != "" {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  ID: %s", selectedCaptain.SkillID), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveID != "" {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  Passive: %s", selectedCaptain.PassiveID), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveValue != 0 {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  Valeur: %.2f", selectedCaptain.PassiveValue), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveIntValue != 0 {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  Valeur (int): %d", selectedCaptain.PassiveIntValue), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.Threshold != 0 {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  Seuil: %d", selectedCaptain.Threshold), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			detailY += lineSpacing

			// Status
			statusText := "Statut: Disponible"
			if selectedCaptain.InjuredUntil != nil && selectedCaptain.InjuredUntil.After(time.Now()) {
				remaining := time.Until(*selectedCaptain.InjuredUntil)
				statusText = fmt.Sprintf("Statut: Blessé (%dm)", int(remaining.Minutes()))
			} else if selectedCaptain.AssignedShipID != nil {
				statusText = "Statut: Assigné à un navire"
			}
			ebitenutil.DebugPrintAt(screen, statusText, int(rightX+padding), int(detailY))
		}
	} else {
		ebitenutil.DebugPrintAt(screen, "Sélectionnez un capitaine", int(rightX+padding), int(y+padding))
	}
}

// drawCaptainUpgradeContent draws the upgrade view
func (g *Game) drawCaptainUpgradeContent(screen *ebiten.Image, x, y, w, h float64) {
	padding := 20.0
	// Full width list
	listX := x + padding
	listW := w - padding*2
	listH := h

	vector.DrawFilledRect(screen, float32(listX), float32(y), float32(listW), float32(listH), color.RGBA{20, 25, 35, 240}, true)

	if len(g.captains) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun capitaine", int(listX+padding), int(y+padding))
		return
	}

	lineHeight := 80.0
	visibleLines := int(listH / lineHeight)
	startIndex := int(g.ui.CaptainUIScroll / lineHeight)
	if startIndex < 0 {
		startIndex = 0
	}

	currentY := y + padding

	for i := startIndex; i < len(g.captains) && i < startIndex+visibleLines+1; i++ {
		if i < 0 || i >= len(g.captains) {
			continue
		}
		captain := g.captains[i]

		itemY := currentY - g.ui.CaptainUIScroll + float64(i-startIndex)*lineHeight
		if itemY < y || itemY+lineHeight > y+listH {
			continue
		}

		// Alternating background
		if i%2 == 0 {
			vector.DrawFilledRect(screen, float32(listX), float32(itemY), float32(listW), float32(lineHeight), color.RGBA{30, 35, 45, 100}, true)
		}

		// Icon (Placeholder) - Color by rarity
		rarityColor := color.RGBA{150, 150, 150, 255} // Common
		if captain.Rarity == "rare" {
			rarityColor = color.RGBA{0, 150, 255, 255}
		}
		if captain.Rarity == "legendary" {
			rarityColor = color.RGBA{255, 215, 0, 255}
		}
		vector.DrawFilledRect(screen, float32(listX+10), float32(itemY+10), 60, 60, rarityColor, true)

		// Name & Rarity
		ebitenutil.DebugPrintAt(screen, captain.Name, int(listX+80), int(itemY+15))
		ebitenutil.DebugPrintAt(screen, strings.ToUpper(captain.Rarity), int(listX+80), int(itemY+35))

		// Stars
		maxStars := 3
		switch captain.Rarity {
		case "rare":
			maxStars = 4
		case "legendary":
			maxStars = 5
		}
		starsText := formatStars(captain.Stars, maxStars)
		ebitenutil.DebugPrintAt(screen, starsText, int(listX+200), int(itemY+25))

		// Progress / Cost
		// Need logic to show "MAX" if max stars
		if captain.Stars >= maxStars {
			ebitenutil.DebugPrintAt(screen, "MAX STARS", int(listX+350), int(itemY+25))
		} else {
			// Shards Progress
			progressText := fmt.Sprintf("%d / %d Éclats", captain.Shards, captain.NextStarCost)
			// Draw progress text
			ebitenutil.DebugPrintAt(screen, progressText, int(listX+350), int(itemY+25))
			if captain.Shards >= captain.NextStarCost {
				ebitenutil.DebugPrintAt(screen, "(Prêt!)", int(listX+350), int(itemY+40))
			}

			// Upgrade Button
			btnX := listX + listW - 120
			btnY := itemY + 20
			btnW := 100.0
			btnH := 40.0

			btnColor := color.RGBA{60, 60, 60, 255} // Disabled
			if captain.CanUpgrade {
				btnColor = color.RGBA{0, 200, 100, 255} // Green
				// Hover effect? Need mouse pos. Handled in Update generally, here simplify.
			}
			vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnColor, true)
			vector.StrokeRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), 1, color.White, true)
			ebitenutil.DebugPrintAt(screen, "UPGRADE", int(btnX+10), int(btnY+12))
		}
	}
}

// updateCaptainListTab handles input for the list tab
func (g *Game) updateCaptainListTab(x, y, w, h float64) bool {
	padding := 20.0
	leftW := w * 0.35
	leftX := x + padding

	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll Logic
	if fmx >= leftX && fmx <= leftX+leftW && fmy >= y && fmy <= y+h {
		_, dy := ebiten.Wheel()
		if dy != 0 {
			g.ui.CaptainUIScroll -= dy * 20.0
			if g.ui.CaptainUIScroll < 0 {
				g.ui.CaptainUIScroll = 0
			}
			// Max scroll calculation
			lineHeight := 50.0
			maxScroll := float64(len(g.captains))*lineHeight - h + padding*2
			if maxScroll < 0 {
				maxScroll = 0
			}
			if g.ui.CaptainUIScroll > maxScroll {
				g.ui.CaptainUIScroll = maxScroll
			}
			return true
		}
	}

	// Click Logic
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Click on captain in list (left column)
		if fmx >= leftX && fmx <= leftX+leftW && fmy >= y && fmy <= y+h {
			lineHeight := 50.0
			// Calculate which captain was clicked based on scroll position
			relativeY := fmy - y
			clickedIndex := int((relativeY + g.ui.CaptainUIScroll - padding) / lineHeight)

			if clickedIndex >= 0 && clickedIndex < len(g.captains) {
				captainID := g.captains[clickedIndex].ID
				g.ui.CaptainUISelectedID = &captainID
				return true
			}
		}
	}
	return false
}

// updateCaptainUpgradeTab handles input for the upgrade tab
func (g *Game) updateCaptainUpgradeTab(x, y, w, h float64) bool {
	padding := 20.0
	listX := x + padding
	listW := w - padding*2

	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll
	if fmx >= listX && fmx <= listX+listW && fmy >= y && fmy <= y+h {
		_, dy := ebiten.Wheel()
		if dy != 0 {
			g.ui.CaptainUIScroll -= dy * 20.0
			if g.ui.CaptainUIScroll < 0 {
				g.ui.CaptainUIScroll = 0
			}
			// Max scroll
			lineHeight := 80.0
			maxScroll := float64(len(g.captains))*lineHeight - h + padding*2
			if maxScroll < 0 {
				maxScroll = 0
			}
			if g.ui.CaptainUIScroll > maxScroll {
				g.ui.CaptainUIScroll = maxScroll
			}
			return true
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Upgrade Button Clicks
		lineHeight := 80.0
		startIndex := int(g.ui.CaptainUIScroll / lineHeight)
		currentY := y + padding

		for i := startIndex; i < len(g.captains); i++ {
			captain := g.captains[i]
			itemY := currentY - g.ui.CaptainUIScroll + float64(i-startIndex)*lineHeight
			if itemY < y || itemY+lineHeight > y+h {
				continue
			}

			btnX := listX + listW - 120
			btnY := itemY + 20
			btnW := 100.0
			btnH := 40.0

			if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				if captain.CanUpgrade {
					// Trigger Upgrade
					g.Log("Upgrading captain %s", captain.Name)
					// Disable button to prevent double click
					g.ui.CaptainUIBusy = true
					go func(id uuid.UUID) {
						defer func() { g.ui.CaptainUIBusy = false }()
						if err := g.api.UpgradeCaptainStars(id); err != nil {
							g.Log("Upgrade failed: %v", err)
						} else {
							// Reload captains to reflect change
							g.loadCaptainsForUI()
						}
					}(captain.ID)
				}
				return true
			}
		}
	}
	return false
}
