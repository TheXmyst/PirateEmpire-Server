package economy

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/gamedata"
	"github.com/google/uuid"
)

const (
	MaxCombatRounds = 10 // Maximum number of combat rounds
)

// CombatResult represents the final result of a naval combat
type CombatResult struct {
	FleetAID        string        `json:"fleet_a_id"`
	FleetBID        string        `json:"fleet_b_id"`
	Winner          string        `json:"winner"`                      // "fleet_a", "fleet_b", or "draw"
	Rounds          []CombatRound `json:"rounds"`                      // All combat rounds
	ShipsDestroyedA []uuid.UUID   `json:"ships_destroyed_a"`           // Ships destroyed from Fleet A
	ShipsDestroyedB []uuid.UUID   `json:"ships_destroyed_b"`           // Ships destroyed from Fleet B
	CaptainInjuredA *uuid.UUID    `json:"captain_injured_a,omitempty"` // Captain injured from Fleet A (if flagship destroyed)
	CaptainInjuredB *uuid.UUID    `json:"captain_injured_b,omitempty"` // Captain injured from Fleet B (if flagship destroyed)
	ReasonCode      string        `json:"reason_code,omitempty"`       // Reason code for early exit or validation failure
	ReasonMessage   string        `json:"reason_message,omitempty"`    // Human-readable reason message (FR)
	Applied         []string      `json:"applied,omitempty"`           // Debug notes
}

// CombatRound represents a single round of combat
type CombatRound struct {
	RoundNumber int            `json:"round_number"`
	Attacks     []CombatAttack `json:"attacks"`       // All attacks in this round
	ShipsAliveA int            `json:"ships_alive_a"` // Ships alive in Fleet A after this round
	ShipsAliveB int            `json:"ships_alive_b"` // Ships alive in Fleet B after this round
}

// CombatAttack represents a single attack in a round
type CombatAttack struct {
	AttackerID         uuid.UUID `json:"attacker_id"`
	AttackerType       string    `json:"attacker_type"`
	TargetID           uuid.UUID `json:"target_id"`
	TargetType         string    `json:"target_type"`
	BaseDamage         float64   `json:"base_damage"`
	EngagementMult     float64   `json:"engagement_mult"`
	CaptainBonus       float64   `json:"captain_bonus"`
	RPSMultiplier      float64   `json:"rps_multiplier"`
	RandomFactor       float64   `json:"random_factor"`
	DamageDealt        float64   `json:"damage_dealt"`
	DamageTaken        float64   `json:"damage_taken"`
	TargetHealthBefore float64   `json:"target_health_before"`
	TargetHealthAfter  float64   `json:"target_health_after"`
	TargetDestroyed    bool      `json:"target_destroyed"`
	Applied            []string  `json:"applied,omitempty"` // Debug notes
}

// ShipCombatState represents a ship's state during combat (runtime only, not persisted)
type ShipCombatState struct {
	Ship           *domain.Ship
	CurrentHealth  float64
	EffectiveSpeed float64
	EffectiveDR    float64
	IsDestroyed    bool
	CombatStats    ShipCombatStats // Pre-computed combat stats
}

