package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DestroyShipHard permanently deletes a ship from the database.
// This function is idempotent: if the ship is already deleted, it returns nil.
// It handles cleanup of all references:
//   - Detaches ship from fleet (FleetID = nil)
//   - Clears FlagshipShipID if this ship was the flagship
//   - Hard deletes the ship from DB
//
// All operations are performed within the provided transaction.
func DestroyShipHard(tx *gorm.DB, shipID uuid.UUID) error {
	// Load ship with FOR UPDATE to prevent race conditions
	var ship domain.Ship
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&ship, "id = ?", shipID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Ship already deleted or doesn't exist - idempotent, return nil
			return nil
		}
		return fmt.Errorf("failed to load ship for deletion: %w", err)
	}

	// Get fleet ID before detaching (for logging)
	fleetIDStr := "none"
	if ship.FleetID != nil {
		fleetIDStr = ship.FleetID.String()[:8]

		// Load fleet to check if this ship is the flagship
		var fleet domain.Fleet
		if err := tx.First(&fleet, "id = ?", *ship.FleetID).Error; err == nil {
			// Check if this ship is the flagship
			if fleet.FlagshipShipID != nil && *fleet.FlagshipShipID == shipID {
				// Clear flagship reference
				fleet.FlagshipShipID = nil
				if err := tx.Save(&fleet).Error; err != nil {
					return fmt.Errorf("failed to clear flagship reference: %w", err)
				}
			}
		}

		// Detach ship from fleet (set FleetID to nil)
		ship.FleetID = nil
		if err := tx.Save(&ship).Error; err != nil {
			return fmt.Errorf("failed to detach ship from fleet: %w", err)
		}
	}

	// Hard delete the ship (Unscoped() ensures permanent deletion even if using soft deletes)
	if err := tx.Unscoped().Delete(&ship).Error; err != nil {
		return fmt.Errorf("failed to hard delete ship: %w", err)
	}

	fmt.Printf("[COMBAT] ship destroyed hard-delete ship=%s fleet=%s\n", shipID.String()[:8], fleetIDStr)
	return nil
}

