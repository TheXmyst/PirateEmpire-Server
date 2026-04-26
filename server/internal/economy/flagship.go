package economy

import (
	"fmt"
	"sort"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// SelectFlagshipShip selects the flagship ship for a fleet deterministically
// Priority:
// 1. If FlagshipShipID is set and ship exists in fleet -> use it
// 2. Fallback: first ship with CaptainID (sorted by ID for determinism)
// 3. Fallback: first ship by ID (sorted for determinism)
// Returns the selected ship and a reason string for logging
func SelectFlagshipShip(fleet *domain.Fleet) (*domain.Ship, bool, string) {
	if len(fleet.Ships) == 0 {
		return nil, false, "no ships in fleet"
	}

	// Sort ships by ID for deterministic fallback
	sort.Slice(fleet.Ships, func(i, j int) bool {
		return fleet.Ships[i].ID.String() < fleet.Ships[j].ID.String()
	})

	// 1. Check explicit FlagshipShipID
	if fleet.FlagshipShipID != nil {
		for i := range fleet.Ships {
			if fleet.Ships[i].ID == *fleet.FlagshipShipID {
				return &fleet.Ships[i], true, fmt.Sprintf("explicit flagship_ship_id=%s", fleet.Ships[i].ID)
			}
		}
		// Explicit ID set but ship not found -> fallback
	}

	// 2. Fallback: first ship with CaptainID
	for i := range fleet.Ships {
		if fleet.Ships[i].CaptainID != nil {
			return &fleet.Ships[i], false, fmt.Sprintf("fallback: first ship with captain (ship_id=%s)", fleet.Ships[i].ID)
		}
	}

	// 3. Fallback: first ship by ID
	return &fleet.Ships[0], false, fmt.Sprintf("fallback: first ship by ID (ship_id=%s)", fleet.Ships[0].ID)
}

