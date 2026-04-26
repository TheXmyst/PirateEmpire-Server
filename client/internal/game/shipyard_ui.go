package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 150}, true)

	// Main window with 9-slice
	draw9Slice(screen, g, winX, winY, winW, winH, 16)

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
	ebitenutil.DebugPrintAt(screen, "CHANTIER NAVAL", int(cx)-60, int(titleY))

	// Close button (X)
	closeX, closeY := winX+winW-35, winY+10
	closeSize := 25.0
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), float32(closeSize), float32(closeSize), color.RGBA{150, 0, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+8, int(closeY)+5)

	// Handle close button click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= closeX && float64(mx) <= closeX+closeSize && float64(my) >= closeY && float64(my) <= closeY+closeSize {
			g.ui.ShowShipyard = false
			return
		}
	}

	// Tabs (drawn on top of content)
	tabW := 200.0
	tab1X, tab2X := winX+20, winX+20+tabW+10

	// Tab 1: CONSTRUCTION
	tab1Col := color.RGBA{100, 100, 100, 255}
	if shipyardUI.CurrentTab == 0 {
		tab1Col = color.RGBA{200, 180, 0, 255} // Yellow for active
	}
	vector.DrawFilledRect(screen, float32(tab1X), float32(tabY), float32(tabW), float32(tabH), tab1Col, true)
	ebitenutil.DebugPrintAt(screen, "CONSTRUCTION", int(tab1X)+40, int(tabY)+12)

	// Tab 2: FLOTTES
	tab2Col := color.RGBA{100, 100, 100, 255}
	if shipyardUI.CurrentTab == 1 {
		tab2Col = color.RGBA{200, 180, 0, 255}
	}
	vector.DrawFilledRect(screen, float32(tab2X), float32(tabY), float32(tabW), float32(tabH), tab2Col, true)
	ebitenutil.DebugPrintAt(screen, "FLOTTES", int(tab2X)+60, int(tabY)+12)

	// Handle tab clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
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

