package engine

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"log"
)

// ReputationConstants defines the base reputation values
const (
	// PvE reputation gains by tier
	PvETier1Reputation = 10  // Small enemies
	PvETier2Reputation = 50  // Medium enemies
	PvETier3Reputation = 200 // Hard enemies

	// PvP reputation logic:
	// When attacking a weaker player (loser rep < winner rep):
	// - Reputation difference threshold: if loser has 50%+ less rep, no gain
	MinReputationGainThreshold = 0.5 // 50% difference threshold

	// Base PvP gains (adjusted by reputation difference)
	PvPBaseGainVsStronger = 100 // Beating a stronger opponent
	PvPBaseGainVsWeaker   = 20  // Beating a weaker opponent
	PvPBaseLossVsStronger = 10  // Losing to a stronger opponent
	PvPBaseLossVsWeaker   = 50  // Losing to a weaker opponent

	// Fleet power to reputation conversion ratio
	// Fleet power includes: ship base power + crew count + captain bonuses
	FleetPowerToReputation = 0.1 // 10 power = 1 reputation
)

// CalculateFleetReputation calculates the base reputation from fleet composition
// Reputation = sum of (ship power + crew count * crew_power + captain bonus)
func CalculateFleetReputation(playerID uuid.UUID) int {
	db := repository.GetDB()

	var fleets []domain.Fleet
	if err := db.Where("player_id = ?", playerID).Find(&fleets).Error; err != nil {
		log.Printf("Error loading fleets for reputation calculation: %v", err)
		return 0
	}

	totalPower := 0.0

	for _, fleet := range fleets {
		var ships []domain.Ship
		if err := db.Where("fleet_id = ?", fleet.ID).Find(&ships).Error; err != nil {
			continue
		}

		for _, ship := range ships {
			// Base ship power (estimated from type and health)
			var shipPower float64
			switch ship.Type {
			case "Corvette":
				shipPower = 100
			case "Frigate":
				shipPower = 200
			case "Galleon":
				shipPower = 300
			case "Flute":
				shipPower = 150
			default:
				shipPower = 100
			}

			// Add crew power
			crewPower := float64(ship.MilitiaWarriors*10 + ship.MilitiaArchers*8 + ship.MilitiaGunners*12)
			totalPower += shipPower + crewPower

			// Add captain bonus if assigned
			if ship.CaptainID != nil {
				var captain domain.Captain
				if err := db.First(&captain, "id = ?", *ship.CaptainID).Error; err == nil {
					// Captain bonus: 20 per level + 50 per star
					captainBonus := float64(captain.Level*20 + captain.Stars*50)
					totalPower += captainBonus
				}
			}
		}
	}

	// Check for unassigned ships as well
	var unassignedShips []domain.Ship
	if err := db.Where("player_id = ? AND fleet_id IS NULL", playerID).Find(&unassignedShips).Error; err == nil {
		for _, ship := range unassignedShips {
			var shipPower float64
			switch ship.Type {
			case "Corvette":
				shipPower = 100
			case "Frigate":
				shipPower = 200
			case "Galleon":
				shipPower = 300
			case "Flute":
				shipPower = 150
			default:
				shipPower = 100
			}

			crewPower := float64(ship.MilitiaWarriors*10 + ship.MilitiaArchers*8 + ship.MilitiaGunners*12)
			totalPower += shipPower + crewPower

			if ship.CaptainID != nil {
				var captain domain.Captain
				if err := db.First(&captain, "id = ?", *ship.CaptainID).Error; err == nil {
					captainBonus := float64(captain.Level*20 + captain.Stars*50)
					totalPower += captainBonus
				}
			}
		}
	}

	// Convert fleet power to reputation
	return int(totalPower * FleetPowerToReputation)
}

