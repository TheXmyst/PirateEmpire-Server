package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// DrawTavernUI draws the dedicated Tavern/Gacha modal
func (g *Game) DrawTavernUI(screen *ebiten.Image) {
	if !g.ui.ShowTavernUI {
		return
	}

	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 600.0, 500.0 // Increased size for x10 results
	x, y := cx-w/2, cy-h/2

	drawDimBackground(screen, 180)
	drawWindowFrame(screen, x, y, w, h, "Taverne")
	drawCloseButton(screen, x+w-25, y+25, 16)

	// Tickets de recrutement
	ticketBalance := 0
	if g.player != nil && len(g.player.Islands) > 0 {
		island := g.player.Islands[0]
		if island.Resources != nil {
			ticketBalance = int(island.Resources[domain.CaptainTicket])
		}
	}
	tix := fmt.Sprintf("%d ticket", ticketBalance)
	if ticketBalance > 1 {
		tix += "s"
	}
	tf := bodyFace(14, true)
	tw := textWidth(tix, tf)
	fillRoundRect(screen, x+w-70-tw-24, y+14, tw+24, 26, 13, color.NRGBA{224, 176, 74, 40})
	strokeRoundRect(screen, x+w-70-tw-24, y+14, tw+24, 26, 13, 1, themeGoldDim)
	drawText(screen, tix, tf, x+w-70-tw-12, y+19, themeGold, alignLeft)

	drawLabel(screen, "Recrute de nouveaux capitaines pour tes flottes.", x+24, y+56, 13, false, themeMuted)
	rx := x + 24
	rateF := bodyFace(11.5, true)
	for _, rr := range []struct{ r, pct string }{{"common", "62 %"}, {"rare", "28 %"}, {"legendary", "8,5 %"}, {"ultra", "1,5 %"}} {
		label := rarityLabel(rr.r)
		if rr.r == "ultra" {
			label = "UL"
		}
		txt := label + " " + rr.pct
		drawText(screen, txt, rateF, rx, y+74, rarityColor(rr.r), alignLeft)
		rx += textWidth(txt, rateF) + 16
	}
	drawText(screen, "Garantis : rare en 10, légendaire en 50, UL en 120", bodyFace(11.5, false), x+w-24, y+74, themeMuted, alignRight)

	// Zone des résultats
	area := hudRect{x + 20, y + 92, w - 40, h - 92 - 120}
	drawInset(screen, area.x, area.y, area.w, area.h)
	ty := area.y + 14
	lineF := bodyFace(13, false)
	wrap := func(txt string, clr color.Color) {
		for _, line := range wrapText(txt, lineF, area.w-32) {
			if ty > area.y+area.h-20 {
				return
			}
			drawText(screen, line, lineF, area.x+16, ty, clr, alignLeft)
			ty += 18
		}
	}
	switch {
	case g.ui.TavernBusy:
		drawText(screen, "Recrutement en cours…", titleFace(24), area.x+area.w/2, area.y+area.h/2-16, themeGold, alignCenter)
	case g.ui.TavernError != "":
		wrap("Erreur : "+g.ui.TavernError, themeOrange)
	case len(g.ui.TavernResults) > 0:
		hasUL := false
		for _, r := range g.ui.TavernResults {
			if r.Rarity == "ultra" {
				hasUL = true
			}
		}
		var titleClr color.Color = themeGold
		title := "Dernier recrutement"
		if hasUL {
			title, titleClr = "✦ ULTRA RARE ! ✦", rarityColor("ultra")
			fillRoundRect(screen, area.x+8, ty-6, area.w-16, 36, 10, color.NRGBA{214, 120, 255, 40})
		}
		drawText(screen, title, titleFace(20), area.x+16, ty, titleClr, alignLeft)
		ty += 32
		rowF := bodyFace(13, true)
		for _, r := range g.ui.TavernResults {
			if ty > area.y+area.h-20 {
				break
			}
			fillRoundRect(screen, area.x+18, ty+4, 8, 8, 4, rarityColor(r.Rarity))
			drawText(screen, r.Name, rowF, area.x+34, ty, rarityColor(r.Rarity), alignLeft)
			info := rarityLabel(r.Rarity) + " · nouveau !"
			if r.Duplicate {
				info = rarityLabel(r.Rarity) + fmt.Sprintf(" · doublon, +%d fragments", r.Shards)
			}
			drawText(screen, info, lineF, area.x+area.w-16, ty, themeMuted, alignRight)
			ty += 20
		}
	case g.ui.TavernLastResult != "":
		drawText(screen, "Dernier recrutement", titleFace(20), area.x+16, ty, themeGold, alignLeft)
		ty += 30
		for _, part := range strings.Split(g.ui.TavernLastResult, "\n") {
			wrap(part, themeText)
		}
	default:
		drawText(screen, "Les meilleurs loups de mer t'attendent ici.", bodyFace(14, false), area.x+area.w/2, area.y+area.h/2-10, themeMuted, alignCenter)
	}

	// Boutons x1 et x10 (mêmes zones de clic qu'avant)
	btnW, btnH := 180.0, 45.0
	btnSpacing := 10.0
	totalBtnWidth := btnW*2 + btnSpacing
	btnStartX := x + w/2 - totalBtnWidth/2
	btnY := y + h - 100

	k1, k10 := btnPrimary, btnSecondary
	if g.ui.TavernBusy || ticketBalance < 1 {
		k1 = btnDisabled
	}
	if g.ui.TavernBusy || ticketBalance < 10 {
		k10 = btnDisabled
	}
	drawButton(screen, btnStartX, btnY, btnW, btnH, "Recruter x1", k1)
	drawButton(screen, btnStartX+btnW+btnSpacing, btnY, btnW, btnH, "Recruter x10", k10)
	drawLabel(screen, "1 ticket", btnStartX+btnW/2-22, btnY+btnH+8, 12, false, themeMuted)
	drawLabel(screen, "10 tickets", btnStartX+btnW+btnSpacing+btnW/2-28, btnY+btnH+8, 12, false, themeMuted)
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
	if isKeyJustPressed(ebiten.KeyEscape) {
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
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
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
	g.ui.TavernResults = nil

	go func() {
		defer func() {
			g.ui.TavernBusy = false
		}()

		result, err := g.api.SummonCaptain(count)
		if err != nil {
			g.ui.TavernError = friendlyError(err)
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.TavernError = "Session expirée, veuillez vous reconnecter"
			}
			g.Log("Tavern summon error: %v", err)
			return
		}

		// Process results
		g.ui.TavernResults = nil
		for _, res := range result.Results {
			name := res.Name
			if res.Captain != nil && res.Captain.Name != "" {
				name = res.Captain.Name
			}
			g.ui.TavernResults = append(g.ui.TavernResults, TavernResultLine{
				Name: name, Rarity: res.Rarity, Duplicate: res.IsDuplicate, Shards: res.ShardsGranted,
			})
		}
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
						case "legendary", "ultra":
							legendaryCount++
						}
					}
				}

				summary := fmt.Sprintf("Communs : %d, Rares : %d, Légendaires : %d", commonCount, rareCount, legendaryCount)
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
