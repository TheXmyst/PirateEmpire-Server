package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// Géométrie de la fenêtre Milice (partagée entre Draw et Update).
const (
	militiaW       = 700.0
	militiaH       = 560.0
	militiaSlider0 = 222.0 // ordonnée (relative) de la première glissière
	militiaSliderH = 14.0
	militiaRowGap  = 60.0
)

func (g *Game) militiaRect() (x, y, w, h float64) {
	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	return cx - militiaW/2, cy - militiaH/2, militiaW, militiaH
}

// DrawMilitiaUI draws the dedicated Militia recruitment modal
func (g *Game) DrawMilitiaUI(screen *ebiten.Image) {
	if !g.ui.ShowMilitiaUI {
		return
	}
	x, y, w, h := g.militiaRect()

	drawDimBackground(screen, 170)
	drawWindowFrame(screen, x, y, w, h, "Milice")
	drawCloseButton(screen, x+w-30, y+27, 15)

	var island *domain.Island
	var militiaBuilding *domain.Building
	militiaLevel := 0
	if g.player != nil && len(g.player.Islands) > 0 {
		island = &g.player.Islands[0]
		for i := range island.Buildings {
			if island.Buildings[i].Type == "Milice" {
				militiaBuilding = &island.Buildings[i]
				militiaLevel = militiaBuilding.Level
				break
			}
		}
	}
	drawText(screen, fmt.Sprintf("Niveau %d", militiaLevel), bodyFace(14, true), x+w-60, y+20, themeText, alignRight)

	// Matelots disponibles
	drawCard(screen, x+24, y+66, w-48, 70, false)
	drawText(screen, "Matelots à quai", bodyFace(12, true), x+40, y+74, themeMuted, alignLeft)
	if island != nil {
		stats := []struct {
			name string
			n    int
		}{{"Guerriers", island.Crew[domain.Warrior]}, {"Archers", island.Crew[domain.Archer]}, {"Artilleurs", island.Crew[domain.Gunner]}}
		for i, st := range stats {
			sx := x + 40 + float64(i)*140
			drawText(screen, fmt.Sprintf("%d", st.n), bodyFace(22, true), sx, y+94, themeGold, alignLeft)
			drawText(screen, st.name, bodyFace(13, false), sx+textWidth(fmt.Sprintf("%d", st.n), bodyFace(22, true))+8, y+102, themeText, alignLeft)
		}
	}
	if militiaBuilding != nil && !militiaBuilding.Constructing {
		ux, uy, uw, uh := militiaUpgradeRect(x, y, w)
		drawButton(screen, ux, uy, uw, uh, "Améliorer", btnSecondary)
	}

	// Recrutement
	uiSubtitle(screen, "Recruter", int(x)+24, int(y)+158)
	isRecruiting := false
	if island != nil && island.MilitiaRecruiting && island.MilitiaRecruitDoneAt != nil {
		isRecruiting = true
		left := "en attente de mise à jour"
		if rem := time.Until(*island.MilitiaRecruitDoneAt); rem > 0 {
			left = "fin dans " + formatRecruitDuration(rem)
		}
		msg := fmt.Sprintf("En cours : %d guerriers, %d archers, %d artilleurs · %s",
			island.MilitiaRecruitWarriors, island.MilitiaRecruitArchers, island.MilitiaRecruitGunners, left)
		drawText(screen, msg, bodyFace(13, true), x+130, y+161, themeOrange, alignLeft)
	}

	sliderX, sliderW := x+24, w-48
	rows := []struct {
		name string
		val  int
	}{{"Guerriers", g.ui.MilitiaWarriors}, {"Archers", g.ui.MilitiaArchers}, {"Artilleurs", g.ui.MilitiaGunners}}
	for i, r := range rows {
		by := y + militiaSlider0 + float64(i)*militiaRowGap
		drawText(screen, r.name, bodyFace(14, false), sliderX, by-24, themeText, alignLeft)
		drawText(screen, fmt.Sprintf("%d", r.val), bodyFace(14, true), sliderX+sliderW, by-24, themeGold, alignRight)
		drawSlider(screen, sliderX, by, sliderW, militiaSliderH, r.val, 0, 200)
	}

	// Coût et durée
	totalUnits := g.ui.MilitiaWarriors + g.ui.MilitiaArchers + g.ui.MilitiaGunners
	goldCost := g.ui.MilitiaWarriors*10 + g.ui.MilitiaArchers*12 + g.ui.MilitiaGunners*15
	rumCost := g.ui.MilitiaWarriors*2 + g.ui.MilitiaArchers*2 + g.ui.MilitiaGunners*3
	rawDuration := 15.0 + float64(totalUnits)*3.0
	bonusPct := float64(militiaLevel) * 0.005 // 0,5 % par niveau, 30 % max
	if bonusPct > 0.30 {
		bonusPct = 0.30
	}
	duration := rawDuration * (1.0 - bonusPct)
	if duration < 10.0 {
		duration = 10.0
	}

	infoY := y + militiaSlider0 + 3*militiaRowGap - 4
	var have map[domain.ResourceType]float64
	if island != nil {
		have = island.Resources
	}
	drawText(screen, "Coût", bodyFace(13, false), x+24, infoY, themeMuted, alignLeft)
	if totalUnits > 0 {
		drawCost(screen, map[domain.ResourceType]float64{domain.Gold: float64(goldCost), domain.Rum: float64(rumCost)}, have, x+64, infoY)
	} else {
		drawText(screen, "choisis combien de matelots recruter", bodyFace(13, false), x+64, infoY, themeMuted, alignLeft)
	}
	drawText(screen, fmt.Sprintf("Durée %s (bonus de la Milice %.1f %%)", formatDuration(int(duration)), bonusPct*100), bodyFace(13, false), x+24, infoY+20, themeMuted, alignLeft)

	canAfford := true
	if island != nil && (island.Resources[domain.Gold] < float64(goldCost) || island.Resources[domain.Rum] < float64(rumCost)) {
		canAfford = false
	}

	bx, by, bw, bh := militiaRecruitRect(x, y, w, h)
	kind, label := btnPrimary, "Recruter"
	switch {
	case g.ui.MilitiaBusy:
		kind, label = btnDisabled, "Envoi…"
	case isRecruiting:
		kind, label = btnDisabled, "Recrutement en cours"
	case totalUnits <= 0:
		kind = btnDisabled
	case !canAfford:
		kind, label = btnDisabled, "Ressources"
	}
	drawButton(screen, bx, by, bw, bh, label, kind)

	if g.ui.MilitiaError != "" {
		for i, l := range wrapText(stripUnsupported(g.ui.MilitiaError), bodyFace(13, true), w-48) {
			if i >= 2 {
				break
			}
			drawText(screen, l, bodyFace(13, true), x+24, by-44+float64(i)*17, color.RGBA{235, 90, 70, 255}, alignLeft)
		}
	}
}