// ExecuteNavalCombat executes a complete naval combat between two fleets
// This is the main entry point for combat simulation
// Returns CombatResult with all rounds, destroyed ships, and injuries
func ExecuteNavalCombat(
	fleetA, fleetB *domain.Fleet,
	captA, captB *domain.Captain,
	engagementResult EngagementResult,
	seed int64, // Deterministic RNG seed
) (CombatResult, error) {
	result := CombatResult{
		FleetAID:        fleetA.ID.String(),
		FleetBID:        fleetB.ID.String(),
		Rounds:          make([]CombatRound, 0, MaxCombatRounds),
		ShipsDestroyedA: make([]uuid.UUID, 0),
		ShipsDestroyedB: make([]uuid.UUID, 0),
		Applied:         make([]string, 0),
	}

	// Initialize deterministic RNG
	rng := rand.New(rand.NewSource(seed))

	// Get flagship ships
	flagshipA, _, _ := SelectFlagshipShip(fleetA)
	flagshipB, _, _ := SelectFlagshipShip(fleetB)

	// Initialize combat states for all ships
	shipsA := make([]*ShipCombatState, 0, len(fleetA.Ships))
	for i := range fleetA.Ships {
		ship := &fleetA.Ships[i]
		// Only include ships that are not destroyed
		if ship.State != "Destroyed" {
			// Compute combat stats (only flagship gets captain bonuses)
			var captain *domain.Captain
			if flagshipA != nil && ship.ID == flagshipA.ID {
				captain = captA
			}
			combatStats, err := ComputeShipCombatStatsWithCaptain(ship, captain)
			if err != nil {
				return result, fmt.Errorf("failed to compute combat stats for ship %s: %w", ship.ID, err)
			}
			shipsA = append(shipsA, &ShipCombatState{
				Ship:           ship,
				CurrentHealth:  ship.Health,
				EffectiveSpeed: combatStats.EffectiveSpeed,
				EffectiveDR:    combatStats.EffectiveDamageReduction,
				IsDestroyed:    false,
				CombatStats:    combatStats,
			})
		}
	}

	shipsB := make([]*ShipCombatState, 0, len(fleetB.Ships))
	for i := range fleetB.Ships {
		ship := &fleetB.Ships[i]
		// Only include ships that are not destroyed
		if ship.State != "Destroyed" {
			// Compute combat stats (only flagship gets captain bonuses)
			var captain *domain.Captain
			if flagshipB != nil && ship.ID == flagshipB.ID {
				captain = captB
			}
			combatStats, err := ComputeShipCombatStatsWithCaptain(ship, captain)
			if err != nil {
				return result, fmt.Errorf("failed to compute combat stats for ship %s: %w", ship.ID, err)
			}
			shipsB = append(shipsB, &ShipCombatState{
				Ship:           ship,
				CurrentHealth:  ship.Health,
				EffectiveSpeed: combatStats.EffectiveSpeed,
				EffectiveDR:    combatStats.EffectiveDamageReduction,
				IsDestroyed:    false,
				CombatStats:    combatStats,
			})
		}
	}

	// Check if we have ships to fight
	if len(shipsA) == 0 || len(shipsB) == 0 {
		if len(shipsA) == 0 && len(shipsB) == 0 {
			result.Winner = "draw"
			result.ReasonCode = "COMBAT_EARLY_EXIT"
			result.ReasonMessage = "Aucun navire actif dans les deux flottes"
			result.Applied = append(result.Applied, "Both fleets have no ships")
		} else if len(shipsA) == 0 {
			result.Winner = "fleet_b"
			result.ReasonCode = "COMBAT_EARLY_EXIT"
			result.ReasonMessage = "Votre flotte n'a aucun navire actif"
			result.Applied = append(result.Applied, "Fleet A has no ships")
		} else {
			result.Winner = "fleet_a"
			result.ReasonCode = "COMBAT_EARLY_EXIT"
			result.ReasonMessage = "La flotte ennemie n'a aucun navire actif"
			result.Applied = append(result.Applied, "Fleet B has no ships")
		}
		return result, nil
	}

	// Execute rounds
	for round := 1; round <= MaxCombatRounds; round++ {
		roundResult := executeCombatRound(
			shipsA, shipsB,
			flagshipA, flagshipB,
			captA, captB,
			engagementResult,
			round,
			rng,
		)

		result.Rounds = append(result.Rounds, roundResult)

		// Collect destroyed ships
		for _, attack := range roundResult.Attacks {
			if attack.TargetDestroyed {
				// Determine which fleet the destroyed ship belongs to
				found := false
				for _, shipState := range shipsA {
					if shipState.Ship.ID == attack.TargetID {
						result.ShipsDestroyedA = append(result.ShipsDestroyedA, attack.TargetID)
						found = true
						break
					}
				}
				if !found {
					for _, shipState := range shipsB {
						if shipState.Ship.ID == attack.TargetID {
							result.ShipsDestroyedB = append(result.ShipsDestroyedB, attack.TargetID)
							break
						}
					}
				}
			}
		}

		// Check win condition
		aliveA := countAliveShips(shipsA)
		aliveB := countAliveShips(shipsB)

		if aliveA == 0 && aliveB == 0 {
			result.Winner = "draw"
			result.Applied = append(result.Applied, fmt.Sprintf("Round %d: Both fleets destroyed", round))
			break
		} else if aliveA == 0 {
			result.Winner = "fleet_b"
			result.Applied = append(result.Applied, fmt.Sprintf("Round %d: Fleet A destroyed", round))
			break
		} else if aliveB == 0 {
			result.Winner = "fleet_a"
			result.Applied = append(result.Applied, fmt.Sprintf("Round %d: Fleet B destroyed", round))
			break
		}
	}

	// Determine winner if no clear winner after max rounds
	if result.Winner == "" {
		aliveA := countAliveShips(shipsA)
		aliveB := countAliveShips(shipsB)
		if aliveA > aliveB {
			result.Winner = "fleet_a"
			result.ReasonCode = "COMBAT_VALID_LOSS"
			result.ReasonMessage = fmt.Sprintf("Victoire après %d rounds (%d navires restants vs %d)", len(result.Rounds), aliveA, aliveB)
			result.Applied = append(result.Applied, fmt.Sprintf("Max rounds reached: Fleet A wins (%d vs %d ships)", aliveA, aliveB))
		} else if aliveB > aliveA {
			result.Winner = "fleet_b"
			result.ReasonCode = "COMBAT_VALID_LOSS"
			result.ReasonMessage = fmt.Sprintf("Défaite après %d rounds (%d navires restants vs %d)", len(result.Rounds), aliveA, aliveB)
			result.Applied = append(result.Applied, fmt.Sprintf("Max rounds reached: Fleet B wins (%d vs %d ships)", aliveB, aliveA))
		} else {
			result.Winner = "draw"
			result.ReasonCode = "COMBAT_VALID_LOSS"
			result.ReasonMessage = fmt.Sprintf("Égalité après %d rounds (%d navires restants de chaque côté)", len(result.Rounds), aliveA)
			result.Applied = append(result.Applied, fmt.Sprintf("Max rounds reached: Draw (%d vs %d ships)", aliveA, aliveB))
		}
	} else if len(result.Rounds) > 0 {
		// Valid combat with rounds
		result.ReasonCode = "COMBAT_VALID_LOSS"
		if result.Winner == "fleet_a" {
			result.ReasonMessage = fmt.Sprintf("Victoire en %d rounds", len(result.Rounds))
		} else if result.Winner == "fleet_b" {
			result.ReasonMessage = fmt.Sprintf("Défaite en %d rounds", len(result.Rounds))
		} else {
			result.ReasonMessage = fmt.Sprintf("Égalité en %d rounds", len(result.Rounds))
		}
	}

	// Check for captain injuries (if flagship destroyed)
	if flagshipA != nil && captA != nil {
		for _, shipID := range result.ShipsDestroyedA {
			if shipID == flagshipA.ID {
				result.CaptainInjuredA = &captA.ID
				result.Applied = append(result.Applied, fmt.Sprintf("Fleet A flagship destroyed: Captain %s injured", captA.ID))
				break
			}
		}
	}
	if flagshipB != nil && captB != nil {
		for _, shipID := range result.ShipsDestroyedB {
			if shipID == flagshipB.ID {
				result.CaptainInjuredB = &captB.ID
				result.Applied = append(result.Applied, fmt.Sprintf("Fleet B flagship destroyed: Captain %s injured", captB.ID))
				break
			}
		}
	}

	return result, nil
}

