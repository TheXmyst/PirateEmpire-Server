package economy

import (
	"fmt"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// ValidateFleetForCombat validates that a fleet is ready for combat
// Returns: (isValid, reasonCode, reasonMessage)
func ValidateFleetForCombat(fleet *domain.Fleet) (bool, string, string) {
	// Check if fleet is locked
	if IsFleetLocked(fleet) {
		remaining := fleet.LockedUntil.Sub(time.Now())
		secs := int(remaining.Seconds())

		var timeMsg string
		if secs >= 60 {
			timeMsg = fmt.Sprintf("%dm%ds", secs/60, secs%60)
		} else {
			timeMsg = fmt.Sprintf("%ds", secs)
		}

		return false, "FLEET_LOCKED", fmt.Sprintf("Flotte verrouillée (%s restantes)", timeMsg)
	}

	// Check if fleet has ships
	if len(fleet.Ships) == 0 {
		return false, "FLEET_INVALID_NO_SHIPS", "Flotte vide"
	}

	// Check if all ships are destroyed
	hasActiveShip := false
	for _, ship := range fleet.Ships {
		if ship.State != "Destroyed" && ship.Health > 0 {
			hasActiveShip = true
			break
		}
	}
	if !hasActiveShip {
		return false, "FLEET_INVALID_ALL_DESTROYED", "Tous les navires de la flotte sont détruits"
	}

	// Check if flagship exists and is active
	flagship, _, _ := SelectFlagshipShip(fleet)
	if flagship == nil {
		return false, "FLEET_INVALID_NO_FLAGSHIP", "Aucun navire amiral sélectionné"
	}
	if flagship.State == "Destroyed" || flagship.Health <= 0 {
		return false, "FLEET_INVALID_NO_FLAGSHIP", "Le navire amiral est détruit"
	}

	// Check if flagship captain is injured (if assigned)
	if flagship.CaptainID != nil {
		// Note: We can't check captain injury here without loading the captain
		// This will be checked in the handler after loading the captain
		// For now, we return true and let the handler check
	}

	// Validate crew for all active ships
	for _, ship := range fleet.Ships {
		// Only validate active ships (not destroyed)
		if ship.State != "Destroyed" && ship.Health > 0 {
			isValid, reasonCode, reasonMsg := ValidateShipCrewBounds(&ship)
			if !isValid {
				return false, reasonCode, reasonMsg
			}
		}
	}

	return true, "", ""
}

// ValidateFleetCaptainForCombat checks if the flagship captain is ready for combat
// Returns: (isValid, reasonCode, reasonMessage)
func ValidateFleetCaptainForCombat(captain *domain.Captain) (bool, string, string) {
	if captain == nil {
		// No captain is OK (optional for PVE v1)
		return true, "", ""
	}

	// Check if captain is injured
	if captain.InjuredUntil != nil && time.Now().Before(*captain.InjuredUntil) {
		remaining := captain.InjuredUntil.Sub(time.Now())
		hours := int(remaining.Hours())
		minutes := int(remaining.Minutes()) % 60

		var timeMsg string
		if hours > 0 {
			timeMsg = fmt.Sprintf("%dh%dm", hours, minutes)
		} else {
			timeMsg = fmt.Sprintf("%dm", minutes)
		}

		return false, "CAPTAIN_INJURED", fmt.Sprintf("Capitaine blessé (%s restantes)", timeMsg)
	}

	return true, "", ""
}
