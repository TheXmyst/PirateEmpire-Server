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

// GetActiveFleetShips returns all ships currently in a fleet.
// Since destroyed ships are permanently deleted on the server,
// every ship in the list is active (Ready, Repaired, or UnderConstruction).
func GetActiveFleetShips(fleet *domain.Fleet) []domain.Ship {
	if fleet == nil {
		return nil
	}
	return fleet.Ships
}

// DrawFleetList renders list of 3 fleets on the left
func (g *Game) DrawFleetList(screen *ebiten.Image, x, y, w, h float64) {
	// Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 30, 40, 200}, true)

	// Calculate limits
	shipyardLevel := g.getShipyardLevel()
	maxFleets := 1
	if shipyardLevel >= 20 {
		maxFleets = 3
	} else if shipyardLevel >= 10 {
		maxFleets = 2
	}

	maxShips := 3
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			if id == "nav_fleet_1" {
				maxShips++
			}
			if id == "nav_fleet_2" {
				maxShips++
			}
		}
	}

	slotH := 120.0
	padding := 10.0
	startY := y + 10

	mx, my := ebiten.CursorPosition()

	for i := 1; i <= 3; i++ {
		slotY := startY + float64(i-1)*(slotH+padding)
		locked := i > maxFleets

		// Check hover (shielded by modal)
		hover := false
		if !shipyardUI.ShowAddShipModal {
			hover = float64(mx) >= x && float64(mx) <= x+w && float64(my) >= slotY && float64(my) <= slotY+slotH
		}

		if locked {
			// Locked state
			vector.DrawFilledRect(screen, float32(x), float32(slotY), float32(w), float32(slotH), color.RGBA{50, 50, 50, 200}, true)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Flotte %d (Verrouillée)", i), int(x)+10, int(slotY)+10)
			req := 10
			if i == 3 {
				req = 20
			}
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Req: Niv. %d", req), int(x)+10, int(slotY)+30)
		} else {
			// Unlocked state
			col := color.RGBA{30, 40, 50, 255}
			if shipyardUI.SelectedFleetIndex == i {
				col = color.RGBA{0, 100, 200, 255} // Blue for selected
			} else if hover {
				col = color.RGBA{50, 60, 70, 255}
			}

			vector.DrawFilledRect(screen, float32(x), float32(slotY), float32(w), float32(slotH), col, true)

			// Get ship count - find fleet by name to get accurate count (filter destroyed ships)
			shipCount := 0
			expectedName := fmt.Sprintf("Flotte %d", i)
			if g.player != nil && len(g.player.Islands) > 0 {
				for _, fleet := range g.player.Islands[0].Fleets {
					if fleet.Name == expectedName {
						// Capacity based on existing ships (len)
						shipCount = len(fleet.Ships)
						break
					}
				}
			}

			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Flotte %d", i), int(x)+10, int(slotY)+10)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d / %d Navires", shipCount, maxShips), int(x)+10, int(slotY)+30)

			// Click to select
			if hover && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				shipyardUI.SelectedFleetIndex = i
			}
		}
	}
}

