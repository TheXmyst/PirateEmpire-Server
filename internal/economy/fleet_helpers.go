package economy

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// GetActiveFleetShips returns only active ships from a fleet (filters destroyed ships)
// Active = ship.State != "Destroyed" AND (ship.State == "UnderConstruction" OR ship.Health > 0)
// This is the single source of truth for ship counting in fleet capacity validation
func GetActiveFleetShips(fleet *domain.Fleet) []domain.Ship {
	if fleet == nil {
		return nil
	}
	activeShips := make([]domain.Ship, 0)
	for _, ship := range fleet.Ships {
		// Filter destroyed ships
		if ship.State != "Destroyed" && (ship.State == "UnderConstruction" || ship.Health > 0) {
			activeShips = append(activeShips, ship)
		}
	}
	return activeShips
}

