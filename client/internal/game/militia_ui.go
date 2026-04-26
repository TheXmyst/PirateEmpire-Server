package game

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawMilitiaUI draws the dedicated Militia recruitment modal
func (g *Game) DrawMilitiaUI(screen *ebiten.Image) {
	if !g.ui.ShowMilitiaUI {
		return
	}

	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 700.0, 600.0
	x, y := cx-w/2, cy-h/2

	// Draw dark overlay
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 180}, false)

	// Draw Window Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{10, 15, 25, 240}, true)
	// Border
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{218, 165, 32, 255}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "MILICE", int(x)+20, int(y)+20)

	// Close button (X) - top right
	closeBtnSize := 30.0
	closeX := x + w - closeBtnSize - 10
	closeY := y + 10
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), color.RGBA{150, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(closeX), float32(closeY), float32(closeBtnSize), float32(closeBtnSize), 1, color.White, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+8)

	// Get island data
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

	// Militia level display
	levelY := int(y) + 50
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Niveau Milice: %d", militiaLevel), int(x)+20, levelY)

	// Upgrade button (if building exists and not constructing)
	if militiaBuilding != nil && !militiaBuilding.Constructing {
		upgradeBtnX := x + w - 150
		upgradeBtnY := y + 45
		upgradeBtnW := 100.0
		upgradeBtnH := 30.0
		vector.DrawFilledRect(screen, float32(upgradeBtnX), float32(upgradeBtnY), float32(upgradeBtnW), float32(upgradeBtnH), color.RGBA{200, 150, 0, 255}, true)
		ebitenutil.DebugPrintAt(screen, "UPGRADE", int(upgradeBtnX)+20, int(upgradeBtnY)+8)
	}

	// Stock actuel display
	stockY := int(y) + 80
	if island != nil {
		ebitenutil.DebugPrintAt(screen, "Stock actuel:", int(x)+20, stockY)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Guerriers: %d | Archers: %d | Artilleurs: %d",
			island.Crew[domain.Warrior], island.Crew[domain.Archer], island.Crew[domain.Gunner]), int(x)+20, stockY+20)
	}

	// Separator
	vector.StrokeLine(screen, float32(x)+20, float32(y)+120, float32(x+w)-20, float32(y)+120, 1, color.Gray{100}, true)

	// Recruitment section
	recruitY := int(y) + 140
	ebitenutil.DebugPrintAt(screen, "Recrutement:", int(x)+20, recruitY)

	// Sliders
	sliderX := x + 20
	sliderW := w - 40
	sliderH := 20.0
	sliderY := float64(recruitY) + 30

	// Warrior slider
	warriorY := sliderY
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Guerriers: %d", g.ui.MilitiaWarriors), int(sliderX), int(warriorY))
	drawSlider(screen, sliderX, warriorY+20, sliderW, sliderH, g.ui.MilitiaWarriors, 0, 200)

	// Archer slider
	archerY := warriorY + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Archers: %d", g.ui.MilitiaArchers), int(sliderX), int(archerY))
	drawSlider(screen, sliderX, archerY+20, sliderW, sliderH, g.ui.MilitiaArchers, 0, 200)

	// Gunner slider
	gunnerY := archerY + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Artilleurs: %d", g.ui.MilitiaGunners), int(sliderX), int(gunnerY))
	drawSlider(screen, sliderX, gunnerY+20, sliderW, sliderH, g.ui.MilitiaGunners, 0, 200)

	// Calculate costs and duration
	totalUnits := g.ui.MilitiaWarriors + g.ui.MilitiaArchers + g.ui.MilitiaGunners
	goldCost := g.ui.MilitiaWarriors*10 + g.ui.MilitiaArchers*12 + g.ui.MilitiaGunners*15
	rumCost := g.ui.MilitiaWarriors*2 + g.ui.MilitiaArchers*2 + g.ui.MilitiaGunners*3

	// Calculate duration (base 15s + 3s per unit, with bonus)
	baseDuration := 15.0
	perUnitDuration := 3.0
	rawDuration := baseDuration + float64(totalUnits)*perUnitDuration

	// Apply Militia bonus (0.5% per level, max 30%)
	bonusPct := float64(militiaLevel) * 0.005
	if bonusPct > 0.30 {
		bonusPct = 0.30
	}
	duration := rawDuration * (1.0 - bonusPct)
	if duration < 10.0 {
		duration = 10.0
	}

	// Cost and duration display
	infoY := int(gunnerY) + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Coût: %d Or, %d Rhum", goldCost, rumCost), int(x)+20, infoY)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Temps estimé: %.1f s (bonus: %.1f%%)", duration, bonusPct*100), int(x)+20, infoY+20)

	// Check if can afford
	canAfford := true
	if island != nil {
		if island.Resources[domain.Gold] < float64(goldCost) {
			canAfford = false
		}
		if island.Resources[domain.Rum] < float64(rumCost) {
			canAfford = false
		}
	}

	// RECRUITMENT STATUS BLOCK (if recruiting)
	isRecruiting := false
	if island != nil && island.MilitiaRecruiting && island.MilitiaRecruitDoneAt != nil {
		isRecruiting = true
		now := time.Now()

		// Dedicated recruitment block (after stock display)
		recruitBlockY := stockY + 60
		recruitBlockH := 80.0
		recruitBlockW := w - 40
		recruitBlockX := x + 20

		// Background block (distinct color)
		vector.DrawFilledRect(screen, float32(recruitBlockX), float32(recruitBlockY), float32(recruitBlockW), float32(recruitBlockH), color.RGBA{20, 40, 60, 240}, true)
		vector.StrokeRect(screen, float32(recruitBlockX), float32(recruitBlockY), float32(recruitBlockW), float32(recruitBlockH), 2, color.RGBA{100, 150, 200, 255}, true)

		// Title
		ebitenutil.DebugPrintAt(screen, "🔄 RECRUTEMENT EN COURS", int(recruitBlockX)+10, int(recruitBlockY)+10)

		// Countdown (formatted)
		if now.Before(*island.MilitiaRecruitDoneAt) {
			remaining := island.MilitiaRecruitDoneAt.Sub(now)
			timeStr := formatRecruitDuration(remaining)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Fin dans: %s", timeStr), int(recruitBlockX)+10, int(recruitBlockY)+30)
		} else {
			ebitenutil.DebugPrintAt(screen, "Fin dans: 00:00 (en attente mise à jour)", int(recruitBlockX)+10, int(recruitBlockY)+30)
		}

		// Quantities
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("→ %d Guerriers | %d Archers | %d Artilleurs",
			island.MilitiaRecruitWarriors, island.MilitiaRecruitArchers, island.MilitiaRecruitGunners),
			int(recruitBlockX)+10, int(recruitBlockY)+50)
	} else if island != nil && !island.MilitiaRecruiting {
		// Show "no recruitment" status
		recruitBlockY := stockY + 60
		ebitenutil.DebugPrintAt(screen, "Aucun recrutement en cours", int(x)+20, int(recruitBlockY)+10)
	}

	// Recruit button
	btnY := y + h - 80
	btnW := 150.0
	btnH := 40.0
	btnX := x + w/2 - btnW/2

	disabled := totalUnits <= 0 || !canAfford || isRecruiting || g.ui.MilitiaBusy
	btnColor := color.RGBA{0, 150, 0, 255}
	if disabled {
		btnColor = color.RGBA{100, 100, 100, 255}
	}

	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnColor, true)
	vector.StrokeRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), 2, color.White, true)

	btnText := "RECRUTER"
	if g.ui.MilitiaBusy {
		btnText = "EN COURS..."
	}
	ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+40, int(btnY)+12)

	// Error display
	if g.ui.MilitiaError != "" {
		errorY := int(btnY) - 30
		errorText := fmt.Sprintf("Erreur: %s", g.ui.MilitiaError)
		maxLen := 60
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

// drawSlider draws a slider control
func drawSlider(screen *ebiten.Image, x, y, w, h float64, value, min, max int) {
	// Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{50, 50, 50, 255}, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 1, color.Gray{150}, true)

	// Fill (value indicator)
	if max > min {
		fillRatio := float64(value-min) / float64(max-min)
		if fillRatio > 1.0 {
			fillRatio = 1.0
		}
		if fillRatio < 0.0 {
			fillRatio = 0.0
		}
		fillW := w * fillRatio
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(fillW), float32(h), color.RGBA{0, 150, 0, 255}, true)
	}

	// Handle (draggable thumb)
	handleW := 10.0
	if max > min {
		handleX := x + (w-handleW)*(float64(value-min)/float64(max-min))
		if handleX < x {
			handleX = x
		}
		if handleX > x+w-handleW {
			handleX = x + w - handleW
		}
		vector.DrawFilledRect(screen, float32(handleX), float32(y-2), float32(handleW), float32(h+4), color.RGBA{200, 200, 200, 255}, true)
		vector.StrokeRect(screen, float32(handleX), float32(y-2), float32(handleW), float32(h+4), 1, color.White, true)
	}
}

