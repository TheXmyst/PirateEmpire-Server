package economy

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
)

// ToDTO converts a CombatResult to a CombatResultDTO with string UUIDs
func (cr *CombatResult) ToDTO() *dto.CombatResultDTO {
	if cr == nil {
		return nil
	}

	// Convert ships destroyed
	shipsDestroyedA := make([]string, len(cr.ShipsDestroyedA))
	for i, shipID := range cr.ShipsDestroyedA {
		shipsDestroyedA[i] = shipID.String()
	}

	shipsDestroyedB := make([]string, len(cr.ShipsDestroyedB))
	for i, shipID := range cr.ShipsDestroyedB {
		shipsDestroyedB[i] = shipID.String()
	}

	// Convert captains injured
	var captainInjuredA, captainInjuredB *string
	if cr.CaptainInjuredA != nil {
		captainID := cr.CaptainInjuredA.String()
		captainInjuredA = &captainID
	}
	if cr.CaptainInjuredB != nil {
		captainID := cr.CaptainInjuredB.String()
		captainInjuredB = &captainID
	}

	// Convert rounds
	roundDTOs := make([]dto.CombatRoundDTO, len(cr.Rounds))
	for i, round := range cr.Rounds {
		attackDTOs := make([]dto.CombatAttackDTO, len(round.Attacks))
		for j, attack := range round.Attacks {
			attackDTOs[j] = dto.CombatAttackDTO{
				AttackerID:     attack.AttackerID.String(),
				AttackerType:   attack.AttackerType,
				TargetID:       attack.TargetID.String(),
				TargetType:     attack.TargetType,
				BaseDamage:     attack.BaseDamage,
				EngagementMult: attack.EngagementMult,
				CaptainBonus:   attack.CaptainBonus,
				RPSMultiplier:  attack.RPSMultiplier,
				RandomFactor:   attack.RandomFactor,
				DamageDealt:    attack.DamageDealt,
			}
		}
		roundDTOs[i] = dto.CombatRoundDTO{
			RoundNumber: round.RoundNumber,
			Attacks:     attackDTOs,
			ShipsAliveA: round.ShipsAliveA,
			ShipsAliveB: round.ShipsAliveB,
		}
	}

	return &dto.CombatResultDTO{
		FleetAID:        cr.FleetAID,
		FleetBID:        cr.FleetBID,
		Winner:          cr.Winner,
		Rounds:          roundDTOs,
		ShipsDestroyedA: shipsDestroyedA,
		ShipsDestroyedB: shipsDestroyedB,
		CaptainInjuredA: captainInjuredA,
		CaptainInjuredB: captainInjuredB,
		Applied:         cr.Applied,
	}
}
