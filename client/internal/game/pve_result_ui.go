package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawPveResultUI draws the PVE combat result UI
func (g *Game) DrawPveResultUI(screen *ebiten.Image) {
	if !g.ui.ShowPveResultUI {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)

	drawDimBackground(screen, 170)

	// Window dimensions
	cx, cy := sw/2, sh/2
	w, h := 800.0, 600.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	// Fenêtre bois et or (titre + filet doré à y+50)
	headerH := 50.0
	drawWindowFrame(screen, x, y, w, h, "Résultat du combat")

	// Close button (X)
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - padding
	closeY := y + padding
	drawCloseButton(screen, float64(closeX)+float64(closeBtnSize)/2, float64(closeY)+float64(closeBtnSize)/2, 15)

	// Content area
	contentY := y + headerH + padding
	contentH := h - headerH - padding*2 - 40
	lineSpacing := 18.0
	currentY := contentY

	// Handle case where result is nil (defensive)
	if g.ui.PveCombatResult == nil {
		uiPrint(screen, "Résultat indisponible", int(x+padding), int(currentY))
		currentY += lineSpacing * 2
		uiPrint(screen, "Le résultat du combat n'a pas pu être chargé.", int(x+padding), int(currentY))
		currentY += lineSpacing * 2
		// Footer
		footerY := y + h - 35
		drawLabel(screen, "Échap ou X pour fermer", x+padding, footerY, 12, false, themeMuted)
		return
	}

	result := g.ui.PveCombatResult

	// Winner - Big colored title
	winnerText := "Égalité"
	var winnerColor color.Color = themeMuted
	switch result.Winner {
	case "fleet_a":
		winnerText = "Victoire !"
		winnerColor = themeGreen
	case "fleet_b":
		winnerText = "Défaite"
		winnerColor = color.RGBA{232, 96, 80, 255}
	}
	drawText(screen, winnerText, titleFace(30), x+padding, currentY-6, winnerColor, alignLeft)
	currentY += lineSpacing * 2.4

	// Reason message (if present) - displayed below winner
	if result.ReasonMessage != "" {
		uiPrint(screen, result.ReasonMessage, int(x+padding), int(currentY))
		currentY += lineSpacing
	}
	currentY += lineSpacing

	// Summary section
	drawLabel(screen, "Résumé", x+padding, currentY-2, 15, true, themeGold)
	currentY += lineSpacing

	// Rounds played
	uiPrint(screen, fmt.Sprintf("Manches jouées : %d", len(result.Rounds)), int(x+padding+10), int(currentY))
	currentY += lineSpacing

	// Flagship info (if available from rounds)
	if len(result.Rounds) > 0 {
		firstRound := result.Rounds[0]
		// Try to find flagship info from first attack or round
		if len(firstRound.Attacks) > 0 {
			// Show first attacker as flagship (simplified)
			uiPrint(screen, fmt.Sprintf("Navire amiral : %s", firstRound.Attacks[0].AttackerType), int(x+padding+10), int(currentY))
			currentY += lineSpacing
		}
	}

	// Ships destroyed
	uiPrint(screen, fmt.Sprintf("Navires perdus : %d", len(result.ShipsDestroyedA)), int(x+padding+10), int(currentY))
	currentY += lineSpacing
	uiPrint(screen, fmt.Sprintf("Navires ennemis coulés : %d", len(result.ShipsDestroyedB)), int(x+padding+10), int(currentY))
	currentY += lineSpacing

	// Captain injured
	if result.CaptainInjuredA != nil {
		// Try to find captain name
		captainName := result.CaptainInjuredA.String()[:8]
		for i := range g.captains {
			if g.captains[i].ID == *result.CaptainInjuredA {
				captainName = g.captains[i].Name
				break
			}
		}
		uiPrint(screen, fmt.Sprintf("Capitaine blessé : %s", captainName), int(x+padding+10), int(currentY))
		currentY += lineSpacing
	}
	currentY += lineSpacing

	// Rounds details section
	if len(result.Rounds) == 0 {
		drawLabel(screen, "Aucune manche jouée", x+padding, currentY-2, 15, true, themeGold)
		currentY += lineSpacing
		if result.ReasonMessage != "" {
			uiPrint(screen, fmt.Sprintf("Raison : %s", result.ReasonMessage), int(x+padding+10), int(currentY))
		} else if result.ReasonCode != "" {
			uiPrint(screen, fmt.Sprintf("Code: %s", result.ReasonCode), int(x+padding+10), int(currentY))
		}
		currentY += lineSpacing * 2
	} else {
		drawLabel(screen, "Déroulé du combat", x+padding, currentY-2, 15, true, themeGold)
		currentY += lineSpacing * 2
	}

	// Scrollable rounds list
	roundStartY := currentY
	roundLineHeight := 60.0
	visibleRounds := int((contentH - (currentY - contentY)) / roundLineHeight)

	startRound := int(g.ui.PveResultScroll / roundLineHeight)
	if startRound < 0 {
		startRound = 0
	}
	if startRound > len(result.Rounds) {
		startRound = len(result.Rounds)
	}

	for i := startRound; i < len(result.Rounds) && i < startRound+visibleRounds+1; i++ {
		if i < 0 || i >= len(result.Rounds) {
			continue
		}
		round := result.Rounds[i]
		roundY := roundStartY - g.ui.PveResultScroll + float64(i-startRound)*roundLineHeight

		if roundY < contentY || roundY+roundLineHeight > contentY+contentH {
			continue
		}

		// Round header
		uiPrint(screen, fmt.Sprintf("Manche %d · navires restants : toi %d, ennemi %d", round.RoundNumber, round.ShipsAliveA, round.ShipsAliveB), int(x+padding), int(roundY))

		// Attacks (truncated if too many)
		maxAttacks := 3
		attacksToShow := round.Attacks
		if len(attacksToShow) > maxAttacks {
			attacksToShow = attacksToShow[:maxAttacks]
		}
		for j, attack := range attacksToShow {
			attackY := roundY + 18 + float64(j)*14
			if attackY+14 > contentY+contentH {
				break
			}
			destroyed := ""
			if attack.TargetDestroyed {
				destroyed = " (coulé)"
			}
			attackText := fmt.Sprintf("  %s → %s : %.0f dégâts%s", attack.AttackerType, attack.TargetType, attack.DamageDealt, destroyed)
			if len(attackText) > 60 {
				attackText = attackText[:60] + "..."
			}
			uiPrint(screen, attackText, int(x+padding+10), int(attackY))
		}
		if len(round.Attacks) > maxAttacks {
			uiPrint(screen, fmt.Sprintf("  ... et %d autres attaques", len(round.Attacks)-maxAttacks), int(x+padding+10), int(roundY+18+float64(maxAttacks)*14))
		}
	}

	// Footer
	footerY := y + h - 35
	drawLabel(screen, "Échap ou X pour fermer · molette pour faire défiler", x+padding, footerY, 12, false, themeMuted)
}

