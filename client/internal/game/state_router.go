package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// state_router.go contains state-based routing logic for Update() and Draw().
// This file was extracted from main.go during Phase 1.4 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// updateByState routes Update() calls to state-specific update functions.
// Returns early if a state-specific handler returns an error or early termination.
func (g *Game) updateByState() error {
	// Always Update DMS for fades
	if g.seaDMS != nil {
		g.seaDMS.Update(g.dt)

		// Automate DMS Mode based on SSOT Combat State
		if g.state == StateWorldMap {
			if g.IsCombatActive() {
				g.seaDMS.SetMode(DMSCombat, "ssot_combat_active")
			} else {
				g.seaDMS.SetMode(DMSCalm, "ssot_combat_inactive")
			}
		}
	}

	// CRITICAL: Prerequisites Modal, PVE Result UI and Error Modal must be checked FIRST in ALL states (except Login)
	// This ensures the modals block all world interactions regardless of state
	if g.state != StateLogin {
		if g.UpdatePrereqModal() {
			return nil // Prerequisites modal consumed input, block everything else
		}
		if g.UpdatePveEngageErrorModal() {
			return nil // Error modal consumed input, block everything else
		}
		// Update PvE result UI but don't block game updates
		// This allows ships to continue moving while viewing combat results
		g.UpdatePveResultUI()
	}

	if g.state == StateLogin {
		if g.audioManager != nil {
			g.audioManager.PlayMusic("title")
		}
		return g.UpdateLogin()
	}

	// Toggle Dev Menu with F1 (now allowed for any logged-in player)
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		if g.player != nil {
			g.showDevMenu = !g.showDevMenu
		}
	}

	// Play island music during gameplay (NOT during world map if using DMS)
	if g.state == StatePlaying && g.audioManager != nil {
		g.audioManager.PlayMusic("island")
	}

	// Camera Pan (only for Playing and WorldMap states)
	if g.state == StatePlaying || g.state == StateWorldMap {
		mx, my := ebiten.CursorPosition()
		// Only allow panning if NO modal is open
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) && !g.ShouldBlockWorldInteraction() {
			dx := mx - g.lastMouseX
			dy := my - g.lastMouseY

			g.camX -= float64(dx) / g.camZoom
			g.camY -= float64(dy) / g.camZoom
		}

		// Zoom (only if NOT blocking interaction)
		_, wy := ebiten.Wheel()
		if wy != 0 && !g.ShouldBlockWorldInteraction() {
			oldZoom := g.camZoom
			if wy > 0 {
				g.camZoom *= 1.1
			} else {
				g.camZoom /= 1.1
			}
			if g.camZoom < 0.5 {
				g.camZoom = 0.5
			}
			if g.camZoom > 3.0 {
				g.camZoom = 3.0
			}

			// Adjust Pos to zoom towards center? Simple logic for now.
			// Ideally zoom towards mouse.
			// g.camX += (float64(mx) / oldZoom) - (float64(mx) / g.camZoom) ...
			_ = oldZoom
		}
		g.lastMouseX, g.lastMouseY = mx, my

		// Clamp Camera to Image Bounds
		if g.bgImage != nil {
			bounds := g.bgImage.Bounds()
			w, h := float64(bounds.Dx()), float64(bounds.Dy())

			// Screen dimensions in world space
			screenVisW := float64(g.screenWidth) / g.camZoom
			screenVisH := float64(g.screenHeight) / g.camZoom

			maxX := w/2 - screenVisW/2
			maxY := h/2 - screenVisH/2
			if maxX < 0 {
				maxX = 0
			}
			if maxY < 0 {
				maxY = 0
			}

			if g.camX < -maxX {
				g.camX = -maxX
			}
			if g.camX > maxX {
				g.camX = maxX
			}
			if g.camY < -maxY {
				g.camY = -maxY
			}
			if g.camY > maxY {
				g.camY = maxY
			}
		}
	}

	if g.state != StateWorldMap && g.seaDMS != nil {
		g.seaDMS.Stop()
	}

	// StatePlaying-specific logic
	if g.state == StatePlaying {
		// Modal Logic - returns true if consumed
		if g.UpdateBuildingModal() {
			return nil
		}

		// CRITICAL: Update crew assignment modal EVERY FRAME when open (not just on click)
		// This is required for continuous drag functionality
		if g.UpdateCrewAssignmentModal() {
			return nil
		}

		// CRITICAL: Update militia modal EVERY FRAME when open (not just on click)
		// This is required for continuous drag functionality
		if g.UpdateMilitiaUI() {
			return nil
		}

		// Update Infirmary UI
		if g.UpdateInfirmaryUI() {
			return nil
		}

		// Sync UI State
		if g.techUI != nil {
			g.ui.ShowTechUI = g.techUI.Visible
		}

		// Check if UI overlays consume the click first
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Update UI overlays that can consume clicks
			if g.UpdateTavernUI() {
				// Tavern UI consumed the click
				return nil
			}
			if g.UpdateCaptainUI() {
				// Captain UI consumed the click
				return nil
			}
			// UpdateBuildingModal is already checked above
			// Other UI updates (construction, fleet, etc.) handle their own clicks internally

			// Only handle world/building clicks if no UI overlay is blocking
			if !g.ShouldBlockWorldInteraction() {
				g.handleBuildingClick()
			} else {
				// Debug log when click is blocked (only once per click)
				g.Log("[INPUT] World click blocked by UI overlay: construction=%v shipyard=%v fleet=%v tech=%v tavern=%v localization=%v devmenu=%v building=%v error=%v",
					g.ui.ShowConstruction,
					g.ui.ShowShipyard,
					g.ui.ShowFleetUI,
					g.ui.ShowTechUI || (g.techUI != nil && g.techUI.Visible),
					g.ui.ShowTavernUI,
					g.ui.LocalizationMode,
					g.showDevMenu,
					g.selectedBuilding != nil,
					g.showError)
			}
		}
	}

	// StateWorldMap-specific logic
	if g.state == StateWorldMap {
		if g.audioManager != nil && g.audioManager.currentTrack == "island" {
			g.audioManager.StopAll()
		}
		if g.seaDMS != nil {
			g.seaDMS.Start()
		}
		// UpdatePveResultUI already checked above

		// SYSTEMIC: Update Global Modals (Fleet, Captain, Crew)
		// This ensures they handle inputs (Scroll, Close, Clicks) even in Sea View
		if g.UpdateCrewAssignmentModal() {
			return nil
		}
		g.UpdateFleetUI() // Handles its own "Close on ESC" check internally
		if g.UpdateCaptainUI() {
			return nil
		}

		// Always Update World Map Logic (Simulation, Interpolation, etc.)
		g.UpdateWorldMapLogic()

		// Only Update World Map Interaction if no external modal is blocking.
		// StationMenu is handled internally by UpdateWorldMapInteraction, so we allow it.
		activeModal := g.GetActiveModalName()
		if activeModal == "" || activeModal == "StationMenu" {
			g.UpdateWorldMapInteraction()
		} else {
			// Log input capture (throttled)
			if g.hammerFrame == 0 {
				// g.Log("[UI_LAYER] modal=%s capture_input=true", activeModal)
			}
		}
	}

	return nil
}

// drawByState routes Draw() calls to state-specific draw functions.
// Returns early if a state-specific handler handles all drawing.
func (g *Game) drawByState(screen *ebiten.Image) {
	// CRITICAL: Prerequisites Modal, PVE Result UI and Error Modal must be drawn in ALL states (except Login)
	// We handle them in specific state blocks to ensure correct Z-order (top layer)

	if g.state == StateLogin {
		g.DrawLogin(screen)
		return
	}
	if g.state == StateWorldMap {
		g.DrawWorldMap(screen)
		// Draw Modals AFTER WorldMap to ensure they are on top
		g.DrawPveResultUI(screen)
		g.DrawPveEngageErrorModal(screen)
		g.DrawPrereqModal(screen)
		return
	}

	// StatePlaying drawing continues below (implicit state)
	// All remaining drawing code in Draw() is for StatePlaying
	// DrawPveResultUI already called above
}
