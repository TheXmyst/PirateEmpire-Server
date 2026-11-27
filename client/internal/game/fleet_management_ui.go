package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
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

// Géométrie de la fenêtre Flottes (partagée par Draw et Update).
type fleetLayout struct {
	winX, winY, winW, winH    float64
	contentX, contentY        float64
	contentW, contentH        float64
	listW, detailsX, detailsW float64
	closeCX, closeCY, closeR  float64
	footBtnX, footBtnY        float64
	footBtnW, footBtnH        float64
	detailsHeaderH            float64
}

func (g *Game) fleetLayout() fleetLayout {
	var l fleetLayout
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	l.winW, l.winH = 900, 700
	l.winX, l.winY = w/2-l.winW/2, h/2-l.winH/2
	l.contentY = l.winY + 60
	l.contentX = l.winX + 20
	l.contentW = l.winW - 40
	l.contentH = l.winH - 60 - 72 // place pour le bouton Fermer en bas
	l.listW = l.contentW * 0.25
	l.detailsX = l.contentX + l.listW + 10
	l.detailsW = l.contentW - l.listW - 10
	l.closeR = 22
	l.closeCX, l.closeCY = l.winX+l.winW-40, l.winY+34
	l.footBtnW, l.footBtnH = 150, 46
	l.footBtnX = l.winX + l.winW - 20 - l.footBtnW
	l.footBtnY = l.winY + l.winH - 12 - l.footBtnH
	l.detailsHeaderH = 60
	return l
}

func (g *Game) closeFleetUI() {
	g.ui.ShowFleetUI = false
	g.ui.FleetMessage = ""
}

// fleetAction lance un appel au serveur depuis le menu flotte, empêche un
// double déclenchement, recharge l'état et affiche l'erreur s'il y en a une.
func (g *Game) fleetAction(okMsg string, call func() error) {
	if g.ui.FleetBusy {
		return
	}
	g.ui.FleetBusy = true
	g.ui.FleetMessage = ""
	go func() {
		defer func() { g.ui.FleetBusy = false }()
		err := call()
		if captains, cerr := g.api.GetCaptains(); cerr == nil {
			g.captains = captains
		}
		g.refreshStatusAfterDevAction()
		if err != nil {
			g.ui.FleetMessage = "Erreur : " + friendlyError(err)
			g.ui.FleetMessageErr = true
			return
		}
		g.ui.FleetMessage = okMsg
		g.ui.FleetMessageErr = false
	}()
}

