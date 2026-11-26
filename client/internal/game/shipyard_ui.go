package game

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

// ShipyardUI manages the tabbed shipyard interface
type ShipyardUI struct {
	CurrentTab         int // 0 = Construction, 1 = Flottes
	ConstructionScroll float64

	// Fleet management (reused from fleet_ui.go)
	SelectedFleetIndex                  int
	ShowAddShipModal                    bool
	TargetFleetIndex                    int
	SelectedShipID                      *uuid.UUID
	ModalOpenDelay                      int
	ModalMessage                        string
	CloseShipAssignModalAfterNextStatus bool       // Flag to close modal after status refresh (thread-safe)
	AssignSuccessTimer                  int        // Timer to keep "Succès !" visible (frames, 60 = 1 second at 60fps)
	AssignSuccessShipID                 *uuid.UUID // Ship ID that was assigned (for verification)
}

var shipyardUI = ShipyardUI{
	CurrentTab:         0,
	SelectedFleetIndex: 1,
}

// DrawShipyardMenu renders the complete tabbed shipyard interface
func (g *Game) DrawShipyardMenu(screen *ebiten.Image) {
	if !g.ui.ShowShipyard {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	winW, winH := 1000.0, 700.0
	winX, winY := cx-winW/2, cy-winH/2

	// Semi-transparent overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 150}, vecAA)

	drawThemePanel(screen, winX, winY, winW, winH, 18)

	// Define explicit boundaries for the scrollable list viewport
	tabY := winY + 50
	tabH := 40.0
	tabBottom := tabY + tabH

	// Bottom upgrade panel position (from DrawShipyardUpgradeBar call below)
	// The upgrade bar is drawn at winY+winH-100 with height 90.0
	upgradeBarY := winY + winH - 100
	upgradeBarTop := upgradeBarY // Top edge of upgrade bar

	// Strict viewport boundaries for the scrollable ship list
	// Top: just below the tabs row (with small margin)
	listViewportTop := tabBottom + 10.0
	// Bottom: just above the upgrade panel (with small margin)
	listViewportBottom := upgradeBarTop - 10.0

	// Calculate content area dimensions - these are the EXACT boundaries
	contentY := listViewportTop
	contentH := listViewportBottom - listViewportTop

	// Ensure contentH is positive (safety check)
	if contentH < 0 {
		contentH = 0
	}

	// Draw content based on active tab (drawn first, so it can be clipped)
	if shipyardUI.CurrentTab == 0 {
		g.DrawConstructionTab(screen, winX, contentY, winW, contentH)
	} else {
		g.DrawFleetsTab(screen, winX, contentY, winW, contentH)
	}

	// Draw header elements on top (title, close button, tabs) so they're always visible
	// Title
	titleY := winY + 15
	uiTitleCentered(screen, "Chantier naval", int(cx), int(titleY))

	// Close button (X)
	closeX, closeY := winX+winW-35, winY+10
	closeSize := 25.0
	drawCloseButton(screen, float64(closeX)+float64(closeSize)/2, float64(closeY)+float64(closeSize)/2, 15)

	// Handle close button click
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(mx) >= closeX && float64(mx) <= closeX+closeSize && float64(my) >= closeY && float64(my) <= closeY+closeSize {
			g.ui.ShowShipyard = false
			return
		}
	}

	// Tabs (drawn on top of content)
	tabW := 200.0
	tab1X, tab2X := winX+20, winX+20+tabW+10

	drawTab(screen, tab1X, tabY, tabW, tabH, "Construction", shipyardUI.CurrentTab == 0)
	drawTab(screen, tab2X, tabY, tabW, tabH, "Flottes", shipyardUI.CurrentTab == 1)

	// Handle tab clicks
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(my) >= tabY && float64(my) <= tabY+tabH {
			if float64(mx) >= tab1X && float64(mx) <= tab1X+tabW {
				shipyardUI.CurrentTab = 0
			} else if float64(mx) >= tab2X && float64(mx) <= tab2X+tabW {
				shipyardUI.CurrentTab = 1
				// Tab switch will be detected in UpdateShipyardMenu() on next frame
			}
		}
	}

	// Upgrade bar at bottom (drawn last to ensure it's on top)
	g.DrawShipyardUpgradeBar(screen, winX, winY+winH-100, winW)
}