func militiaUpgradeRect(x, y, w float64) (float64, float64, float64, float64) {
	return x + w - 24 - 16 - 120, y + 84, 120.0, 34.0
}

func militiaRecruitRect(x, y, w, h float64) (float64, float64, float64, float64) {
	return x + w/2 - 100, y + h - 62, 200.0, 42.0
}

// formatRecruitDuration formats a duration as mm:ss or hh:mm:ss
func formatRecruitDuration(d time.Duration) string {
	if d <= 0 {
		return "00:00"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// drawSlider : glissière arrondie, remplie en or, avec une poignée ronde.
func drawSlider(screen *ebiten.Image, x, y, w, h float64, value, min, max int) {
	fillRoundRect(screen, x, y, w, h, h/2, color.NRGBA{0, 0, 0, 110})
	strokeRoundRect(screen, x, y, w, h, h/2, 1, color.NRGBA{224, 176, 74, 60})
	if max <= min {
		return
	}
	ratio := float64(value-min) / float64(max-min)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	if ratio > 0 {
		fillRoundRect(screen, x, y, math.Max(h, w*ratio), h, h/2, themeGoldDark)
	}
	r := h/2 + 5
	cx := x + w*ratio
	fillRoundRect(screen, cx-r, y+h/2-r, 2*r, 2*r, r, themeGold)
	strokeRoundRect(screen, cx-r, y+h/2-r, 2*r, 2*r, r, 2, themeInk)
}

// UpdateMilitiaUI handles input for the Militia UI
func (g *Game) UpdateMilitiaUI() bool {
	if !g.ui.ShowMilitiaUI {
		return false
	}

	x, y, w, h := g.militiaRect()

	// Close on ESC
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowMilitiaUI = false
		g.ui.MilitiaError = ""
		// Reset drag state
		g.ui.DraggingMilitiaWarrior = false
		g.ui.DraggingMilitiaArcher = false
		g.ui.DraggingMilitiaGunner = false
		return true
	}

	// Get island data
	var island *domain.Island
	var militiaBuilding *domain.Building
	if g.player != nil && len(g.player.Islands) > 0 {
		island = &g.player.Islands[0]
		for i := range island.Buildings {
			if island.Buildings[i].Type == "Milice" {
				militiaBuilding = &island.Buildings[i]
				break
			}
		}
	}

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Slider positions (synchronized with Draw)
	sliderX := x + 24
	sliderW := w - 48
	sliderH := militiaSliderH
	sliderCaptureH := 30.0 // Larger hit area

	warriorY := y + militiaSlider0
	archerY := warriorY + militiaRowGap
	gunnerY := archerY + militiaRowGap

	// Stop dragging when mouse released
	if !isMousePressed(ebiten.MouseButtonLeft) {
		g.ui.DraggingMilitiaWarrior = false
		g.ui.DraggingMilitiaArcher = false
		g.ui.DraggingMilitiaGunner = false
	}

	// Helper function to calculate slider value from mouse X position
	calcSliderValue := func(mouseX float64) int {
		ratio := (mouseX - sliderX) / sliderW
		if ratio < 0.0 {
			ratio = 0.0
		}
		if ratio > 1.0 {
			ratio = 1.0
		}
		value := int(ratio * 200) // Max 200 for now, will be clamped by caps
		if value < 0 {
			value = 0
		}
		if value > 200 {
			value = 200
		}
		return value
	}

	// PRIORITY 1: Handle close button FIRST
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		if fmx >= x+w-47 && fmx <= x+w-13 && fmy >= y+10 && fmy <= y+44 {
			g.ui.ShowMilitiaUI = false
			g.ui.MilitiaError = ""
			g.ui.DraggingMilitiaWarrior = false
			g.ui.DraggingMilitiaArcher = false
			g.ui.DraggingMilitiaGunner = false
			return true
		}
	}

	// PRIORITY 2: Start dragging on mouse down over slider (EXCLUSIVE with else if)
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		// Warrior slider (expanded hit area)
		if fmy >= warriorY-sliderCaptureH/2 && fmy <= warriorY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			g.ui.DraggingMilitiaWarrior = true
			g.ui.DraggingMilitiaArcher = false // Disable others
			g.ui.DraggingMilitiaGunner = false // Disable others
			g.ui.MilitiaWarriors = calcSliderValue(fmx)
		} else if fmy >= archerY-sliderCaptureH/2 && fmy <= archerY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			// Archer slider (expanded hit area)
			g.ui.DraggingMilitiaWarrior = false // Disable others
			g.ui.DraggingMilitiaArcher = true
			g.ui.DraggingMilitiaGunner = false // Disable others
			g.ui.MilitiaArchers = calcSliderValue(fmx)
		} else if fmy >= gunnerY-sliderCaptureH/2 && fmy <= gunnerY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			// Gunner slider (expanded hit area)
			g.ui.DraggingMilitiaWarrior = false // Disable others
			g.ui.DraggingMilitiaArcher = false  // Disable others
			g.ui.DraggingMilitiaGunner = true
			g.ui.MilitiaGunners = calcSliderValue(fmx)
		}
	}

	// PRIORITY 3: Continue dragging if mouse button held down
	if isMousePressed(ebiten.MouseButtonLeft) {
		if g.ui.DraggingMilitiaWarrior {
			g.ui.MilitiaWarriors = calcSliderValue(fmx)
		} else if g.ui.DraggingMilitiaArcher {
			g.ui.MilitiaArchers = calcSliderValue(fmx)
		} else if g.ui.DraggingMilitiaGunner {
			g.ui.MilitiaGunners = calcSliderValue(fmx)
		}
	}

	// PRIORITY 4: Handle buttons (only if not currently dragging)
	if isMouseJustPressed(ebiten.MouseButtonLeft) && !g.ui.DraggingMilitiaWarrior && !g.ui.DraggingMilitiaArcher && !g.ui.DraggingMilitiaGunner {
		// Upgrade button
		if militiaBuilding != nil && !militiaBuilding.Constructing {
			upgradeBtnX, upgradeBtnY, upgradeBtnW, upgradeBtnH := militiaUpgradeRect(x, y, w)
			if fmx >= upgradeBtnX && fmx <= upgradeBtnX+upgradeBtnW && fmy >= upgradeBtnY && fmy <= upgradeBtnY+upgradeBtnH {
				// Call upgrade endpoint
				go func(bid string) {
					err := g.api.Upgrade(g.player.ID.String(), bid)
					if err == nil {
						g.refreshStatusAfterDevAction()
					} else {
						// Try to handle as prerequisites error
						if !g.handleAPIError(err, "Amélioration Milice") {
							// Not a prerequisites error - show standard error
							g.ui.MilitiaError = friendlyError(err)
						}
					}
				}(militiaBuilding.ID.String())
				return true
			}
		}

		// Recruit button
		totalUnits := g.ui.MilitiaWarriors + g.ui.MilitiaArchers + g.ui.MilitiaGunners
		canAfford := true
		if island != nil {
			goldCost := g.ui.MilitiaWarriors*10 + g.ui.MilitiaArchers*12 + g.ui.MilitiaGunners*15
			rumCost := g.ui.MilitiaWarriors*2 + g.ui.MilitiaArchers*2 + g.ui.MilitiaGunners*3
			if island.Resources[domain.Gold] < float64(goldCost) || island.Resources[domain.Rum] < float64(rumCost) {
				canAfford = false
			}
		}
		isRecruiting := false
		if island != nil && island.MilitiaRecruiting {
			isRecruiting = true
		}

		btnX, btnY, btnW, btnH := militiaRecruitRect(x, y, w, h)

		if !g.ui.MilitiaBusy && totalUnits > 0 && canAfford && !isRecruiting {
			if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				g.recruitCrew()
				return true
			}
		}

		// Click inside modal (consume input)
		if fmx >= x && fmx <= x+w && fmy >= y && fmy <= y+h {
			return true
		}
	}

	// Always consume input when modal is open
	return true
}

