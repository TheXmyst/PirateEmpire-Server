package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawPvpButton renders the PvP button on the world map
func (g *Game) DrawPvpButton(screen *ebiten.Image) {
	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())

	// Position: Right side, below Fleet button
	btnX := w - 220
	btnY := h - 200
	btnW := 200.0
	btnH := 50.0

	// Check hover
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)
	hoverPvp := fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH

	// Button color (red theme for PvP)
	btnColor := color.RGBA{180, 30, 30, 255}
	if hoverPvp {
		btnColor = color.RGBA{220, 50, 50, 255}
	}

	// Draw button
	drawButton(screen, float64(btnX), float64(btnY), float64(btnW), float64(btnH), "⚔️ PvP", kindFromColor(btnColor))

	// Button text

	// Handle click
	if hoverPvp && isMousePressed(ebiten.MouseButtonLeft) {
		if !g.ui.ShowPvpUI {
			g.ui.ShowPvpUI = true
			// Load PvP targets if not already loaded
			if len(g.ui.PvpTargets) == 0 && !g.ui.PvpTargetsBusy {
				g.loadPvpTargets()
			}
		}
	}
}

// loadPvpTargets fetches PvP targets from server
func (g *Game) loadPvpTargets() {
	if g.ui.PvpTargetsBusy {
		return
	}
	g.ui.PvpTargetsBusy = true
	g.ui.PvpTargetsError = ""

	go func() {
		defer func() { g.ui.PvpTargetsBusy = false }()

		targets, err := g.api.GetPvpTargets()
		if err != nil {
			g.ui.PvpTargetsError = err.Error()
			return
		}

		g.ui.PvpTargets = targets

		// Phase A1: Diagnostic Log
		g.Log("[PVP_TARGETS] count=%d", len(targets))
		for _, t := range targets {
			g.Log("  - id=%s owner=\"%s\" th=%d x=%.0f y=%.0f", t.ID, t.Name, t.Tier, t.X, t.Y)
		}
	}()
}

// DrawPvpTargetList renders the PvP target selection modal
func (g *Game) DrawPvpTargetList(screen *ebiten.Image) {
	if !g.ui.ShowPvpUI {
		return
	}

	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())

	// Modal background
	modalW := 600.0
	modalH := 500.0
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2

	// Semi-transparent overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 180}, vecAA)

	// Modal box
	drawButton(screen, float64(modalX), float64(modalY), float64(modalW), float64(modalH), "Cibles PvP Disponibles", kindFromColor(color.RGBA{40, 40, 60, 255}))

	// Title

	// Close button
	closeX := modalX + modalW - 40
	closeY := modalY + 10
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)
	hoverClose := fmx >= closeX && fmx <= closeX+30 && fmy >= closeY && fmy <= closeY+30

	drawCloseButton(screen, float64(closeX)+float64(30)/2, float64(closeY)+float64(30)/2, 15)

	if hoverClose && isMousePressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpUI = false
		return
	}

	// Loading state
	if g.ui.PvpTargetsBusy {
		uiPrint(screen, "Chargement...", int(modalX)+250, int(modalY)+250)
		return
	}

	// Error state
	if g.ui.PvpTargetsError != "" {
		uiPrint(screen, "Erreur: "+g.ui.PvpTargetsError, int(modalX)+20, int(modalY)+60)
		return
	}

	// Target list
	if len(g.ui.PvpTargets) == 0 {
		uiPrint(screen, "Aucune cible disponible", int(modalX)+200, int(modalY)+250)
		return
	}

	// Render targets
	yOffset := modalY + 60
	for i, target := range g.ui.PvpTargets {
		if yOffset > modalY+modalH-80 {
			break // Don't overflow modal
		}

		// Target box
		boxY := yOffset + float64(i*90)
		boxH := 80.0

		// Check if protected
		isProtected := false
		// Note: Protection info would need to be added to PveTarget struct
		// For now, assume not protected

		// Hover check
		hoverTarget := fmx >= modalX+20 && fmx <= modalX+modalW-20 &&
			fmy >= boxY && fmy <= boxY+boxH

		boxColor := color.RGBA{60, 60, 80, 255}
		if hoverTarget {
			boxColor = color.RGBA{80, 80, 100, 255}
		}

		uiFillRect(screen, float32(modalX+20), float32(boxY), float32(modalW-40), float32(boxH), boxColor, vecAA)
		uiStrokeRect(screen, float32(modalX+20), float32(boxY), float32(modalW-40), float32(boxH), 1, color.White, vecAA)

		// Target info
		uiPrint(screen, fmt.Sprintf("Joueur: %s", target.Name), int(modalX+30), int(boxY+10))
		uiPrint(screen, fmt.Sprintf("Niveau: Tier %d", target.Tier), int(modalX+30), int(boxY+30))

		// Attack button or protected status
		btnX := modalX + modalW - 160
		btnY := boxY + 25
		btnW := 120.0
		btnH := 30.0

		if isProtected {
			drawButton(screen, float64(btnX), float64(btnY), float64(btnW), float64(btnH), "🛡️ protégé", kindFromColor(color.RGBA{100, 100, 100, 255}))
		} else {
			hoverAttack := fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH

			attackColor := color.RGBA{180, 30, 30, 255}
			if hoverAttack {
				attackColor = color.RGBA{220, 50, 50, 255}
			}

			drawButton(screen, float64(btnX), float64(btnY), float64(btnW), float64(btnH), "Attaquer", kindFromColor(attackColor))

			if hoverAttack && isMousePressed(ebiten.MouseButtonLeft) {
				// Select this target and show fleet selection
				g.ui.PvpSelectedTarget = &g.ui.PvpTargets[i]
				g.ui.ShowPvpFleetSelect = true
			}
		}
	}
}