// DrawFleetDetails renders details of selected fleet on the right
func (g *Game) DrawFleetDetails(screen *ebiten.Image, x, y, w, h float64) {
	// Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 30, 40, 200}, true)

	if shipyardUI.SelectedFleetIndex == 0 {
		ebitenutil.DebugPrintAt(screen, "Sélectionnez une flotte.", int(x)+10, int(y)+10)
		return
	}

	// Calculate maxFleets
	shipyardLevel := g.getShipyardLevel()
	maxFleets := 1
	if shipyardLevel >= 20 {
		maxFleets = 3
	} else if shipyardLevel >= 10 {
		maxFleets = 2
	}

	selIdx := shipyardUI.SelectedFleetIndex
	if selIdx > maxFleets {
		ebitenutil.DebugPrintAt(screen, "Flotte Verrouillée.", int(x)+10, int(y)+10)
		return
	}

	// Title
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FLOTTE %d", selIdx), int(x)+10, int(y)+10)

	// Get max ships
	maxShips := 3
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			if id == "nav_fleet_1" {
				maxShips++
			}
			if id == "nav_fleet_2" {
				maxShips++
			}
		}
	}

	listY := y + 40
	var currentFleet *domain.Fleet
	currentShips := 0

	// Find fleet by index (Flotte 1, 2, or 3) - use actual fleet from server data
	// Use single source of truth function to get active ships
	if g.player != nil && len(g.player.Islands) > 0 {
		// Find fleet by matching name (Flotte 1, Flotte 2, Flotte 3)
		expectedName := fmt.Sprintf("Flotte %d", selIdx)
		for i := range g.player.Islands[0].Fleets {
			if g.player.Islands[0].Fleets[i].Name == expectedName {
				currentFleet = &g.player.Islands[0].Fleets[i]
				break
			}
		}
		// Fallback: if not found by name, try by index (for backward compatibility)
		if currentFleet == nil && selIdx <= len(g.player.Islands[0].Fleets) {
			currentFleet = &g.player.Islands[0].Fleets[selIdx-1]
		}
	}

	// Get active ships using single source of truth function
	var activeShips []domain.Ship
	if currentFleet != nil {
		activeShips = GetActiveFleetShips(currentFleet)
		currentShips = len(activeShips)
		g.Log("[SHIPYARD] draw fleet=%s ships_total=%d ships_active=%d", currentFleet.ID.String()[:8], len(currentFleet.Ships), currentShips)
	}

	if currentFleet == nil {
		ebitenutil.DebugPrintAt(screen, "Flotte non initialisée.", int(x)+10, int(listY))
	} else {
		if currentShips == 0 {
			ebitenutil.DebugPrintAt(screen, "Aucun navire", int(x)+10, int(listY))
		} else {
			// Display only active ships (already filtered)
			for k, s := range activeShips {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("- %s (%s)", s.Name, s.Type), int(x)+10, int(listY+float64(k)*25))
			}
		}
	}

	// Capacity display
	capacityY := y + h - 80
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Capacité : %d / %d", currentShips, maxShips), int(x)+10, int(capacityY))

	// Add ship button
	btnW, btnH := 200.0, 40.0
	btnX, btnY := x+10, y+h-50

	btnCol := color.RGBA{0, 150, 0, 255}
	btnText := "Ajouter un navire"
	disabled := currentShips >= maxShips

	if disabled {
		btnCol = color.RGBA{80, 80, 80, 255}
		btnText = "Flotte Complète"
	}

	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
	ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+20, int(btnY)+12)

	// Handle click (shielded by modal)
	if !disabled && !shipyardUI.ShowAddShipModal && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= btnX && float64(mx) <= btnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
			shipyardUI.ShowAddShipModal = true
			shipyardUI.TargetFleetIndex = selIdx
			shipyardUI.SelectedShipID = nil
			shipyardUI.ModalOpenDelay = 10
			shipyardUI.ModalMessage = ""                           // Clear any previous error message
			shipyardUI.CloseShipAssignModalAfterNextStatus = false // Reset flag
		}
	}
}