// recruitCrew performs the recruitment API call
func (g *Game) recruitCrew() {
	g.ui.MilitiaBusy = true
	g.ui.MilitiaError = ""

	fmt.Printf("[DEBUG] recruitCrew clicked: IslandID=%s W=%d A=%d G=%d\n",
		g.player.Islands[0].ID, g.ui.MilitiaWarriors, g.ui.MilitiaArchers, g.ui.MilitiaGunners)

	go func() {
		defer func() {
			g.ui.MilitiaBusy = false
		}()

		result, err := g.api.MilitiaRecruit(g.player.Islands[0].ID, g.ui.MilitiaWarriors, g.ui.MilitiaArchers, g.ui.MilitiaGunners)
		if err != nil {
			// Check if it's an auth error
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.MilitiaError = "Session expirée, veuillez vous reconnecter"
				return
			}
			// Try to handle as prerequisites error
			if !g.handleAPIError(err, "Recrutement Milice") {
				// Not a prerequisites error - show standard error
				g.ui.MilitiaError = friendlyError(err)
			}
			g.Log("Militia recruit error: %v", err)
			return
		}

		// Success - refresh status
		g.refreshStatusAfterDevAction()
		g.Log("Militia recruit: warriors=%d archers=%d gunners=%d done_at=%s",
			g.ui.MilitiaWarriors, g.ui.MilitiaArchers, g.ui.MilitiaGunners, result.DoneAt)

		// Clear sliders
		g.ui.MilitiaWarriors = 0
		g.ui.MilitiaArchers = 0
		g.ui.MilitiaGunners = 0
	}()
}