// UpdateMilitiaUI handles input for the Militia UI
func (g *Game) UpdateMilitiaUI() bool {
	if !g.ui.ShowMilitiaUI {
		return false
	}

	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 700.0, 600.0
	x, y := cx-w/2, cy-h/2

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
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

	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Slider positions (synchronized with Draw)
	sliderX := x + 20
	sliderW := w - 40
	sliderH := 20.0
	sliderY := float64(y) + 170
	sliderCaptureH := 30.0 // Larger hit area

	warriorY := sliderY
	archerY := warriorY + 60
	gunnerY := archerY + 60

	// Stop dragging when mouse released
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		closeBtnSize := 30.0
		closeX := x + w - closeBtnSize - 10
		closeY := y + 10
		if fmx >= closeX && fmx <= closeX+closeBtnSize && fmy >= closeY && fmy <= closeY+closeBtnSize {
			g.ui.ShowMilitiaUI = false
			g.ui.MilitiaError = ""
			g.ui.DraggingMilitiaWarrior = false
			g.ui.DraggingMilitiaArcher = false
			g.ui.DraggingMilitiaGunner = false
			return true
		}
	}

	// PRIORITY 2: Start dragging on mouse down over slider (EXCLUSIVE with else if)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.ui.DraggingMilitiaWarrior {
			g.ui.MilitiaWarriors = calcSliderValue(fmx)
		} else if g.ui.DraggingMilitiaArcher {
			g.ui.MilitiaArchers = calcSliderValue(fmx)
		} else if g.ui.DraggingMilitiaGunner {
			g.ui.MilitiaGunners = calcSliderValue(fmx)
		}
	}

	// PRIORITY 4: Handle buttons (only if not currently dragging)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.ui.DraggingMilitiaWarrior && !g.ui.DraggingMilitiaArcher && !g.ui.DraggingMilitiaGunner {
		// Upgrade button
		if militiaBuilding != nil && !militiaBuilding.Constructing {
			upgradeBtnX := x + w - 150
			upgradeBtnY := y + 45
			upgradeBtnW := 100.0
			upgradeBtnH := 30.0
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
							g.ui.MilitiaError = err.Error()
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

		btnY := y + h - 80
		btnW := 150.0
		btnH := 40.0
		btnX := x + w/2 - btnW/2

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
				g.ui.MilitiaError = err.Error()
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