// DrawConstructionTab renders the ship construction list and available ships
func (g *Game) DrawConstructionTab(screen *ebiten.Image, x, y, w, h float64) {
	leftW := w * 0.45
	rightX := x + leftW + 20
	rightW := w - leftW - 40

	// Left panel: Ship list
	g.DrawShipList(screen, x+20, y, leftW, h)

	// Right panel: Available ships
	g.DrawAvailableShips(screen, rightX, y, rightW, h)
}

// shipCardH : hauteur d'une carte de navire dans le chantier.
const shipCardH = 150.0

// DrawShipList renders scrollable list of ships that can be built
// x, y, w, h define the exact viewport boundaries (y = top, y+h = bottom)
func (g *Game) DrawShipList(screen *ebiten.Image, x, y, w, h float64) {
	drawInset(screen, x, y, w, h)

	ships := g.getAvailableShipTypes()
	itemH := shipCardH
	gap := 10.0
	startY := y + 10 - shipyardUI.ConstructionScroll
	clipTop, clipBottom := y, y+h

	// Tout ce qui dépasse de la liste est coupé net.
	view := screen.SubImage(image.Rect(int(x), int(y), int(x+w), int(y+h))).(*ebiten.Image)

	var have map[domain.ResourceType]float64
	if g.player != nil && len(g.player.Islands) > 0 {
		have = g.player.Islands[0].Resources
	}
	shipyardLevel := g.getShipyardLevel()

	for i, ship := range ships {
		cardTop := startY + float64(i)*(itemH+gap)
		if cardTop+itemH <= clipTop || cardTop >= clipBottom {
			continue
		}
		itemX := x + 10
		itemW := w - 20
		drawCard(view, itemX, cardTop, itemW, itemH, false)

		textX := itemX + 16
		drawText(view, ship.Name, titleFace(22), textX, cardTop+8, themeGold, alignLeft)
		for j, line := range strings.Split(ship.Stats, "\n") {
			clr := color.Color(themeText)
			if j > 0 {
				clr = themeMuted
				line = strings.ReplaceAll(strings.ReplaceAll(line, " | ", " · "), "PV:", "PV")
				line = strings.ReplaceAll(line, "Cargo:", "Cargaison")
			}
			drawText(view, line, bodyFace(13, false), textX, cardTop+40+float64(j)*18, clr, alignLeft)
		}
		drawCost(view, ship.Cost, have, textX, cardTop+84)
		drawText(view, "Durée "+ship.BuildTime, bodyFace(13, false), textX, cardTop+104, themeMuted, alignLeft)

		// Prérequis
		canBuild := shipyardLevel >= ship.ReqLevel
		hasTech := true
		if ship.RequiredTech != "" && g.player != nil {
			hasTech = false
			for _, techID := range g.player.UnlockedTechs {
				if techID == ship.RequiredTech {
					hasTech = true
					break
				}
			}
		}
		canBuild = canBuild && hasTech

		btnW, btnH := 150.0, 38.0
		btnX := itemX + itemW - btnW - 14
		btnY := cardTop + itemH - btnH - 14
		kind, label := btnPrimary, "Construire"
		if shipyardLevel < ship.ReqLevel {
			kind, label = btnDisabled, fmt.Sprintf("Chantier niv. %d", ship.ReqLevel)
		} else if !hasTech {
			kind, label = btnDisabled, "Technologie requise"
		}
		drawButton(view, btnX, btnY, btnW, btnH, label, kind)

		// Handle button click
		btnVisible := btnY >= clipTop && btnY+btnH <= clipBottom
		if canBuild && btnVisible && isMouseJustPressed(ebiten.MouseButtonLeft) {
			mx, my := cursorPosition()
			if float64(mx) >= btnX && float64(mx) <= btnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
				g.Log("[SHIP BUILD] Button clicked: %s (Type: %s)", ship.Name, ship.ID)
				go func(shipType, shipName string) {
					g.Log("[SHIP BUILD] Sending request to server: %s", shipType)
					err := g.api.BuildShip(shipType)
					if err != nil {
						g.Log("[SHIP BUILD ERROR] Server refused: %v", err)
						if !g.handleAPIError(err, fmt.Sprintf("Construction %s", shipName)) {
							g.showError = true
							g.errorMessage = fmt.Sprintf("Construction refusée : %v", err)
							g.errorDebounce = 30
						}
					} else {
						g.Log("[SHIP BUILD SUCCESS] Server accepted construction: %s", shipName)
						time.Sleep(100 * time.Millisecond)
					}
				}(ship.ID, ship.Name)
			}
		}
	}

	// Handle mouse wheel scrolling
	// Constrain scroll to prevent items from escaping the viewport
	_, wy := wheelDelta()
	if wy != 0 {
		totalH := float64(len(ships)) * (itemH + gap)

		// Calculate scroll bounds to keep all items within viewport
		// When scroll = 0: first item is at y+10
		// When scroll increases: items move up (startY decreases)
		//
		// Maximum scroll: when the last item's bottom aligns with clipBottom
		// Last item bottom = startY + totalH = (y+10 - scroll) + totalH
		// We want: (y+10 - maxScroll) + totalH <= clipBottom
		// So: maxScroll >= y + 10 + totalH - clipBottom
		maxScroll := y + 10 + totalH - clipBottom
		if maxScroll < 0 {
			maxScroll = 0
		}

		shipyardUI.ConstructionScroll -= wy * 30
		if shipyardUI.ConstructionScroll < 0 {
			shipyardUI.ConstructionScroll = 0
		}
		if shipyardUI.ConstructionScroll > maxScroll {
			shipyardUI.ConstructionScroll = maxScroll
		}
	}
}

