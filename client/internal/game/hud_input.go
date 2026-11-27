package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// hud_input.go contains all HUD input handling and UI routing logic.
// This file was extracted from main.go during Phase 1.3 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// UpdateHUDInput : clics sur la barre du bas (Flotte, Mer, Construire,
// Capitaines, Social). Bloqué tant qu'une fenêtre est ouverte, comme avant.
func (g *Game) UpdateHUDInput() {
	if !g.hudVisible() || g.IsModalOpen() {
		return
	}
	if !isMouseJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := cursorPosition()
	for _, b := range g.dockButtons() {
		if b.r.has(float64(mx), float64(my)) {
			g.onDockButton(b.id)
			return
		}
	}
}

func (g *Game) closeMainPanels() {
	g.ui.ShowConstruction = false
	g.ui.ShowShipyard = false
	g.ui.ShowTechUI = false
	g.ui.ShowFleetUI = false
	g.ui.ShowCaptainUI = false
	g.ui.ShowSocialUI = false
}

func (g *Game) onDockButton(id string) {
	g.Log("[UI_NAV] click tab=%s", id)
	switch id {
	case "build":
		open := !g.ui.ShowConstruction
		g.closeMainPanels()
		g.ui.ShowConstruction = open
	case "fleet":
		open := !g.ui.ShowFleetUI
		g.closeMainPanels()
		g.ui.ShowFleetUI = open
	case "captain":
		open := !g.ui.ShowCaptainUI
		g.closeMainPanels()
		g.ui.ShowCaptainUI = open
		if open {
			g.loadCaptainsForUI()
		}
	case "social":
		open := !g.ui.ShowSocialUI
		g.closeMainPanels()
		g.ui.ShowSocialUI = open
		if open {
			g.ensureSocialDefaults()
		}
	case "sea":
		if g.state == StateWorldMap {
			g.enterIslandView()
		} else {
			g.enterSeaView()
		}
		g.closeMainPanels()
	}
}

// L'île et la mer partagent la même caméra (camX, camY, camZoom) : chaque
// changement de vue la remet à sa place, sinon on retrouverait la position et
// le zoom de l'autre vue.

// enterSeaView passe en mer, la caméra centrée sur l'île du joueur.
func (g *Game) enterSeaView() {
	g.state = StateWorldMap
	g.camZoom = 1
	g.camX, g.camY = 0, 0
	if g.player != nil && len(g.player.Islands) > 0 {
		g.camX = float64(g.player.Islands[0].X)
		g.camY = float64(g.player.Islands[0].Y)
	}
	g.clearSeaNavMarker()
}

// enterIslandView revient sur l'île avec le zoom et la position de départ.
func (g *Game) enterIslandView() {
	g.state = StatePlaying
	g.camZoom = 1
	g.camX, g.camY = 0, 0
	g.clearSeaNavMarker()
}

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
		g.ui.ShowPveConfirmModal ||
		g.ui.LocalizationMode ||
		g.showDevMenu ||
		g.selectedBuilding != nil ||
		g.showError
}

// IsHoveringAnyButton checks if the cursor is over any HUD or map button.
func (g *Game) IsHoveringAnyButton() bool {
	mx, my := cursorPosition()
	cmx, cmy := float64(mx), float64(my)

	if g.hudHit(cmx, cmy) {
		return true
	}

	if g.state == StateWorldMap {
		// Bouton PvP
		if x, y, bw, bh := g.pvpButtonRect(); cmx >= x && cmx <= x+bw && cmy >= y && cmy <= y+bh {
			return true
		}
		// Bouton « Retour au port » (sous la boussole)
		if g.recallButtonVisible() {
			if x, y, bw, bh := g.recallButtonRect(); cmx >= x && cmx <= x+bw && cmy >= y && cmy <= y+bh {
				return true
			}
		}
		// Boussole du vent
		if cx, cy, r := g.windRect(); math.Hypot(cmx-cx, cmy-cy) <= r {
			return true
		}
		// Bouton « Naviguer ici »
		if g.seaNavButtonHit(cmx, cmy) {
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
	if g.ui.ShowPveConfirmModal {
		return "PveConfirm"
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
