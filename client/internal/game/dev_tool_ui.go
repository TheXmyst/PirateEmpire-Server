package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawDevTool renders the comprehensive developer tool panel
func (g *Game) DrawDevTool(screen *ebiten.Image) {
	if !g.showDevMenu {
		return
	}

	// Initialize DevSimTier to 1 if not set
	if g.ui.DevSimTier == 0 {
		g.ui.DevSimTier = 1
	}

	w := float64(screen.Bounds().Dx())

	// Dev Tool Panel
	panelW := 600.0
	panelH := 700.0
	panelX := w - panelW - 20
	panelY := 20.0

	// Background
	drawButton(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), "=== dev tool ===", kindFromColor(color.RGBA{20, 20, 40, 240}))

	// Title
	uiPrint(screen, "[F1] to close", int(panelX)+240, int(panelY)+25)

	// Stats Section
	currentY := panelY + 50
	uiPrint(screen, fmt.Sprintf("FPS: %.1f | TPS: %.1f | State: %d", ebiten.ActualFPS(), ebiten.ActualTPS(), g.state), int(panelX)+20, int(currentY))

	if g.player != nil {
		currentY += 20
		uiPrint(screen, fmt.Sprintf("Player: %s (Admin: %v)", g.player.Username, g.player.IsAdmin), int(panelX)+20, int(currentY))
	}

	// Separator
	currentY += 30
	vector.StrokeLine(screen, float32(panelX+20), float32(currentY), float32(panelX+panelW-20), float32(currentY), 1, color.RGBA{100, 150, 255, 150}, vecAA)

	// Cheat Buttons Section
	currentY += 20
	uiSubtitle(screen, "Triches", int(panelX)+20, int(currentY))

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Button helper
	drawButton := func(x, y, w, h float64, label string, hoverColor, normalColor color.RGBA) bool {
		hover := fmx >= x && fmx <= x+w && fmy >= y && fmy <= y+h
		btnColor := normalColor
		if hover {
			btnColor = hoverColor
		}
		drawButton(screen, float64(x), float64(y), float64(w), float64(h), label, kindFromColor(btnColor))
		return hover && isMouseJustPressed(ebiten.MouseButtonLeft)
	}

	// Row 1: Add Resources
	currentY += 30
	btnW := 130.0
	btnH := 30.0
	gap := 10.0

	if drawButton(panelX+20, currentY, btnW, btnH, "+1000 Wood", color.RGBA{100, 200, 100, 255}, color.RGBA{60, 120, 60, 255}) {
		g.devAddResources(1000)
	}
	if drawButton(panelX+20+btnW+gap, currentY, btnW, btnH, "+1000 Stone", color.RGBA{150, 150, 150, 255}, color.RGBA{90, 90, 90, 255}) {
		g.devAddResources(1000)
	}
	if drawButton(panelX+20+(btnW+gap)*2, currentY, btnW, btnH, "+1000 Gold", color.RGBA{255, 215, 0, 255}, color.RGBA{180, 150, 0, 255}) {
		g.devAddResources(1000)
	}
	if drawButton(panelX+20+(btnW+gap)*3, currentY, btnW, btnH, "+1000 Rum", color.RGBA{139, 69, 19, 255}, color.RGBA{100, 50, 10, 255}) {
		g.devAddResources(1000)
	}

	// Row 2: Instant Finish
	currentY += btnH + gap
	if drawButton(panelX+20, currentY, btnW*2+gap, btnH, "Finish Building", color.RGBA{100, 150, 255, 255}, color.RGBA{60, 90, 150, 255}) {
		g.devFinishBuilding()
	}
	if drawButton(panelX+20+(btnW*2+gap)+gap, currentY, btnW*2+gap, btnH, "Finish Research", color.RGBA{255, 150, 100, 255}, color.RGBA{150, 90, 60, 255}) {
		g.devFinishResearch()
	}

	// Row 3: More Instant Finish
	currentY += btnH + gap
	if drawButton(panelX+20, currentY, btnW*2+gap, btnH, "Finish Ship", color.RGBA{100, 200, 255, 255}, color.RGBA{60, 120, 150, 255}) {
		g.devFinishShip()
	}
	if drawButton(panelX+20+(btnW*2+gap)+gap, currentY, btnW*2+gap, btnH, "Time Skip +1h", color.RGBA{200, 100, 255, 255}, color.RGBA{120, 60, 150, 255}) {
		g.devTimeSkip(1) // 1 hour
	}

	// Row 4: Captains & Tickets
	currentY += btnH + gap
	if drawButton(panelX+20, currentY, btnW*2+gap, btnH, "Grant Captain", color.RGBA{255, 200, 100, 255}, color.RGBA{150, 120, 60, 255}) {
		g.devGrantCaptain()
	}
	if drawButton(panelX+20+(btnW*2+gap)+gap, currentY, btnW*2+gap, btnH, "+10 Tickets", color.RGBA{255, 100, 200, 255}, color.RGBA{150, 60, 120, 255}) {
		g.devGrantTickets(10)
	}

	// Row 5: Guild Ticket
	currentY += btnH + gap
	if drawButton(panelX+20, currentY, btnW*2+gap, btnH, "+1 Guild Ticket", color.RGBA{100, 180, 200, 255}, color.RGBA{60, 120, 140, 255}) {
		g.devGrantGuildTicket()
	}

	// Separator
	currentY += 50
	vector.StrokeLine(screen, float32(panelX+20), float32(currentY), float32(panelX+panelW-20), float32(currentY), 1, color.RGBA{100, 150, 255, 150}, vecAA)

	// Combat Simulator Section
	currentY += 20
	uiSubtitle(screen, "Simulateur de combat", int(panelX)+20, int(currentY))

	currentY += 30
	uiPrint(screen, "Fleet A (Your Fleet):", int(panelX)+30, int(currentY))
	currentY += 20
	uiPrint(screen, "Select your active fleet from Fleet UI", int(panelX)+40, int(currentY))

	currentY += 30
	uiPrint(screen, "Fleet B (Enemy):", int(panelX)+30, int(currentY))
	currentY += 20

	// Enemy tier selector
	tierBtnW := 80.0
	if drawButton(panelX+40, currentY, tierBtnW, btnH, "Tier 1", color.RGBA{100, 255, 100, 255}, color.RGBA{60, 150, 60, 255}) {
		g.ui.DevSimTier = 1
	}
	if drawButton(panelX+40+tierBtnW+gap, currentY, tierBtnW, btnH, "Tier 2", color.RGBA{255, 255, 100, 255}, color.RGBA{150, 150, 60, 255}) {
		g.ui.DevSimTier = 2
	}
	if drawButton(panelX+40+(tierBtnW+gap)*2, currentY, tierBtnW, btnH, "Tier 3", color.RGBA{255, 150, 100, 255}, color.RGBA{150, 90, 60, 255}) {
		g.ui.DevSimTier = 3
	}
	if drawButton(panelX+40+(tierBtnW+gap)*3, currentY, tierBtnW, btnH, "Tier 4", color.RGBA{255, 100, 100, 255}, color.RGBA{150, 60, 60, 255}) {
		g.ui.DevSimTier = 4
	}
	if drawButton(panelX+40+(tierBtnW+gap)*4, currentY, tierBtnW, btnH, "Tier 5", color.RGBA{200, 100, 255, 255}, color.RGBA{120, 60, 150, 255}) {
		g.ui.DevSimTier = 5
	}

	currentY += btnH + 20
	uiPrint(screen, fmt.Sprintf("Selected Enemy Tier: %d", g.ui.DevSimTier), int(panelX)+40, int(currentY))

	currentY += 30
	if drawButton(panelX+20, currentY, panelW-40, btnH+10, "SIMULATE COMBAT", color.RGBA{255, 100, 100, 255}, color.RGBA{180, 60, 60, 255}) {
		g.devSimulateEngagement()
	}

	// Separator
	currentY += 60
	vector.StrokeLine(screen, float32(panelX+20), float32(currentY), float32(panelX+panelW-20), float32(currentY), 1, color.RGBA{100, 150, 255, 150}, vecAA)

	// Logs Section
	currentY += 20
	uiSubtitle(screen, "Journaux récents", int(panelX)+20, int(currentY))

	currentY += 25
	// Display last 5 logs
	logStartIdx := len(g.devLogs) - 5
	if logStartIdx < 0 {
		logStartIdx = 0
	}
	for i := logStartIdx; i < len(g.devLogs); i++ {
		uiPrint(screen, g.devLogs[i], int(panelX)+30, int(currentY))
		currentY += 15
	}
}

