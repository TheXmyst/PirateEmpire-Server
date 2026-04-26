package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawPveResultUI draws the PVE combat result UI
func (g *Game) DrawPveResultUI(screen *ebiten.Image) {
	if !g.ui.ShowPveResultUI {
		return
	}

	sw, sh := float64(g.screenWidth), float64(g.screenHeight)

	// Draw dark overlay
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 153}, false)

	// Window dimensions
	cx, cy := sw/2, sh/2
	w, h := 800.0, 600.0
	x, y := cx-w/2, cy-h/2
	padding := 20.0

	// Draw Window Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{10, 15, 25, 240}, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{218, 165, 32, 255}, true)

	// Header
	headerH := 50.0
	ebitenutil.DebugPrintAt(screen, "RÉSULTAT DU COMBAT", int(x+padding), int(y+padding+10))

	// Close button (X)
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - padding
	closeY := y + padding
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), color.RGBA{150, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+8)

	// Separator
	vector.StrokeLine(screen, float32(x+padding), float32(y+headerH), float32(x+w-padding), float32(y+headerH), 1, color.Gray{100}, true)

	// Content area
	contentY := y + headerH + padding
	contentH := h - headerH - padding*2 - 40
	lineSpacing := 18.0
	currentY := contentY

	// Handle case where result is nil (defensive)
	if g.ui.PveCombatResult == nil {
		ebitenutil.DebugPrintAt(screen, "Résultat indisponible", int(x+padding), int(currentY))
		currentY += lineSpacing * 2
		ebitenutil.DebugPrintAt(screen, "Le résultat du combat n'a pas pu être chargé.", int(x+padding), int(currentY))
		currentY += lineSpacing * 2
		// Footer
		footerY := y + h - 35
		ebitenutil.DebugPrintAt(screen, "ESC ou X pour fermer", int(x+padding), int(footerY))
		return
	}

	result := g.ui.PveCombatResult

	// Winner - Big colored title
	winnerText := "ÉGALITÉ"
	var winnerColor color.RGBA = color.RGBA{150, 150, 150, 255}
	switch result.Winner {
	case "fleet_a":
		winnerText = "VICTOIRE"
		winnerColor = color.RGBA{0, 255, 0, 255}
	case "fleet_b":
		winnerText = "DÉFAITE"
		winnerColor = color.RGBA{255, 0, 0, 255}
	}

	// Draw colored background for winner text
	textW := float64(len(winnerText) * 8) // Approximate width
	textH := 30.0
	textX := x + padding
	textY := currentY
	vector.DrawFilledRect(screen, float32(textX-5), float32(textY-5), float32(textW+10), float32(textH), color.RGBA{winnerColor.R / 4, winnerColor.G / 4, winnerColor.B / 4, 200}, true)
	ebitenutil.DebugPrintAt(screen, winnerText, int(textX), int(textY+5))
	currentY += lineSpacing * 2

	// Reason message (if present) - displayed below winner
	if result.ReasonMessage != "" {
		ebitenutil.DebugPrintAt(screen, result.ReasonMessage, int(x+padding), int(currentY))
		currentY += lineSpacing
	}
	currentY += lineSpacing

	// Summary section
	ebitenutil.DebugPrintAt(screen, "--- RÉSUMÉ ---", int(x+padding), int(currentY))
	currentY += lineSpacing

	// Rounds played
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Rounds joués: %d", len(result.Rounds)), int(x+padding+10), int(currentY))
	currentY += lineSpacing

	// Flagship info (if available from rounds)
	if len(result.Rounds) > 0 {
		firstRound := result.Rounds[0]
		// Try to find flagship info from first attack or round
		if len(firstRound.Attacks) > 0 {
			// Show first attacker as flagship (simplified)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Flagship joueur: %s", firstRound.Attacks[0].AttackerType), int(x+padding+10), int(currentY))
			currentY += lineSpacing
		}
	}

	// Ships destroyed
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Navires détruits (vous): %d", len(result.ShipsDestroyedA)), int(x+padding+10), int(currentY))
	currentY += lineSpacing
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Navires détruits (ennemi): %d", len(result.ShipsDestroyedB)), int(x+padding+10), int(currentY))
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
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Capitaine blessé: %s", captainName), int(x+padding+10), int(currentY))
		currentY += lineSpacing
	}
	currentY += lineSpacing

	// Rounds details section
	if len(result.Rounds) == 0 {
		ebitenutil.DebugPrintAt(screen, "--- AUCUN ROUND JOUÉ ---", int(x+padding), int(currentY))
		currentY += lineSpacing
		if result.ReasonMessage != "" {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Raison: %s", result.ReasonMessage), int(x+padding+10), int(currentY))
		} else if result.ReasonCode != "" {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Code: %s", result.ReasonCode), int(x+padding+10), int(currentY))
		}
		currentY += lineSpacing * 2
	} else {
		ebitenutil.DebugPrintAt(screen, "--- DÉTAILS DES ROUNDS ---", int(x+padding), int(currentY))
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
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Round %d - Navires: Vous %d / Ennemi %d", round.RoundNumber, round.ShipsAliveA, round.ShipsAliveB), int(x+padding), int(roundY))

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
				destroyed = " [DÉTRUIT]"
			}
			attackText := fmt.Sprintf("  %s → %s: %.0f dmg%s", attack.AttackerType, attack.TargetType, attack.DamageDealt, destroyed)
			if len(attackText) > 60 {
				attackText = attackText[:60] + "..."
			}
			ebitenutil.DebugPrintAt(screen, attackText, int(x+padding+10), int(attackY))
		}
		if len(round.Attacks) > maxAttacks {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  ... et %d autres attaques", len(round.Attacks)-maxAttacks), int(x+padding+10), int(roundY+18+float64(maxAttacks)*14))
		}
	}

	// Footer
	footerY := y + h - 35
	ebitenutil.DebugPrintAt(screen, "ESC ou X pour fermer | Molette pour scroller", int(x+padding), int(footerY))
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowPveResultUI = false
		g.Log("[PVE UI] close")
		return true
	}

	// Handle mouse wheel scrolling
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	if fmx >= x && fmx <= x+w && fmy >= contentY && fmy <= contentY+contentH {
		_, dy := ebiten.Wheel()
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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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