// DrawPvpFleetSelection renders the fleet selection dialog
func (g *Game) DrawPvpFleetSelection(screen *ebiten.Image) {
	if !g.ui.ShowPvpFleetSelect || g.ui.PvpSelectedTarget == nil {
		return
	}

	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())

	// Modal
	modalW := 500.0
	modalH := 400.0
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2

	// Overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, vecAA)

	// Modal box
	drawButton(screen, float64(modalX), float64(modalY), float64(modalW), float64(modalH), "Sélectionner une Flotte", kindFromColor(color.RGBA{40, 40, 60, 255}))

	// Title
	uiPrint(screen, fmt.Sprintf("Cible: %s", g.ui.PvpSelectedTarget.Name), int(modalX)+20, int(modalY)+45)

	// Close button
	closeX := modalX + modalW - 40
	closeY := modalY + 10
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)
	hoverClose := fmx >= closeX && fmx <= closeX+30 && fmy >= closeY && fmy <= closeY+30

	drawCloseButton(screen, float64(closeX)+float64(30)/2, float64(closeY)+float64(30)/2, 15)

	if hoverClose && isMousePressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpFleetSelect = false
		g.ui.PvpSelectedFleetID = ""
		return
	}

	// Fleet list
	if g.player == nil || len(g.player.Islands) == 0 {
		uiPrint(screen, "Aucune flotte disponible", int(modalX)+150, int(modalY)+200)
		return
	}

	yOffset := modalY + 80
	for i, fleet := range g.player.Islands[0].Fleets {
		boxY := yOffset + float64(i*70)
		boxH := 60.0

		// Check if fleet is available (Note: LockedUntil not in client struct yet)
		isLocked := false // TODO: Add LockedUntil to client Fleet struct
		isAway := fleet.State == "Moving" || fleet.State == "Returning" || fleet.State == "Stationed"

		hoverFleet := fmx >= modalX+20 && fmx <= modalX+modalW-20 &&
			fmy >= boxY && fmy <= boxY+boxH

		boxColor := color.RGBA{60, 60, 80, 255}
		if hoverFleet && !isLocked && !isAway {
			boxColor = color.RGBA{80, 80, 100, 255}
		}

		uiFillRect(screen, float32(modalX+20), float32(boxY), float32(modalW-40), float32(boxH), boxColor, vecAA)
		uiStrokeRect(screen, float32(modalX+20), float32(boxY), float32(modalW-40), float32(boxH), 1, color.White, vecAA)

		// Fleet info
		uiPrint(screen, fmt.Sprintf("○ %s (%d navires)", fleet.Name, len(fleet.Ships)), int(modalX+30), int(boxY+10))

		status := "Disponible"
		if isLocked {
			status = "Verrouillée"
		} else if isAway {
			status = "En mission"
		}
		uiPrint(screen, fmt.Sprintf("   État: %s", status), int(modalX+30), int(boxY+30))

		// Click to select
		if hoverFleet && !isLocked && !isAway && isMousePressed(ebiten.MouseButtonLeft) {
			g.ui.PvpSelectedFleetID = fleet.ID.String()
			g.ui.ShowPvpConfirm = true
		}
	}

	// Buttons
	btnY := modalY + modalH - 60
	cancelX := modalX + 50

	// Cancel button
	hoverCancel := fmx >= cancelX && fmx <= cancelX+120 && fmy >= btnY && fmy <= btnY+40
	cancelColor := color.RGBA{100, 100, 100, 255}
	if hoverCancel {
		cancelColor = color.RGBA{130, 130, 130, 255}
	}
	drawButton(screen, float64(cancelX), float64(btnY), 120, 40, "Annuler", kindFromColor(cancelColor))

	if hoverCancel && isMousePressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpFleetSelect = false
		g.ui.PvpSelectedFleetID = ""
	}
}

