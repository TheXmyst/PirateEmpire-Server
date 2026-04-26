package game

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// GetCaptainForShip returns the captain assigned to a given ship, or nil if none
func (g *Game) GetCaptainForShip(shipID string) *client.Captain {
	shipUUID, err := uuid.Parse(shipID)
	if err != nil {
		return nil
	}
	for i := range g.captains {
		if g.captains[i].AssignedShipID != nil && *g.captains[i].AssignedShipID == shipUUID {
			return &g.captains[i]
		}
	}
	return nil
}

// FormatCaptainPassiveSummary returns a human-readable summary of a captain's passive effect
func FormatCaptainPassiveSummary(captain *client.Captain) string {
	if captain == nil || captain.PassiveID == "" {
		return ""
	}

	switch captain.PassiveID {
	// COMMON
	case "nav_morale_decay_reduction":
		return fmt.Sprintf("Réduction perte moral: -%.0f%%", captain.PassiveValue*100)
	case "rum_consumption_reduction":
		return fmt.Sprintf("Réduction consommation rhum: -%.0f%%", captain.PassiveValue*100)
	case "morale_floor":
		return fmt.Sprintf("Moral minimum: ≥%d", captain.PassiveIntValue)
	case "wind_favorable_speed_bonus":
		return fmt.Sprintf("Vitesse vent favorable: +%.0f%%", captain.PassiveValue*100)
	case "port_morale_recovery_bonus":
		return fmt.Sprintf("Récupération moral au port: +%.0f%%", captain.PassiveValue*100)
	case "crew_loss_reduction":
		return fmt.Sprintf("Réduction pertes équipage: -%.0f%%", captain.PassiveValue*100)
	case "wind_unfavorable_penalty_reduction":
		return fmt.Sprintf("Réduction pénalité vent défavorable: -%.0f%%", captain.PassiveValue*100)
	case "low_morale_decay_slowdown":
		return fmt.Sprintf("Ralentissement perte moral (si moral < 40): -%.0f%%", captain.PassiveValue*100)

	// RARE
	case "interception_chance_bonus":
		return fmt.Sprintf("Chance interception: +%.0f%%", captain.PassiveValue*100)
	case "opening_enemy_morale_damage":
		return fmt.Sprintf("Dégâts moral ennemi (ouverture): +%d moral", captain.PassiveIntValue)
	case "enemy_morale_decay_multiplier":
		return fmt.Sprintf("Décomposition moral ennemi: +%.0f%%", captain.PassiveValue*100)
	case "low_morale_speed_bonus":
		return fmt.Sprintf("Vitesse bonus (si moral < %d): +%.0f%%", captain.Threshold, captain.PassiveValue*100)
	case "panic_immunity_threshold":
		return fmt.Sprintf("Immunité panique (si moral > %d)", captain.Threshold)

	// LEGENDARY
	case "wind_never_unfavorable":
		bonus := ""
		if captain.PassiveValue > 0 {
			bonus = fmt.Sprintf(" (+%.0f%% vitesse lvl80)", captain.PassiveValue*100)
		}
		return "Vent jamais défavorable" + bonus
	case "terror_engagement":
		drain := ""
		if captain.DrainPerMinute > 0 {
			drain = fmt.Sprintf(" +%.1f/min", captain.DrainPerMinute)
		}
		return fmt.Sprintf("Terreur: +%d moral (ouverture)%s", captain.PassiveIntValue, drain)
	case "absolute_morale_floor":
		return fmt.Sprintf("Moral absolu minimum: ≥%d", captain.PassiveIntValue)

	default:
		return fmt.Sprintf("Passif: %s", captain.PassiveID)
	}
}

