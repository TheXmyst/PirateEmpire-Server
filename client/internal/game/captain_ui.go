package game

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawCaptainUI draws the Captain roster/encyclopedia UI
func (g *Game) DrawCaptainUI(screen *ebiten.Image) {
	if !g.ui.ShowCaptainUI {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)

	drawDimBackground(screen, 150)

	// Window dimensions
	cx, cy := sw/2, sh/2
	w, h := 900.0, 600.0
	x, y := cx-w/2, cy-h/2

	drawThemePanel(screen, x, y, w, h, 18)

	// Closing button (X)
	padding := 20.0
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - padding
	closeY := y + padding
	drawCloseButton(screen, closeX+closeBtnSize/2, closeY+closeBtnSize/2, 16)

	// Tabs
	tabW := 150.0
	tabH := 30.0
	tabY := y + 10 // Top aligned

	// Titre et onglets
	drawText(screen, "Capitaines", titleFace(28), x+w/2, y+10, themeGold, alignCenter)
	drawTab(screen, x+padding, tabY, tabW, tabH, "Liste", g.ui.CaptainUITab == "list" || g.ui.CaptainUITab == "")
	drawTab(screen, x+padding+tabW+10, tabY, tabW, tabH, "Améliorer", g.ui.CaptainUITab == "upgrade")

	// Content bounds
	contentY := tabY + tabH + 10
	contentH := h - (contentY - y) - 20

	// Busy state
	if g.ui.CaptainUIBusy {
		uiPrint(screen, "Chargement...", int(x+padding), int(contentY+padding))
		return
	}
	// Error display
	if g.ui.CaptainUIError != "" {
		uiPrint(screen, fmt.Sprintf("Erreur: %s", g.ui.CaptainUIError), int(x+padding), int(contentY+padding))
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
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowCaptainUI = false
		return true
	}

	// Don't process clicks if busy
	if g.ui.CaptainUIBusy {
		return true
	}

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
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
			g.ui.CaptainUIError = friendlyError(err)
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

	// Colonne de droite : le détail (reste de la largeur)
	rightX := leftX + leftW + padding
	rightW := x + w - padding - rightX
	rightH := h

	drawInset(screen, leftX, y, leftW, leftH)
	drawInset(screen, rightX, y, rightW, rightH)

	// Draw captain list (left column)
	if len(g.captains) == 0 {
		uiPrint(screen, "Aucun capitaine", int(leftX+padding), int(y+padding))
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
			drawCard(screen, leftX+6, itemY, leftW-12, lineHeight-5, isSelected)
			vector.DrawFilledCircle(screen, float32(leftX+22), float32(itemY+lineHeight/2-2), 6, rarityColor(captain.Rarity), false)

			// Nom, rareté, étoiles
			nameY := int(itemY + 5)
			drawLabel(screen, captain.Name, leftX+36, float64(nameY), 14, true, themeText)
			drawLabel(screen, rarityLabel(captain.Rarity), leftX+36, float64(nameY)+20, 12, false, rarityColor(captain.Rarity))

			// Stars
			maxStars := 3
			switch captain.Rarity {
			case "rare":
				maxStars = 4
			case "legendary":
				maxStars = 5
			}
			drawStars(screen, leftX+leftW-20, float64(nameY)+12, captain.Stars, maxStars, 7, true)
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

			// Nom et rareté
			drawText(screen, selectedCaptain.Name, titleFace(30), rightX+padding, detailY-4, themeGold, alignLeft)
			drawLabel(screen, rarityLabel(selectedCaptain.Rarity), rightX+padding, detailY+34, 14, true, rarityColor(selectedCaptain.Rarity))
			detailY += lineSpacing * 3

			// Stars
			maxStars := 3
			switch selectedCaptain.Rarity {
			case "rare":
				maxStars = 4
			case "legendary":
				maxStars = 5
			}
			drawStars(screen, rightX+padding, detailY+8, selectedCaptain.Stars, maxStars, 9, false)
			detailY += lineSpacing * 2

			// Naval Bonuses (from stars)
			if selectedCaptain.NavalHPBonusPct != 0 || selectedCaptain.NavalSpeedBonusPct != 0 ||
				selectedCaptain.NavalDamageReductionPct != 0 || selectedCaptain.RumConsumptionReductionPct != 0 {
				drawLabel(screen, "Bonus navals", rightX+padding, detailY, 14, true, themeGold)
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
				uiPrint(screen, bonusText, int(rightX+padding+10), int(detailY))
				detailY += lineSpacing * 2
			}

			// Skill/Effect
			drawLabel(screen, "Compétence", rightX+padding, detailY, 14, true, themeGold)
			detailY += lineSpacing
			if selectedCaptain.SkillID != "" {
				uiPrint(screen, fmt.Sprintf("  ID: %s", selectedCaptain.SkillID), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveID != "" {
				uiPrint(screen, fmt.Sprintf("  Passive: %s", selectedCaptain.PassiveID), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveValue != 0 {
				uiPrint(screen, fmt.Sprintf("  Valeur: %.2f", selectedCaptain.PassiveValue), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.PassiveIntValue != 0 {
				uiPrint(screen, fmt.Sprintf("  Valeur (int): %d", selectedCaptain.PassiveIntValue), int(rightX+padding+10), int(detailY))
				detailY += lineSpacing
			}
			if selectedCaptain.Threshold != 0 {
				uiPrint(screen, fmt.Sprintf("  Seuil: %d", selectedCaptain.Threshold), int(rightX+padding+10), int(detailY))
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
			uiPrint(screen, statusText, int(rightX+padding), int(detailY))
		}
	} else {
		uiPrint(screen, "Sélectionnez un capitaine", int(rightX+padding), int(y+padding))
	}
}

// drawCaptainUpgradeContent draws the upgrade view
func (g *Game) drawCaptainUpgradeContent(screen *ebiten.Image, x, y, w, h float64) {
	padding := 20.0
	// Full width list
	listX := x + padding
	listW := w - padding*2
	listH := h

	drawInset(screen, listX, y, listW, listH)

	if len(g.captains) == 0 {
		uiPrint(screen, "Aucun capitaine", int(listX+padding), int(y+padding))
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

		drawCard(screen, listX+6, itemY+4, listW-12, lineHeight-8, false)
		// Médaillon de rareté
		vector.DrawFilledCircle(screen, float32(listX+44), float32(itemY+40), 28, color.RGBA{60, 38, 20, 255}, false)
		vector.StrokeCircle(screen, float32(listX+44), float32(itemY+40), 28, 3, rarityColor(captain.Rarity), false)
		if r := []rune(captain.Name); len(r) > 0 {
			drawText(screen, strings.ToUpper(string(r[0])), titleFace(28), listX+44, itemY+22, rarityColor(captain.Rarity), alignCenter)
		}

		// Nom et rareté
		drawLabel(screen, captain.Name, listX+86, itemY+18, 15, true, themeText)
		drawLabel(screen, rarityLabel(captain.Rarity), listX+86, itemY+40, 12.5, false, rarityColor(captain.Rarity))

		// Stars
		maxStars := 3
		switch captain.Rarity {
		case "rare":
			maxStars = 4
		case "legendary":
			maxStars = 5
		}
		drawStars(screen, listX+230, itemY+38, captain.Stars, maxStars, 9, false)

		// Progress / Cost
		// Need logic to show "MAX" if max stars
		if captain.Stars >= maxStars {
			drawLabel(screen, "Étoiles au maximum", listX+380, itemY+30, 13, true, themeGold)
		} else {
			// Shards Progress
			progressText := fmt.Sprintf("%d / %d éclats", captain.Shards, captain.NextStarCost)
			drawLabel(screen, progressText, listX+380, itemY+20, 13, true, themeText)
			p := 0.0
			if captain.NextStarCost > 0 {
				p = float64(captain.Shards) / float64(captain.NextStarCost)
			}
			drawProgressBar(screen, listX+380, itemY+44, 200, 8, p, themeGold)

			// Upgrade Button
			btnX := listX + listW - 120
			btnY := itemY + 20
			btnW := 100.0
			btnH := 40.0

			kind := btnDisabled
			if captain.CanUpgrade {
				kind = btnPrimary
			}
			drawButton(screen, btnX, btnY, btnW, btnH, "Améliorer", kind)
		}
	}
}

// updateCaptainListTab handles input for the list tab
func (g *Game) updateCaptainListTab(x, y, w, h float64) bool {
	padding := 20.0
	leftW := w * 0.35
	leftX := x + padding

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll Logic
	if fmx >= leftX && fmx <= leftX+leftW && fmy >= y && fmy <= y+h {
		_, dy := wheelDelta()
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
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
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

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll
	if fmx >= listX && fmx <= listX+listW && fmy >= y && fmy <= y+h {
		_, dy := wheelDelta()
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

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
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
