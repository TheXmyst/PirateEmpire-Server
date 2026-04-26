package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawCrewAssignmentModal draws a modal to assign/unassign crew to/from a ship
func (g *Game) DrawCrewAssignmentModal(screen *ebiten.Image) {
	if !g.ui.ShowCrewModal {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	modalW, modalH := 600.0, 550.0
	modalX, modalY := cx-modalW/2, cy-modalH/2

	// Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, true)

	// Modal background
	vector.DrawFilledRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), color.RGBA{40, 30, 20, 255}, true)
	vector.StrokeRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), 2, color.RGBA{200, 180, 100, 255}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "ASSIGNATION DE MILICE", int(modalX)+20, int(modalY)+20)

	// Close button (X)
	closeX := modalX + modalW - 30
	closeY := modalY + 10
	closeSize := 20.0
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeSize), float32(closeSize), color.RGBA{150, 0, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+6, int(closeY)+3)

	// Get ship and island data
	var ship *domain.Ship
	var island *domain.Island
	shipID, err := uuid.Parse(g.ui.SelectedShipForCrew)
	if err == nil && g.player != nil && len(g.player.Islands) > 0 {
		island = &g.player.Islands[0]
		for i := range island.Ships {
			if island.Ships[i].ID == shipID {
				ship = &island.Ships[i]
				break
			}
		}
	}

	if ship == nil {
		ebitenutil.DebugPrintAt(screen, "Navire introuvable", int(modalX)+20, int(modalY)+50)
		return
	}

	// Ship info
	contentY := modalY + 50
	contentX := modalX + 20
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Navire: %s (%s)", ship.Name, ship.Type), int(contentX), int(contentY))

	// Current crew on ship
	currentTotal := ship.MilitiaWarriors + ship.MilitiaArchers + ship.MilitiaGunners
	maxCrew := 50 // Default
	switch ship.Type {
	case "sloop":
		maxCrew = 50
	case "brigantine":
		maxCrew = 100
	case "frigate":
		maxCrew = 150
	case "galleon":
		maxCrew = 200
	case "manowar":
		maxCrew = 250
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Milice actuelle: %d/%d (G:%d A:%d Ar:%d)", currentTotal, maxCrew, ship.MilitiaWarriors, ship.MilitiaArchers, ship.MilitiaGunners), int(contentX), int(contentY)+20)

	// Stock global disponible
	stockY := contentY + 50
	if island != nil {
		ebitenutil.DebugPrintAt(screen, "Stock global disponible:", int(contentX), int(stockY))
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Guerriers: %d | Archers: %d | Artilleurs: %d", island.Crew[domain.Warrior], island.Crew[domain.Archer], island.Crew[domain.Gunner]), int(contentX), int(stockY)+20)
	}

	// Separator
	vector.StrokeLine(screen, float32(contentX), float32(stockY)+45, float32(modalX+modalW-20), float32(stockY)+45, 1, color.Gray{100}, true)

	// Sliders section
	slidersY := stockY + 60
	ebitenutil.DebugPrintAt(screen, "Assignation:", int(contentX), int(slidersY))

	sliderX := contentX
	sliderW := modalW - 40
	sliderH := 20.0
	sliderCaptureH := 30.0 // Match UpdateCrewAssignmentModal

	// Get mouse position for hover detection
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Warrior slider
	warriorY := slidersY + 30
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Guerriers: %d", g.ui.CrewModalWarriors), int(sliderX), int(warriorY))
	warriorSliderY := warriorY + 20
	isWarriorHovered := fmy >= warriorSliderY-sliderCaptureH/2 && fmy <= warriorSliderY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW
	drawCrewSlider(screen, sliderX, warriorSliderY, sliderW, sliderH, g.ui.CrewModalWarriors, 0, maxCrew, isWarriorHovered, g.ui.DraggingCrewWarrior)

	// Archer slider
	archerY := warriorY + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Archers: %d", g.ui.CrewModalArchers), int(sliderX), int(archerY))
	archerSliderY := archerY + 20
	isArcherHovered := fmy >= archerSliderY-sliderCaptureH/2 && fmy <= archerSliderY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW
	drawCrewSlider(screen, sliderX, archerSliderY, sliderW, sliderH, g.ui.CrewModalArchers, 0, maxCrew, isArcherHovered, g.ui.DraggingCrewArcher)

	// Gunner slider
	gunnerY := archerY + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Artilleurs: %d", g.ui.CrewModalGunners), int(sliderX), int(gunnerY))
	gunnerSliderY := gunnerY + 20
	isGunnerHovered := fmy >= gunnerSliderY-sliderCaptureH/2 && fmy <= gunnerSliderY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW
	drawCrewSlider(screen, sliderX, gunnerSliderY, sliderW, sliderH, g.ui.CrewModalGunners, 0, maxCrew, isGunnerHovered, g.ui.DraggingCrewGunner)

	// Calculate differences (how much to assign/unassign)
	warriorDiff := g.ui.CrewModalWarriors - ship.MilitiaWarriors
	archerDiff := g.ui.CrewModalArchers - ship.MilitiaArchers
	gunnerDiff := g.ui.CrewModalGunners - ship.MilitiaGunners

	// Info section
	infoY := gunnerY + 60
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Différence: G:%+d A:%+d Ar:%+d", warriorDiff, archerDiff, gunnerDiff), int(contentX), int(infoY))

	// Calculate stock after (for display)
	stockAfterW := island.Crew[domain.Warrior]
	stockAfterA := island.Crew[domain.Archer]
	stockAfterG := island.Crew[domain.Gunner]
	if warriorDiff > 0 {
		stockAfterW -= warriorDiff
	} else {
		stockAfterW -= warriorDiff // warriorDiff is negative, so this adds
	}
	if archerDiff > 0 {
		stockAfterA -= archerDiff
	} else {
		stockAfterA -= archerDiff
	}
	if gunnerDiff > 0 {
		stockAfterG -= gunnerDiff
	} else {
		stockAfterG -= gunnerDiff
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Stock après: G:%d A:%d Ar:%d", stockAfterW, stockAfterA, stockAfterG), int(contentX), int(infoY)+20)

	// Buttons
	btnY := modalY + modalH - 80
	btnW := 120.0
	btnH := 35.0
	btnSpacing := 20.0

	// Assign button (if positive diff)
	hasAssign := warriorDiff > 0 || archerDiff > 0 || gunnerDiff > 0
	assignBtnX := modalX + modalW/2 - btnW - btnSpacing/2
	assignBtnCol := color.RGBA{0, 150, 0, 255}
	if !hasAssign || g.ui.CrewModalBusy {
		assignBtnCol = color.RGBA{100, 100, 100, 255}
	}
	vector.DrawFilledRect(screen, float32(assignBtnX), float32(btnY), float32(btnW), float32(btnH), assignBtnCol, true)
	vector.StrokeRect(screen, float32(assignBtnX), float32(btnY), float32(btnW), float32(btnH), 2, color.White, true)
	ebitenutil.DebugPrintAt(screen, "ASSIGNER", int(assignBtnX)+25, int(btnY)+10)

	// Unassign button (if negative diff)
	hasUnassign := warriorDiff < 0 || archerDiff < 0 || gunnerDiff < 0
	unassignBtnX := modalX + modalW/2 + btnSpacing/2
	unassignBtnCol := color.RGBA{150, 50, 50, 255}
	if !hasUnassign || g.ui.CrewModalBusy {
		unassignBtnCol = color.RGBA{100, 100, 100, 255}
	}
	vector.DrawFilledRect(screen, float32(unassignBtnX), float32(btnY), float32(btnW), float32(btnH), unassignBtnCol, true)
	vector.StrokeRect(screen, float32(unassignBtnX), float32(btnY), float32(btnW), float32(btnH), 2, color.White, true)
	ebitenutil.DebugPrintAt(screen, "RETIRER", int(unassignBtnX)+30, int(btnY)+10)

	// Error display
	if g.ui.CrewModalError != "" {
		errorY := int(btnY) - 30
		errorText := fmt.Sprintf("Erreur: %s", g.ui.CrewModalError)
		maxLen := 55
		for len(errorText) > maxLen {
			line := errorText[:maxLen]
			errorText = errorText[maxLen:]
			ebitenutil.DebugPrintAt(screen, line, int(contentX), errorY)
			errorY += 15
		}
		if errorText != "" {
			ebitenutil.DebugPrintAt(screen, errorText, int(contentX), errorY)
		}
	}

	// Busy indicator
	if g.ui.CrewModalBusy {
		ebitenutil.DebugPrintAt(screen, "Assignation en cours...", int(contentX), int(btnY)-15)
	}
}

// drawCrewSlider draws a slider with bounds based on stock and capacity
func drawCrewSlider(screen *ebiten.Image, x, y, w, h float64, value, min, max int, isHovered, isDragging bool) {
	// Background (with hover/drag feedback)
	bgColor := color.RGBA{50, 50, 50, 255}
	if isDragging {
		bgColor = color.RGBA{70, 70, 70, 255} // Lighter when dragging
	} else if isHovered {
		bgColor = color.RGBA{60, 60, 60, 255} // Slightly lighter when hovered
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), bgColor, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 1, color.Gray{150}, true)

	// Fill (value indicator) with glow effect when dragging
	if max > min {
		fillRatio := float64(value-min) / float64(max-min)
		if fillRatio > 1.0 {
			fillRatio = 1.0
		}
		if fillRatio < 0.0 {
			fillRatio = 0.0
		}
		fillW := w * fillRatio
		fillColor := color.RGBA{0, 150, 0, 255}
		if isDragging {
			fillColor = color.RGBA{0, 180, 0, 255} // Brighter green when dragging
		}
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(fillW), float32(h), fillColor, true)
	}

	// Handle (draggable thumb) with enhanced visuals
	handleW := 10.0
	if max > min {
		handleX := x + (w-handleW)*(float64(value-min)/float64(max-min))
		if handleX < x {
			handleX = x
		}
		if handleX > x+w-handleW {
			handleX = x + w - handleW
		}

		// Handle color changes based on state
		handleColor := color.RGBA{200, 200, 200, 255}
		if isDragging {
			handleColor = color.RGBA{255, 255, 255, 255} // White when dragging
		} else if isHovered {
			handleColor = color.RGBA{230, 230, 230, 255} // Lighter when hovered
		}

		vector.DrawFilledRect(screen, float32(handleX), float32(y-2), float32(handleW), float32(h+4), handleColor, true)
		vector.StrokeRect(screen, float32(handleX), float32(y-2), float32(handleW), float32(h+4), 1, color.White, true)
	}
}