// UpdateFleetUI handles input for the Fleet Management UI
func (g *Game) UpdateFleetUI() {
	if !g.ui.ShowFleetUI {
		return
	}

	// Load captains when opening the UI (refresh on each open to get latest state)
	// Only run this occasionally or if logic requires, but keeping existing pattern is fine
	if g.captains == nil {
		go func() {
			captains, err := g.api.GetCaptains()
			if err == nil {
				g.captains = captains
			}
		}()
	}

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.ui.ShowCaptainModal {
			g.ui.ShowCaptainModal = false
			g.ui.SelectedShipForCaptain = ""
			g.ui.CaptainModalScroll = 0
			return
		}
		if g.ui.ShowCrewModal {
			g.ui.ShowCrewModal = false
			g.ui.SelectedShipForCrew = ""
			g.ui.CrewModalError = ""
			return
		}
		g.ui.ShowFleetUI = false
		return
	}

	// Modals handle their own input in Draw calls (Immediate Mode style in this codebase)
	// We just return to prevent clicking through to the main UI
	if g.ui.ShowCaptainModal {
		// Handle scrolling for captain modal here as it uses ebiten.Wheel() which is an implicit event
		_, dy := ebiten.Wheel()
		if dy != 0 {
			g.ui.CaptainModalScroll -= dy * 20.0
			if g.ui.CaptainModalScroll < 0 {
				g.ui.CaptainModalScroll = 0
			}
			// Max scroll clamping happens in Draw
		}
		return
	}
	if g.ui.ShowCrewModal {
		return
	}

	// Layout Geometry (Must match DrawFleetUI)
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	winW, winH := 900.0, 700.0
	winX, winY := cx-winW/2, cy-winH/2
	contentY := winY + 60
	contentX := winX + 20
	contentW := winW - 40
	contentH := winH - 80

	// Metrics
	listW := contentW * 0.25
	detailsX := contentX + listW + 10
	detailsW := contentW - listW - 10

	// Inputs
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)
	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	_, dy := ebiten.Wheel()

	// 1. FLEET LIST INTERACTION (Left Column)
	if fmx >= contentX && fmx <= contentX+listW && fmy >= contentY && fmy <= contentY+contentH {
		// Scroll
		if dy != 0 {
			g.ui.FleetListScroll -= dy * 20.0
			if g.ui.FleetListScroll < 0 {
				g.ui.FleetListScroll = 0
			}
			// Max scroll clamping happens in Draw
		}

		// Selection
		if clicked {
			if g.player != nil && len(g.player.Islands) > 0 {
				fleets := g.player.Islands[0].Fleets
				itemHeight := 60.0
				itemGap := 5.0

				// Calculate index in list relative to scroll
				offsetY := fmy - contentY + g.ui.FleetListScroll
				idx := int(offsetY / (itemHeight + itemGap))

				if idx >= 0 && idx < len(fleets) {
					// Check if click was actually on the item (simple bounds check)
					itemTop := contentY + float64(idx)*(itemHeight+itemGap) - g.ui.FleetListScroll
					if fmy >= itemTop && fmy <= itemTop+itemHeight {
						g.ui.SelectedFleetID = fleets[idx].ID.String()
					}
				}
			}
		}
		return // Consumed input in list area
	}

	// 2. DETAILS INTERACTION (Right Column)
	if fmx >= detailsX && fmx <= detailsX+detailsW && fmy >= contentY && fmy <= contentY+contentH {
		if clicked {
			// Find Selected Fleet
			var selectedFleet *domain.Fleet
			if g.player != nil && len(g.player.Islands) > 0 {
				island := g.player.Islands[0]
				// Match logic in DrawFleetUI
				if g.ui.SelectedFleetID != "" {
					for _, f := range island.Fleets {
						if f.ID.String() == g.ui.SelectedFleetID {
							selectedFleet = &f
							break
						}
					}
				}
				if selectedFleet == nil && len(island.Fleets) > 0 {
					selectedFleet = &island.Fleets[0] // Fallback
				}
			}

			if selectedFleet != nil {
				// Re-verify active status and filtered ships
				island := g.player.Islands[0]
				isActive := island.ActiveFleetID != nil && *island.ActiveFleetID == selectedFleet.ID

				// Button A: ACTIVER
				// Position: startX + 200, startY + 15, 100x30
				btnX := detailsX + 200
				btnY := contentY + 15
				btnW, btnH := 100.0, 30.0

				if !isActive {
					if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
						go func(fid string) {
							uid, _ := uuid.Parse(fid)
							g.api.SetActiveFleet(uid)
							time.Sleep(100 * time.Millisecond)
							g.api.GetStatus()
						}(selectedFleet.ID.String())
						return
					}
				} else if selectedFleet.State == domain.FleetStateChasingPvP {
					// ABORT Button logic
					btnAbortX := detailsX + 400
					if fmx >= btnAbortX && fmx <= btnAbortX+btnW && fmy >= btnY && fmy <= btnY+btnH {
						go func(fid string) {
							g.api.AbortIntercept(fid)
							time.Sleep(100 * time.Millisecond)
							g.loadPlayerData()
						}(selectedFleet.ID.String())
						return
					}
				}

				// Ship List Actions
				headerH := 60.0
				shipsStartY := contentY + headerH + 10
				currentY := shipsStartY + 25
				lineH := 100.0

				// Re-filter active ships to match display
				activeShips := make([]domain.Ship, 0)
				for _, ship := range selectedFleet.Ships {
					if ship.State != "Destroyed" && ship.Health > 0 {
						activeShips = append(activeShips, ship)
					}
				}

				// Layout columns for button detection
				// Col 2: Crew - subCol2X = col1X + subCol1W + colGap
				// Col 4: Actions - subCol4X = subCol3X + subCol3W + colGap
				colGap := 10.0
				colW := detailsW - 20
				// Proportions from DrawManagementFleetDetails
				// subCol1W := colW * 0.25
				subCol2W := colW * 0.30
				subCol3W := colW * 0.25
				subCol4W := colW * 0.20

				col1X := detailsX + 10
				subCol2X := col1X + (colW * 0.25) + colGap
				// subCol3X := subCol2X + subCol2W + colGap
				subCol4X := (subCol2X + subCol2W + colGap) + subCol3W + colGap

				for _, ship := range activeShips {
					// Check vertical bounds of this row
					if fmy >= currentY && fmy <= currentY+lineH {

						// Button B: GÉRER (Crew)
						// Pos: subCol2X, currentY + 50, W=subCol2W-10, H=20
						btnCrewY := currentY + 50
						btnCrewW := subCol2W - 10
						btnCrewH := 20.0

						if fmx >= subCol2X && fmx <= subCol2X+btnCrewW && fmy >= btnCrewY && fmy <= btnCrewY+btnCrewH {
							g.ui.SelectedShipForCrew = ship.ID.String()
							g.ui.CrewModalWarriors = ship.MilitiaWarriors
							g.ui.CrewModalArchers = ship.MilitiaArchers
							g.ui.CrewModalGunners = ship.MilitiaGunners
							g.ui.ShowCrewModal = true
							return
						}

						// Buttons C: Captain Actions
						captain := g.GetCaptainForShip(ship.ID.String())
						btnCaptY := currentY + 5
						btnCaptW := subCol4W - 5
						btnCaptH := 20.0

						if captain != nil {
							// UPGRADE
							// Pos: subCol4X, btnCaptY
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY && fmy <= btnCaptY+btnCaptH {
								go func(cid uuid.UUID) {
									g.api.UpgradeCaptainStars(cid)
									g.api.GetCaptains()
								}(captain.ID)
								return
							}

							// RETIRER
							// Pos: subCol4X, btnCaptY + 25
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY+25 && fmy <= btnCaptY+25+btnCaptH {
								go func(cid string) {
									g.api.UnassignCaptain(cid)
									time.Sleep(100 * time.Millisecond)
									g.api.GetStatus()
									g.api.GetCaptains()
								}(captain.ID.String())
								return
							}
						} else {
							// ASSIGNER
							// Pos: subCol4X, btnCaptY
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY && fmy <= btnCaptY+btnCaptH {
								g.ui.SelectedShipForCaptain = ship.ID.String()
								g.ui.ShowCaptainModal = true
								return
							}
						}
					}

					currentY += lineH + 10
				}
			}
		}
	}

	// Close Button (Window X)
	closeX, closeY := winX+winW-35, winY+10
	if clicked && fmx >= closeX && fmx <= closeX+25 && fmy >= closeY && fmy <= closeY+25 {
		g.ui.ShowFleetUI = false
	}
}

