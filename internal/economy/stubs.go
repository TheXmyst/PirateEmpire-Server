package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DestroyShipHard permanently deletes a ship from the database (STUB - no logic)
// This is a stub function to fix compilation errors. Real implementation will be added later.
// DestroyShipHard permanently deletes a ship from the database and logs the event
func DestroyShipHard(db *gorm.DB, shipID uuid.UUID) error {
	var ship domain.Ship
	if err := db.First(&ship, "id = ?", shipID).Error; err != nil {
		return err
	}

	fleetID := "N/A"
	if ship.FleetID != nil {
		fleetID = ship.FleetID.String()
	}

	// Unassign captain if present (Ghost Captain Fix)
	if ship.CaptainID != nil {
		var assignedCaptain domain.Captain
		if err := db.First(&assignedCaptain, "id = ?", *ship.CaptainID).Error; err == nil {
			assignedCaptain.AssignedShipID = nil
			if err := db.Save(&assignedCaptain).Error; err != nil {
				fmt.Printf("[ERROR] Failed to unassign captain %s from destroyed ship %s: %v\n", assignedCaptain.ID, shipID, err)
			} else {
				fmt.Printf("[INFO] Unassigned captain %s from destroyed ship %s\n", assignedCaptain.ID, shipID)
			}
		}
	}

	if err := db.Delete(&ship).Error; err != nil {
		fmt.Printf("[ERROR] Failed to delete ship %s: %v\n", shipID, err)
		return err
	}

	fmt.Printf("[SHIP_DESTROYED] ship_id=%s fleet_id=%s reason=COMBAT_LOSS\n", shipID, fleetID)
	return nil
}

// ValidateFleetForCombat validates if a fleet is ready for combat (STUB - no logic)
// This is a stub function to fix compilation errors. Real implementation will be added later.
// Returns: (isValid bool, errorCode int, errorMessage string)
func ValidateFleetForCombat(fleet *domain.Fleet) (bool, int, string) {
	if fleet == nil {
		return false, 404, "Flotte introuvable"
	}
	if len(fleet.Ships) == 0 {
		return false, 400, "La flotte est vide"
	}

	aliveCount := 0
	destroyedCount := 0
	for _, ship := range fleet.Ships {
		if ship.State == "Destroyed" {
			destroyedCount++
		} else {
			aliveCount++
		}
	}

	if aliveCount == 0 {
		fmt.Printf("[DEBUG] ValidateFleet: Fleet %s has 0 alive ships (%d destroyed)\n", fleet.ID, destroyedCount)
		return false, 400, "Tous les navires sont détruits"
	}

	return true, 0, ""
}

// IsPositionClear checks if a position is clear of islands (STUB - no logic)
// This is a stub function to fix compilation errors. Real implementation will be added later.
func IsPositionClear(x, y float64, allIslands []domain.Island) bool {
	// STUB: Always return true for now
	// Real implementation would check collision with islands
	return true
}