// UpdateCrewAssignmentModal handles input for the crew assignment modal
func (g *Game) UpdateCrewAssignmentModal() bool {
	if !g.ui.ShowCrewModal {
		return false
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	modalW, modalH := 600.0, 550.0
	modalX, modalY := cx-modalW/2, cy-modalH/2

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowCrewModal = false
		g.ui.SelectedShipForCrew = ""
		g.ui.CrewModalError = ""
		// Reset drag state
		g.ui.DraggingCrewWarrior = false
		g.ui.DraggingCrewArcher = false
		g.ui.DraggingCrewGunner = false
		return true
	}

	// Get ship and island data
	var ship *domain.Ship
	var island *domain.Island
	shipID, err := uuid.Parse(g.ui.SelectedShipForCrew)
	if err == nil && g.player != nil && len(g.player.Islands) > 0 {
		island = &g.player.Islands[0]
		for i := range island.Ships {
			if island.Ships[i].ID == shipID {
				ship = &island.Ships[i]
				break
			}
		}
	}

	if ship == nil {
		return true
	}

	maxCrew := 50
	switch ship.Type {
	case "sloop":
		maxCrew = 50
	case "brigantine":
		maxCrew = 100
	case "frigate":
		maxCrew = 150
	case "galleon":
		maxCrew = 200
	case "manowar":
		maxCrew = 250
	}

	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Synchronized slider positions (match DrawCrewAssignmentModal)
	contentY := modalY + 50
	stockY := contentY + 50
	slidersY := stockY + 60
	sliderX := modalX + 20
	sliderW := modalW - 40
	sliderH := 20.0
	sliderCaptureH := 30.0 // Larger hit area for easier clicking

	warriorY := slidersY + 30 + 20 // +20 for label offset
	archerY := warriorY + 60
	gunnerY := archerY + 60

	// Stop dragging when mouse released
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.ui.DraggingCrewWarrior = false
		g.ui.DraggingCrewArcher = false
		g.ui.DraggingCrewGunner = false
	}

	// Helper function to calculate slider value from mouse X position
	calcSliderValue := func(mouseX float64, currentCrew int, stockCrew int) int {
		// Calculate raw value based on mouse position
		ratio := (mouseX - sliderX) / sliderW
		if ratio < 0.0 {
			ratio = 0.0
		}
		if ratio > 1.0 {
			ratio = 1.0
		}
		value := int(ratio * float64(maxCrew))

		// Clamp to available stock + current crew
		maxAvailable := currentCrew + stockCrew
		if value > maxAvailable {
			value = maxAvailable
		}
		if value < 0 {
			value = 0
		}

		return value
	}

	// PRIORITY 1: Handle close button FIRST (before sliders consume input)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		closeX := modalX + modalW - 30
		closeY := modalY + 10
		closeSize := 20.0
		if fmx >= closeX && fmx <= closeX+closeSize && fmy >= closeY && fmy <= closeY+closeSize {
			g.ui.ShowCrewModal = false
			g.ui.SelectedShipForCrew = ""
			g.ui.CrewModalError = ""
			g.ui.DraggingCrewWarrior = false
			g.ui.DraggingCrewArcher = false
			g.ui.DraggingCrewGunner = false
			return true
		}
	}

	// PRIORITY 2: Start dragging on mouse down over slider (EXCLUSIVE with else if)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Warrior slider (expanded hit area)
		if fmy >= warriorY-sliderCaptureH/2 && fmy <= warriorY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			g.ui.DraggingCrewWarrior = true
			g.ui.DraggingCrewArcher = false // Disable others
			g.ui.DraggingCrewGunner = false // Disable others
			g.ui.CrewModalWarriors = calcSliderValue(fmx, ship.MilitiaWarriors, island.Crew[domain.Warrior])
		} else if fmy >= archerY-sliderCaptureH/2 && fmy <= archerY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			// Archer slider (expanded hit area)
			g.ui.DraggingCrewWarrior = false // Disable others
			g.ui.DraggingCrewArcher = true
			g.ui.DraggingCrewGunner = false // Disable others
			g.ui.CrewModalArchers = calcSliderValue(fmx, ship.MilitiaArchers, island.Crew[domain.Archer])
		} else if fmy >= gunnerY-sliderCaptureH/2 && fmy <= gunnerY+sliderH+sliderCaptureH/2 && fmx >= sliderX && fmx <= sliderX+sliderW {
			// Gunner slider (expanded hit area)
			g.ui.DraggingCrewWarrior = false // Disable others
			g.ui.DraggingCrewArcher = false  // Disable others
			g.ui.DraggingCrewGunner = true
			g.ui.CrewModalGunners = calcSliderValue(fmx, ship.MilitiaGunners, island.Crew[domain.Gunner])
		}
	}

	// PRIORITY 3: Continue dragging if mouse button held down
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.ui.DraggingCrewWarrior {
			g.ui.CrewModalWarriors = calcSliderValue(fmx, ship.MilitiaWarriors, island.Crew[domain.Warrior])
		} else if g.ui.DraggingCrewArcher {
			g.ui.CrewModalArchers = calcSliderValue(fmx, ship.MilitiaArchers, island.Crew[domain.Archer])
		} else if g.ui.DraggingCrewGunner {
			g.ui.CrewModalGunners = calcSliderValue(fmx, ship.MilitiaGunners, island.Crew[domain.Gunner])
		}
	}

	// PRIORITY 4: Handle buttons (only if not currently dragging)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.ui.DraggingCrewWarrior && !g.ui.DraggingCrewArcher && !g.ui.DraggingCrewGunner {
		btnY := modalY + modalH - 80
		btnW := 120.0
		btnH := 35.0
		btnSpacing := 20.0

		// Calculate differences
		warriorDiff := g.ui.CrewModalWarriors - ship.MilitiaWarriors
		archerDiff := g.ui.CrewModalArchers - ship.MilitiaArchers
		gunnerDiff := g.ui.CrewModalGunners - ship.MilitiaGunners

		// Assign button
		hasAssign := warriorDiff > 0 || archerDiff > 0 || gunnerDiff > 0
		assignBtnX := modalX + modalW/2 - btnW - btnSpacing/2
		if hasAssign && !g.ui.CrewModalBusy {
			if fmx >= assignBtnX && fmx <= assignBtnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				g.assignCrewToShip(shipID, warriorDiff, archerDiff, gunnerDiff)
				return true
			}
		}

		// Unassign button
		hasUnassign := warriorDiff < 0 || archerDiff < 0 || gunnerDiff < 0
		unassignBtnX := modalX + modalW/2 + btnSpacing/2
		if hasUnassign && !g.ui.CrewModalBusy {
			if fmx >= unassignBtnX && fmx <= unassignBtnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				// Unassign: use positive values
				unassignW := -warriorDiff
				unassignA := -archerDiff
				unassignG := -gunnerDiff
				g.unassignCrewFromShip(shipID, unassignW, unassignA, unassignG)
				return true
			}
		}

		// Click inside modal (consume input)
		if fmx >= modalX && fmx <= modalX+modalW && fmy >= modalY && fmy <= modalY+modalH {
			return true
		}
	}

	// Always consume input when modal is open
	return true
}