// DrawFleetUI renders the Fleet Management window (Master-Detail)
func (g *Game) DrawFleetUI(screen *ebiten.Image) {
	if !g.ui.ShowFleetUI {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	winW, winH := 900.0, 700.0
	winX, winY := cx-winW/2, cy-winH/2

	// Semi-transparent overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 150}, true)

	// Main window
	draw9Slice(screen, g, winX, winY, winW, winH, 16)

	// Title
	titleY := winY + 15
	ebitenutil.DebugPrintAt(screen, "GESTION DES FLOTTES", int(cx)-80, int(titleY))

	// Close button (X)
	closeX, closeY := winX+winW-35, winY+10
	closeSize := 25.0
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeSize), float32(closeSize), color.RGBA{150, 0, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+8, int(closeY)+5)

	// Handle close button click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= closeX && float64(mx) <= closeX+closeSize && float64(my) >= closeY && float64(my) <= closeY+closeSize {
			g.ui.ShowFleetUI = false
			return
		}
	}

	// Content area
	contentY := winY + 60
	contentX := winX + 20
	contentW := winW - 40
	contentH := winH - 80 // Footer space

	if g.player == nil || len(g.player.Islands) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucune île disponible.", int(contentX), int(contentY))
		return
	}

	island := g.player.Islands[0]
	if len(island.Fleets) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucune flotte disponible.", int(contentX), int(contentY))
		return
	}

	// Layout: 25% Left (List), 75% Right (Details)
	listW := contentW * 0.25
	detailsX := contentX + listW + 10
	detailsW := contentW - listW - 10

	// Draw List
	g.DrawManagementFleetList(screen, contentX, contentY, listW, contentH, island.Fleets)

	// Draw Details
	// Find selected fleet
	var selectedFleet *domain.Fleet
	if g.ui.SelectedFleetID != "" {
		for _, f := range island.Fleets {
			if f.ID.String() == g.ui.SelectedFleetID {
				selectedFleet = &f
				break
			}
		}
	}
	// Default to first if none selected or not found
	if selectedFleet == nil && len(island.Fleets) > 0 {
		g.ui.SelectedFleetID = island.Fleets[0].ID.String()
		selectedFleet = &island.Fleets[0]
	}

	if selectedFleet != nil {
		g.DrawManagementFleetDetails(screen, detailsX, contentY, detailsW, contentH, *selectedFleet)
	}

	// Footer info
	footerY := winY + winH - 30
	ebitenutil.DebugPrintAt(screen, "[ESC] pour fermer", int(contentX), int(footerY))

	// Draw captain selection modal if open (overlay on top)
	if g.ui.ShowCaptainModal {
		g.DrawCaptainSelectionModal(screen)
	}

	// Draw crew assignment modal if open (overlay on top)
	if g.ui.ShowCrewModal {
		g.DrawCrewAssignmentModal(screen)
	}
}