// DrawPvpConfirmation renders the final attack confirmation dialog
func (g *Game) DrawPvpConfirmation(screen *ebiten.Image) {
	if !g.ui.ShowPvpConfirm || g.ui.PvpSelectedTarget == nil || g.ui.PvpSelectedFleetID == "" {
		return
	}

	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())

	// Modal
	modalW := 450.0
	modalH := 300.0
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2

	// Overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 220}, vecAA)

	// Modal box
	drawButton(screen, float64(modalX), float64(modalY), float64(modalW), float64(modalH), "⚠️  confirmer l'attaque", kindFromColor(color.RGBA{50, 30, 30, 255}))

	// Warning title

	// Info
	uiPrint(screen, fmt.Sprintf("Cible: %s", g.ui.PvpSelectedTarget.Name), int(modalX)+30, int(modalY)+60)

	// Find fleet name
	fleetName := "Flotte"
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, f := range g.player.Islands[0].Fleets {
			if f.ID.String() == g.ui.PvpSelectedFleetID {
				fleetName = f.Name
				break
			}
		}
	}
	uiPrint(screen, fmt.Sprintf("Flotte: %s", fleetName), int(modalX)+30, int(modalY)+85)

	// Warnings
	uiPrint(screen, "• Les navires détruits sont perdus", int(modalX)+30, int(modalY)+120)
	uiPrint(screen, "  définitivement", int(modalX)+30, int(modalY)+135)
	uiPrint(screen, "• Votre flotte sera verrouillée", int(modalX)+30, int(modalY)+155)
	uiPrint(screen, "  pendant 5-10 minutes", int(modalX)+30, int(modalY)+170)

	// Buttons
	btnY := modalY + modalH - 60
	cancelX := modalX + 50
	attackX := modalX + modalW - 180

	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Cancel
	hoverCancel := fmx >= cancelX && fmx <= cancelX+120 && fmy >= btnY && fmy <= btnY+40
	cancelColor := color.RGBA{100, 100, 100, 255}
	if hoverCancel {
		cancelColor = color.RGBA{130, 130, 130, 255}
	}
	drawButton(screen, float64(cancelX), float64(btnY), 120, 40, "Annuler", kindFromColor(cancelColor))

	if hoverCancel && isMousePressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpConfirm = false
	}

	// Attack
	hoverAttack := fmx >= attackX && fmx <= attackX+120 && fmy >= btnY && fmy <= btnY+40
	attackColor := color.RGBA{180, 30, 30, 255}
	if hoverAttack {
		attackColor = color.RGBA{220, 50, 50, 255}
	}
	drawButton(screen, float64(attackX), float64(btnY), 120, 40, "Attaquer", kindFromColor(attackColor))

	if hoverAttack && isMouseJustPressed(ebiten.MouseButtonLeft) && !g.ui.PvpAttackBusy {
		g.Log("[PVP UI] Confirmation button clicked!")
		g.executePvpAttack()
	}
}

// executePvpAttack calls the API to attack the selected target
func (g *Game) executePvpAttack() {
	if g.ui.PvpAttackBusy {
		return
	}

	g.ui.PvpAttackBusy = true
	g.ui.PvpAttackError = ""

	targetID := g.ui.PvpSelectedTarget.ID
	fleetID := g.ui.PvpSelectedFleetID

	go func() {
		defer func() { g.ui.PvpAttackBusy = false }()

		// Send fleet to attack (travel system)
		travelTime, distance, err := g.api.SendPvpAttack(fleetID, targetID)
		if err != nil {
			g.ui.PvpAttackError = err.Error()
			return
		}

		// Close dialogs and show travel confirmation
		g.ui.ShowPvpUI = false
		g.ui.ShowPvpFleetSelect = false
		g.ui.ShowPvpConfirm = false

		// Show success message (fleet is now traveling)
		g.ui.PvpAttackError = fmt.Sprintf("Flotte en route ! Distance: %.0f, Temps: %.1f min", distance, travelTime)

		// Reload player data to show fleet state change
		g.loadPlayerData()
	}()
}
