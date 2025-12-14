package economy

import (
	"fmt"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Militia recruitment constants
const (
	MilitiaRecruitBaseDuration = 15 * time.Second // Base duration
	MilitiaRecruitPerUnit      = 3 * time.Second  // Per unit duration
	MilitiaRecruitMinDuration  = 10 * time.Second // Minimum duration (safety)
	MilitiaRecruitMaxBatch     = 200               // Maximum units per batch
)

// Militia bonus constants
const (
	MilitiaBonusPerLevel = 0.005  // 0.5% per level
	MilitiaBonusMax      = 0.20   // 20% max
)

// Recruitment costs per unit type
const (
	RecruitCostWarriorGold = 10
	RecruitCostWarriorRum  = 2
	RecruitCostArcherGold  = 12
	RecruitCostArcherRum   = 2
	RecruitCostGunnerGold  = 15
	RecruitCostGunnerRum   = 3
)

// GetMilitiaBuilding returns the Militia building for an island, or nil if not found
func GetMilitiaBuilding(island *domain.Island) *domain.Building {
	for i := range island.Buildings {
		if island.Buildings[i].Type == "Milice" {
			return &island.Buildings[i]
		}
	}
	return nil
}

// ComputeMilitiaRecruitTimeBonus calculates the time reduction bonus based on Militia level
// Returns multiplier (e.g., 0.80 = 20% reduction = 80% of original time)
func ComputeMilitiaRecruitTimeBonus(militiaLevel int) float64 {
	bonusPct := float64(militiaLevel) * MilitiaBonusPerLevel
	if bonusPct > MilitiaBonusMax {
		bonusPct = MilitiaBonusMax
	}
	// Return multiplier: 1.0 - bonusPct (e.g., 20% bonus = 0.80 multiplier)
	return 1.0 - bonusPct
}

// CalculateRecruitDuration calculates the total recruitment duration
// base = 15s, perUnit = 3s, with Militia bonus applied
func CalculateRecruitDuration(warriors, archers, gunners int, militiaLevel int) time.Duration {
	totalUnits := warriors + archers + gunners
	if totalUnits <= 0 {
		return MilitiaRecruitMinDuration
	}

	// Base calculation
	duration := MilitiaRecruitBaseDuration + time.Duration(totalUnits)*MilitiaRecruitPerUnit

	// Apply Militia bonus (reduces duration)
	multiplier := ComputeMilitiaRecruitTimeBonus(militiaLevel)
	duration = time.Duration(float64(duration) * multiplier)

	// Clamp to minimum
	if duration < MilitiaRecruitMinDuration {
		duration = MilitiaRecruitMinDuration
	}

	return duration
}

// CalculateRecruitCost calculates the total cost (gold and rum) for recruitment
func CalculateRecruitCost(warriors, archers, gunners int) (gold int, rum int) {
	gold = warriors*RecruitCostWarriorGold + archers*RecruitCostArcherGold + gunners*RecruitCostGunnerGold
	rum = warriors*RecruitCostWarriorRum + archers*RecruitCostArcherRum + gunners*RecruitCostGunnerRum
	return gold, rum
}

// ValidateRecruitRequest validates a recruitment request
// Returns: (isValid, reasonCode, reasonMessage)
func ValidateRecruitRequest(island *domain.Island, warriors, archers, gunners int) (bool, string, string) {
	total := warriors + archers + gunners

	// Check total > 0
	if total <= 0 {
		return false, "RECRUIT_INVALID_ZERO", "Vous devez recruter au moins un matelot"
	}

	// Check total <= max batch
	if total > MilitiaRecruitMaxBatch {
		return false, "RECRUIT_INVALID_OVER_BATCH", fmt.Sprintf("Maximum %d matelots par recrutement", MilitiaRecruitMaxBatch)
	}

	// Check if already recruiting
	if island.MilitiaRecruiting {
		return false, "RECRUIT_ALREADY_BUSY", "Un recrutement est déjà en cours"
	}

	// Check resources
	gold, rum := CalculateRecruitCost(warriors, archers, gunners)
	if island.Resources[domain.Gold] < float64(gold) {
		return false, "RECRUIT_INSUFFICIENT_GOLD", fmt.Sprintf("Or insuffisant (nécessaire: %d, disponible: %.0f)", gold, island.Resources[domain.Gold])
	}
	if island.Resources[domain.Rum] < float64(rum) {
		return false, "RECRUIT_INSUFFICIENT_RUM", fmt.Sprintf("Rhum insuffisant (nécessaire: %d, disponible: %.0f)", rum, island.Resources[domain.Rum])
	}

	// Check for negative values
	if warriors < 0 || archers < 0 || gunners < 0 {
		return false, "RECRUIT_INVALID_NEGATIVE", "Les quantités ne peuvent pas être négatives"
	}

	return true, "", ""
}

// ProcessMilitiaRecruitment processes completed militia recruitment and adds crew to stock
// Should be called in GetStatus or a tick loop
func ProcessMilitiaRecruitment(island *domain.Island, now time.Time) bool {
	if !island.MilitiaRecruiting {
		return false
	}

	if island.MilitiaRecruitDoneAt == nil {
		// Invalid state: recruiting but no doneAt
		island.MilitiaRecruiting = false
		return true
	}

	if now.Before(*island.MilitiaRecruitDoneAt) {
		// Not done yet
		return false
	}

	// Recruitment is complete - add to stock
	island.CrewWarriors += island.MilitiaRecruitWarriors
	island.CrewArchers += island.MilitiaRecruitArchers
	island.CrewGunners += island.MilitiaRecruitGunners

	// Reset recruitment state
	island.MilitiaRecruiting = false
	island.MilitiaRecruitDoneAt = nil
	island.MilitiaRecruitWarriors = 0
	island.MilitiaRecruitArchers = 0
	island.MilitiaRecruitGunners = 0

	return true
}

