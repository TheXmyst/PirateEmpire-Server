package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
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

	w := float64(g.screenWidth)

	panelW, panelH := 600.0, 610.0
	panelX, panelY := w-panelW-20, 86.0
	drawThemePanel(screen, panelX, panelY, panelW, panelH, 18)
	drawText(screen, "Outils dev", titleFace(26), panelX+20, panelY+10, themeGold, alignLeft)
	drawText(screen, "F1 pour fermer", bodyFace(12, false), panelX+panelW-20, panelY+20, themeMuted, alignRight)

	stats := fmt.Sprintf("FPS %.0f · TPS %.0f · état %d", ebiten.ActualFPS(), ebiten.ActualTPS(), g.state)
	if g.player != nil {
		admin := "non admin"
		if g.player.IsAdmin {
			admin = "admin"
		}
		stats += fmt.Sprintf(" · %s (%s)", g.player.Username, admin)
	}
	drawText(screen, stats, bodyFace(12, false), panelX+20, panelY+50, themeMuted, alignLeft)

	btn := func(x, y, bw, bh float64, label string, kind buttonKind) bool {
		if g.ui.DevActionBusy {
			kind = btnDisabled
		}
		drawButton(screen, x, y, bw, bh, label, kind)
		return !g.ui.DevActionBusy && isHovered(x, y, bw, bh) && isMouseJustPressed(ebiten.MouseButtonLeft)
	}

	x0 := panelX + 20
	btnW, btnH, gap := 130.0, 30.0, 10.0
	wideW := btnW*2 + gap
	y := panelY + 80
	uiSubtitle(screen, "Triches", int(x0), int(y))
	y += 28
	// Le serveur ajoute la même quantité à toutes les ressources.
	if btn(x0, y, panelW-40, btnH, "+1000 de chaque ressource", btnPrimary) {
		g.devAddResources(1000)
	}
	y += btnH + gap
	if btn(x0, y, wideW, btnH, "Finir la construction", btnSecondary) {
		g.devFinishBuilding()
	}
	if btn(x0+wideW+gap, y, wideW, btnH, "Finir la recherche", btnSecondary) {
		g.devFinishResearch()
	}
	y += btnH + gap
	if btn(x0, y, wideW, btnH, "Finir le navire", btnSecondary) {
		g.devFinishShip()
	}
	if btn(x0+wideW+gap, y, wideW, btnH, "Avancer d'1 h", btnSecondary) {
		g.devTimeSkip(1)
	}
	y += btnH + gap
	if btn(x0, y, wideW, btnH, "Donner un capitaine", btnSecondary) {
		g.devGrantCaptain()
	}
	if btn(x0+wideW+gap, y, wideW, btnH, "+10 tickets", btnSecondary) {
		g.devGrantTickets(10)
	}
	y += btnH + gap
	if btn(x0, y, wideW, btnH, "+1 ticket de guilde", btnSecondary) {
		g.devGrantGuildTicket()
	}

	// Journal : juste sous les boutons, pour voir tout de suite le résultat.
	y += btnH + 22
	uiSubtitle(screen, "Journal", int(x0), int(y))
	if g.ui.DevActionBusy {
		drawText(screen, "Envoi au serveur…", bodyFace(12, true), panelX+panelW-20, y+4, themeOrange, alignRight)
	}
	y += 28
	logH := 5 * 17.0
	drawInset(screen, x0, y, panelW-40, logH+12)
	if len(g.devLogs) == 0 {
		drawText(screen, "Rien pour l'instant. Clique sur une triche.", bodyFace(12, false), x0+12, y+8, themeMuted, alignLeft)
	}
	start := len(g.devLogs) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(g.devLogs); i++ {
		line := stripUnsupported(g.devLogs[i])
		clr := color.Color(themeText)
		if strings.HasPrefix(line, "Échec") {
			clr = color.RGBA{235, 90, 70, 255}
		}
		drawText(screen, ellipsize(line, bodyFace(12, false), panelW-64), bodyFace(12, false), x0+12, y+8+float64(i-start)*17, clr, alignLeft)
	}
	y += logH + 32

	// Simulateur de combat
	uiSubtitle(screen, "Simulateur de combat", int(x0), int(y))
	y += 28
	drawText(screen, "Ta flotte active contre une flotte ennemie de palier :", bodyFace(13, false), x0, y, themeText, alignLeft)
	y += 24
	tierW := 80.0
	for t := 1; t <= 5; t++ {
		tx := x0 + float64(t-1)*(tierW+gap)
		active := g.ui.DevSimTier == t
		drawTab(screen, tx, y, tierW, btnH, fmt.Sprintf("Palier %d", t), active)
		if isHovered(tx, y, tierW, btnH) && isMouseJustPressed(ebiten.MouseButtonLeft) {
			g.ui.DevSimTier = t
		}
	}
	y += btnH + 16
	if btn(x0, y, panelW-40, btnH+8, "Lancer le combat simulé", btnDanger) {
		g.devSimulateEngagement()
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
			g.addDevLog(fmt.Sprintf("Échec ajout de ressources : %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("+%.0f de chaque ressource", amount))
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
			g.addDevLog(fmt.Sprintf("Échec fin de construction : %v", err))
			return
		}

		g.addDevLog("Construction terminée")
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
			g.addDevLog(fmt.Sprintf("Échec fin de recherche : %v", err))
			return
		}

		g.addDevLog("Recherche terminée")
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
			g.addDevLog(fmt.Sprintf("Échec fin du navire : %v", err))
			return
		}

		g.addDevLog("Navire terminé")
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
			g.addDevLog(fmt.Sprintf("Échec avance du temps : %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("Temps avancé de %d h", hours))
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
			g.addDevLog(fmt.Sprintf("Échec don de capitaine : %v", err))
			return
		}

		g.addDevLog("Capitaine ajouté")
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
			g.addDevLog(fmt.Sprintf("Échec don de tickets : %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("%d tickets ajoutés", amount))
		g.refreshStatusAfterDevAction()
	}()
}

func (g *Game) devGrantGuildTicket() {
	// Local-only toggle for now (no backend endpoint yet)
	g.ui.SocialGuildHasTicket = true
	g.persistSocialState()
	g.addDevLog("Ticket de guilde ajouté (local)")
}

func (g *Game) devSimulateEngagement() {
	if g.ui.DevActionBusy {
		return
	}

	if g.player == nil || len(g.player.Islands) == 0 || g.player.Islands[0].ActiveFleetID == nil {
		g.addDevLog("Échec : aucune flotte active")
		return
	}

	g.ui.DevActionBusy = true

	go func() {
		defer func() { g.ui.DevActionBusy = false }()

		result, err := g.api.DevSimulateEngagement(g.player.Islands[0].ActiveFleetID.String(), g.ui.DevSimTier)
		if err != nil {
			g.addDevLog(fmt.Sprintf("Échec simulation : %v", err))
			return
		}

		g.addDevLog(fmt.Sprintf("Combat simulé, vainqueur : %s", result.Winner))

		// Show result in PvE result UI
		g.ui.PveCombatResult = result
		g.ui.ShowPveResultUI = true
	}()
}

func (g *Game) addDevLog(message string) {
	if strings.Contains(message, "404") {
		message += " (triches désactivées sur ce serveur)"
	} else if strings.Contains(message, "403") {
		message += " (compte non admin)"
	}
	g.devLogs = append(g.devLogs, message)
	// Keep only last 50 logs
	if len(g.devLogs) > 50 {
		g.devLogs = g.devLogs[len(g.devLogs)-50:]
	}
}
