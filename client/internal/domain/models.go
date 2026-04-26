package domain

import (
	"time"

	"github.com/google/uuid"
)

// Resource types
type ResourceType string

const (
	Wood          ResourceType = "wood"
	Gold          ResourceType = "gold"
	Stone         ResourceType = "stone"
	Iron          ResourceType = "iron"
	Food          ResourceType = "food"
	Rum           ResourceType = "rum"
	CaptainTicket ResourceType = "captain_ticket"
)

// FleetState defines the possible states of a fleet
type FleetState string

const (
	FleetStateIdle                FleetState = "Idle"
	FleetStateMoving              FleetState = "Moving"
	FleetStateReturning           FleetState = "Returning"
	FleetStateStationed           FleetState = "Stationed"
	FleetStateChasingPvE          FleetState = "Chasing_PvE"
	FleetStateTravelingToAttack   FleetState = "Traveling_To_Attack"
	FleetStateReturningFromAttack FleetState = "Returning_From_Attack"
	FleetStateSeaStationed        FleetState = "SeaStationed"
	FleetStateChasingPvP          FleetState = "Chasing_PvP"
)

// Resource represents a quantity of a specific resource
type Resource struct {
	Type   ResourceType `json:"type"`
	Amount float64      `json:"amount"`
}

// ChatMessage represents a chat entry from the server.
type ChatMessage struct {
	ID        uuid.UUID `json:"id"`
	SeaID     uuid.UUID `json:"sea_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"created_at"`
}

// ResourceNode represents a resource gathering point
type ResourceNode struct {
	ID           string       `json:"id"`
	Type         ResourceType `json:"type"`
	X            int          `json:"x"`
	Y            int          `json:"y"`
	Amount       float64      `json:"amount"`
	Regeneration float64      `json:"regeneration"`
	Richness     float64      `json:"richness"`
}

// Requirement represents a building or technology requirement
type Requirement struct {
	Level       int    `json:"level"`
	ReqTownHall int    `json:"req_townhall"`
	ReqTech     string `json:"req_tech"`
	ReqBuilding string `json:"req_building"`
	ReqMinLevel int    `json:"req_min_level"`

	// Client-side display fields
	Kind    string `json:"kind,omitempty"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message,omitempty"`
	Needed  int    `json:"needed,omitempty"`
	Current int    `json:"current,omitempty"`
}

// Player represents a user in the game
type Player struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Role      string    `json:"role"`
	IsAdmin   bool      `json:"is_admin"`

	UnlockedTechs []string `json:"unlocked_techs"`

	ResearchingTechID            string    `json:"researching_tech_id"`
	ResearchFinishTime           time.Time `json:"research_finish_time"`
	ResearchTotalDurationSeconds float64   `json:"current_research_total_duration_seconds"` // Total duration in seconds (after bonuses)

	// Gacha Pity System
	PityLegendaryCount int `json:"pity_legendary_count"`
	PityRareCount      int `json:"pity_rare_count"`

	// Reset cooldown
	LastResetAt *time.Time `json:"last_reset_at,omitempty"`

	// Shard exchange daily cap
	DailyShardExchangeCount int    `json:"daily_shard_exchange_count"`
	DailyShardExchangeDay   string `json:"daily_shard_exchange_day"`

	Islands  []Island  `json:"islands,omitempty"`
	Captains []Captain `json:"captains,omitempty"`
}

// Island represents a player's base
type Island struct {
	ID       uuid.UUID `json:"id"`
	PlayerID uuid.UUID `json:"player_id"`
	Player   *Player   `json:"player,omitempty"`
	SeaID    string    `json:"sea_id,omitempty"`
	Name     string    `json:"name"`
	Level    int       `json:"level"`

	// Coordinate system (X, Y) for world map placement
	X int `json:"x"`
	Y int `json:"y"`

	// Storing resources as JSON for simplicity
	ResourcesJSON      []byte                   `json:"-"`
	Resources          map[ResourceType]float64 `json:"resources"`
	StorageLimits      map[ResourceType]float64 `json:"storage_limits"`
	ResourceGeneration map[ResourceType]float64 `json:"resource_generation"`

	// Idle Crew on Island
	CrewJSON []byte           `json:"-"`
	Crew     map[UnitType]int `json:"militia_stock"`

	Buildings []Building `json:"buildings,omitempty"`
	Fleets    []Fleet    `json:"fleets,omitempty"`
	Ships     []Ship     `json:"ships,omitempty"`

	LastUpdated time.Time `json:"last_updated"`

	// PVP fields
	ProtectedUntil *time.Time `json:"protected_until,omitempty"`
	ActiveFleetID  *uuid.UUID `json:"active_fleet_id,omitempty"`

	// Militia fields
	MilitiaRecruiting      bool       `json:"militia_recruiting"`
	MilitiaRecruitDoneAt   *time.Time `json:"militia_recruit_done_at,omitempty"`
	MilitiaRecruitWarriors int        `json:"militia_recruit_warriors"`
	MilitiaRecruitArchers  int        `json:"militia_recruit_archers"`
	MilitiaRecruitGunners  int        `json:"militia_recruit_gunners"`

	// Resource generation transient fields
	ResourceGenerationBase  map[ResourceType]float64 `json:"resource_generation_base,omitempty"`
	ResourceGenerationBonus map[ResourceType]float64 `json:"resource_generation_bonus,omitempty"`
}

