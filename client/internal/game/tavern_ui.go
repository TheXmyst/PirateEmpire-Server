package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawTavernUI draws the dedicated Tavern/Gacha modal
func (g *Game) DrawTavernUI(screen *ebiten.Image) {
	if !g.ui.ShowTavernUI {
		return
	}

	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 600.0, 500.0 // Increased size for x10 results
	x, y := cx-w/2, cy-h/2

	// Draw dark overlay
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 180}, false)

	// Draw Window Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{10, 15, 25, 240}, true)
	// Border
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{218, 165, 32, 255}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "Tavern", int(x)+20, int(y)+20)

	// Close button (X) - top right
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - 10
	closeY := y + 10
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), color.RGBA{150, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+8)

	// Get ticket balance from island resources
	ticketBalance := 0
	if g.player != nil && len(g.player.Islands) > 0 {
		island := g.player.Islands[0]
		if island.Resources != nil {
			ticketBalance = int(island.Resources[domain.CaptainTicket])
		}
	}

	// Ticket balance display
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Tickets: %d", ticketBalance), int(x)+20, int(y)+50)

	// Separator
	vector.StrokeLine(screen, float32(x)+20, float32(y)+80, float32(x+w)-20, float32(y)+80, 1, color.Gray{100}, true)

	// Busy state indicator
	if g.ui.TavernBusy {
		ebitenutil.DebugPrintAt(screen, "Recrutement...", int(x)+20, int(y)+100)
	}

	// Error display
	if g.ui.TavernError != "" {
		errorY := int(y) + 100
		if g.ui.TavernBusy {
			errorY += 20
		}
		errorText := fmt.Sprintf("Erreur: %s", g.ui.TavernError)
		// Split long error messages
		maxLen := 55
		for len(errorText) > maxLen {
			line := errorText[:maxLen]
			errorText = errorText[maxLen:]
			ebitenutil.DebugPrintAt(screen, line, int(x)+20, errorY)
			errorY += 15
		}
		if errorText != "" {
			ebitenutil.DebugPrintAt(screen, errorText, int(x)+20, errorY)
		}
	}

	// Result area
	resultStartY := int(y) + 120
	if g.ui.TavernBusy {
		resultStartY += 20
	}
	if g.ui.TavernError != "" {
		resultStartY += 20
	}

	// Display results based on last summon
	if g.ui.TavernLastResult != "" && !g.ui.TavernBusy {
		resultY := resultStartY
		ebitenutil.DebugPrintAt(screen, "Dernier recrutement:", int(x)+20, resultY)
		resultY += 20
		// Split long messages
		resultText := g.ui.TavernLastResult
		maxLen := 55
		for len(resultText) > maxLen {
			line := resultText[:maxLen]
			resultText = resultText[maxLen:]
			ebitenutil.DebugPrintAt(screen, line, int(x)+20, resultY)
			resultY += 15
		}
		if resultText != "" {
			ebitenutil.DebugPrintAt(screen, resultText, int(x)+20, resultY)
		}
	}

	// Summon buttons (x1 and x10)
	btnW, btnH := 180.0, 45.0
	btnSpacing := 10.0
	totalBtnWidth := btnW*2 + btnSpacing
	btnStartX := x + w/2 - totalBtnWidth/2
	btnY := y + h - 100

	// x1 button
	btn1X := btnStartX
	btn1Col := color.RGBA{100, 100, 100, 255}
	if !g.ui.TavernBusy && ticketBalance >= 1 {
		btn1Col = color.RGBA{0, 150, 0, 255}
	}
	if g.ui.TavernBusy {
		btn1Col = color.RGBA{80, 80, 80, 255}
	}
	vector.DrawFilledRect(screen, float32(btn1X), float32(btnY), float32(btnW), float32(btnH), btn1Col, true)
	vector.StrokeRect(screen, float32(btn1X), float32(btnY), float32(btnW), float32(btnH), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "RECRUTER x1", int(btn1X)+40, int(btnY)+15)

	// x10 button
	btn10X := btnStartX + btnW + btnSpacing
	btn10Col := color.RGBA{100, 100, 100, 255}
	if !g.ui.TavernBusy && ticketBalance >= 10 {
		btn10Col = color.RGBA{0, 100, 200, 255}
	}
	if g.ui.TavernBusy {
		btn10Col = color.RGBA{80, 80, 80, 255}
	}
	vector.DrawFilledRect(screen, float32(btn10X), float32(btnY), float32(btnW), float32(btnH), btn10Col, true)
	vector.StrokeRect(screen, float32(btn10X), float32(btnY), float32(btnW), float32(btnH), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "RECRUTER x10", int(btn10X)+35, int(btnY)+15)
}

