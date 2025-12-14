package economy

import (
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
)

// IsFleetLocked checks if a fleet is currently locked (engagement in progress)
// Returns true if LockedUntil is set and the current time is before that timestamp
func IsFleetLocked(fleet *domain.Fleet) bool {
	if fleet.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*fleet.LockedUntil)
}

// LockFleetForEngagement locks a fleet for a specified duration (for future use in real combat)
// This function is not called by /dev/simulate-engagement (dev tool doesn't change state)
// TODO: Real combat system will call this to lock fleets during engagements
func LockFleetForEngagement(fleetID uuid.UUID, duration time.Duration) error {
	// This is a placeholder for future implementation
	// When real combat is implemented, this will set LockedUntil = now + duration
	return nil
}