// Fleet represents a group of ships
type Fleet struct {
	ID       uuid.UUID `json:"id"`
	IslandID uuid.UUID `json:"island_id"`
	Name     string    `json:"name"`
	Ships    []Ship    `json:"ships,omitempty"`

	State   FleetState `json:"state"`
	FreeNav bool       `json:"free_nav"`
	TargetX *int       `json:"target_x,omitempty"`
	TargetY *int       `json:"target_y,omitempty"`

	// PvP Interception Tracking
	ChasingFleetID          *uuid.UUID `json:"chasing_fleet_id,omitempty"`
	ChasedByFleetID         *uuid.UUID `json:"chased_by_fleet_id,omitempty"`
	InterceptStartedAt      *time.Time `json:"intercept_started_at,omitempty"`
	InterceptTargetPlayerID *uuid.UUID `json:"intercept_target_player_id,omitempty"`

	MoraleCruise   *int       `json:"morale_cruise,omitempty"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	FlagshipShipID *uuid.UUID `json:"flagship_ship_id,omitempty"`

	TargetIslandID  *uuid.UUID               `json:"target_island_id,omitempty"`
	TargetPveID     *string                  `json:"target_pve_id,omitempty"`
	AttackLoot      map[ResourceType]float64 `json:"attack_loot,omitempty"`
	Cargo           map[ResourceType]float64 `json:"cargo,omitempty"`
	StationedAt     *time.Time               `json:"stationed_at,omitempty"`
	StationedNodeID *string                  `json:"stationed_node_id,omitempty"`
	StoredAmount    float64                  `json:"stored_amount"`
	StoredResource  string                   `json:"stored_resource"`

	// Capacity Fields (Received from Server)
	CargoCapacity float64 `json:"cargo_capacity"`
	CargoUsed     float64 `json:"cargo_used"`
	CargoFree     float64 `json:"cargo_free"`
}

// IsMoving returns true if the fleet is visually moving on the map
func (f *Fleet) IsMoving() bool {
	return f.State == FleetStateMoving ||
		f.State == FleetStateReturning ||
		f.State == FleetStateChasingPvE ||
		f.State == FleetStateChasingPvP ||
		f.State == FleetStateTravelingToAttack ||
		f.State == FleetStateReturningFromAttack
}

// Building represents a structure on an island
type Building struct {
	ID           uuid.UUID `json:"id"`
	IslandID     uuid.UUID `json:"island_id"`
	Type         string    `json:"type"`
	Level        int       `json:"level"`
	X            float64   `json:"x"`
	Y            float64   `json:"y"`
	Constructing bool      `json:"constructing"`
	FinishTime   time.Time `json:"finish_time"`

	ConstructionTotalDuration float64 `json:"-"`
}

// --- Naval & Combat Models ---

type UnitType string

const (
	Warrior UnitType = "warrior"
	Archer  UnitType = "archer"
	Gunner  UnitType = "gunner"
)

type Captain struct {
	ID             uuid.UUID  `json:"id"`
	PlayerID       uuid.UUID  `json:"player_id"`
	TemplateID     string     `json:"template_id"`
	Name           string     `json:"name"`
	Rarity         string     `json:"rarity"`
	Level          int        `json:"level"`
	XP             int        `json:"xp"`
	Stars          int        `json:"stars"`
	SkillID        string     `json:"skill_id"`
	AssignedShipID *uuid.UUID `json:"assigned_ship_id"`
	Portrait       string     `json:"-"`
	InjuredUntil   *time.Time `json:"injured_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Stats (legacy, may be unused)
	Command int `json:"-"`
	Naval   int `json:"-"`
	Combat  int `json:"-"`

	// Naval bonuses from stars (computed server-side)
	NavalHPBonusPct            float64 `json:"-"`
	NavalSpeedBonusPct         float64 `json:"-"`
	NavalDamageReductionPct    float64 `json:"-"`
	RumConsumptionReductionPct float64 `json:"-"`

	// Passive effects (computed server-side)
	PassiveID       string          `json:"-"`
	PassiveValue    float64         `json:"-"`
	PassiveIntValue int             `json:"-"`
	Threshold       int             `json:"-"`
	DrainPerMinute  float64         `json:"-"`
	Flags           map[string]bool `json:"-"`
}