func (g *Game) UpdateFleetUI() {
	if !g.ui.ShowFleetUI {
		return
	}

	// Load captains when opening the UI (refresh on each open to get latest state)
	if g.captains == nil && !g.ui.FleetCaptainsLoading {
		g.ui.FleetCaptainsLoading = true
		go func() {
			defer func() { g.ui.FleetCaptainsLoading = false }()
			captains, err := g.api.GetCaptains()
			if err == nil {
				g.captains = captains
			}
		}()
	}

	// Échap ferme seulement la fenêtre du dessus.
	if g.ui.ShowCrewModal {
		// La fenêtre Équipage gère ses propres entrées (UpdateCrewAssignmentModal).
		return
	}
	if consumeEscape() {
		if g.ui.ShowCaptainModal {
			g.ui.ShowCaptainModal = false
			g.ui.SelectedShipForCaptain = ""
			g.ui.CaptainModalScroll = 0
			return
		}
		g.closeFleetUI()
		return
	}

	// Modals handle their own input in Draw calls (Immediate Mode style in this codebase)
	// We just return to prevent clicking through to the main UI
	if g.ui.ShowCaptainModal {
		// Handle scrolling for captain modal here as it uses wheelDelta() which is an implicit event
		_, dy := wheelDelta()
		if dy != 0 {
			g.ui.CaptainModalScroll -= dy * 20.0
			if g.ui.CaptainModalScroll < 0 {
				g.ui.CaptainModalScroll = 0
			}
			// Max scroll clamping happens in Draw
		}
		return
	}

	l := g.fleetLayout()
	contentX, contentY, contentH := l.contentX, l.contentY, l.contentH
	listW, detailsX, detailsW := l.listW, l.detailsX, l.detailsW

	// Inputs
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)
	clicked := isMouseJustPressed(ebiten.MouseButtonLeft)
	_, dy := wheelDelta()

	// Fermer : grosse croix, bouton « Fermer » en bas, ou clic en dehors.
	if clicked {
		onClose := (fmx-l.closeCX)*(fmx-l.closeCX)+(fmy-l.closeCY)*(fmy-l.closeCY) <= (l.closeR+6)*(l.closeR+6)
		onFoot := fmx >= l.footBtnX && fmx <= l.footBtnX+l.footBtnW && fmy >= l.footBtnY && fmy <= l.footBtnY+l.footBtnH
		outside := fmx < l.winX || fmx > l.winX+l.winW || fmy < l.winY || fmy > l.winY+l.winH
		if onClose || onFoot || outside {
			g.closeFleetUI()
			return
		}
	}

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
						if g.ui.SelectedFleetID != fleets[idx].ID.String() {
							g.ui.FleetDetailsScroll = 0
							g.ui.FleetMessage = ""
						}
						g.ui.SelectedFleetID = fleets[idx].ID.String()
					}
				}
			}
		}
		return // Consumed input in list area
	}

	// 2. DETAILS INTERACTION (Right Column)
	if fmx >= detailsX && fmx <= detailsX+detailsW && fmy >= contentY && fmy <= contentY+contentH {
		// Défilement de la liste des navires + cargaison (la fiche peut être plus haute que la fenêtre)
		if dy != 0 {
			g.ui.FleetDetailsScroll -= dy * 30.0
			if g.ui.FleetDetailsScroll < 0 {
				g.ui.FleetDetailsScroll = 0
			}
			// Max scroll clamping happens in Draw
		}
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
						fid := selectedFleet.ID
						g.fleetAction("Flotte activée", func() error { return g.api.SetActiveFleet(fid) })
						return
					}
				} else if selectedFleet.State == domain.FleetStateChasingPvP {
					// ABORT Button logic
					btnAbortX := detailsX + 400
					if fmx >= btnAbortX && fmx <= btnAbortX+btnW && fmy >= btnY && fmy <= btnY+btnH {
						fid := selectedFleet.ID.String()
						g.fleetAction("Poursuite abandonnée", func() error { return g.api.AbortIntercept(fid) })
						return
					}
				}

				// Ship List Actions (zone qui défile, sous l'en-tête)
				headerH := l.detailsHeaderH
				if fmy < contentY+headerH {
					return
				}
				shipsStartY := contentY + headerH + 10 - g.ui.FleetDetailsScroll
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
				colW := detailsW - 20 - 3*colGap
				// Proportions from DrawManagementFleetDetails
				subCol2W := colW * 0.30
				subCol3W := colW * 0.25
				subCol4W := colW * 0.20

				col1X := detailsX + 10
				subCol2X := col1X + (colW * 0.25) + colGap
				subCol4X := (subCol2X + subCol2W + colGap) + subCol3W + colGap

				fleetDocked := shipDocked(selectedFleet)
				for _, ship := range activeShips {
					// Check vertical bounds of this row
					if fmy >= currentY && fmy <= currentY+lineH {

						// Button B: GÉRER (Crew)
						btnCrewY := currentY + 50
						btnCrewW := subCol2W - 10
						btnCrewH := 34.0

						if fmx >= subCol2X && fmx <= subCol2X+btnCrewW && fmy >= btnCrewY && fmy <= btnCrewY+btnCrewH {
							if !fleetDocked {
								g.fleetAtSeaMessage()
								return
							}
							g.ui.SelectedShipForCrew = ship.ID.String()
							g.ui.CrewModalWarriors = ship.MilitiaWarriors
							g.ui.CrewModalArchers = ship.MilitiaArchers
							g.ui.CrewModalGunners = ship.MilitiaGunners
							g.ui.CrewModalError = ""
							g.ui.ShowCrewModal = true
							return
						}

						// Buttons C: Captain Actions
						captain := g.GetCaptainForShip(ship.ID.String())
						btnCaptY := currentY + 5
						btnCaptW := subCol4W - 5
						btnCaptH := 30.0

						if captain != nil {
							// UPGRADE
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY && fmy <= btnCaptY+btnCaptH {
								cid := captain.ID
								g.fleetAction("Capitaine amélioré", func() error { return g.api.UpgradeCaptainStars(cid) })
								return
							}

							// RETIRER
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY+36 && fmy <= btnCaptY+36+btnCaptH {
								if !fleetDocked {
									g.fleetAtSeaMessage()
									return
								}
								cid := captain.ID.String()
								g.fleetAction("Capitaine retiré", func() error { return g.api.UnassignCaptain(cid) })
								return
							}
						} else {
							// ASSIGNER
							if fmx >= subCol4X && fmx <= subCol4X+btnCaptW && fmy >= btnCaptY && fmy <= btnCaptY+btnCaptH {
								if !fleetDocked {
									g.fleetAtSeaMessage()
									return
								}
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
}

// DrawFleetUI renders the Fleet Management window (Master-Detail)
func (g *Game) DrawFleetUI(screen *ebiten.Image) {
	if !g.ui.ShowFleetUI {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	l := g.fleetLayout()
	cx := w / 2
	winX, winY, winW, winH := l.winX, l.winY, l.winW, l.winH

	// Semi-transparent overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 150}, vecAA)

	// Main window
	draw9Slice(screen, g, winX, winY, winW, winH, 16)

	// Title
	titleY := winY + 15
	uiTitleCentered(screen, "Gestion des flottes", int(cx), int(titleY))

	// Grosse croix (le clic est géré dans UpdateFleetUI)
	drawCloseButton(screen, l.closeCX, l.closeCY, l.closeR)

	// Bouton Fermer en bas + message des actions
	drawButton(screen, l.footBtnX, l.footBtnY, l.footBtnW, l.footBtnH, "Fermer", btnSecondary)
	msgY := l.footBtnY + 14
	switch {
	case g.ui.FleetBusy:
		drawText(screen, "Envoi en cours…", bodyFace(14, false), l.contentX, msgY, themeMuted, alignLeft)
	case g.ui.FleetMessage != "":
		clr := color.Color(themeGreen)
		if g.ui.FleetMessageErr {
			clr = themeOrange
		}
		drawText(screen, ellipsize(g.ui.FleetMessage, bodyFace(14, false), l.footBtnX-l.contentX-20), bodyFace(14, false), l.contentX, msgY, clr, alignLeft)
	}

	// Content area
	contentY := l.contentY
	contentX := l.contentX
	contentH := l.contentH

	if g.player == nil || len(g.player.Islands) == 0 {
		uiPrint(screen, "Aucune île disponible.", int(contentX), int(contentY))
		return
	}

	island := g.player.Islands[0]
	if len(island.Fleets) == 0 {
		uiPrint(screen, "Aucune flotte disponible.", int(contentX), int(contentY))
		return
	}

	// Layout: 25% Left (List), 75% Right (Details)
	listW := l.listW
	detailsX := l.detailsX
	detailsW := l.detailsW

	// Quand une fenêtre est ouverte par-dessus, les boutons de la fiche
	// (cargaison...) ne doivent pas recevoir le clic qui la traverse.
	savedClick := pointer.clickForDraw
	if g.ui.ShowCrewModal || g.ui.ShowCaptainModal {
		pointer.clickForDraw = false
	}

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
	pointer.clickForDraw = savedClick

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
	uiFillRect(screen, float32(startX), float32(startY), float32(w), float32(headerH), color.RGBA{30, 40, 50, 255}, vecAA)
	uiSubtitle(screen, fleet.Name, int(startX)+20, int(startY)+20)

	docked := shipDocked(&fleet)

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
		uiPrint(screen, "Île non trouvée pour cette flotte.", int(startX)+20, int(startY)+20)
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
		uiPrint(screen, "Status: Verrouillée (Tech requise)", int(startX)+200, int(startY)+20)
		return // Stop drawing details if locked
	}

	if isActive {
		statusText := "✅ FLOTTE ACTIVE (PvE)"
		if !docked {
			statusText = "EN MER"
		}
		if fleet.State == domain.FleetStateChasingPvP {
			statusText = "🏹 EN POURSUITE (PvP)"
		}
		uiPrint(screen, statusText, int(startX)+200, int(startY)+20)

		if fleet.State == domain.FleetStateChasingPvP {
			// Abort Button
			btnX := startX + 400
			btnY := startY + 15
			btnW := 100.0
			btnH := 30.0
			drawButton(screen, float64(btnX), float64(btnY), float64(btnW), float64(btnH), "Abandon", kindFromColor(color.RGBA{150, 0, 0, 255}))
		}
	} else {
		// Activate Button
		btnX := startX + 200
		btnY := startY + 15
		btnW := 100.0
		btnH := 30.0
		drawButton(screen, float64(btnX), float64(btnY), float64(btnW), float64(btnH), "Activer", kindFromColor(color.RGBA{0, 100, 200, 255}))

		// Handle Activate Click
		// Note: Logic is usually in Update, but handling simple clicks here for UI context is common in immediate mode GUIs
		// though strict separation would put it in UpdateFleetUI.
		// For this refactor, we just render here. Click logic in UpdateFleetUI is safer to avoid multiple triggers.
	}

	// Flotte partie : on le dit, et les boutons qui la modifient sont grisés.
	if !docked {
		drawText(screen, "Ramène-la au port pour la modifier", bodyFace(14, true), startX+w-16, startY+22, themeOrange, alignRight)
	}

	// List of ships : cette partie défile sous l'en-tête (molette ou doigt)
	bodyTop := startY + headerH
	visibleH := h - headerH
	maxScroll := math.Max(0, g.ui.FleetDetailsContentH-visibleH)
	if g.ui.FleetDetailsScroll > maxScroll {
		g.ui.FleetDetailsScroll = maxScroll
	}
	scroll := g.ui.FleetDetailsScroll
	screen = screen.SubImage(image.Rect(int(startX), int(bodyTop), int(startX+w), int(startY+h))).(*ebiten.Image)
	if _, py := cursorPosition(); float64(py) < bodyTop || float64(py) > startY+h {
		// Un bouton qui a défilé hors de la zone visible ne doit pas être cliquable.
		pointer.clickForDraw = false
	}
	shipsStartY := startY + headerH + 10 - scroll
	contentEnd := shipsStartY + 40
	defer func() { g.ui.FleetDetailsContentH = contentEnd + scroll - bodyTop + 20 }()
	activeShips := make([]domain.Ship, 0)
	for _, ship := range fleet.Ships {
		if ship.State != "Destroyed" && ship.Health > 0 {
			activeShips = append(activeShips, ship)
		}
	}

	if len(activeShips) == 0 {
		uiPrint(screen, "Aucun navire dans cette flotte.", int(startX)+20, int(shipsStartY))
		return
	}

	// Draw Ships (Scrollable if needed, but fleet size is small usually 5 max)
	// We can reuse the column layout logic
	colGap := 10.0
	col1X := startX + 10
	colW := w - 20 - 3*colGap
	// Sub-columns
	subCol1W := colW * 0.25 // Name
	subCol2W := colW * 0.30 // Crew
	subCol3W := colW * 0.25 // Captain
	subCol4W := colW * 0.20 // Actions

	subCol2X := col1X + subCol1W + colGap
	subCol3X := subCol2X + subCol2W + colGap
	subCol4X := subCol3X + subCol3W + colGap

	// Headers
	hdr := bodyFace(12, true)
	drawText(screen, "Navire", hdr, col1X+10, shipsStartY, themeMuted, alignLeft)
	drawText(screen, "Équipage", hdr, subCol2X, shipsStartY, themeMuted, alignLeft)
	drawText(screen, "Capitaine", hdr, subCol3X, shipsStartY, themeMuted, alignLeft)
	drawText(screen, "Actions", hdr, subCol4X, shipsStartY, themeMuted, alignLeft)

	currentY := shipsStartY + 25
	lineH := 100.0 // Taller cards for better detail

	for _, ship := range activeShips {
		// Card BG
		drawCard(screen, col1X, currentY, colW+3*colGap, lineH, false)

		// 1. Ship Info
		drawText(screen, ellipsize(ship.Name, titleFace(20), subCol1W-10), titleFace(20), col1X+10, currentY+6, themeGold, alignLeft)
		uiPrint(screen, shipTypeLabel(ship.Type), int(col1X)+10, int(currentY)+34)
		if ship.MaxHealth > 0 {
			hpPct := ship.Health / ship.MaxHealth
			hpCol := color.Color(themeGreen)
			if hpPct < 0.5 {
				hpCol = themeOrange
			}
			if hpPct < 0.25 {
				hpCol = themeRed
			}
			drawProgressBar(screen, col1X+10, currentY+60, subCol1W-20, 8, hpPct, hpCol)
			uiPrint(screen, fmt.Sprintf("Coque %.0f / %.0f", ship.Health, ship.MaxHealth), int(col1X)+10, int(currentY)+74)
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
		uiPrint(screen, fmt.Sprintf("Milice : %d / %d", totalMilitia, maxMilitia), int(subCol2X), int(currentY)+8)
		uiPrint(screen, fmt.Sprintf("G %d · A %d · C %d", ship.MilitiaWarriors, ship.MilitiaArchers, ship.MilitiaGunners), int(subCol2X), int(currentY)+28)

		// Crew Button placeholder (Visual only, inputs handled in Update)
		btnCrewY := currentY + 50
		drawButton(screen, subCol2X, btnCrewY, subCol2W-10, 34, "Gérer l'équipage", dockedKind(docked, btnSecondary))

		// 3. Captain Info
		captain := g.GetCaptainForShip(ship.ID.String())
		if captain != nil {
			maxStars := captainMaxStars(captain.Rarity)
			drawLabel(screen, ellipsize(captain.Name, bodyFace(14, true), subCol3W-10), subCol3X, currentY+8, 14, true, rarityColor(captain.Rarity))
			uiPrint(screen, fmt.Sprintf("Niveau %d", captain.Level), int(subCol3X), int(currentY)+30)
			drawStars(screen, subCol3X, currentY+60, captain.Stars, maxStars, 6, false)
		} else {
			uiPrint(screen, "Aucun capitaine", int(subCol3X), int(currentY)+8)
		}

		// 4. Actions (Captain buttons)
		btnCaptY := currentY + 5
		if captain != nil {
			// Upgrade
			drawButton(screen, subCol4X, btnCaptY, subCol4W-5, 30, "Améliorer", btnPrimary)

			// Unassign
			drawButton(screen, subCol4X, btnCaptY+36, subCol4W-5, 30, "Retirer", dockedKind(docked, btnDanger))
		} else {
			// Assign
			drawButton(screen, subCol4X, btnCaptY, subCol4W-5, 30, "Assigner", dockedKind(docked, btnPrimary))
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

	// Moral de la flotte (remonte au port, baisse en mer)
	morale := 50
	if fleet.MoraleCruise != nil {
		morale = *fleet.MoraleCruise
	}
	uiSubtitle(screen, "Moral", int(startX)+20, int(currentY))
	mClr := color.Color(color.RGBA{110, 200, 110, 255})
	if morale < 40 {
		mClr = themeOrange
	} else if morale < 70 {
		mClr = themeGold
	}
	drawProgressBar(screen, startX+150, currentY+6, 180, 9, float64(morale)/100, mClr)
	mHint := "baisse en mer"
	if docked {
		mHint = "remonte au port"
	}
	if fleet.Cargo == nil || fleet.Cargo[domain.Rum] <= 0 {
		if !docked {
			mHint = "sans rhum : baisse vite, −20 au combat"
		}
	}
	uiPrint(screen, fmt.Sprintf("%d / 100 · %s", morale, mHint), int(startX)+345, int(currentY))
	currentY += 30

	uiSubtitle(screen, "Cargaison", int(startX)+20, int(currentY))
	// Summary Line
	uiPrint(screen, fmt.Sprintf("Chargée %s / %s · Libre %s", formatThousands(curLoad), formatThousands(capacity), formatThousands(remainingCap)), int(startX)+150, int(currentY))
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
		uiPrint(screen, "Cargaison vide", int(startX)+20, int(currentY))
	} else {
		uiPrint(screen, cargoLine, int(startX)+20, int(currentY))
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
	uiSubtitle(screen, "Transférer des ressources", int(startX)+20, int(currentY))
	currentY += 20

	// Check eligibility
	canTransfer := shipDocked(&fleet)
	if !canTransfer {
		drawText(screen, "Transfert possible uniquement au port de ton île", bodyFace(14, true), startX+20, currentY, themeOrange, alignLeft)
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
			domain.Wood: "Bois", domain.Stone: "Pierre", domain.Gold: "Or", domain.Rum: "Rhum",
		}

		for i, r := range resOrder {
			rx := tfX + float64(i)*(resW+resGap)
			isSelected := string(r) == g.ui.TransferSelectedResource

			col := color.RGBA{40, 40, 40, 255}
			if isSelected {
				col = color.RGBA{0, 100, 200, 255}
			} else {
				// Hover
				cmx, cmy := cursorPosition()
				if float64(cmx) >= rx && float64(cmx) <= rx+resW && float64(cmy) >= currentY && float64(cmy) <= currentY+resH {
					col = color.RGBA{60, 60, 60, 255}
					if isMouseJustPressed(ebiten.MouseButtonLeft) {
						g.ui.TransferSelectedResource = string(r)
						g.ui.TransferAmount = 0
						g.ui.TransferMessage = ""
					}
				}
			}

			_ = col
			drawTab(screen, rx, currentY, resW-4, resH, resourceLabels[r], isSelected)
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
		uiPrint(screen, fmt.Sprintf("Île: %.0f   Flotte: %.0f   Libre: %.0f", islandStock, fleetStock, remainingCap), int(tfX), int(currentY)+5)

		// Right Controls
		// Input Box
		// Input Box
		inputW := 80.0
		inputX := tfX + tfW - inputW - 130 // Make space for buttons

		// Input Interaction
		cmx, cmy := cursorPosition()
		inputHover := float64(cmx) >= inputX && float64(cmx) <= inputX+inputW && float64(cmy) >= currentY && float64(cmy) <= currentY+25

		if isMouseJustPressed(ebiten.MouseButtonLeft) {
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

		registerTextField(textField{id: "fleet-transfer", x: inputX, y: currentY, w: inputW, h: 25,
			value: &g.ui.TransferInputString, focused: g.ui.TransferInputActive, label: "Quantité", numeric: true,
			onChange: func(v string) {
				if val, err := strconv.ParseFloat(v, 64); err == nil {
					g.ui.TransferAmount = val
				} else if v == "" {
					g.ui.TransferAmount = 0
				}
			}})

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
			if isKeyJustPressed(ebiten.KeyBackspace) {
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
			if isKeyJustPressed(ebiten.KeyEnter) || isKeyJustPressed(ebiten.KeyKPEnter) {
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

		uiFillRect(screen, float32(inputX), float32(currentY), float32(inputW), 25, color.RGBA{10, 10, 10, 255}, vecAA)
		uiStrokeRect(screen, float32(inputX), float32(currentY), float32(inputW), 25, 1, borderCol, vecAA)

		displayText := fmt.Sprintf("%.0f", g.ui.TransferAmount)
		if g.ui.TransferInputActive {
			displayText = g.ui.TransferInputString
			// Blink cursor
			if (time.Now().UnixMilli()/500)%2 == 0 {
				displayText += "|"
			}
		}

		uiPrint(screen, displayText, int(inputX)+5, int(currentY)+5)

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

			cmx, cmy = cursorPosition()
			hover := float64(cmx) >= bx && float64(cmx) <= bx+bw && float64(cmy) >= currentY && float64(cmy) <= currentY+25
			col := color.RGBA{60, 60, 60, 255}
			if hover {
				col = color.RGBA{90, 90, 90, 255}
				if isMouseJustPressed(ebiten.MouseButtonLeft) {
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

			uiFillRect(screen, float32(bx), float32(currentY), float32(bw), 25, col, vecAA)
			uiStrokeRect(screen, float32(bx), float32(currentY), float32(bw), 25, 1, color.RGBA{100, 100, 100, 255}, vecAA)

			// Center Label
			lX := int(bx) + 2
			if len(b.label) > 3 {
				lX = int(bx) + 1
			}
			uiPrint(screen, b.label, lX, int(currentY)+5)
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
		loadKind := btnSecondary
		if !canLoad {
			loadKind = btnDisabled
		}
		_ = colLoad
		drawButton(screen, loadX, currentY, actW, actH, "Charger →", loadKind)

		cmx, cmy = cursorPosition()
		if canLoad && float64(cmx) >= loadX && float64(cmx) <= loadX+actW && float64(cmy) >= currentY && float64(cmy) <= currentY+actH {
			if isMouseJustPressed(ebiten.MouseButtonLeft) {
				fid := fleet.ID // Capture locally for closure
				iid := island.ID
				amount := g.ui.TransferAmount
				go func() {
					// Update API context for multi-island safety
					g.api.IslandID = iid
					resp, err := g.api.TransferToFleet(fid, selRes, amount)
					if err != nil {
						g.ui.TransferMessage = "Erreur : " + friendlyError(err)
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
		unloadKind := btnSecondary
		if !canUnload {
			unloadKind = btnDisabled
		}
		_ = colUnload
		drawButton(screen, unloadX, currentY, actW, actH, "← Décharger", unloadKind)

		if canUnload && float64(cmx) >= unloadX && float64(cmx) <= unloadX+actW && float64(cmy) >= currentY && float64(cmy) <= currentY+actH {
			if isMouseJustPressed(ebiten.MouseButtonLeft) {
				fid := fleet.ID
				iid := island.ID
				amount := g.ui.TransferAmount
				go func() {
					g.api.IslandID = iid
					resp, err := g.api.TransferToIsland(fid, selRes, amount)
					if err != nil {
						g.ui.TransferMessage = "Erreur : " + friendlyError(err)
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
			uiPrint(screen, g.ui.TransferMessage, int(startX)+20, int(currentY)+35)
		}
	}
	contentEnd = currentY + 60
}

// DrawManagementFleetList renders the left column list of fleets
func (g *Game) DrawManagementFleetList(screen *ebiten.Image, startX, startY, w, h float64, fleets []domain.Fleet) {
	// List background
	uiFillRect(screen, float32(startX), float32(startY), float32(w), float32(h), color.RGBA{20, 30, 40, 255}, vecAA)
	uiStrokeRect(screen, float32(startX), float32(startY), float32(w), float32(h), 1, color.RGBA{100, 100, 100, 255}, vecAA)

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

		_ = bgColor
		_ = borderColor
		drawCard(screen, startX, itemY, w, itemHeight-4, g.ui.SelectedFleetID == fleet.ID.String())

		// Nom
		drawLabel(screen, fleet.Name, startX+10, itemY+10, 14, true, themeGold)

		// Active Icon
		if isActive {
			drawLabel(screen, "Active", startX+10, itemY+34, 12.5, true, themeGreen)
		} else {
			// Count ships
			activeShips := 0
			for _, s := range fleet.Ships {
				if s.State != "Destroyed" && s.Health > 0 {
					activeShips++
				}
			}
			uiPrint(screen, fmt.Sprintf("%d navires", activeShips), int(startX)+10, int(itemY)+35)
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
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, vecAA)

	// Modal background
	drawButton(screen, float64(modalX), float64(modalY), float64(modalW), float64(modalH), "Selectionner un capitaine", kindFromColor(color.RGBA{40, 30, 20, 255}))

	// Title

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
		uiPrint(screen, "Aucun capitaine disponible", int(contentX), int(contentY))
		uiPrint(screen, "Tous les capitaines sont assignés", int(contentX), int(contentY)+20)
	} else {
		uiPrint(screen, fmt.Sprintf("Capitaines disponibles (%d):", len(availableCaptains)), int(contentX), int(contentY))

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
			uiFillRect(screen, float32(contentX), float32(captY), float32(modalW-40), float32(cardH), color.RGBA{60, 50, 40, 255}, vecAA)
			uiStrokeRect(screen, float32(contentX), float32(captY), float32(modalW-40), float32(cardH), 1, color.RGBA{150, 150, 150, 255}, vecAA)

			// Captain info
			maxStars := captainMaxStars(capt.Rarity)
			starsText := ""
			for i := 0; i < maxStars; i++ {
				if i < capt.Stars {
					starsText += "★"
				} else {
					starsText += "☆"
				}
			}
			uiPrint(screen, capt.Name, int(contentX)+10, int(captY)+5)
			uiPrint(screen, fmt.Sprintf("Niveau %d | %s | %s", capt.Level, capt.Rarity, starsText), int(contentX)+10, int(captY)+20)

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
				drawButton(screen, float64(upgradeBtnX), float64(btnY), float64(btnW), float64(btnH), "Upgrade ★", kindFromColor(upgradeBtnCol))

				// Handle upgrade click
				if isMouseJustPressed(ebiten.MouseButtonLeft) {
					mx, my := cursorPosition()
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
			drawButton(screen, float64(selectBtnX), float64(btnY), float64(btnW), float64(btnH), "Select", kindFromColor(btnCol))

			// Handle select click
			if isMouseJustPressed(ebiten.MouseButtonLeft) {
				mx, my := cursorPosition()
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
	drawCloseButton(screen, float64(closeX)+float64(closeSize)/2, float64(closeY)+float64(closeSize)/2, 15)

	// Handle close button click
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(mx) >= closeX && float64(mx) <= closeX+closeSize && float64(my) >= closeY && float64(my) <= closeY+closeSize {
			g.ui.ShowCaptainModal = false
			g.ui.SelectedShipForCaptain = ""
			g.ui.CaptainModalScroll = 0 // Reset scroll when closing
		}
	}
}

// shipTypeLabel : nom français d'un type de navire.
func shipTypeLabel(t string) string {
	switch t {
	case "sloop":
		return "Sloop"
	case "brigantine":
		return "Brigantin"
	case "frigate":
		return "Frégate"
	case "galleon":
		return "Galion"
	case "manowar":
		return "Vaisseau de guerre"
	}
	return t
}

// dockedKind : bouton normal au port, grisé quand la flotte est en mer.
func dockedKind(docked bool, kind buttonKind) buttonKind {
	if !docked {
		return btnDisabled
	}
	return kind
}

// fleetAtSeaMessage : clic sur une action bloquée parce que la flotte est partie.
func (g *Game) fleetAtSeaMessage() {
	g.ui.FleetMessage = "Ta flotte est en mer : ramène-la au port pour la modifier"
	g.ui.FleetMessageErr = true
}