// UpdatePveResultUI handles input for the PVE result UI
func (g *Game) UpdatePveResultUI() bool {
	if !g.ui.ShowPveResultUI {
		return false
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := sw/2, sh/2
	w, h := 800.0, 600.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0
	headerH := 50.0
	contentY := y + headerH + padding
	contentH := h - headerH - padding*2 - 40

	// Close on ESC
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowPveResultUI = false
		g.Log("[PVE UI] close")
		return true
	}

	// Handle mouse wheel scrolling
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	if fmx >= x && fmx <= x+w && fmy >= contentY && fmy <= contentY+contentH {
		_, dy := wheelDelta()
		if dy != 0 {
			g.ui.PveResultScroll -= dy * 20.0
			if g.ui.PveResultScroll < 0 {
				g.ui.PveResultScroll = 0
			}
			// Max scroll
			if g.ui.PveCombatResult != nil {
				roundLineHeight := 60.0
				maxScroll := float64(len(g.ui.PveCombatResult.Rounds)) * roundLineHeight
				if maxScroll < contentH {
					maxScroll = 0
				} else {
					maxScroll = maxScroll - contentH + padding*2
				}
				if g.ui.PveResultScroll > maxScroll {
					g.ui.PveResultScroll = maxScroll
				}
			}
			return true
		}
	}

	// Handle mouse clicks
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		// Close button
		closeBtnSize := 30.0
		closeX := x + w - closeBtnSize - padding
		closeY := y + padding
		if fmx >= closeX && fmx <= closeX+closeBtnSize && fmy >= closeY && fmy <= closeY+closeBtnSize {
			g.ui.ShowPveResultUI = false
			g.Log("[PVE UI] close")
			return true
		}

		// Click outside modal (close)
		if fmx < x || fmx > x+w || fmy < y || fmy > y+h {
			g.ui.ShowPveResultUI = false
			g.Log("[PVE UI] close")
			return true
		}

		// Click inside modal (consume input)
		return true
	}

	return false
}
