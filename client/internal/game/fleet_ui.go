package game

import (
	"fmt"
	"image/color"
	"log"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// FleetUI State
type FleetUI struct {
	SelectedFleetIndex int // 1, 2, 3 (0 = None)

	// Modal State
	ShowAddModal     bool
	TargetFleetIndex int
	SelectedShipID   *uuid.UUID // ID of ship selected in modal
	ModalOpenDelay   int        // Debounce input
	ModalMessage     string     // Feedback message
}

var fleetUI = FleetUI{
	SelectedFleetIndex: 1, // Default select first
}

// DrawFleetTab renders the Split Fleet Management Interface
func (g *Game) DrawFleetTab(screen *ebiten.Image, lx, ly, lw, lh, rx, ry, rw, rh float64) {
	// 1. Calculate Limits
	shipyardLevel := 0
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			if b.Type == "Chantier Naval" {
				shipyardLevel = b.Level
				break
			}
		}
	}

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

	// 2. Draw Left Panel (Fleet List)
	slotH := 120.0
	padding := 20.0
	startY := ly

	mx, my := ebiten.CursorPosition()

	// Disable interaction if modal is open
	modalOpen := fleetUI.ShowAddModal

	for i := 1; i <= 3; i++ {
		slotY := startY + float64(i-1)*(slotH+padding)
		msg := fmt.Sprintf("Flotte %d", i)
		locked := i > maxFleets

		// Interaction
		hover := false
		if !modalOpen {
			hover = float64(mx) >= lx && float64(mx) <= lx+lw && float64(my) >= slotY && float64(my) <= slotY+slotH
		}

		if locked {
			// Locked State
			vector.DrawFilledRect(screen, float32(lx), float32(slotY), float32(lw), float32(slotH), color.RGBA{50, 50, 50, 200}, false)
			ebitenutil.DebugPrintAt(screen, msg+" (Verrouillée)", int(lx)+10, int(slotY)+10)
			req := 10
			if i == 3 {
				req = 20
			}
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Req: Niv. %d", req), int(lx)+10, int(slotY)+30)
		} else {
			// Unlocked State
			col := color.RGBA{0, 0, 0, 100} // Default Dark
			if fleetUI.SelectedFleetIndex == i {
				col = color.RGBA{0, 100, 200, 150} // Selected Blue
			} else if hover {
				col = color.RGBA{50, 50, 50, 150} // Hover Gray
			}

			// Draw BG
			vector.DrawFilledRect(screen, float32(lx), float32(slotY), float32(lw), float32(slotH), col, false)
			// Border
			if fleetUI.SelectedFleetIndex == i {
				vector.StrokeLine(screen, float32(lx), float32(slotY), float32(lx+lw), float32(slotY), 1, color.White, false)
				vector.StrokeLine(screen, float32(lx), float32(slotY+slotH), float32(lx+lw), float32(slotY+slotH), 1, color.White, false)
				vector.StrokeLine(screen, float32(lx), float32(slotY), float32(lx), float32(slotY+slotH), 1, color.White, false)
				vector.StrokeLine(screen, float32(lx+lw), float32(slotY), float32(lx+lw), float32(slotY+slotH), 1, color.White, false)
			}

			// Get Fleet Info
			shipCount := 0
			if g.player != nil && len(g.player.Islands) > 0 && len(g.player.Islands[0].Fleets) >= i {
				if i <= len(g.player.Islands[0].Fleets) {
					shipCount = len(g.player.Islands[0].Fleets[i-1].Ships)
				}
			}

			ebitenutil.DebugPrintAt(screen, msg, int(lx)+10, int(slotY)+10)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d / %d Navires", shipCount, maxShips), int(lx)+10, int(slotY)+30)

			// Click to Select
			if hover && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fleetUI.SelectedFleetIndex = i
			}
		}
	}

	// 3. Draw Right Panel (Details of Selected Fleet)
	// Background for Details
	vector.DrawFilledRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{0, 0, 0, 80}, false)

	if fleetUI.SelectedFleetIndex == 0 {
		ebitenutil.DebugPrintAt(screen, "Sélectionnez une flotte.", int(rx)+20, int(ry)+20)
		return
	}

	selIdx := fleetUI.SelectedFleetIndex

	// Double check lock
	if selIdx > maxFleets {
		ebitenutil.DebugPrintAt(screen, "Flotte Verrouillée.", int(rx)+20, int(ry)+20)
		return
	}

	title := fmt.Sprintf("FLOTTE %d", selIdx)
	ebitenutil.DebugPrintAt(screen, title, int(rx)+20, int(ry)+10)

	// Determine Ships in this Fleet
	currentShips := 0
	capacityMsg := fmt.Sprintf("Capacité : %d / %d", currentShips, maxShips)

	listY := ry + 40
	var currentFleet *domain.Fleet

	// Find fleet by name (Flotte 1, 2, or 3) - use actual fleet from server data
	expectedName := fmt.Sprintf("Flotte %d", selIdx)
	if g.player != nil && len(g.player.Islands) > 0 {
		for i := range g.player.Islands[0].Fleets {
			if g.player.Islands[0].Fleets[i].Name == expectedName {
				currentFleet = &g.player.Islands[0].Fleets[i]
				currentShips = len(currentFleet.Ships)
				capacityMsg = fmt.Sprintf("Capacité : %d / %d", currentShips, maxShips)
				log.Printf("[FLEET UI] Found fleet: name=%s, id=%s, ships=%d", currentFleet.Name, currentFleet.ID, currentShips)
				break
			}
		}
		// Fallback: if not found by name, try by index (for backward compatibility)
		if currentFleet == nil && selIdx <= len(g.player.Islands[0].Fleets) {
			currentFleet = &g.player.Islands[0].Fleets[selIdx-1]
			currentShips = len(currentFleet.Ships)
			capacityMsg = fmt.Sprintf("Capacité : %d / %d", currentShips, maxShips)
			log.Printf("[FLEET UI] Found fleet by index fallback: name=%s, id=%s, ships=%d", currentFleet.Name, currentFleet.ID, currentShips)
		}
	}

	if currentFleet == nil {
		ebitenutil.DebugPrintAt(screen, "Flotte non initialisée.", int(rx)+20, int(listY))
	} else {
		if currentShips == 0 {
			ebitenutil.DebugPrintAt(screen, "Aucun navire assigné.", int(rx)+20, int(listY))
		} else {
			for k, s := range currentFleet.Ships {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("- %s (%s)", s.Name, s.Type), int(rx)+20, int(listY+float64(k)*30))
			}
			// Update listY to after ships
			listY += float64(len(currentFleet.Ships)) * 30
		}
	}

	ebitenutil.DebugPrintAt(screen, capacityMsg, int(rx)+20, int(ry+rh-60))

	// Add Ship Button
	btnAddW, btnAddH := 200.0, 40.0
	btnAddX, btnAddY := rx+20, ry+rh-50

	hoverAdd := false
	if !modalOpen {
		hoverAdd = float64(mx) >= btnAddX && float64(mx) <= btnAddX+btnAddW && float64(my) >= btnAddY && float64(my) <= btnAddY+btnAddH
	}

	colAdd := color.RGBA{0, 150, 0, 255}
	if currentShips >= maxShips {
		colAdd = color.RGBA{100, 100, 100, 255} // Disabled
	} else if hoverAdd {
		colAdd = color.RGBA{0, 200, 0, 255}
	}

	vector.DrawFilledRect(screen, float32(btnAddX), float32(btnAddY), float32(btnAddW), float32(btnAddH), colAdd, false)
	labelAdd := "Ajouter un navire"
	if currentShips >= maxShips {
		labelAdd = "Flotte Complète"
	}
	ebitenutil.DebugPrintAt(screen, labelAdd, int(btnAddX)+20, int(btnAddY)+12)

	if hoverAdd && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && currentShips < maxShips {
		// Open Modal
		fleetUI.ShowAddModal = true
		fleetUI.TargetFleetIndex = selIdx
		fleetUI.SelectedShipID = nil
		fleetUI.ModalOpenDelay = 10
		fleetUI.ModalMessage = ""
	}

	// Draw Modal if Open
	if fleetUI.ShowAddModal {
		g.DrawAddShipModal(screen)
	}
}

