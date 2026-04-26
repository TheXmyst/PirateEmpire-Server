package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// hud_input.go contains all HUD input handling and UI routing logic.
// This file was extracted from main.go during Phase 1.3 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// UpdateHUDInput handles all HUD button input (hover, clicks, UI toggling).
// This includes the Build button (CONST), Sea button (MER), and Fleet button (FLOTTE).
func (g *Game) UpdateHUDInput() {
	// Update Fleet UI button animation
	if g.ui.FleetButtonScale > 1.0 {
		g.ui.FleetButtonScale -= 0.05 // Return to normal size
		if g.ui.FleetButtonScale < 1.0 {
			g.ui.FleetButtonScale = 1.0
		}
	}

	// Update Sea button animation
	if g.ui.SeaButtonScale > 1.0 {
		g.ui.SeaButtonScale -= 0.05 // Return to normal size
		if g.ui.SeaButtonScale < 1.0 {
			g.ui.SeaButtonScale = 1.0
		}
	}

	// Update Captain button animation
	if g.ui.CaptainButtonScale > 1.0 {
		g.ui.CaptainButtonScale -= 0.05 // Return to normal size
		if g.ui.CaptainButtonScale < 1.0 {
			g.ui.CaptainButtonScale = 1.0
		}
	}

	// Handle Build Button click (bottom right)
	if g.IsModalOpen() {
		return
	}

	if g.btnBuild != nil {
		w, h := g.screenWidth, g.screenHeight

		// MATCH DRAW LOGIC
		targetSize := 100.0
		// x,y matches DrawBuildButton
		margin := 20.0
		x := float64(w) - targetSize - margin
		y := float64(h) - targetSize - margin

		mx, my := ebiten.CursorPosition()
		cmx, cmy := float64(mx), float64(my)

		if cmx >= x && cmx <= x+targetSize && cmy >= y && cmy <= y+targetSize {
			g.ui.HoverBuildButton = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.Log("Build Button Clicked")
				// Toggle Construction Menu
				g.ui.ShowConstruction = !g.ui.ShowConstruction
				// Close others
				if g.ui.ShowConstruction {
					g.ui.ShowShipyard = false
					g.ui.ShowTechUI = false
					g.ui.ShowFleetUI = false
				}
			}
		} else {
			g.ui.HoverBuildButton = false
		}
	}

	// Handle Sea Button click (positioned to the left of fleet button, bottom right)
	if g.player != nil && len(g.player.Islands) > 0 {
		w, h := g.screenWidth, g.screenHeight
		btnSize := 100.0
		btnGap := 10.0
		margin := 20.0
		buildBtnX := float64(w) - btnSize - margin
		buildBtnY := float64(h) - btnSize - margin

		// Fleet button is to the left of the build button
		fleetBtnX := buildBtnX - btnSize - btnGap
		fleetBtnY := buildBtnY

		// Sea button is to the left of the fleet button
		seaBtnX := fleetBtnX - btnSize - btnGap
		seaBtnY := buildBtnY

		// Captain button is to the left of the sea button
		captainBtnX := seaBtnX - btnSize - btnGap
		captainBtnY := buildBtnY

		// Social button is to the left of the captain button
		socialBtnX := captainBtnX - btnSize - btnGap
		socialBtnY := buildBtnY

		mx, my := ebiten.CursorPosition()
		cmx, cmy := float64(mx), float64(my)

		// Handle Social Button
		if cmx >= socialBtnX && cmx <= socialBtnX+btnSize && cmy >= socialBtnY && cmy <= socialBtnY+btnSize {
			g.ui.HoverSocialButton = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.Log("Social Button Clicked")
				g.ui.SocialButtonScale = 1.05
				g.ui.ShowSocialUI = !g.ui.ShowSocialUI
				if g.ui.ShowSocialUI {
					g.ensureSocialDefaults()
					g.ui.ShowConstruction = false
					g.ui.ShowShipyard = false
					g.ui.ShowTechUI = false
					g.ui.ShowFleetUI = false
					g.ui.ShowCaptainUI = false
				}
			}
		} else {
			g.ui.HoverSocialButton = false
		}

		// Handle Captain Button
		if cmx >= captainBtnX && cmx <= captainBtnX+btnSize && cmy >= captainBtnY && cmy <= captainBtnY+btnSize {
			g.ui.HoverCaptainButton = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.Log("Captain Button Clicked")
				// Scale animation
				g.ui.CaptainButtonScale = 1.05
				// Toggle Captain UI
				g.ui.ShowCaptainUI = !g.ui.ShowCaptainUI
				// Close other UIs
				if g.ui.ShowCaptainUI {
					g.ui.ShowConstruction = false
					g.ui.ShowShipyard = false
					g.ui.ShowTechUI = false
					g.ui.ShowFleetUI = false
					// Load captains if not already loaded or refresh
					g.loadCaptainsForUI()
				}
			}
		} else {
			g.ui.HoverCaptainButton = false
		}

		// Handle Sea Button
		if cmx >= seaBtnX && cmx <= seaBtnX+btnSize && cmy >= seaBtnY && cmy <= seaBtnY+btnSize {
			g.ui.HoverSeaButton = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.Log("[UI_NAV] click tab=SEA prev_state=%v", g.state)
				// Scale animation
				g.ui.SeaButtonScale = 1.05
				// Switch to World Map state
				if g.state == StateWorldMap {
					g.Log("[STATE] Returning to Island")
					g.state = StatePlaying
				} else {
					g.Log("[STATE] Switching to Sea/WorldMap")
					g.state = StateWorldMap
				}
				// Close all UIs when switching to/from World Map
				g.ui.ShowConstruction = false
				g.ui.ShowShipyard = false
				g.ui.ShowTechUI = false
				g.ui.ShowFleetUI = false
			}
		} else {
			g.ui.HoverSeaButton = false
		}

		// Handle Fleet Button
		if cmx >= fleetBtnX && cmx <= fleetBtnX+btnSize && cmy >= fleetBtnY && cmy <= fleetBtnY+btnSize {
			g.ui.HoverFleetButton = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.Log("[UI_NAV] click tab=FLEET")
				// Scale animation
				g.ui.FleetButtonScale = 1.05
				// Toggle Fleet UI
				g.ui.ShowFleetUI = !g.ui.ShowFleetUI
				// Close other UIs
				if g.ui.ShowFleetUI {
					g.ui.ShowConstruction = false
					g.ui.ShowShipyard = false
					g.ui.ShowTechUI = false
				}
			}
		} else {
			g.ui.HoverFleetButton = false
		}
	}
}