type Ship struct {
	ID       uuid.UUID  `json:"id"`
	PlayerID uuid.UUID  `json:"player_id"`
	IslandID uuid.UUID  `json:"island_id"`
	FleetID  *uuid.UUID `json:"fleet_id"`

	Name string `json:"name"`
	Type string `json:"type"`

	Health    float64 `json:"health"`
	MaxHealth float64 `json:"max_health"`

	State string  `json:"state"` // "DOCKED", "SAILING", "COMBAT", "UnderConstruction"
	X     float64 `json:"x"`
	Y     float64 `json:"y"`

	FinishTime time.Time `json:"finish_time"`

	CaptainID *uuid.UUID `json:"captain_id"`

	// Militia composition per ship (for RPS combat system)
	MilitiaWarriors int `json:"militia_warriors"`
	MilitiaArchers  int `json:"militia_archers"`
	MilitiaGunners  int `json:"militia_gunners"`
	MilitiaCapacity int `json:"militia_capacity"`

	// Legacy crew map (kept for backward compatibility)
	CrewJSON []byte           `json:"-"`
	Crew     map[UnitType]int `json:"-"`

	// Dynamic positioning fields
	RealX         float64 `json:"-"`
	RealY         float64 `json:"-"`
	ServerChanged bool    `json:"-"`
	LastServerX   float64 `json:"-"`
	LastServerY   float64 `json:"-"`

	ConstructionTotalDuration float64 `json:"-"`
}

// EngagementResult represents the computed engagement morale snapshot and multipliers
type EngagementResult struct {
	FleetAID          string   `json:"fleet_a_id"`
	FleetBID          string   `json:"fleet_b_id"`
	EngagementMoraleA int      `json:"engagement_morale_a"`
	EngagementMoraleB int      `json:"engagement_morale_b"`
	Delta             int      `json:"delta"`
	BonusPercent      float64  `json:"bonus_percent"`
	AtkMultA          float64  `json:"atk_mult_a"`
	DefMultA          float64  `json:"def_mult_a"`
	AtkMultB          float64  `json:"atk_mult_b"`
	DefMultB          float64  `json:"def_mult_b"`
	PanicThresholdA   int      `json:"panic_threshold_a,omitempty"`
	PanicThresholdB   int      `json:"panic_threshold_b,omitempty"`
	Applied           []string `json:"applied,omitempty"`
}

// PvPInterceptCooldown tracks anti-harassment cooldowns
type PvPInterceptCooldown struct {
	AttackerPlayerID uuid.UUID `json:"attacker_player_id"`
	TargetPlayerID   uuid.UUID `json:"target_player_id"`
	BlockedUntil     time.Time `json:"blocked_until"`
}

// PVE Models

// PveTarget represents a PVE target (NPC fleet) on the world map
type PveTarget struct {
	ID   string  `json:"id"`   // Stable ID: UUID (SSOT)
	X    float64 `json:"x"`    // Position X on world map
	Y    float64 `json:"y"`    // Position Y on world map
	Tier int     `json:"tier"` // Tier 1, 2, or 3 (difficulty)
	Name string  `json:"name"` // Display name (e.g., "Corsaires égarés")

	RealX   float64  `json:"real_x,omitempty"`
	RealY   float64  `json:"real_y,omitempty"`
	TargetX *float64 `json:"target_x,omitempty"`
	TargetY *float64 `json:"target_y,omitempty"`
	Speed   float64  `json:"speed,omitempty"`
	IsFleet bool     `json:"is_fleet"`
	FleetID string   `json:"fleet_id,omitempty"`
}

// CombatResult represents the final result of a naval combat
type CombatResult struct {
	FleetAID        string        `json:"fleet_a_id"`
	FleetBID        string        `json:"fleet_b_id"`
	Winner          string        `json:"winner"` // "fleet_a", "fleet_b", or "draw"
	Rounds          []CombatRound `json:"rounds"` // All combat rounds
	ShipsDestroyedA []uuid.UUID   `json:"ships_destroyed_a"`
	ShipsDestroyedB []uuid.UUID   `json:"ships_destroyed_b"`
	CaptainInjuredA *uuid.UUID    `json:"captain_injured_a,omitempty"`
	CaptainInjuredB *uuid.UUID    `json:"captain_injured_b,omitempty"`
	Applied         []string      `json:"applied,omitempty"`

	ReasonMessage string `json:"reason_message,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
	DoneAt        string `json:"done_at,omitempty"`
}

// CombatRound represents a single round of combat
type CombatRound struct {
	RoundNumber int            `json:"round_number"`
	Attacks     []CombatAttack `json:"attacks"`
	ShipsAliveA int            `json:"ships_alive_a"`
	ShipsAliveB int            `json:"ships_alive_b"`
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
	Applied            []string  `json:"applied,omitempty"`
}