func (g *Game) DrawAddShipModal(screen *ebiten.Image) {
	if fleetUI.ModalOpenDelay > 0 {
		fleetUI.ModalOpenDelay--
	}
	inputEnabled := fleetUI.ModalOpenDelay == 0

	w, h := float64(g.screenWidth), float64(g.screenHeight)

	// Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, false)

	// Modal Window
	modalW, modalH := 500.0, 400.0
	mxStart, myStart := (w-modalW)/2, (h-modalH)/2

	draw9Slice(screen, g, mxStart, myStart, modalW, modalH, 12)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Ajouter un navire a la Flotte %d", fleetUI.TargetFleetIndex), int(mxStart)+20, int(myStart)+20)

	// Feedback Message
	if fleetUI.ModalMessage != "" {
		ebitenutil.DebugPrintAt(screen, fleetUI.ModalMessage, int(mxStart)+20, int(myStart)+40)
	}

	// List available ships
	shipsY := myStart + 60

	var availableShips []*domain.Ship
	if g.player != nil && len(g.player.Islands) > 0 {
		for i := range g.player.Islands[0].Ships {
			s := &g.player.Islands[0].Ships[i]
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
			if fleetUI.SelectedShipID != nil && *fleetUI.SelectedShipID == s.ID {
				col = color.RGBA{0, 100, 200, 255} // Selected
			} else if hover {
				col = color.RGBA{80, 80, 80, 255}
			}

			vector.DrawFilledRect(screen, float32(rowX), float32(rowY), float32(rowW), float32(rowH), col, false)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%s)", s.Name, s.Type), int(rowX)+10, int(rowY)+8)

			if hover && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				id := s.ID
				fleetUI.SelectedShipID = &id
			}
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

	if inputEnabled && hoverCancel && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		fleetUI.ShowAddModal = false
		fleetUI.SelectedShipID = nil
	}

	// Assign
	btnOkX, btnOkY := mxStart+20, myStart+modalH-50
	hoverOk := float64(curMx) >= btnOkX && float64(curMx) <= btnOkX+btnW && float64(curMy) >= btnOkY && float64(curMy) <= btnOkY+btnH
	colOk := color.RGBA{0, 150, 0, 255}
	disabled := fleetUI.SelectedShipID == nil
	if disabled {
		colOk = color.RGBA{100, 100, 100, 255}
	} else if hoverOk {
		colOk = color.RGBA{0, 200, 0, 255}
	}

	vector.DrawFilledRect(screen, float32(btnOkX), float32(btnOkY), float32(btnW), float32(btnH), colOk, false)
	ebitenutil.DebugPrintAt(screen, "ASSIGNER", int(btnOkX)+30, int(btnOkY)+8)

	if inputEnabled && !disabled && hoverOk && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Execute Assign - find fleet by name to get the correct ID from server
		if g.player != nil && len(g.player.Islands) > 0 {
			// Find fleet by matching name (Flotte 1, Flotte 2, Flotte 3)
			expectedName := fmt.Sprintf("Flotte %d", fleetUI.TargetFleetIndex)
			var targetFleet *domain.Fleet
			for i := range g.player.Islands[0].Fleets {
				if g.player.Islands[0].Fleets[i].Name == expectedName {
					targetFleet = &g.player.Islands[0].Fleets[i]
					log.Printf("[FLEET MODAL] Found target fleet: name=%s, id=%s, index=%d", targetFleet.Name, targetFleet.ID, fleetUI.TargetFleetIndex)
					break
				}
			}

			// Fallback: if not found by name, try by index (for backward compatibility)
			if targetFleet == nil && fleetUI.TargetFleetIndex <= len(g.player.Islands[0].Fleets) {
				targetFleet = &g.player.Islands[0].Fleets[fleetUI.TargetFleetIndex-1]
				log.Printf("[FLEET MODAL] Found target fleet by index fallback: name=%s, id=%s, index=%d", targetFleet.Name, targetFleet.ID, fleetUI.TargetFleetIndex)
			}

			if targetFleet == nil {
				log.Printf("[FLEET ERROR] Fleet not found: index=%d, available_fleets=%d", fleetUI.TargetFleetIndex, len(g.player.Islands[0].Fleets))
				fleetUI.ModalMessage = "Erreur: Flotte introuvable"
				return
			}

			shipID := *fleetUI.SelectedShipID
			fleetID := targetFleet.ID

			fleetUI.ModalMessage = "Assignation en cours..."

			go func() {
				log.Printf("[FLEET] Adding ship %s to fleet %s (name=%s, index=%d)", shipID, fleetID, targetFleet.Name, fleetUI.TargetFleetIndex)
				log.Printf("[FLEET DEBUG] Available fleets from server:")
				if g.player != nil && len(g.player.Islands) > 0 {
					for i, f := range g.player.Islands[0].Fleets {
						log.Printf("[FLEET DEBUG]   Fleet[%d]: name=%s, id=%s, ships=%d", i, f.Name, f.ID, len(f.Ships))
					}
				}
				err := g.api.AddShipToFleet(fleetID, shipID)
				if err != nil {
					log.Printf("Failed to add ship: %v", err)
					fleetUI.ModalMessage = "Erreur: Echec assignation"
				} else {
					log.Printf("Ship added successfully!")
					fleetUI.ModalMessage = "Succes !"
					// Refresh Data
					player, err := g.api.GetStatus()
					if err == nil {
						g.player = player
						fleetUI.ShowAddModal = false
						fleetUI.SelectedShipID = nil
					} else {
						fleetUI.ModalMessage = "Erreur Sync"
					}
				}
			}()
		}
	}
}