// IsHoveringUI checks if the cursor is over any UI overlay.

// IsHoveringUI checks if the cursor is over any UI overlay.
// Used to block world interactions (e.g. camera zoom) when UIs are open.
func (g *Game) IsHoveringUI() bool {
	return g.ui.ShowConstruction || g.ui.ShowShipyard || g.ui.ShowTechUI || g.ui.ShowFleetUI || g.ui.ShowSocialUI
}

// IsModalOpen checks if any modal UI is currently open.
// This is the single source of truth for "Context Blocking".
func (g *Game) IsModalOpen() bool {
	return g.isMenuOpen ||
		g.ui.ShowConstruction ||
		g.ui.ShowShipyard ||
		g.ui.ShowFleetUI ||
		g.ui.ShowTechUI ||
		g.ui.ShowSocialUI ||
		(g.techUI != nil && g.techUI.Visible) ||
		g.ui.ShowTavernUI ||
		g.ui.ShowMilitiaUI ||
		g.ui.ShowInfirmaryUI ||
		g.ui.ShowCaptainUI ||
		g.ui.ShowCrewModal ||
		g.ui.ShowCaptainModal || // Added this one explicitly as it was missing in original list but is a modal
		g.ui.ShowPveResultUI ||
		g.ui.ShowStationMenu ||
		g.ui.ShowPvpUI ||
		g.ui.ShowPvpFleetSelect ||
		g.ui.ShowPvpConfirm ||
		g.ui.ShowPvpResultUI ||
		g.ui.ShowPrereqModal || // Added Prereq Modal
		g.ui.ShowPveEngageError || // Added Error Modal
		g.ui.ShowNavFreeConfirm || // Free Navigation Modal
		g.ui.LocalizationMode ||
		g.showDevMenu ||
		g.selectedBuilding != nil ||
		g.showError
}