// executeCombatRound executes a single round of combat
func executeCombatRound(
	shipsA, shipsB []*ShipCombatState,
	flagshipA, flagshipB *domain.Ship,
	captA, captB *domain.Captain,
	engagementResult EngagementResult,
	roundNumber int,
	rng *rand.Rand,
) CombatRound {
	round := CombatRound{
		RoundNumber: roundNumber,
		Attacks:     make([]CombatAttack, 0),
	}

	// Determine attack order by effective speed (descending)
	allShips := make([]*ShipCombatState, 0, len(shipsA)+len(shipsB))
	allShips = append(allShips, shipsA...)
	allShips = append(allShips, shipsB...)

	// Sort by effective speed (descending), then by ID for determinism
	sort.Slice(allShips, func(i, j int) bool {
		if allShips[i].EffectiveSpeed != allShips[j].EffectiveSpeed {
			return allShips[i].EffectiveSpeed > allShips[j].EffectiveSpeed
		}
		return allShips[i].Ship.ID.String() < allShips[j].Ship.ID.String()
	})

	// Each ship attacks once per round
	for _, attacker := range allShips {
		if attacker.IsDestroyed {
			continue
		}

		// Find valid targets (alive ships from enemy fleet)
		var validTargets []*ShipCombatState
		isAttackerInFleetA := false
		for _, ship := range shipsA {
			if ship.Ship.ID == attacker.Ship.ID {
				isAttackerInFleetA = true
				break
			}
		}

		if isAttackerInFleetA {
			// Attacker is in Fleet A, target Fleet B
			for _, ship := range shipsB {
				if !ship.IsDestroyed {
					validTargets = append(validTargets, ship)
				}
			}
		} else {
			// Attacker is in Fleet B, target Fleet A
			for _, ship := range shipsA {
				if !ship.IsDestroyed {
					validTargets = append(validTargets, ship)
				}
			}
		}

		if len(validTargets) == 0 {
			// No valid targets, skip attack
			continue
		}

		// Select target (deterministic: lowest health, then lowest ID)
		target := selectTarget(validTargets)

		// Execute attack
		attack := executeAttack(
			attacker, target,
			flagshipA, flagshipB,
			captA, captB,
			engagementResult,
			isAttackerInFleetA,
			rng,
		)

		round.Attacks = append(round.Attacks, attack)

		// Update target health
		target.CurrentHealth = attack.TargetHealthAfter
		if target.CurrentHealth <= 0 {
			target.IsDestroyed = true
			target.CurrentHealth = 0
		}
	}

	// Count alive ships after round
	round.ShipsAliveA = countAliveShips(shipsA)
	round.ShipsAliveB = countAliveShips(shipsB)

	return round
}