// UpdateAddShipModalShipyard handles input for the ship assignment modal
// Returns true if input was consumed (modal is open and handled clicks)
func (g *Game) UpdateAddShipModalShipyard() bool {
	if !shipyardUI.ShowAddShipModal {
		return false
	}

	// Update delay counter
	if shipyardUI.ModalOpenDelay > 0 {
		shipyardUI.ModalOpenDelay--
		return true // Block input while delay is active
	}
	inputEnabled := shipyardUI.ModalOpenDelay == 0

	// Update success timer (decrement each frame)
	if shipyardUI.AssignSuccessTimer > 0 {
		shipyardUI.AssignSuccessTimer--
	}

	if !inputEnabled {
		return true
	}

	// Handle ESC key to close modal
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		shipyardUI.ShowAddShipModal = false
		shipyardUI.SelectedShipID = nil
		shipyardUI.ModalMessage = ""
		return true
	}

	// Handle mouse clicks
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true // Modal is open, consume input even if no click
	}

	mx, my := ebiten.CursorPosition()
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	modalW, modalH := 500.0, 450.0
	mxStart, myStart := (w-modalW)/2, (h-modalH)/2

	g.Log("[SHIPYARD_DEBUG] Click at (%d, %d). Modal at (%.0f, %.0f)", mx, my, mxStart, myStart)

	// Find target fleet and calculate capacity (same logic as Draw)
	var targetFleet *domain.Fleet
	fleetCapacity := 3
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			if id == "nav_fleet_1" {
				fleetCapacity++
			}
			if id == "nav_fleet_2" {
				fleetCapacity++
			}
		}
	}
	if g.player != nil && len(g.player.Islands) > 0 {
		expectedName := fmt.Sprintf("Flotte %d", shipyardUI.TargetFleetIndex)
		for i := range g.player.Islands[0].Fleets {
			if g.player.Islands[0].Fleets[i].Name == expectedName {
				targetFleet = &g.player.Islands[0].Fleets[i]
				break
			}
		}
		if targetFleet == nil && shipyardUI.TargetFleetIndex <= len(g.player.Islands[0].Fleets) {
			targetFleet = &g.player.Islands[0].Fleets[shipyardUI.TargetFleetIndex-1]
		}
	}

	// Button dimensions
	btnW, btnH := 120.0, 30.0

	// Cancel button
	btnCancelX, btnCancelY := mxStart+modalW-btnW-20, myStart+modalH-50
	hoverCancel := float64(mx) >= btnCancelX && float64(mx) <= btnCancelX+btnW && float64(my) >= btnCancelY && float64(my) <= btnCancelY+btnH
	if hoverCancel {
		g.Log("[SHIPYARD_DEBUG] Cancel button clicked")
		shipyardUI.ShowAddShipModal = false
		shipyardUI.SelectedShipID = nil
		shipyardUI.ModalMessage = ""
		return true
	}

	// Assign button
	btnOkX, btnOkY := mxStart+20, myStart+modalH-50
	hoverOk := float64(mx) >= btnOkX && float64(mx) <= btnOkX+btnW && float64(my) >= btnOkY && float64(my) <= btnOkY+btnH
	disabled := shipyardUI.SelectedShipID == nil

	if inputEnabled && hoverOk {
		if disabled {
			g.Log("[SHIPYARD_DEBUG] Assign button clicked but DISABLED (no ship selected)")
			shipyardUI.ModalMessage = "Sélectionnez un navire"
			return true
		}
		g.Log("[SHIPYARD_DEBUG] Assign button clicked (Ship ID: %v)", shipyardUI.SelectedShipID)
		// Clear any previous error message before new attempt
		shipyardUI.ModalMessage = ""

		// Re-check capacity before assigning (defensive: recalculate from g.player, no cache)
		if targetFleet == nil {
			shipyardUI.ModalMessage = "Erreur: Flotte introuvable (status non synchronisé)"
			return true
		}

		// Recalculate active ships from current state (no cache)
		activeShips := GetActiveFleetShips(targetFleet)
		activeCount := len(activeShips)

		// Final capacity check using active ships (single source of truth)
		if activeCount >= fleetCapacity {
			shipyardUI.ModalMessage = fmt.Sprintf("Flotte pleine (%d/%d)", activeCount, fleetCapacity)
			g.Log("[SHIPYARD] assign blocked client-side active=%d cap=%d", activeCount, fleetCapacity)
			return true
		}

		shipID := *shipyardUI.SelectedShipID
		fleetID := targetFleet.ID

		shipyardUI.ModalMessage = "Assignation en cours..."
		shipyardUI.AssignSuccessTimer = 0 // Reset timer

		// Log assign click with full details
		g.Log("[SHIPYARD] assign click fleet=%s ship=%s cap=%d active=%d -> calling api", fleetID.String()[:8], shipID.String()[:8], fleetCapacity, activeCount)

		go func() {
			g.Log("[SHIPYARD] api call start fleet=%s ship=%s", fleetID.String()[:8], shipID.String()[:8])
			err := g.api.AddShipToFleet(fleetID, shipID)
			if err != nil {
				// Parse error to extract reason_code if present
				errStr := err.Error()
				reasonCode := ""
				errorMsg := errStr
				if strings.HasPrefix(errStr, "REASON_CODE:") {
					parts := strings.SplitN(errStr, "|", 2)
					if len(parts) == 2 {
						reasonCode = strings.TrimPrefix(parts[0], "REASON_CODE:")
						errorMsg = parts[1]
					}
				}
				g.Log("[SHIPYARD] assign refused err=%s code=%s", errorMsg, reasonCode)
				// Display the actual error message from the server
				shipyardUI.ModalMessage = fmt.Sprintf("Erreur: %s", errorMsg)
				shipyardUI.AssignSuccessTimer = 0    // Reset timer on error
				shipyardUI.AssignSuccessShipID = nil // Clear on error
			} else {
				g.Log("[SHIPYARD] assign api success -> waiting for status refresh ship=%s", shipID.String()[:8])
				shipyardUI.ModalMessage = "Succès !"
				// Store ship ID for verification after status refresh
				shipyardUI.AssignSuccessShipID = &shipID
				// Set timer to keep "Succès !" visible for at least 1 second (60 frames at 60fps)
				shipyardUI.AssignSuccessTimer = 60
				// Refresh data using updateChan (thread-safe, applies on main thread)
				// Set flag to close modal after status is applied AND timer expires
				shipyardUI.CloseShipAssignModalAfterNextStatus = true
				g.refreshStatusAfterDevAction()
				g.Log("[SHIPYARD] status refresh requested after assign success")
			}
		}()
		return true
	}

	// Handle ship selection clicks
	shipsY := myStart + 110
	var availableShips []*domain.Ship
	if g.player != nil && len(g.player.Islands) > 0 {
		for i := range g.player.Islands[0].Ships {
			s := &g.player.Islands[0].Ships[i]
			if s.FleetID == nil && s.State != "UnderConstruction" {
				availableShips = append(availableShips, s)
			}
		}
	}

	for i, s := range availableShips {
		rowY := shipsY + float64(i)*35
		rowW, rowH := modalW-40, 30.0
		rowX := mxStart + 20

		hover := float64(mx) >= rowX && float64(mx) <= rowX+rowW && float64(my) >= rowY && float64(my) <= rowY+rowH
		if hover {
			id := s.ID
			g.Log("[SHIPYARD_DEBUG] Ship selected: %s (%s)", s.Name, id.String()[:8])
			shipyardUI.SelectedShipID = &id
			return true
		}
	}

	// Click outside modal (on overlay) - close modal
	if float64(mx) < mxStart || float64(mx) > mxStart+modalW || float64(my) < myStart || float64(my) > myStart+modalH {
		// Don't close on overlay click, only on buttons
		// This allows clicking outside to close if desired, but we'll keep it open for now
	}

	return true // Modal is open, always consume input
}