// DrawAvailableShips shows ships already built or under construction
func (g *Game) DrawAvailableShips(screen *ebiten.Image, x, y, w, h float64) {
	drawInset(screen, x, y, w, h)
	uiSubtitle(screen, "Navires au port", int(x)+16, int(y)+14)

	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	listY := y + 50
	island := g.player.Islands[0]

	idx := 0
	for _, ship := range island.Ships {
		// Ni les navires détruits, ni ceux déjà dans une flotte
		if ship.State == "Destroyed" || ship.FleetID != nil {
			continue
		}
		itemY := listY + float64(idx)*72
		if itemY > y+h-72 {
			break
		}
		itemX, itemW, itemH := x+12, w-24, 62.0
		drawCard(screen, itemX, itemY, itemW, itemH, false)
		drawText(screen, ship.Name, titleFace(20), itemX+14, itemY+6, themeGold, alignLeft)
		status, clr := "Prêt à prendre la mer", color.Color(themeGreen)
		if ship.State == "UnderConstruction" {
			status, clr = "En construction…", themeOrange
		}
		drawText(screen, shipTypeLabel(ship.Type)+" · "+status, bodyFace(13, false), itemX+14, itemY+36, clr, alignLeft)
		idx++
	}
	if idx == 0 {
		drawText(screen, "Aucun navire au port. Construis-en un à gauche !", bodyFace(14, false), x+16, listY, themeMuted, alignLeft)
	}
}