// selectTarget selects a target deterministically (lowest health, then lowest ID)
func selectTarget(targets []*ShipCombatState) *ShipCombatState {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) == 1 {
		return targets[0]
	}

	// Sort by health (ascending), then by ID (ascending) for determinism
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].CurrentHealth != targets[j].CurrentHealth {
			return targets[i].CurrentHealth < targets[j].CurrentHealth
		}
		return targets[i].Ship.ID.String() < targets[j].Ship.ID.String()
	})

	return targets[0]
}

// executeAttack executes a single attack
func executeAttack(
	attacker, target *ShipCombatState,
	flagshipA, flagshipB *domain.Ship,
	captA, captB *domain.Captain,
	engagementResult EngagementResult,
	isAttackerInFleetA bool,
	rng *rand.Rand,
) CombatAttack {
	attack := CombatAttack{
		AttackerID:         attacker.Ship.ID,
		AttackerType:       attacker.Ship.Type,
		TargetID:           target.Ship.ID,
		TargetType:         target.Ship.Type,
		TargetHealthBefore: target.CurrentHealth,
		Applied:            make([]string, 0),
	}

	// Get base damage for attacker ship type
	baseStats, err := gamedata.GetShipBaseStats(attacker.Ship.Type)
	if err != nil {
		attack.Applied = append(attack.Applied, fmt.Sprintf("ERROR: failed to get base stats: %v", err))
		return attack
	}
	// Base damage is proportional to HP (simple formula: baseDamage = HP * 0.1)
	attack.BaseDamage = baseStats.HP * 0.1

	// Get engagement multiplier
	if isAttackerInFleetA {
		attack.EngagementMult = engagementResult.AtkMultA
	} else {
		attack.EngagementMult = engagementResult.AtkMultB
	}

	// Get captain bonus (only if attacker is flagship)
	var captainBonus float64 = 1.0
	if isAttackerInFleetA && flagshipA != nil && attacker.Ship.ID == flagshipA.ID && captA != nil {
		// Use naval bonuses from captain stars (HP bonus as damage bonus approximation)
		navalBonuses := ComputeNavalBonuses(*captA)
		captainBonus = 1.0 + navalBonuses.NavalHPBonusPct // Use HP bonus as damage multiplier
		attack.Applied = append(attack.Applied, fmt.Sprintf("Captain bonus: +%.1f%%", navalBonuses.NavalHPBonusPct*100))
	} else if !isAttackerInFleetA && flagshipB != nil && attacker.Ship.ID == flagshipB.ID && captB != nil {
		navalBonuses := ComputeNavalBonuses(*captB)
		captainBonus = 1.0 + navalBonuses.NavalHPBonusPct
		attack.Applied = append(attack.Applied, fmt.Sprintf("Captain bonus: +%.1f%%", navalBonuses.NavalHPBonusPct*100))
	}

	// Get RPS multiplier
	attackerDominant := GetDominantCrewType(attacker.Ship)
	targetDominant := GetDominantCrewType(target.Ship)
	attack.RPSMultiplier = computeRPSMultiplierByType(attackerDominant, targetDominant)
	if attack.RPSMultiplier != 1.0 {
		attack.Applied = append(attack.Applied, fmt.Sprintf("RPS: attacker=%s defender=%s mult=%.2fx", attackerDominant, targetDominant, attack.RPSMultiplier))
	} else if attackerDominant != "none" || targetDominant != "none" {
		attack.Applied = append(attack.Applied, fmt.Sprintf("RPS: attacker=%s defender=%s mult=%.2fx (neutral)", attackerDominant, targetDominant, attack.RPSMultiplier))
	}

	// Get crew quantity bonuses (ATK for attacker, DEF for defender)
	atkCrewTotal := CrewTotal(attacker.Ship)
	defCrewTotal := CrewTotal(target.Ship)
	atkCrewBonus, _ := ComputeCrewAtkDefBonus(atkCrewTotal)
	_, defCrewBonus := ComputeCrewAtkDefBonus(defCrewTotal)

	// Log crew bonuses if non-zero
	if atkCrewBonus > 0 {
		attack.Applied = append(attack.Applied, fmt.Sprintf("Crew ATK bonus: total=%d -> +%.2f%%", atkCrewTotal, atkCrewBonus*100))
	}
	if defCrewBonus > 0 {
		attack.Applied = append(attack.Applied, fmt.Sprintf("Crew DEF bonus: total=%d -> -%.2f%% dmg taken", defCrewTotal, defCrewBonus*100))
	}

	// Random factor (0.9 - 1.1)
	attack.RandomFactor = 0.9 + rng.Float64()*0.2

	// Calculate damage dealt (apply crew ATK bonus)
	attack.DamageDealt = attack.BaseDamage *
		attack.EngagementMult *
		captainBonus *
		(1.0 + atkCrewBonus) * // Crew ATK bonus
		attack.RPSMultiplier *
		attack.RandomFactor

	// Calculate damage taken (with target's damage reduction, engagement def multiplier, and crew DEF bonus)
	var targetDefMult float64
	if isAttackerInFleetA {
		targetDefMult = engagementResult.DefMultB
	} else {
		targetDefMult = engagementResult.DefMultA
	}

	// Apply crew DEF bonus: reduces damage taken (multiply by (1 - defCrewBonus))
	// Clamp defCrewBonus to ensure (1 - defCrewBonus) doesn't go below 0.50 (safety)
	defReductionMult := 1.0 - defCrewBonus
	if defReductionMult < 0.50 {
		defReductionMult = 0.50 // Minimum 50% damage taken (safety cap)
	}

	attack.DamageTaken = attack.DamageDealt *
		(1.0 - target.EffectiveDR) *
		(1.0 / targetDefMult) *
		defReductionMult // Crew DEF bonus reduces damage taken

	// Clamp damage taken to minimum 0.1 (always at least 1% damage)
	if attack.DamageTaken < 0.1 {
		attack.DamageTaken = 0.1
	}

	// Apply damage
	attack.TargetHealthAfter = target.CurrentHealth - attack.DamageTaken
	if attack.TargetHealthAfter <= 0 {
		attack.TargetHealthAfter = 0
		attack.TargetDestroyed = true
	}

	attack.Applied = append(attack.Applied, fmt.Sprintf("Damage: %.1f -> %.1f (%.1f taken)", attack.TargetHealthBefore, attack.TargetHealthAfter, attack.DamageTaken))

	return attack
}