// UpdateTavernUI handles input for the Tavern UI
func (g *Game) UpdateTavernUI() bool {
	if !g.ui.ShowTavernUI {
		return false
	}

	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 600.0, 500.0
	x, y := cx-w/2, cy-h/2

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowTavernUI = false
		g.ui.TavernError = ""
		g.ui.TavernLastResult = ""
		return true
	}

	// Don't process clicks if busy
	if g.ui.TavernBusy {
		return true
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// Close button (X) - top right
		closeBtnSize := 30.0
		closeX := x + w - closeBtnSize - 10
		closeY := y + 10
		if fmx >= closeX && fmx <= closeX+closeBtnSize && fmy >= closeY && fmy <= closeY+closeBtnSize {
			g.ui.ShowTavernUI = false
			g.ui.TavernError = ""
			g.ui.TavernLastResult = ""
			return true
		}

		// Get ticket balance
		ticketBalance := 0
		if g.player != nil && len(g.player.Islands) > 0 {
			island := g.player.Islands[0]
			if island.Resources != nil {
				ticketBalance = int(island.Resources[domain.CaptainTicket])
			}
		}

		// Button dimensions
		btnW, btnH := 180.0, 45.0
		btnSpacing := 10.0
		totalBtnWidth := btnW*2 + btnSpacing
		btnStartX := x + w/2 - totalBtnWidth/2
		btnY := y + h - 100

		// x1 button
		btn1X := btnStartX
		if fmx >= btn1X && fmx <= btn1X+btnW && fmy >= btnY && fmy <= btnY+btnH {
			if ticketBalance >= 1 {
				g.summonCaptains(1)
			} else {
				g.ui.TavernError = "Pas assez de tickets (besoin: 1)"
				g.ui.TavernLastResult = ""
			}
			return true
		}

		// x10 button
		btn10X := btnStartX + btnW + btnSpacing
		if fmx >= btn10X && fmx <= btn10X+btnW && fmy >= btnY && fmy <= btnY+btnH {
			if ticketBalance >= 10 {
				g.summonCaptains(10)
			} else {
				g.ui.TavernError = "Pas assez de tickets (besoin: 10)"
				g.ui.TavernLastResult = ""
			}
			return true
		}

		// Click outside modal (close)
		if fmx < x || fmx > x+w || fmy < y || fmy > y+h {
			g.ui.ShowTavernUI = false
			g.ui.TavernError = ""
			g.ui.TavernLastResult = ""
			return true
		}

		// Click inside modal (consume input)
		return true
	}

	return false
}

// summonCaptains performs the summon API call
func (g *Game) summonCaptains(count int) {
	g.ui.TavernBusy = true
	g.ui.TavernError = ""
	g.ui.TavernLastResult = ""

	go func() {
		defer func() {
			g.ui.TavernBusy = false
		}()

		result, err := g.api.SummonCaptain(count)
		if err != nil {
			g.ui.TavernError = err.Error()
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.TavernError = "Session expirée, veuillez vous reconnecter"
			}
			g.Log("Tavern summon error: %v", err)
			return
		}

		// Process results
		if len(result.Results) > 0 {
			// x10 summon
			if count == 10 {
				commonCount := 0
				rareCount := 0
				legendaryCount := 0
				duplicateCount := 0

				for _, res := range result.Results {
					if res.IsDuplicate {
						duplicateCount++
					} else {
						switch res.Rarity {
						case "common":
							commonCount++
						case "rare":
							rareCount++
						case "legendary":
							legendaryCount++
						}
					}
				}

				summary := fmt.Sprintf("Commons: %d, Rares: %d, Legendary: %d", commonCount, rareCount, legendaryCount)
				if duplicateCount > 0 {
					summary += fmt.Sprintf(", Duplicatas: %d", duplicateCount)
				}
				if result.ShardsTotalGranted > 0 {
					summary += fmt.Sprintf(" (+%d fragments)", result.ShardsTotalGranted)
				}

				g.ui.TavernLastResult = summary
			} else {
				// x1 summon
				res := result.Results[0]
				if res.IsDuplicate {
					shardsText := ""
					if res.ShardsGranted > 0 {
						shardsText = fmt.Sprintf(" (+%d fragments)", res.ShardsGranted)
					}
					g.ui.TavernLastResult = fmt.Sprintf("Duplicata! %s (Rareté: %s)%s", res.Name, res.Rarity, shardsText)
				} else if res.Captain != nil {
					g.ui.TavernLastResult = fmt.Sprintf("Nouveau capitaine: %s (Rareté: %s)", res.Captain.Name, res.Rarity)
				} else {
					g.ui.TavernLastResult = fmt.Sprintf("Recrutement réussi: %s (Rareté: %s)", res.Name, res.Rarity)
				}
			}
		} else {
			// Fallback to legacy format (backward compatibility)
			if result.Duplicate {
				g.ui.TavernLastResult = fmt.Sprintf("Duplicata! Template: %s (Rareté: %s). Ticket remboursé.", result.TemplateID, result.Rarity)
			} else if result.Captain != nil {
				g.ui.TavernLastResult = fmt.Sprintf("Nouveau capitaine: %s (Rareté: %s, Template: %s)", result.Captain.Name, result.Rarity, result.TemplateID)
			} else {
				g.ui.TavernLastResult = fmt.Sprintf("Recrutement réussi (Rareté: %s)", result.Rarity)
			}
		}

		// Refresh status to update ticket balance
		g.api.GetStatus()
		g.Log("Tavern summon: count=%d tickets_after=%d", count, result.TicketsAfter)
	}()
}
