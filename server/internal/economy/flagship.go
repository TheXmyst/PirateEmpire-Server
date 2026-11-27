package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// SelectFlagshipShip selects the flagship ship for a fleet deterministically.
// Le navire amiral est celui qui porte le capitaine de la flotte :
//  1. le navire choisi (FlagshipShipID) s'il a un capitaine ;
//  2. sinon le navire avec capitaine au plus petit ID ;
//  3. sinon le navire choisi ;
//  4. sinon le navire au plus petit ID.
//
// Ne modifie pas l'ordre de fleet.Ships (avant, la fonction triait la liste
// sur place). Renvoie le navire, s'il a été choisi explicitement, et une raison
// pour les logs.
func SelectFlagshipShip(fleet *domain.Fleet) (*domain.Ship, bool, string) {
	if len(fleet.Ships) == 0 {
		return nil, false, "no ships in fleet"
	}

	var explicit, firstWithCaptain, first *domain.Ship
	for i := range fleet.Ships {
		s := &fleet.Ships[i]
		if fleet.FlagshipShipID != nil && s.ID == *fleet.FlagshipShipID {
			explicit = s
		}
		if s.CaptainID != nil && (firstWithCaptain == nil || s.ID.String() < firstWithCaptain.ID.String()) {
			firstWithCaptain = s
		}
		if first == nil || s.ID.String() < first.ID.String() {
			first = s
		}
	}

	if explicit != nil && explicit.CaptainID != nil {
		return explicit, true, fmt.Sprintf("explicit flagship_ship_id=%s", explicit.ID)
	}
	if firstWithCaptain != nil {
		return firstWithCaptain, false, fmt.Sprintf("first ship with captain (ship_id=%s)", firstWithCaptain.ID)
	}
	if explicit != nil {
		return explicit, true, fmt.Sprintf("explicit flagship_ship_id=%s (no captain)", explicit.ID)
	}
	return first, false, fmt.Sprintf("fallback: first ship by ID (ship_id=%s)", first.ID)
}