// DrawFleetsTab renders fleet management interface
func (g *Game) DrawFleetsTab(screen *ebiten.Image, x, y, w, h float64) {
	leftW := w * 0.45
	rightX := x + leftW + 20
	rightW := w - leftW - 40

	// Reuse fleet drawing code from fleet_ui.go
	g.DrawFleetList(screen, x+20, y, leftW, h)
	g.DrawFleetDetails(screen, rightX, y, rightW, h)

	// Draw modal if open
	if shipyardUI.ShowAddShipModal {
		g.DrawAddShipModalShipyard(screen)
	}
}

// DrawShipyardUpgradeBar renders upgrade information and button at bottom
func (g *Game) DrawShipyardUpgradeBar(screen *ebiten.Image, x, y, w float64) {
	barH := 90.0

	drawCard(screen, x+20, y, w-40, barH, false)

	currentLevel := g.getShipyardLevel()
	textX := x + 36
	drawText(screen, fmt.Sprintf("Chantier naval · niveau %d", currentLevel), titleFace(22), textX, y+10, themeGold, alignLeft)

	if currentLevel >= 30 {
		drawText(screen, "Niveau maximum atteint", bodyFace(14, true), textX, y+44, themeGreen, alignLeft)
		return
	}

	upgradeCost := g.getBuildingCost("Chantier Naval", currentLevel)
	var have map[domain.ResourceType]float64
	if g.player != nil && len(g.player.Islands) > 0 {
		have = g.player.Islands[0].Resources
	}
	drawText(screen, "Niveau suivant :", bodyFace(13, false), textX, y+44, themeMuted, alignLeft)
	drawCost(screen, upgradeCost, have, textX+textWidth("Niveau suivant : ", bodyFace(13, false)), y+44)
	buildTime := g.getBuildingDuration("Chantier Naval", currentLevel+1)
	drawText(screen, "Durée "+formatDuration(int(buildTime.Seconds())), bodyFace(13, false), textX, y+64, themeMuted, alignLeft)

	// Upgrade button
	btnW, btnH := 150.0, 40.0
	btnX := x + w - btnW - 40
	btnY := y + barH/2 - btnH/2

	// Check if can afford
	canAfford := true
	if g.player == nil || len(g.player.Islands) == 0 {
		canAfford = false
	}
	for res, amt := range upgradeCost {
		if have[res] < amt {
			canAfford = false
		}
	}

	// Check if busy
	isBusy := false
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			if b.Constructing {
				isBusy = true
				break
			}
		}
	}

	kind, btnText := btnPrimary, "Améliorer"
	if isBusy {
		kind, btnText = btnDisabled, "Occupé"
		canAfford = false
	} else if !canAfford {
		kind, btnText = btnDisabled, "Ressources"
	}
	drawButton(screen, btnX, btnY, btnW, btnH, btnText, kind)

	// Handle upgrade button click
	if canAfford && !isBusy && isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(mx) >= btnX && float64(mx) <= btnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
			// Find shipyard building ID
			var shipyardID string
			if g.player != nil && len(g.player.Islands) > 0 {
				for _, b := range g.player.Islands[0].Buildings {
					if b.Type == "Chantier Naval" {
						shipyardID = b.ID.String()
						break
					}
				}
			}
			if shipyardID != "" {
				g.Log("Ordering Upgrade: Chantier Naval")
				go func(pid, bid string) {
					err := g.api.Upgrade(pid, bid)
					if err != nil {
						g.Log("Shipyard upgrade failed: %v", err)
						// Try to handle as prerequisites error
						if !g.handleAPIError(err, "Amélioration Chantier Naval") {
							// Not a prerequisites error - show standard error
							g.showError = true
							g.errorMessage = err.Error()
							g.errorDebounce = 60
						}
					} else {
						time.Sleep(100 * time.Millisecond)
					}
				}(g.player.ID.String(), shipyardID)
			}
		}
	}
}

// Helper: Get shipyard level
func (g *Game) getShipyardLevel() int {
	if g.player == nil || len(g.player.Islands) == 0 {
		return 0
	}
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type == "Chantier Naval" {
			return b.Level
		}
	}
	return 0
}