// RecordPvEVictory records a PvE victory and updates player reputation
func RecordPvEVictory(playerID uuid.UUID, tier int) (reputationGain int, err error) {
	db := repository.GetDB()

	// Determine reputation gain based on tier
	switch tier {
	case 1:
		reputationGain = PvETier1Reputation
	case 2:
		reputationGain = PvETier2Reputation
	case 3:
		reputationGain = PvETier3Reputation
	default:
		reputationGain = PvETier1Reputation
	}

	// Create victory record
	victory := domain.PvEVictory{
		ID:               uuid.New(),
		PlayerID:         playerID,
		Tier:             tier,
		ReputationGain:   reputationGain,
	}

	if err := db.Create(&victory).Error; err != nil {
		log.Printf("Error recording PvE victory: %v", err)
		return 0, err
	}

	// Update player reputation
	if err := db.Model(&domain.Player{}).Where("id = ?", playerID).
		Update("reputation", db.Raw("reputation + ?", reputationGain)).Error; err != nil {
		log.Printf("Error incrementing player reputation: %v", err)
		return 0, err
	}

	return reputationGain, nil
}

// RecordPvPVictory records a PvP victory and updates reputation for both players
// based on reputation difference
func RecordPvPVictory(winnerID, loserID uuid.UUID) error {
	db := repository.GetDB()

	// Get both players' current reputation
	var winner, loser domain.Player
	if err := db.First(&winner, "id = ?", winnerID).Error; err != nil {
		log.Printf("Error loading winner player: %v", err)
		return err
	}
	if err := db.First(&loser, "id = ?", loserID).Error; err != nil {
		log.Printf("Error loading loser player: %v", err)
		return err
	}

	winnerRepBefore := winner.Reputation
	loserRepBefore := loser.Reputation

	// Calculate reputation gains/losses based on reputation difference
	repRatio := 1.0
	if loser.Reputation > 0 {
		repRatio = float64(winner.Reputation) / float64(loser.Reputation)
	}

	var winnerRepGain int
	var loserRepLoss int

	// If winner has much higher reputation than loser, no gain (anti-farming)
	// If winner rep / loser rep > 2 (loser has 50% less), no gain
	if repRatio > 2.0 {
		winnerRepGain = 0
		loserRepLoss = 0
	} else if winner.Reputation > loser.Reputation {
		// Winner had higher rep, so this is a smaller upset
		winnerRepGain = PvPBaseGainVsWeaker
		loserRepLoss = PvPBaseLossVsWeaker
	} else if winner.Reputation == loser.Reputation {
		// Equal reputation
		winnerRepGain = PvPBaseGainVsStronger / 2
		loserRepLoss = PvPBaseLossVsStronger / 2
	} else {
		// Winner had lower rep, big upset!
		winnerRepGain = PvPBaseGainVsStronger
		loserRepLoss = PvPBaseLossVsStronger
	}

	// Create victory record
	victory := domain.PvPVictory{
		ID:              uuid.New(),
		WinnerID:        winnerID,
		LoserID:         loserID,
		WinnerRepGain:   winnerRepGain,
		LoserRepLoss:    loserRepLoss,
		WinnerRepBefore: winnerRepBefore,
		LoserRepBefore:  loserRepBefore,
	}

	if err := db.Create(&victory).Error; err != nil {
		log.Printf("Error recording PvP victory: %v", err)
		return err
	}

	// Update both players' reputation
	if err := db.Model(&domain.Player{}).Where("id = ?", winnerID).
		Update("reputation", winnerRepBefore+winnerRepGain).Error; err != nil {
		log.Printf("Error updating winner reputation: %v", err)
		return err
	}

	newLoserRep := loserRepBefore - loserRepLoss
	if newLoserRep < 0 {
		newLoserRep = 0 // Reputation can't go below 0
	}
	if err := db.Model(&domain.Player{}).Where("id = ?", loserID).
		Update("reputation", newLoserRep).Error; err != nil {
		log.Printf("Error updating loser reputation: %v", err)
		return err
	}

	return nil
}

// RefreshPlayerReputation recalculates a player's reputation from fleet composition
// This should be called periodically or when fleet changes
func RefreshPlayerReputation(playerID uuid.UUID) error {
	db := repository.GetDB()

	fleetRep := CalculateFleetReputation(playerID)

	// Update player with calculated reputation
	if err := db.Model(&domain.Player{}).Where("id = ?", playerID).
		Update("reputation", fleetRep).Error; err != nil {
		log.Printf("Error refreshing player reputation: %v", err)
		return err
	}

	return nil
}