// DrawManagementFleetDetails renders the right column details of the selected fleet
func (g *Game) DrawManagementFleetDetails(screen *ebiten.Image, startX, startY, w, h float64, fleet domain.Fleet) {
	// Header
	headerH := 60.0
	vector.DrawFilledRect(screen, float32(startX), float32(startY), float32(w), float32(headerH), color.RGBA{30, 40, 50, 255}, true)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("DÉTAILS: %s", fleet.Name), int(startX)+20, int(startY)+20)

	// Status / Activate button area
	// FIND CORRECT ISLAND for this fleet
	var island *domain.Island
	if g.player != nil {
		for i := range g.player.Islands {
			if g.player.Islands[i].ID == fleet.IslandID {
				island = &g.player.Islands[i]
				break
			}
		}
	}
	// Fallback to first if not found (shouldn't happen with valid data)
	if island == nil && g.player != nil && len(g.player.Islands) > 0 {
		island = &g.player.Islands[0]
	}

	if island == nil {
		ebitenutil.DebugPrintAt(screen, "Île non trouvée pour cette flotte.", int(startX)+20, int(startY)+20)
		return
	}
	isActive := false
	if island.ActiveFleetID != nil && *island.ActiveFleetID == fleet.ID {
		isActive = true
	}

	// Determine status (Unlocked/Locked) - Simplified logic based on fleet name for now
	// Reuse logic from original DrawFleetUI...
	statusText := "Débloquée"
	if fleet.Name == "Flotte 2" {
		hasTech := false
		for _, techID := range g.player.UnlockedTechs {
			if techID == "nav_fleet_1" {
				hasTech = true
				break
			}
		}
		if !hasTech {
			statusText = "Verrouillée"
		}
	} else if fleet.Name == "Flotte 3" {
		hasTech := false
		for _, techID := range g.player.UnlockedTechs {
			if techID == "nav_fleet_2" {
				hasTech = true
				break
			}
		}
		if !hasTech {
			statusText = "Verrouillée"
		}
	}

	if statusText == "Verrouillée" {
		ebitenutil.DebugPrintAt(screen, "Status: Verrouillée (Tech requise)", int(startX)+200, int(startY)+20)
		return // Stop drawing details if locked
	}

	if isActive {
		statusText := "✅ FLOTTE ACTIVE (PvE)"
		if fleet.State == domain.FleetStateChasingPvP {
			statusText = "🏹 EN POURSUITE (PvP)"
		}
		ebitenutil.DebugPrintAt(screen, statusText, int(startX)+200, int(startY)+20)

		if fleet.State == domain.FleetStateChasingPvP {
			// Abort Button
			btnX := startX + 400
			btnY := startY + 15
			btnW := 100.0
			btnH := 30.0
			vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), color.RGBA{150, 0, 0, 255}, true)
			ebitenutil.DebugPrintAt(screen, "ABANDON", int(btnX)+20, int(btnY)+8)
		}
	} else {
		// Activate Button
		btnX := startX + 200
		btnY := startY + 15
		btnW := 100.0
		btnH := 30.0
		vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), color.RGBA{0, 100, 200, 255}, true)
		ebitenutil.DebugPrintAt(screen, "ACTIVER", int(btnX)+20, int(btnY)+8)

		// Handle Activate Click
		// Note: Logic is usually in Update, but handling simple clicks here for UI context is common in immediate mode GUIs
		// though strict separation would put it in UpdateFleetUI.
		// For this refactor, we just render here. Click logic in UpdateFleetUI is safer to avoid multiple triggers.
	}

	// List of ships
	shipsStartY := startY + headerH + 10
	activeShips := make([]domain.Ship, 0)
	for _, ship := range fleet.Ships {
		if ship.State != "Destroyed" && ship.Health > 0 {
			activeShips = append(activeShips, ship)
		}
	}

	if len(activeShips) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun navire dans cette flotte.", int(startX)+20, int(shipsStartY))
		return
	}

	// Draw Ships (Scrollable if needed, but fleet size is small usually 5 max)
	// We can reuse the column layout logic
	colGap := 10.0
	col1X := startX + 10
	colW := w - 20
	// Sub-columns
	subCol1W := colW * 0.25 // Name
	subCol2W := colW * 0.30 // Crew
	subCol3W := colW * 0.25 // Captain
	subCol4W := colW * 0.20 // Actions

	subCol2X := col1X + subCol1W + colGap
	subCol3X := subCol2X + subCol2W + colGap
	subCol4X := subCol3X + subCol3W + colGap

	// Headers
	ebitenutil.DebugPrintAt(screen, "NAVIRE", int(col1X), int(shipsStartY))
	ebitenutil.DebugPrintAt(screen, "ÉQUIPAGE", int(subCol2X), int(shipsStartY))
	ebitenutil.DebugPrintAt(screen, "CAPITAINE", int(subCol3X), int(shipsStartY))
	ebitenutil.DebugPrintAt(screen, "ACTIONS", int(subCol4X), int(shipsStartY))

	currentY := shipsStartY + 25
	lineH := 100.0 // Taller cards for better detail

	for _, ship := range activeShips {
		// Card BG
		vector.StrokeRect(screen, float32(col1X), float32(currentY), float32(colW), float32(lineH), 1, color.RGBA{100, 100, 100, 255}, true)

		// 1. Ship Info
		ebitenutil.DebugPrintAt(screen, ship.Name, int(col1X)+5, int(currentY)+5)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("(%s)", ship.Type), int(col1X)+5, int(currentY)+25)
		if ship.MaxHealth > 0 {
			hpPct := (ship.Health / ship.MaxHealth) * 100
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("HP: %.0f/%.0f (%.0f%%)", ship.Health, ship.MaxHealth, hpPct), int(col1X)+5, int(currentY)+45)
		}

		// 2. Militia Info
		totalMilitia := ship.MilitiaWarriors + ship.MilitiaArchers + ship.MilitiaGunners
		maxMilitia := 50
		switch ship.Type {
		case "sloop":
			maxMilitia = 50
		case "brigantine":
			maxMilitia = 100
		case "frigate":
			maxMilitia = 150
		case "galleon":
			maxMilitia = 200
		case "manowar":
			maxMilitia = 250
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Milice: %d/%d", totalMilitia, maxMilitia), int(subCol2X), int(currentY)+5)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("G:%d A:%d Ar:%d", ship.MilitiaWarriors, ship.MilitiaArchers, ship.MilitiaGunners), int(subCol2X), int(currentY)+25)

		// Crew Button placeholder (Visual only, inputs handled in Update)
		btnCrewY := currentY + 50
		vector.DrawFilledRect(screen, float32(subCol2X), float32(btnCrewY), float32(subCol2W-10), 20, color.RGBA{0, 120, 200, 255}, true)
		ebitenutil.DebugPrintAt(screen, "GÉRER", int(subCol2X)+10, int(btnCrewY)+3)

		// 3. Captain Info
		captain := g.GetCaptainForShip(ship.ID.String())
		if captain != nil {
			stars := formatStars(captain.Stars, 5) // Simplified stars
			if captain.Rarity == "common" {
				stars = formatStars(captain.Stars, 3)
			}
			if captain.Rarity == "rare" {
				stars = formatStars(captain.Stars, 4)
			}

			ebitenutil.DebugPrintAt(screen, captain.Name, int(subCol3X), int(currentY)+5)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Lvl %d %s", captain.Level, stars), int(subCol3X), int(currentY)+25)
		} else {
			ebitenutil.DebugPrintAt(screen, "Aucun", int(subCol3X), int(currentY)+5)
		}

		// 4. Actions (Captain buttons)
		btnCaptY := currentY + 5
		if captain != nil {
			// Upgrade
			vector.DrawFilledRect(screen, float32(subCol4X), float32(btnCaptY), float32(subCol4W-5), 20, color.RGBA{200, 150, 0, 255}, true)
			ebitenutil.DebugPrintAt(screen, "UPGRADE", int(subCol4X)+5, int(btnCaptY)+3)

			// Unassign
			vector.DrawFilledRect(screen, float32(subCol4X), float32(btnCaptY+25), float32(subCol4W-5), 20, color.RGBA{150, 50, 50, 255}, true)
			ebitenutil.DebugPrintAt(screen, "RETIRER", int(subCol4X)+5, int(btnCaptY+28))
		} else {
			// Assign
			vector.DrawFilledRect(screen, float32(subCol4X), float32(btnCaptY), float32(subCol4W-5), 20, color.RGBA{0, 150, 0, 255}, true)
			ebitenutil.DebugPrintAt(screen, "ASSIGNER", int(subCol4X)+5, int(btnCaptY)+3)
		}

		currentY += lineH + 10
	}

	// CARGO SECTION
	currentY += 20
	// 1. Header & Summary
	// Capacity is now provided by Server (SSOT)
	capacity := fleet.CargoCapacity
	curLoad := fleet.CargoUsed

	// Fallback for visual safety if server sent 0 (e.g. during dev/failed refresh), though unlikely with proper SSOT
	// logic: if capacity is 0 but we have ships, something is wrong. But let's trust server for now as requested.

	remainingCap := fleet.CargoFree
	// Recalculate remaining locally to be perfectly in sync with displayed load if needed,
	// but usage of server fields is the goal.
	// Actually, let's just use the fields.

	if remainingCap < 0 {
		remainingCap = 0
	}

	ebitenutil.DebugPrintAt(screen, "CARGAISON", int(startX)+20, int(currentY))
	// Summary Line
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Capacité: %.0f/%.0f   Libre: %.0f", curLoad, capacity, remainingCap), int(startX)+150, int(currentY))
	currentY += 25

	// 2. Cargo List
	resourceNames := map[domain.ResourceType]string{
		domain.Wood:  "Bois",
		domain.Stone: "Pierre",
		domain.Gold:  "Or",
		domain.Rum:   "Rhum",
	}
	resOrder := []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum}

	hasCargo := false
	cargoLine := ""
	if fleet.Cargo != nil {
		for _, res := range resOrder {
			amt, ok := fleet.Cargo[res]
			if ok && amt > 0 {
				hasCargo = true
				name := resourceNames[res]
				if name == "" {
					name = string(res)
				}
				if cargoLine != "" {
					cargoLine += "  "
				}
				cargoLine += fmt.Sprintf("%s: %.0f", name, amt)
			}
		}
	}

	if !hasCargo {
		ebitenutil.DebugPrintAt(screen, "Cargaison vide", int(startX)+20, int(currentY))
	} else {
		ebitenutil.DebugPrintAt(screen, cargoLine, int(startX)+20, int(currentY))
	}
	currentY += 25

	// 3. Debug Log (Client-side console)
	if g.ui.LastLogFleetID != fleet.ID.String() {
		g.ui.LastLogFleetID = fleet.ID.String()

		rumAmt := 0.0
		keyCount := 0
		if fleet.Cargo != nil {
			keyCount = len(fleet.Cargo)
			if val, ok := fleet.Cargo[domain.Rum]; ok {
				rumAmt = val
			}
		}
		fmt.Printf("[FLEET_UI] open fleet=%s cargo_keys=%d rum=%.0f\n", fleet.ID, keyCount, rumAmt)
	}

	// TRANSFER SECTION COMPACT
	currentY += 15
	ebitenutil.DebugPrintAt(screen, "TRANSFERT (ÎLE <-> FLOTTE)", int(startX)+20, int(currentY))
	currentY += 20

	// Check eligibility
	canTransfer := fleet.State == domain.FleetStateIdle || fleet.State == domain.FleetStateStationed
	if !canTransfer {
		ebitenutil.DebugPrintAt(screen, "Transfert possible uniquement à quai (Idle/Stationné)", int(startX)+20, int(currentY))
		currentY += 20
	} else {
		// Layout Constants
		tfX := startX + 20
		tfW := w - 40

		// Line A - Resources Tabs
		// Single line, equal width
		resW := 70.0 // Adjusted for 4 items
		resH := 25.0
		resGap := 0.0 // Visual connector style

		if g.ui.TransferSelectedResource == "" {
			g.ui.TransferSelectedResource = "wood"
		}

		resourceLabels := map[domain.ResourceType]string{
			domain.Wood: "BOIS", domain.Stone: "PIERRE", domain.Gold: "OR", domain.Rum: "RHUM",
		}

		for i, r := range resOrder {
			rx := tfX + float64(i)*(resW+resGap)
			isSelected := string(r) == g.ui.TransferSelectedResource

			col := color.RGBA{40, 40, 40, 255}
			if isSelected {
				col = color.RGBA{0, 100, 200, 255}
			} else {
				// Hover
				cmx, cmy := ebiten.CursorPosition()
				if float64(cmx) >= rx && float64(cmx) <= rx+resW && float64(cmy) >= currentY && float64(cmy) <= currentY+resH {
					col = color.RGBA{60, 60, 60, 255}
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						g.ui.TransferSelectedResource = string(r)
						g.ui.TransferAmount = 0
						g.ui.TransferMessage = ""
					}
				}
			}

			vector.DrawFilledRect(screen, float32(rx), float32(currentY), float32(resW), float32(resH), col, true)
			vector.StrokeRect(screen, float32(rx), float32(currentY), float32(resW), float32(resH), 1, color.RGBA{100, 100, 100, 255}, true)

			// Center Text
			label := resourceLabels[r]
			// Simple centering approximation: 6px per char
			textX := int(rx) + int(resW/2) - (len(label) * 3) // approx
			if textX < int(rx)+5 {
				textX = int(rx) + 5
			}
			ebitenutil.DebugPrintAt(screen, label, textX, int(currentY)+5)
		}
		currentY += resH + 10

		// Line B - Info + Quantity
		selRes := domain.ResourceType(g.ui.TransferSelectedResource)
		islandStock := island.Resources[selRes]
		fleetStock := 0.0
		if fleet.Cargo != nil {
			fleetStock = fleet.Cargo[selRes]
		}

		// Left Info
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Île: %.0f   Flotte: %.0f   Libre: %.0f", islandStock, fleetStock, remainingCap), int(tfX), int(currentY)+5)

		// Right Controls
		// Input Box
		// Input Box
		inputW := 80.0
		inputX := tfX + tfW - inputW - 130 // Make space for buttons

		// Input Interaction
		cmx, cmy := ebiten.CursorPosition()
		inputHover := float64(cmx) >= inputX && float64(cmx) <= inputX+inputW && float64(cmy) >= currentY && float64(cmy) <= currentY+25

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if inputHover {
				g.ui.TransferInputActive = true
				g.ui.TransferInputString = fmt.Sprintf("%.0f", g.ui.TransferAmount)
				if g.ui.TransferAmount == 0 {
					g.ui.TransferInputString = ""
				}
			} else {
				g.ui.TransferInputActive = false
				// Parse on blur
				if val, err := strconv.ParseFloat(g.ui.TransferInputString, 64); err == nil {
					g.ui.TransferAmount = val
				} else if g.ui.TransferInputString == "" {
					g.ui.TransferAmount = 0
				}
			}
		}

		// Keyboard Input
		if g.ui.TransferInputActive {
			runes := ebiten.AppendInputChars(nil)
			if len(runes) > 0 {
				g.ui.TransferInputString += string(runes)
				// REACTIVE PARSE: Update Amount as they type to enable/disable buttons immediately
				if val, err := strconv.ParseFloat(g.ui.TransferInputString, 64); err == nil {
					g.ui.TransferAmount = val
				}
			}

			// Backspace
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
				if len(g.ui.TransferInputString) > 0 {
					g.ui.TransferInputString = g.ui.TransferInputString[:len(g.ui.TransferInputString)-1]
					// REACTIVE PARSE: Update Amount
					if val, err := strconv.ParseFloat(g.ui.TransferInputString, 64); err == nil {
						g.ui.TransferAmount = val
					} else if g.ui.TransferInputString == "" {
						g.ui.TransferAmount = 0
					}
				}
			}
			// Enter (Commit)
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
				g.ui.TransferInputActive = false
				if val, err := strconv.ParseFloat(g.ui.TransferInputString, 64); err == nil {
					g.ui.TransferAmount = val
				}
			}
		}

		// Draw logic
		borderCol := color.RGBA{100, 100, 100, 255}
		if g.ui.TransferInputActive {
			borderCol = color.RGBA{0, 200, 255, 255}
		}

		vector.DrawFilledRect(screen, float32(inputX), float32(currentY), float32(inputW), 25, color.RGBA{10, 10, 10, 255}, true)
		vector.StrokeRect(screen, float32(inputX), float32(currentY), float32(inputW), 25, 1, borderCol, true)

		displayText := fmt.Sprintf("%.0f", g.ui.TransferAmount)
		if g.ui.TransferInputActive {
			displayText = g.ui.TransferInputString
			// Blink cursor
			if (time.Now().UnixMilli()/500)%2 == 0 {
				displayText += "|"
			}
		}

		ebitenutil.DebugPrintAt(screen, displayText, int(inputX)+5, int(currentY)+5)

		// Quick Buttons (Pills)
		btns := []struct {
			label string
			val   float64
		}{
			{"+100", 100}, {"+1k", 1000}, {"MAX", -1}, {"X", 0},
		}
		bx := inputX + inputW + 5
		for _, b := range btns {
			bw := 30.0
			if b.label == "MAX" {
				bw = 35.0
			}

			cmx, cmy = ebiten.CursorPosition()
			hover := float64(cmx) >= bx && float64(cmx) <= bx+bw && float64(cmy) >= currentY && float64(cmy) <= currentY+25
			col := color.RGBA{60, 60, 60, 255}
			if hover {
				col = color.RGBA{90, 90, 90, 255}
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					switch b.label {
					case "MAX":
						// Smart MAX: Load preference (fill cargo)
						maxLoad := islandStock
						if remainingCap < maxLoad {
							maxLoad = remainingCap
						}
						// If user has 0 stock on island but cargo has stock, presume unload intent for MAX?
						if maxLoad == 0 && fleetStock > 0 {
							// If we can't load anything but have cargo, default to unloading everything
							g.ui.TransferAmount = fleetStock
						} else {
							g.ui.TransferAmount = maxLoad
						}
					case "X":
						g.ui.TransferAmount = 0
					default:
						g.ui.TransferAmount += b.val
					}
					// SYNC Input string so blur doesn't reset amount
					g.ui.TransferInputString = fmt.Sprintf("%.0f", g.ui.TransferAmount)
				}
			}

			vector.DrawFilledRect(screen, float32(bx), float32(currentY), float32(bw), 25, col, true)
			vector.StrokeRect(screen, float32(bx), float32(currentY), float32(bw), 25, 1, color.RGBA{100, 100, 100, 255}, true)

			// Center Label
			lX := int(bx) + 2
			if len(b.label) > 3 {
				lX = int(bx) + 1
			}
			ebitenutil.DebugPrintAt(screen, b.label, lX, int(currentY)+5)
			bx += bw + 3
		}
		currentY += 35

		// Line C - Actions
		actW := 140.0
		actH := 30.0
		// Center buttons
		centerX := tfX + (tfW / 2)
		loadX := centerX - actW - 10
		unloadX := centerX + 10

		// Validation
		canLoad := g.ui.TransferAmount > 0 && g.ui.TransferAmount <= islandStock && g.ui.TransferAmount <= remainingCap
		canUnload := g.ui.TransferAmount > 0 && g.ui.TransferAmount <= fleetStock

		// Load Button
		colLoad := color.RGBA{0, 100, 0, 255}
		if !canLoad {
			colLoad = color.RGBA{40, 40, 40, 255}
		}
		vector.DrawFilledRect(screen, float32(loadX), float32(currentY), float32(actW), float32(actH), colLoad, true)
		ebitenutil.DebugPrintAt(screen, "CHARGER ->", int(loadX)+35, int(currentY)+8)

		cmx, cmy = ebiten.CursorPosition()
		if canLoad && float64(cmx) >= loadX && float64(cmx) <= loadX+actW && float64(cmy) >= currentY && float64(cmy) <= currentY+actH {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fid := fleet.ID // Capture locally for closure
				iid := island.ID
				amount := g.ui.TransferAmount
				go func() {
					// Update API context for multi-island safety
					g.api.IslandID = iid
					resp, err := g.api.TransferToFleet(fid, selRes, amount)
					if err != nil {
						g.ui.TransferMessage = fmt.Sprintf("Erreur: %v", err)
					} else {
						g.ui.TransferMessage = resp.Message
						// Update master state
						for i := range g.player.Islands {
							if g.player.Islands[i].ID == iid {
								g.player.Islands[i].Resources = resp.IslandResources
								for idx, f := range g.player.Islands[i].Fleets {
									if f.ID == fid {
										g.player.Islands[i].Fleets[idx].Cargo = resp.FleetCargo
										g.player.Islands[i].Fleets[idx].CargoCapacity = resp.CargoCapacity
										g.player.Islands[i].Fleets[idx].CargoUsed = resp.CargoUsed
										g.player.Islands[i].Fleets[idx].CargoFree = resp.CargoFree
										break
									}
								}
								break
							}
						}
					}
				}()
			}
		}

		// Unload Button
		colUnload := color.RGBA{0, 100, 0, 255}
		if !canUnload {
			colUnload = color.RGBA{40, 40, 40, 255}
		}
		vector.DrawFilledRect(screen, float32(unloadX), float32(currentY), float32(actW), float32(actH), colUnload, true)
		ebitenutil.DebugPrintAt(screen, "<- DÉCHARGER", int(unloadX)+30, int(currentY)+8) // text centered approx

		if canUnload && float64(cmx) >= unloadX && float64(cmx) <= unloadX+actW && float64(cmy) >= currentY && float64(cmy) <= currentY+actH {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fid := fleet.ID
				iid := island.ID
				amount := g.ui.TransferAmount
				go func() {
					g.api.IslandID = iid
					resp, err := g.api.TransferToIsland(fid, selRes, amount)
					if err != nil {
						g.ui.TransferMessage = fmt.Sprintf("Erreur: %v", err)
					} else {
						g.ui.TransferMessage = resp.Message
						for i := range g.player.Islands {
							if g.player.Islands[i].ID == iid {
								g.player.Islands[i].Resources = resp.IslandResources
								for idx, f := range g.player.Islands[i].Fleets {
									if f.ID == fid {
										g.player.Islands[i].Fleets[idx].Cargo = resp.FleetCargo
										g.player.Islands[i].Fleets[idx].CargoCapacity = resp.CargoCapacity
										g.player.Islands[i].Fleets[idx].CargoUsed = resp.CargoUsed
										g.player.Islands[i].Fleets[idx].CargoFree = resp.CargoFree
										break
									}
								}
								break
							}
						}
					}
				}()
			}
		}

		// Feedback message
		if g.ui.TransferMessage != "" {
			ebitenutil.DebugPrintAt(screen, g.ui.TransferMessage, int(startX)+20, int(currentY)+35)
		}
	}
}