// computeRPSMultiplierByType computes the Rock/Paper/Scissors multiplier based on crew type strings
// Returns: 1.15 (advantage), 0.85 (disadvantage), or 1.0 (neutral)
// If either type is "none", returns 1.0 (neutral)
func computeRPSMultiplierByType(attackerType, targetType string) float64 {
	// If either side has no dominant crew type, return neutral
	if attackerType == "none" || targetType == "none" {
		return 1.0
	}

	// RPS rules:
	// Warrior > Gunner
	// Gunner > Archer
	// Archer > Warrior

	if attackerType == "warrior" {
		if targetType == "gunner" {
			return 1.15 // Advantage
		} else if targetType == "archer" {
			return 0.85 // Disadvantage
		}
	} else if attackerType == "gunner" {
		if targetType == "archer" {
			return 1.15 // Advantage
		} else if targetType == "warrior" {
			return 0.85 // Disadvantage
		}
	} else if attackerType == "archer" {
		if targetType == "warrior" {
			return 1.15 // Advantage
		} else if targetType == "gunner" {
			return 0.85 // Disadvantage
		}
	}

	return 1.0 // Same type = neutral
}

// GetDominantCrewType returns the crew type with the highest count
// Uses CrewWarriors, CrewArchers, CrewGunners fields
// Returns "warrior", "archer", "gunner", or "none" (if tie or all zero)
// Exported for use in handlers
func GetDominantCrewType(ship *domain.Ship) string {
	warriors := ship.CrewWarriors
	archers := ship.CrewArchers
	gunners := ship.CrewGunners

	// All zero or negative -> no dominant type
	if warriors <= 0 && archers <= 0 && gunners <= 0 {
		return "none"
	}

	// Find maximum count
	maxCount := warriors
	dominant := "warrior"

	if archers > maxCount {
		maxCount = archers
		dominant = "archer"
	} else if archers == maxCount && archers > 0 {
		// Tie: use deterministic selection (alphabetical: archer < warrior)
		if "archer" < dominant {
			dominant = "archer"
		}
	}

	if gunners > maxCount {
		maxCount = gunners
		dominant = "gunner"
	} else if gunners == maxCount && gunners > 0 {
		// Tie: use deterministic selection (alphabetical: archer < gunner < warrior)
		if "gunner" < dominant {
			dominant = "gunner"
		}
	}

	// If there's a tie (multiple types with same max count), return "none" for neutral
	if (warriors == maxCount && archers == maxCount) ||
		(warriors == maxCount && gunners == maxCount) ||
		(archers == maxCount && gunners == maxCount) {
		return "none"
	}

	return dominant
}

// countAliveShips counts the number of alive ships in a fleet
func countAliveShips(ships []*ShipCombatState) int {
	count := 0
	for _, ship := range ships {
		if !ship.IsDestroyed {
			count++
		}
	}
	return count
}

// GetBaseDamage returns the base damage for a ship type
func GetBaseDamage(shipType string) (float64, error) {
	baseStats, err := gamedata.GetShipBaseStats(shipType)
	if err != nil {
		return 0, err
	}
	// Base damage is proportional to HP
	return baseStats.HP * 0.1, nil
}

// GetCaptainInjuryDuration returns the injury duration for a captain based on rarity
func GetCaptainInjuryDuration(rarity domain.CaptainRarity) time.Duration {
	switch rarity {
	case domain.RarityCommon:
		return 30 * time.Minute
	case domain.RarityRare:
		return 2 * time.Hour
	case domain.RarityLegendary:
		return 5 * time.Hour
	default:
		return 30 * time.Minute // Default to common
	}
}