// Dev Tool API Calls
func (g *Game) devAddResources(amount float64) {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevAddResources(amount)
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Add resources failed: %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("✅ Added %.0f to all resources", amount))
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devFinishBuilding() {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevFinishBuilding()
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Finish building failed: %v", err))
			return
		}

		g.addDevLog("✅ Building finished")
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devFinishResearch() {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevFinishResearch()
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Finish research failed: %v", err))
			return
		}

		g.addDevLog("✅ Research finished")
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devFinishShip() {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevFinishShip()
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Finish ship failed: %v", err))
			return
		}

		g.addDevLog("✅ Ship finished")
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devTimeSkip(hours int) {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevTimeSkip(hours)
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Time skip failed: %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("✅ Skipped %d hours", hours))
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devGrantCaptain() {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevGrantCaptain(g.player.ID, "")
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Grant captain failed: %v", err))
			return
		}

		g.addDevLog("✅ Captain granted")
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devGrantTickets(amount int) {
	if g.ui.DevActionBusy {
		return
	}
	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		err := g.api.DevGrantTickets(g.player.ID, amount)
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Grant tickets failed: %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("✅ Granted %d tickets", amount))
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devGrantGuildTicket() {
	// Local-only toggle for now (no backend endpoint yet)
	g.ui.SocialGuildHasTicket = true
	g.persistSocialState()
	g.addDevLog("✅ Guild ticket granted (local)")
}

func (g *Game) devSimulateEngagement() {
	if g.ui.DevActionBusy {
		return
	}

	if g.player == nil || len(g.player.Islands) == 0 || g.player.Islands[0].ActiveFleetID == nil {
		g.addDevLog("❌ No active fleet selected")
		return
	}

	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		result, err := g.api.DevSimulateEngagement(g.player.Islands[0].ActiveFleetID.String(), g.ui.DevSimTier)
		if err != nil {
			g.addDevLog(fmt.Sprintf("❌ Simulation failed: %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("✅ Combat simulated - Winner: %s", result.Winner))

		// Show result in PvE result UI
		g.ui.PveCombatResult = result
		g.ui.ShowPveResultUI = true
	}()
}

func (g *Game) addDevLog(message string) {
	g.devLogs = append(g.devLogs, message)
	// Keep only last 50 logs
	if len(g.devLogs) > 50 {
		g.devLogs = g.devLogs[len(g.devLogs)-50:]
	}
}