// DrawManagementFleetList renders the left column list of fleets
func (g *Game) DrawManagementFleetList(screen *ebiten.Image, startX, startY, w, h float64, fleets []domain.Fleet) {
	// List background
	vector.DrawFilledRect(screen, float32(startX), float32(startY), float32(w), float32(h), color.RGBA{20, 30, 40, 255}, true)
	vector.StrokeRect(screen, float32(startX), float32(startY), float32(w), float32(h), 1, color.RGBA{100, 100, 100, 255}, true)

	itemHeight := 60.0
	itemGap := 5.0
	listContentHeight := float64(len(fleets)) * (itemHeight + itemGap)
	maxScroll := listContentHeight - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	// Clamp scroll
	if g.ui.FleetListScroll > maxScroll {
		g.ui.FleetListScroll = maxScroll
	}
	if g.ui.FleetListScroll < 0 {
		g.ui.FleetListScroll = 0
	}

	visibleCount := int(h/(itemHeight+itemGap)) + 2
	scrollIdx := int(g.ui.FleetListScroll / (itemHeight + itemGap))

	for i := 0; i < visibleCount; i++ {
		idx := scrollIdx + i
		if idx >= len(fleets) {
			break
		}

		fleet := fleets[idx]
		itemY := startY + float64(idx)*(itemHeight+itemGap) - g.ui.FleetListScroll

		// Clip to visible area (simple check)
		if itemY+itemHeight < startY || itemY > startY+h {
			continue
		}

		// Item Background
		bgColor := color.RGBA{40, 50, 60, 255}
		borderColor := color.RGBA{80, 80, 80, 255}

		isActive := false
		if g.player != nil && len(g.player.Islands) > 0 {
			island := g.player.Islands[0]
			if island.ActiveFleetID != nil && *island.ActiveFleetID == fleet.ID {
				isActive = true
			}
		}

		if g.ui.SelectedFleetID == fleet.ID.String() {
			bgColor = color.RGBA{60, 70, 90, 255}
			borderColor = color.RGBA{200, 150, 50, 255}
		}

		vector.DrawFilledRect(screen, float32(startX), float32(itemY), float32(w), float32(itemHeight), bgColor, true)
		vector.StrokeRect(screen, float32(startX), float32(itemY), float32(w), float32(itemHeight), 1, borderColor, true)

		// Name
		ebitenutil.DebugPrintAt(screen, fleet.Name, int(startX)+10, int(itemY)+10)

		// Active Icon
		if isActive {
			ebitenutil.DebugPrintAt(screen, "✅ ACTIVE", int(startX)+10, int(itemY)+35)
		} else {
			// Count ships
			activeShips := 0
			for _, s := range fleet.Ships {
				if s.State != "Destroyed" && s.Health > 0 {
					activeShips++
				}
			}
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d navires", activeShips), int(startX)+10, int(itemY)+35)
		}
	}
}

