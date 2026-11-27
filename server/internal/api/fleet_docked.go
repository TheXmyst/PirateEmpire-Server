package api

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// msgFleetAtSea : une flotte partie ne se modifie pas (navires, capitaine,
// matelots, cargaison). Il faut d'abord la ramener au port.
const msgFleetAtSea = "Ta flotte est en mer : ramène-la au port pour la modifier"

// fleetDocked : la flotte est au port de son île.
func fleetDocked(fleet *domain.Fleet) bool {
	return fleet.State == "" || fleet.State == domain.FleetStateIdle
}

// shipFleetAtSea : vrai si le navire fait partie d'une flotte qui n'est pas au port.
func shipFleetAtSea(db *gorm.DB, shipID uuid.UUID) bool {
	var ship domain.Ship
	if err := db.First(&ship, "id = ?", shipID).Error; err != nil || ship.FleetID == nil {
		return false
	}
	var fleet domain.Fleet
	if err := db.First(&fleet, "id = ?", *ship.FleetID).Error; err != nil {
		return false
	}
	return !fleetDocked(&fleet)
}
