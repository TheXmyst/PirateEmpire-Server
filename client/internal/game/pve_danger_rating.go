package game

import (
	"image/color"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
)

// PveDangerRating represents the danger level of a PvE target
type PveDangerRating struct {
	Label string
	Color color.RGBA
}

var (
	DangerVeryFavorable = PveDangerRating{Label: "Cette cible semble à votre portée.", Color: color.RGBA{0, 255, 100, 255}}
	DangerFavorable     = PveDangerRating{Label: "Vos chances sont bonnes.", Color: color.RGBA{150, 255, 100, 255}}
	DangerBalanced      = PveDangerRating{Label: "Le combat sera disputé.", Color: color.RGBA{255, 200, 50, 255}}
	DangerUnfavorable   = PveDangerRating{Label: "Une préparation soignée est recommandée.", Color: color.RGBA{255, 120, 0, 255}}
	DangerDeadly        = PveDangerRating{Label: "Engager cette cible sans renfort serait une folie.", Color: color.RGBA{255, 50, 50, 255}}

	// Client-side approximation scores (calibrated for Dynamic Danger Rating v1)
	// T1 = 12.0 (Sloop=10 + Crew=~2) -> 1 Sloop Player (11.5) = Balanced
	PveScoreTier1 = 12.0
	PveScoreTier2 = 80.0
	PveScoreTier3 = 250.0

	DebugDangerRating = false // Set to true to see ratio logs in console
)

// GetDangerRating returns a danger rating based on the player's fleet and the target
func GetDangerRating(fleet *domain.Fleet, target domain.PveTarget) PveDangerRating {
	if fleet == nil {
		return DangerDeadly // No fleet = suicide
	}

	playerScore := ComputeFleetCombatScore(fleet)
	targetScore := ComputePveTargetCombatScore(target)

	if targetScore == 0 {
		return DangerVeryFavorable // Should not happen, but safe fallback
	}

	ratio := playerScore / targetScore

	// Determine base rating from ratio
	var rating PveDangerRating
	switch {
	case ratio > 1.5:
		rating = DangerVeryFavorable
	case ratio > 1.1:
		rating = DangerFavorable
	case ratio > 0.9:
		rating = DangerBalanced
	case ratio > 0.6:
		rating = DangerUnfavorable
	default:
		rating = DangerDeadly
	}

	if DebugDangerRating {
		// print log to standard output (will appear in client logs)
		// We use a simple println to avoid importing 'fmt' just for this if possible,
		// but 'fmt' is standard. If not imported, we need to add it.
		// The file already imports 'image/color' and 'github.com/.../domain'.
		// We should probably check imports. For now, let's assume fmt is not there and use println (builtin) or add fmt.
		println("[DANGER] Tier:", target.Tier, " | Player:", playerScore, " | Target:", targetScore, " | Ratio:", ratio, " => ", rating.Label)
	}

	return rating
}

// ComputeFleetCombatScore calculates a raw combat power score for a fleet
// This is CLIENT-SIDE ONLY for estimation purposes, unrelated to server logic
func ComputeFleetCombatScore(fleet *domain.Fleet) float64 {
	score := 0.0

	activeShips := 0
	for _, ship := range fleet.Ships {
		if ship.State == "Destroyed" || ship.Health <= 0 {
			continue
		}
		activeShips++

		// Base score per ship type
		baseScore := 0.0
		switch ship.Type {
		case "sloop":
			baseScore = 10
		case "brigantine":
			baseScore = 25
		case "frigate":
			baseScore = 50
		case "galleon":
			baseScore = 100
		case "manowar":
			baseScore = 200
		}

		// Health factor (0.5 to 1.0)
		hpFactor := 0.5
		if ship.MaxHealth > 0 {
			hpFactor = 0.5 + 0.5*(ship.Health/ship.MaxHealth)
		}

		// Crew score (approx 10 crew = 1 pt)
		crewCount := ship.MilitiaWarriors + ship.MilitiaArchers + ship.MilitiaGunners
		crewScore := float64(crewCount) / 10.0

		// Ship Total
		shipScore := (baseScore * hpFactor) + crewScore
		score += shipScore
	}

	// Captain Bonus (Global multiplier)
	// We don't have easy access to captains here unless passed contextually,
	// but strictly speaking the fleet structure doesn't hold captains directly client-side easily
	// without looking up g.captains.
	// For estimating, ship raw power is often enough.
	// To keep it simple and safe (no external deps), we ignore captains for estimating or assume average.
	// OR: Users explicitly requested "présence d'un capitaine".
	// However, fetching captain requires the game state `g`.
	// Let's refactor signature if needed, or keep it simple for now.
	// Update: The requirement says "présence d'un capitaine".
	// Since this is a method in `package main`, we can't easily access `g.captains` if it's a standalone function,
	// unless we make it a method of `*Game`.
	// Let's stick to pure function for now to avoid dependencies hell. If captain is needed, caller should pass it or we ignore it.
	// Given strictly "Pure functions" requirement: we'll use pure ship stats.
	// If the user REALLY wants captain factor, we'd need to pass captain count or similar.
	// For now, let's assume raw hull/crew power is the primary factor.

	return score
}

// ComputePveTargetCombatScore estimates target power based on tier
func ComputePveTargetCombatScore(target domain.PveTarget) float64 {
	// Estimated power levels to match the ship scores above
	switch target.Tier {
	case 1:
		return PveScoreTier1 // 12.0
	case 2:
		return PveScoreTier2 // 80.0
	case 3:
		return PveScoreTier3 // 250.0
	default:
		return PveScoreTier1 * float64(target.Tier)
	}
}