// DrawCaptainSelectionModal draws a modal to select a captain for a ship
func (g *Game) DrawCaptainSelectionModal(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	modalW, modalH := 500.0, 400.0
	modalX, modalY := cx-modalW/2, cy-modalH/2

	// Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, true)

	// Modal background
	vector.DrawFilledRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), color.RGBA{40, 30, 20, 255}, true)
	vector.StrokeRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), 2, color.RGBA{200, 180, 100, 255}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "SELECTIONNER UN CAPITAINE", int(modalX)+20, int(modalY)+20)

	// List available captains (not assigned to any ship)
	contentY := modalY + 50
	contentX := modalX + 20
	availableCaptains := []client.Captain{}
	for i := range g.captains {
		if g.captains[i].AssignedShipID == nil {
			availableCaptains = append(availableCaptains, g.captains[i])
		}
	}

	// Scrollable area dimensions
	listStartY := contentY + 40
	listEndY := modalY + modalH - 50 // Leave space for close button
	listHeight := listEndY - listStartY
	captainCardHeight := 50.0
	captainCardSpacing := 5.0
	totalItemHeight := captainCardHeight + captainCardSpacing

	if len(availableCaptains) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun capitaine disponible", int(contentX), int(contentY))
		ebitenutil.DebugPrintAt(screen, "Tous les capitaines sont assignés", int(contentX), int(contentY)+20)
	} else {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Capitaines disponibles (%d):", len(availableCaptains)), int(contentX), int(contentY))

		// Calculate scroll bounds
		totalHeight := float64(len(availableCaptains)) * totalItemHeight
		maxScroll := totalHeight - listHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if g.ui.CaptainModalScroll > maxScroll {
			g.ui.CaptainModalScroll = maxScroll
		}
		if g.ui.CaptainModalScroll < 0 {
			g.ui.CaptainModalScroll = 0
		}

		// Calculate visible range
		startIdx := int(g.ui.CaptainModalScroll / totalItemHeight)
		visibleCount := int(listHeight/totalItemHeight) + 2 // +2 for smooth scrolling
		if visibleCount > len(availableCaptains) {
			visibleCount = len(availableCaptains)
		}
		endIdx := startIdx + visibleCount
		if endIdx > len(availableCaptains) {
			endIdx = len(availableCaptains)
		}

		// Draw visible captains
		for i := startIdx; i < endIdx; i++ {
			capt := availableCaptains[i]
			captY := listStartY + float64(i)*totalItemHeight - g.ui.CaptainModalScroll

			// Skip if outside visible area
			if captY+captainCardHeight < listStartY || captY > listEndY {
				continue
			}

			// Captain card
			cardH := captainCardHeight
			vector.DrawFilledRect(screen, float32(contentX), float32(captY), float32(modalW-40), float32(cardH), color.RGBA{60, 50, 40, 255}, true)
			vector.StrokeRect(screen, float32(contentX), float32(captY), float32(modalW-40), float32(cardH), 1, color.RGBA{150, 150, 150, 255}, true)

			// Captain info
			maxStars := 3
			switch capt.Rarity {
			case "rare":
				maxStars = 4
			case "legendary":
				maxStars = 5
			}
			starsText := ""
			for i := 0; i < maxStars; i++ {
				if i < capt.Stars {
					starsText += "★"
				} else {
					starsText += "☆"
				}
			}
			ebitenutil.DebugPrintAt(screen, capt.Name, int(contentX)+10, int(captY)+5)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Niveau %d | %s | %s", capt.Level, capt.Rarity, starsText), int(contentX)+10, int(captY)+20)

			// Buttons: Select and Upgrade
			btnX := modalX + modalW - 200
			btnY := captY + 10
			btnW := 70.0
			btnH := 25.0
			btnSpacing := 10.0

			// Upgrade button (if not max stars; Stars == MaxStars means fully upgraded)
			canUpgrade := capt.Stars < maxStars // Disable when Stars == MaxStars

			if canUpgrade {
				upgradeBtnX := btnX
				upgradeBtnCol := color.RGBA{200, 150, 0, 255}
				vector.DrawFilledRect(screen, float32(upgradeBtnX), float32(btnY), float32(btnW), float32(btnH), upgradeBtnCol, true)
				ebitenutil.DebugPrintAt(screen, "UPGRADE ★", int(upgradeBtnX)+5, int(btnY)+7)

				// Handle upgrade click
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					mx, my := ebiten.CursorPosition()
					if float64(mx) >= upgradeBtnX && float64(mx) <= upgradeBtnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
						go func() {
							err := g.api.UpgradeCaptainStars(capt.ID)
							if err == nil {
								// Refresh captains
								captains, _ := g.api.GetCaptains()
								g.captains = captains
								g.Log("Capitaine amélioré: %s", capt.Name)
							} else {
								g.Log("Erreur amélioration capitaine: %v", err)
							}
						}()
					}
				}
			}

			// Select button
			selectBtnX := btnX
			if canUpgrade {
				selectBtnX = btnX + btnW + btnSpacing
			}
			btnCol := color.RGBA{0, 150, 0, 255}
			vector.DrawFilledRect(screen, float32(selectBtnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
			ebitenutil.DebugPrintAt(screen, "SELECT", int(selectBtnX)+15, int(btnY)+7)

			// Handle select click
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				mx, my := ebiten.CursorPosition()
				if float64(mx) >= selectBtnX && float64(mx) <= selectBtnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
					go func(captID, shipID string) {
						err := g.api.AssignCaptain(captID, shipID)
						if err == nil {
							// Refresh captains and status
							captains, _ := g.api.GetCaptains()
							g.captains = captains
							g.ui.ShowCaptainModal = false
							g.ui.SelectedShipForCaptain = ""
							g.ui.CaptainModalScroll = 0 // Reset scroll when closing
							time.Sleep(100 * time.Millisecond)
							g.api.GetStatus()
						} else {
							g.Log("Erreur assignation capitaine: %v", err)
						}
					}(capt.ID.String(), g.ui.SelectedShipForCaptain)
				}
			}
		}
	}

	// Close button
	closeX := modalX + modalW - 30
	closeY := modalY + 10
	closeSize := 20.0
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeSize), float32(closeSize), color.RGBA{150, 0, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+6, int(closeY)+3)

	// Handle close button click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= closeX && float64(mx) <= closeX+closeSize && float64(my) >= closeY && float64(my) <= closeY+closeSize {
			g.ui.ShowCaptainModal = false
			g.ui.SelectedShipForCaptain = ""
			g.ui.CaptainModalScroll = 0 // Reset scroll when closing
		}
	}
}