// DrawShipList renders scrollable list of ships that can be built
// x, y, w, h define the exact viewport boundaries (y = top, y+h = bottom)
func (g *Game) DrawShipList(screen *ebiten.Image, x, y, w, h float64) {
	// Background with strict clipping box
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 30, 40, 200}, true)

	// Visible border to show clipping area (optional, can be removed for production)
	// vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{200, 150, 50, 128}, true)

	ships := g.getAvailableShipTypes()
	itemH := 200.0
	gap := 10.0
	startY := y + 10 - shipyardUI.ConstructionScroll

	// Strict clipping bounds: exactly match the viewport passed to this function
	// These are the absolute boundaries - nothing should render outside this range
	clipTop := y
	clipBottom := y + h

	for i, ship := range ships {
		// Single source of truth for card geometry
		cardTop := startY + float64(i)*(itemH+gap)
		cardBottom := cardTop + itemH

		// Check if card is visible (at least partially)
		if cardBottom <= clipTop || cardTop >= clipBottom {
			continue // Completely outside viewport
		}

		// Card is visible - draw complete card using cardTop as reference
		itemX := x + 10
		itemW := w - 20

		// Draw card background - but clip it to viewport if partially outside
		// Calculate visible portion of card
		drawCardTop := cardTop
		drawCardH := itemH

		// Clip top if card extends above viewport
		if cardTop < clipTop {
			drawCardH = itemH - (clipTop - cardTop)
			drawCardTop = clipTop
		}
		// Clip bottom if card extends below viewport
		if cardBottom > clipBottom {
			drawCardH = clipBottom - drawCardTop
		}

		// Only draw if there's a visible portion
		if drawCardH > 0 && drawCardTop < clipBottom {
			vector.DrawFilledRect(screen, float32(itemX), float32(drawCardTop), float32(itemW), float32(drawCardH), color.RGBA{30, 40, 50, 255}, true)
		}

		// Draw full border only if card is fully or mostly visible
		// If card is partially clipped, draw border only for visible portion
		if cardTop >= clipTop && cardBottom <= clipBottom {
			// Fully visible - draw complete border
			vector.StrokeRect(screen, float32(itemX), float32(cardTop), float32(itemW), float32(itemH), 2, color.RGBA{200, 150, 50, 255}, true)
		} else if drawCardH > 0 {
			// Partially visible - draw border for visible portion only
			vector.StrokeRect(screen, float32(itemX), float32(drawCardTop), float32(itemW), float32(drawCardH), 2, color.RGBA{200, 150, 50, 255}, true)
		}

		// All text positions are relative to cardTop (single source of truth)
		textX := int(itemX) + 10
		textHeight := 12

		// Fixed offsets from cardTop (all relative to cardTop, not cardBottom)
		titleOffset := 10.0
		statsOffset := 30.0
		costLabelOffset := 60.0
		costValueOffset := 75.0
		timeOffset := 95.0
		buttonOffset := itemH - 50.0 // Button is 40px high, 10px from bottom

		// Ship name (relative to cardTop)
		nameY := int(cardTop + titleOffset)
		if float64(nameY+textHeight) <= clipBottom && float64(nameY) >= clipTop {
			ebitenutil.DebugPrintAt(screen, ship.Name, textX, nameY)
		}

		// Stats (multiline, relative to cardTop)
		statsY := int(cardTop + statsOffset)
		if float64(statsY+textHeight*2) <= clipBottom && float64(statsY) >= clipTop {
			ebitenutil.DebugPrintAt(screen, ship.Stats, textX, statsY)
		}

		// Cost label (relative to cardTop)
		costLabelY := int(cardTop + costLabelOffset)
		if float64(costLabelY+textHeight) <= clipBottom && float64(costLabelY) >= clipTop {
			ebitenutil.DebugPrintAt(screen, "Cout:", textX, costLabelY)
		}

		// Cost values (relative to cardTop)
		// Build cost string in fixed order to prevent flickering
		costStr := buildCostString(ship.Cost, false, nil)
		costValueY := int(cardTop + costValueOffset)
		// Only draw if fully within visible bounds (prevent text bleeding)
		if float64(costValueY+textHeight) <= clipBottom && float64(costValueY) >= clipTop {
			// Draw cost text with opaque background to prevent ghosting
			drawTextWithBackground(screen, costStr, textX, costValueY, color.RGBA{30, 40, 50, 255})
		}

		// Build time (relative to cardTop)
		timeY := int(cardTop + timeOffset)
		// Only draw if fully within visible bounds (prevent text bleeding)
		if float64(timeY+textHeight) <= clipBottom && float64(timeY) >= clipTop {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Temps: %s", ship.BuildTime), textX, timeY)
		}

		// Build button (relative to cardTop, not cardBottom for consistency)
		btnW, btnH := 180.0, 40.0
		btnX := itemX + (itemW-btnW)/2
		btnY := cardTop + buttonOffset

		// Check prerequisites
		shipyardLevel := g.getShipyardLevel()
		canBuild := shipyardLevel >= ship.ReqLevel

		// Check tech prerequisites
		if ship.RequiredTech != "" && g.player != nil {
			hasTech := false
			for _, techID := range g.player.UnlockedTechs {
				if techID == ship.RequiredTech {
					hasTech = true
					break
				}
			}
			if !hasTech {
				canBuild = false
			}
		}

		btnCol := color.RGBA{200, 180, 0, 255} // Yellow
		btnText := "CONSTRUIRE"
		if !canBuild {
			btnCol = color.RGBA{80, 80, 80, 255} // Gray locked
			if shipyardLevel < ship.ReqLevel {
				btnText = fmt.Sprintf("NIV %d", ship.ReqLevel)
			} else {
				btnText = "TECH REQ"
			}
		}

		// Only draw button if fully within visible bounds (button is relative to cardTop)
		if btnY+btnH <= clipBottom && btnY >= clipTop {
			vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
			ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+40, int(btnY)+12)
		}

		// Handle button click
		if canBuild && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if float64(mx) >= btnX && float64(mx) <= btnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
				// Build ship
				g.Log("[SHIP BUILD] Button clicked: %s (Type: %s)", ship.Name, ship.ID)
				go func(shipType, shipName string) {
					g.Log("[SHIP BUILD] Sending request to server: %s", shipType)
					err := g.api.BuildShip(shipType)
					if err != nil {
						g.Log("[SHIP BUILD ERROR] Server refused: %v", err)
						// Try to handle as prerequisites error
						if !g.handleAPIError(err, fmt.Sprintf("Construction %s", shipName)) {
							// Not a prerequisites error - show standard error
							g.showError = true
							g.errorMessage = fmt.Sprintf("Construction refusée: %v", err)
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
	_, wy := ebiten.Wheel()
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
	// Background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 30, 40, 200}, true)

	// Title
	ebitenutil.DebugPrintAt(screen, "BATEAU(X) DISPONIBLE(S)", int(x)+10, int(y)+10)

	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	listY := y + 40
	island := g.player.Islands[0]

	if len(island.Ships) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun navire", int(x)+10, int(listY))
		return
	}

	// List ships
	idx := 0
	for _, ship := range island.Ships {
		// Skip destroyed ships AND ships already assigned to a fleet
		if ship.State == "Destroyed" || ship.FleetID != nil {
			continue
		}

		itemY := listY + float64(idx)*80
		if itemY > y+h-80 {
			break // Don't overflow
		}

		// Ship box with golden border
		itemX := x + 10
		itemW := w - 20
		itemH := 70.0
		vector.DrawFilledRect(screen, float32(itemX), float32(itemY), float32(itemW), float32(itemH), color.RGBA{30, 40, 50, 255}, true)
		vector.StrokeRect(screen, float32(itemX), float32(itemY), float32(itemW), float32(itemH), 2, color.RGBA{200, 150, 50, 255}, true)

		// Ship name
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%s)", ship.Name, ship.Type), int(itemX)+10, int(itemY)+10)

		// Status
		if ship.State == "UnderConstruction" {
			ebitenutil.DebugPrintAt(screen, "En Construction...", int(itemX)+10, int(itemY)+30)
		} else {
			ebitenutil.DebugPrintAt(screen, "Disponible", int(itemX)+10, int(itemY)+30)
		}

		idx++
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

	// Background with golden border
	vector.DrawFilledRect(screen, float32(x+20), float32(y), float32(w-40), float32(barH), color.RGBA{30, 40, 50, 255}, true)
	vector.StrokeRect(screen, float32(x+20), float32(y), float32(w-40), float32(barH), 2, color.RGBA{200, 150, 50, 255}, true)

	// Get current level
	currentLevel := g.getShipyardLevel()

	// Display current level
	textX := int(x) + 30
	textY := int(y) + 10
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Niveau Actuel: %d", currentLevel), textX, textY)

	if currentLevel >= 30 {
		ebitenutil.DebugPrintAt(screen, "NIVEAU MAX ATTEINT", textX, textY+20)
		return
	}

	// Get upgrade cost
	upgradeCost := g.getBuildingCost("Chantier Naval", currentLevel)
	costY := textY + 25
	// Draw cost text with opaque background to prevent ghosting
	// Use fixed order to prevent flickering
	costStr := buildCostString(upgradeCost, false, nil)
	costText := fmt.Sprintf("Cout Niv Suivant: %s", costStr)
	drawTextWithBackground(screen, costText, textX, costY, color.RGBA{30, 40, 50, 255})

	// Build time
	buildTime := g.getBuildingDuration("Chantier Naval", currentLevel+1)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Temps Construction: %ds", int(buildTime.Seconds())), textX, costY+15)

	// Upgrade button
	btnW, btnH := 150.0, 40.0
	btnX := x + w - btnW - 40
	btnY := y + barH/2 - btnH/2

	// Check if can afford
	canAfford := true
	if g.player != nil && len(g.player.Islands) == 0 {
		canAfford = false
	}
	for res, amt := range upgradeCost {
		if g.player.Islands[0].Resources[res] < amt {
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

	btnCol := color.RGBA{0, 150, 0, 255} // Green
	btnText := "AMELIORER"
	if isBusy {
		btnCol = color.RGBA{80, 80, 80, 255}
		btnText = "OCCUPE"
		canAfford = false
	} else if !canAfford {
		btnCol = color.RGBA{150, 50, 50, 255}
		btnText = "MANQUE"
	}

	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
	ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+30, int(btnY)+12)

	// Handle upgrade button click
	if canAfford && !isBusy && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
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