// assignCrewToShip assigns crew from island stock to a ship
func (g *Game) assignCrewToShip(shipID uuid.UUID, warriors, archers, gunners int) {
	if warriors <= 0 && archers <= 0 && gunners <= 0 {
		return
	}

	g.ui.CrewModalBusy = true
	g.ui.CrewModalError = ""

	go func() {
		defer func() {
			g.ui.CrewModalBusy = false
		}()

		err := g.api.AssignCrew(shipID, warriors, archers, gunners)
		if err != nil {
			g.ui.CrewModalError = err.Error()
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.CrewModalError = "Session expirée, veuillez vous reconnecter"
			}
			g.Log("Crew assign error: %v", err)
			return
		}

		// Success - refresh status
		g.refreshStatusAfterDevAction()
		g.Log("Crew assigned: ship=%s warriors=%d archers=%d gunners=%d", shipID.String()[:8], warriors, archers, gunners)
	}()
}

// unassignCrewFromShip removes crew from a ship and returns it to island stock
func (g *Game) unassignCrewFromShip(shipID uuid.UUID, warriors, archers, gunners int) {
	if warriors <= 0 && archers <= 0 && gunners <= 0 {
		return
	}

	g.ui.CrewModalBusy = true
	g.ui.CrewModalError = ""

	go func() {
		defer func() {
			g.ui.CrewModalBusy = false
		}()

		err := g.api.UnassignCrew(shipID, warriors, archers, gunners)
		if err != nil {
			g.ui.CrewModalError = err.Error()
			if strings.Contains(err.Error(), "session expirée") {
				g.ui.CrewModalError = "Session expirée, veuillez vous reconnecter"
			}
			g.Log("Crew unassign error: %v", err)
			return
		}

		// Success - refresh status
		g.refreshStatusAfterDevAction()
		g.Log("Crew unassigned: ship=%s warriors=%d archers=%d gunners=%d", shipID.String()[:8], warriors, archers, gunners)
	}()
}