// DrawAddShipModalShipyard renders modal for adding ship to fleet
func (g *Game) DrawAddShipModalShipyard(screen *ebiten.Image) {
	if !shipyardUI.ShowAddShipModal {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)

	// Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, false)

	// Modal window (increased height to accommodate capacity info block)
	modalW, modalH := 500.0, 450.0
	mxStart, myStart := (w-modalW)/2, (h-modalH)/2

	draw9Slice(screen, g, mxStart, myStart, modalW, modalH, 12)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Ajouter un navire a la Flotte %d", shipyardUI.TargetFleetIndex), int(mxStart)+20, int(myStart)+20)

	// Recalculate fleet capacity from current g.player (no cache) - single source of truth
	var targetFleet *domain.Fleet
	var totalShipsCount int

	// Calculate fleet capacity FIRST (independent of targetFleet, needed for display)
	fleetCapacity := 3
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			if id == "nav_fleet_1" {
				fleetCapacity++
			}
			if id == "nav_fleet_2" {
				fleetCapacity++
			}
		}
	}

	// Find target fleet and calculate active ships
	if g.player != nil && len(g.player.Islands) > 0 {
		expectedName := fmt.Sprintf("Flotte %d", shipyardUI.TargetFleetIndex)
		for i := range g.player.Islands[0].Fleets {
			if g.player.Islands[0].Fleets[i].Name == expectedName {
				targetFleet = &g.player.Islands[0].Fleets[i]
				break
			}
		}
		if targetFleet == nil && shipyardUI.TargetFleetIndex <= len(g.player.Islands[0].Fleets) {
			targetFleet = &g.player.Islands[0].Fleets[shipyardUI.TargetFleetIndex-1]
		}

		if targetFleet != nil {
			totalShipsCount = len(targetFleet.Ships)

			// Log on modal open (only once, avoid spam)
			if shipyardUI.ModalOpenDelay == 9 { // Log once when delay is about to finish
				g.Log("[SHIPYARD] open assign-modal fleet=%s cap=%d total=%d", targetFleet.ID.String()[:8], fleetCapacity, totalShipsCount)
			}

			// Check if fleet is full
			if totalShipsCount >= fleetCapacity {
				g.Log("[SHIPYARD] fleet full computed total=%d cap=%d", totalShipsCount, fleetCapacity)
			}
		}
	}

	// Auto-clear stale error messages if capacity has changed
	if shipyardUI.ModalMessage != "" && targetFleet != nil {
		if len(targetFleet.Ships) < fleetCapacity {
			if strings.Contains(shipyardUI.ModalMessage, "Flotte pleine") {
				shipyardUI.ModalMessage = ""
				g.Log("[SHIPYARD] cleared stale 'Flotte pleine' message (total=%d < cap=%d)", len(targetFleet.Ships), fleetCapacity)
			}
		}
	}

	// Feedback message
	if shipyardUI.ModalMessage != "" {
		ebitenutil.DebugPrintAt(screen, shipyardUI.ModalMessage, int(mxStart)+20, int(myStart)+40)
	}

	// Display capacity info block
	infoY := myStart + 60
	if targetFleet != nil {
		ebitenutil.DebugPrintAt(screen, "Capacite:", int(mxStart)+20, int(infoY))
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  %d / %d", totalShipsCount, fleetCapacity), int(mxStart)+30, int(infoY)+15)
	} else {
		ebitenutil.DebugPrintAt(screen, "Flotte introuvable (status non synchronise)", int(mxStart)+20, int(infoY))
		ebitenutil.DebugPrintAt(screen, "Rafraichir...", int(mxStart)+20, int(infoY)+15)
	}

	// List available ships (below capacity info)
	shipsY := myStart + 110

	var availableShips []*domain.Ship
	if g.player != nil && len(g.player.Islands) > 0 {
		for i := range g.player.Islands[0].Ships {
			s := &g.player.Islands[0].Ships[i]
			// Only show ships that are available (not in a fleet, not under construction)
			if s.FleetID == nil && s.State != "UnderConstruction" {
				availableShips = append(availableShips, s)
			}
		}
	}

	curMx, curMy := ebiten.CursorPosition()

	if len(availableShips) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun navire disponible.", int(mxStart)+20, int(shipsY))
	} else {
		for i, s := range availableShips {
			rowY := shipsY + float64(i)*35
			rowW, rowH := modalW-40, 30.0
			rowX := mxStart + 20

			hover := float64(curMx) >= rowX && float64(curMx) <= rowX+rowW && float64(curMy) >= rowY && float64(curMy) <= rowY+rowH

			col := color.RGBA{50, 50, 50, 255}
			if shipyardUI.SelectedShipID != nil && *shipyardUI.SelectedShipID == s.ID {
				col = color.RGBA{0, 100, 200, 255} // Selected
			} else if hover {
				col = color.RGBA{80, 80, 80, 255}
			}

			vector.DrawFilledRect(screen, float32(rowX), float32(rowY), float32(rowW), float32(rowH), col, false)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%s)", s.Name, s.Type), int(rowX)+10, int(rowY)+8)

			// Ship selection is now handled in UpdateAddShipModalShipyard()
			// Visual feedback only here
		}
	}

	// Buttons (Assign, Cancel)
	btnW, btnH := 120.0, 30.0

	// Cancel
	btnCancelX, btnCancelY := mxStart+modalW-btnW-20, myStart+modalH-50
	hoverCancel := float64(curMx) >= btnCancelX && float64(curMx) <= btnCancelX+btnW && float64(curMy) >= btnCancelY && float64(curMy) <= btnCancelY+btnH
	colCancel := color.RGBA{150, 0, 0, 255}
	if hoverCancel {
		colCancel = color.RGBA{200, 50, 50, 255}
	}

	vector.DrawFilledRect(screen, float32(btnCancelX), float32(btnCancelY), float32(btnW), float32(btnH), colCancel, false)
	ebitenutil.DebugPrintAt(screen, "ANNULER", int(btnCancelX)+30, int(btnCancelY)+8)

	// Cancel button click is now handled in UpdateAddShipModalShipyard()

	// Assign
	btnOkX, btnOkY := mxStart+20, myStart+modalH-50
	hoverOk := float64(curMx) >= btnOkX && float64(curMx) <= btnOkX+btnW && float64(curMy) >= btnOkY && float64(curMy) <= btnOkY+btnH
	colOk := color.RGBA{0, 150, 0, 255}
	disabled := shipyardUI.SelectedShipID == nil
	if disabled {
		colOk = color.RGBA{100, 100, 100, 255}
	} else if hoverOk {
		colOk = color.RGBA{0, 200, 0, 255}
	}

	vector.DrawFilledRect(screen, float32(btnOkX), float32(btnOkY), float32(btnW), float32(btnH), colOk, false)
	ebitenutil.DebugPrintAt(screen, "ASSIGNER", int(btnOkX)+30, int(btnOkY)+8)

	// Input handling is now in UpdateAddShipModalShipyard()
}

