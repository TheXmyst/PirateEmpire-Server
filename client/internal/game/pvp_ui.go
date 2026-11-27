package game

import "fmt"

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
			g.ui.PvpTargetsError = friendlyError(err)
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
			g.ui.PvpAttackError = friendlyError(err)
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
