package dto

// CombatResultDTO represents combat result with string UUIDs for API responses
type CombatResultDTO struct {
	FleetAID          string           `json:"fleet_a_id"`
	FleetBID          string           `json:"fleet_b_id"`
	Winner            string           `json:"winner"`
	Rounds            []CombatRoundDTO `json:"rounds"`
	ShipsDestroyedA   []string         `json:"ships_destroyed_a"`
	ShipsDestroyedB   []string         `json:"ships_destroyed_b"`
	CaptainInjuredA   *string          `json:"captain_injured_a,omitempty"`
	CaptainInjuredB   *string          `json:"captain_injured_b,omitempty"`
	Applied           []string         `json:"applied,omitempty"`
}

// CombatRoundDTO represents a single combat round with string UUIDs
type CombatRoundDTO struct {
	RoundNumber int              `json:"round_number"`
	Attacks     []CombatAttackDTO `json:"attacks"`
	ShipsAliveA int              `json:"ships_alive_a"`
	ShipsAliveB int              `json:"ships_alive_b"`
}

// CombatAttackDTO represents a single attack with string UUIDs
type CombatAttackDTO struct {
	AttackerID     string  `json:"attacker_id"`
	AttackerType   string  `json:"attacker_type"`
	TargetID       string  `json:"target_id"`
	TargetType     string  `json:"target_type"`
	BaseDamage     float64 `json:"base_damage"`
	EngagementMult float64 `json:"engagement_mult"`
	CaptainBonus   float64 `json:"captain_bonus"`
	RPSMultiplier  float64 `json:"rps_multiplier"`
	RandomFactor   float64 `json:"random_factor"`
	DamageDealt    float64 `json:"damage_dealt"`
}