// getAvailableShipTypes returns list of ships that can be built
func (g *Game) getAvailableShipTypes() []struct {
	ID           string
	Name         string
	Stats        string
	ReqLevel     int
	RequiredTech string
	Cost         map[domain.ResourceType]float64
	BuildTime    string
} {
	return []struct {
		ID           string
		Name         string
		Stats        string
		ReqLevel     int
		RequiredTech string
		Cost         map[domain.ResourceType]float64
		BuildTime    string
	}{
		{
			ID:           "sloop",
			Name:         "Sloop",
			Stats:        "Coque legere et rapide.\nPV: 100 | Cargo: 500",
			ReqLevel:     1,
			RequiredTech: "tech_naval_1", // Voiles légères
			Cost:         map[domain.ResourceType]float64{domain.Wood: 12000, domain.Gold: 4000},
			BuildTime:    "1h",
		},
		{
			ID:           "brigantine",
			Name:         "Brigantin",
			Stats:        "Polyvalent et robuste.\nPV: 250 | Cargo: 1200",
			ReqLevel:     5,
			RequiredTech: "tech_naval_2", // Voiles latines
			Cost:         map[domain.ResourceType]float64{domain.Wood: 25000, domain.Gold: 10000, domain.Rum: 1000, domain.Stone: 500},
			BuildTime:    "2h",
		},
		{
			ID:           "frigate",
			Name:         "Frégate",
			Stats:        "Navire de guerre.\nPV: 500 | Cargo: 1500",
			ReqLevel:     10,
			RequiredTech: "tech_naval_3", // Coque en cuivre
			Cost:         map[domain.ResourceType]float64{domain.Wood: 50000, domain.Gold: 25000, domain.Rum: 5000, domain.Stone: 2000},
			BuildTime:    "4h",
		},
		{
			ID:           "galleon",
			Name:         "Galion",
			Stats:        "Transport massif.\nPV: 1200 | Cargo: 5000",
			ReqLevel:     18,
			RequiredTech: "tech_naval_4", // Cartographie
			Cost:         map[domain.ResourceType]float64{domain.Wood: 100000, domain.Gold: 60000, domain.Rum: 10000, domain.Stone: 10000},
			BuildTime:    "8h",
		},
		{
			ID:           "manowar",
			Name:         "Vaisseau de Guerre",
			Stats:        "Forteresse flottante.\nPV: 3000 | Cargo: 8000",
			ReqLevel:     25,
			RequiredTech: "tech_naval_5", // Clipper
			Cost:         map[domain.ResourceType]float64{domain.Wood: 300000, domain.Gold: 200000, domain.Rum: 50000, domain.Stone: 50000},
			BuildTime:    "24h",
		},
	}
}