// IsHoveringAnyButton checks if the cursor is over any HUD or map button.
func (g *Game) IsHoveringAnyButton() bool {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	mx, my := ebiten.CursorPosition()
	cmx, cmy := float64(mx), float64(my)

	// 1. HUD Buttons (Bottom Right)
	btnSize := 100.0
	btnGap := 10.0
	margin := 20.0
	buildBtnX := w - btnSize - margin
	buildBtnY := h - btnSize - margin

	// Social button is the leftmost in the bottom row
	socialBtnX := buildBtnX - (btnSize+btnGap)*4

	if cmx >= socialBtnX && cmx <= w && cmy >= buildBtnY && cmy <= h {
		return true
	}

	// 2. PvP Button (World Map only, usually bottom right cluster but higher)
	if g.state == StateWorldMap {
		pvpBtnX := w - 220
		pvpBtnY := h - 200
		pvpBtnW := 200.0
		pvpBtnH := 50.0
		if cmx >= pvpBtnX && cmx <= pvpBtnX+pvpBtnW && cmy >= pvpBtnY && cmy <= pvpBtnY+pvpBtnH {
			return true
		}

		// 3. Return to Island Button (Top Right cluster)
		retBtnX := w - 110
		retBtnY := 200.0
		retBtnW := 100.0
		retBtnH := 30.0
		if cmx >= retBtnX && cmx <= retBtnX+retBtnW && cmy >= retBtnY && cmy <= retBtnY+retBtnH {
			return true
		}
	}

	return false
}

// ShouldBlockWorldInteraction checks if world interactions should be blocked.
func (g *Game) ShouldBlockWorldInteraction() bool {
	blocked := g.IsModalOpen()
	// Debug log only when a click is actually blocked (logged once per click in state_router.go)
	return blocked
}

// GetActiveModalName returns the name of the currently open modal, or empty string.
func (g *Game) GetActiveModalName() string {
	if g.isMenuOpen {
		return "Menu"
	}
	if g.ui.ShowConstruction {
		return "Construction"
	}
	if g.ui.ShowShipyard {
		return "Shipyard"
	}
	if g.ui.ShowFleetUI {
		return "FleetUI"
	}
	if g.ui.ShowTechUI || (g.techUI != nil && g.techUI.Visible) {
		return "TechUI"
	}
	if g.ui.ShowTavernUI {
		return "Tavern"
	}
	if g.ui.ShowMilitiaUI {
		return "Militia"
	}
	if g.ui.ShowInfirmaryUI {
		return "Infirmary"
	}
	if g.ui.ShowCaptainUI {
		return "CaptainUI"
	}
	if g.ui.ShowCrewModal {
		return "CrewAssignment"
	}
	if g.ui.ShowCaptainModal {
		return "CaptainSelection"
	}
	if g.ui.ShowPveResultUI {
		return "PveResult"
	}
	if g.ui.ShowStationMenu {
		return "StationMenu"
	}
	if g.ui.ShowPvpUI {
		return "PvpUI"
	}
	if g.ui.ShowPvpFleetSelect {
		return "PvpFleetSelect"
	}
	if g.ui.ShowPvpConfirm {
		return "PvpConfirm"
	}
	if g.ui.ShowPvpResultUI {
		return "PvpResult"
	}
	if g.ui.ShowPrereqModal {
		return "Prerequisites"
	}
	if g.ui.ShowPveEngageError {
		return "PveError"
	}
	if g.ui.ShowNavFreeConfirm {
		return "NavFreeConfirm"
	}
	if g.ui.LocalizationMode {
		return "Localization"
	}
	if g.showDevMenu {
		return "DevMenu"
	}
	if g.selectedBuilding != nil {
		return "BuildingContext"
	}
	if g.showError {
		return "ErrorModal"
	}
	return ""
}
