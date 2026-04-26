package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	Type   ResourceType `json:"type" gorm:"-"`
	Amount float64      `json:"amount" gorm:"-"`
}

// ChatMessage is a persistent chat entry scoped by sea.
type ChatMessage struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	SeaID     uuid.UUID `json:"sea_id" gorm:"type:uuid;index"`
	PlayerID  uuid.UUID `json:"player_id" gorm:"type:uuid;index"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// ResourceNode represents a resource gathering point (STUB - no logic implemented)
type ResourceNode struct {
	ID           string       `json:"id"`
	Type         ResourceType `json:"type"`
	X            int          `json:"x"`
	Y            int          `json:"y"`
	Amount       float64      `json:"amount"`
	Regeneration float64      `json:"regeneration"`
	Richness     float64      `json:"richness"` // Quality/multiplier of the node
}

// Player represents a user in the game
type Player struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	Username  string    `json:"username" gorm:"uniqueIndex"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Role      string    `json:"role"`
	IsAdmin   bool      `json:"is_admin" gorm:"default:false"`

	// Unlocked Technologies (JSON Array of IDs)
	UnlockedTechsJSON []byte   `json:"-" gorm:"column:unlocked_techs"`
	UnlockedTechs     []string `json:"unlocked_techs" gorm:"-"`

	// Active Research
	ResearchingTechID            string    `json:"researching_tech_id" gorm:"default:''"`
	ResearchFinishTime           time.Time `json:"research_finish_time"`
	ResearchTotalDurationSeconds float64   `json:"current_research_total_duration_seconds" gorm:"default:0"` // Total duration in seconds (after bonuses)

	// Gacha Pity System
	PityLegendaryCount int `json:"pity_legendary_count" gorm:"default:0"` // Pulls since last legendary
	PityRareCount      int `json:"pity_rare_count" gorm:"default:0"`      // Pulls since last rare (optional)

	// Reset cooldown (anti-farm protection)
	LastResetAt *time.Time `json:"last_reset_at,omitempty" gorm:"column:last_reset_at"` // Nullable, tracks last reset time

	// Shard exchange daily cap (anti-abuse protection)
	DailyShardExchangeCount int    `json:"daily_shard_exchange_count" gorm:"default:0"` // Count of exchanges today
	DailyShardExchangeDay   string `json:"daily_shard_exchange_day" gorm:"default:''"`  // Format: YYYY-MM-DD

	// Social
	GuildTicket int        `json:"guild_ticket" gorm:"default:1"` // Guild creation tickets (1 per account)
	GuildID     *uuid.UUID `json:"guild_id,omitempty" gorm:"type:uuid;index"`

	// Reputation System
	Reputation int `json:"reputation" gorm:"default:0"` // Player reputation points (from fleet power + wins)

	Islands  []Island  `json:"islands,omitempty" gorm:"foreignKey:PlayerID"`
	Captains []Captain `json:"captains,omitempty" gorm:"foreignKey:PlayerID"`
}

func (p *Player) BeforeSave(tx *gorm.DB) (err error) {
	if p.UnlockedTechs != nil {
		data, err := json.Marshal(p.UnlockedTechs)
		if err != nil {
			return err
		}
		p.UnlockedTechsJSON = data
	}
	return nil
}

func (p *Player) AfterFind(tx *gorm.DB) (err error) {
	if len(p.UnlockedTechsJSON) > 0 {
		if err := json.Unmarshal(p.UnlockedTechsJSON, &p.UnlockedTechs); err != nil {
			return err
		}
	}
	if p.UnlockedTechs == nil {
		p.UnlockedTechs = []string{}
	}
	return nil
}

// Sea represents a game world instanced zone
type Sea struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`

	Islands []Island `json:"islands,omitempty" gorm:"foreignKey:SeaID"`
}

// Island represents a player's base
type Island struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	PlayerID uuid.UUID `json:"player_id" gorm:"type:uuid;index"`
	Player   Player    `json:"player" gorm:"foreignKey:PlayerID"`
	SeaID    uuid.UUID `json:"sea_id" gorm:"type:uuid;index"`
	Name     string    `json:"name"`
	Level    int       `json:"level"`

	X int `json:"x"`
	Y int `json:"y"`

	ResourcesJSON []byte                   `json:"-" gorm:"column:resources"`
	Resources     map[ResourceType]float64 `json:"resources" gorm:"-"`
	StorageLimits map[ResourceType]float64 `json:"storage_limits" gorm:"-"`

	CrewJSON []byte           `json:"-" gorm:"column:crew"`
	Crew     map[UnitType]int `json:"militia_stock" gorm:"-"`

	Buildings []Building `json:"buildings,omitempty" gorm:"foreignKey:IslandID"`
	Fleets    []Fleet    `json:"fleets,omitempty" gorm:"foreignKey:IslandID"`
	Ships     []Ship     `json:"ships,omitempty" gorm:"foreignKey:IslandID"` // Initial pool (can be in fleet or unassigned)

	LastUpdated time.Time `json:"last_updated"`

	// Checkpoint throttling: tracks last time island was persisted from /status endpoint
	// Used to reduce DB writes: /status only saves island every 5 seconds max
	LastCheckpointSavedAt *time.Time `json:"-" gorm:"column:last_checkpoint_saved_at"`

	// PVP fields (STUB - no logic implemented)
	ProtectedUntil *time.Time `json:"protected_until,omitempty"`
	ActiveFleetID  *uuid.UUID `json:"active_fleet_id,omitempty" gorm:"type:uuid"`

	// Resource generation (transient, for UI - not persisted)
	ResourceGeneration      map[ResourceType]float64 `json:"resource_generation,omitempty" gorm:"-"`
	ResourceGenerationBase  map[ResourceType]float64 `json:"resource_generation_base,omitempty" gorm:"-"`
	ResourceGenerationBonus map[ResourceType]float64 `json:"resource_generation_bonus,omitempty" gorm:"-"`

	// Militia Recruitment fields
	MilitiaRecruiting      bool       `json:"militia_recruiting" gorm:"default:false"`
	MilitiaRecruitDoneAt   *time.Time `json:"militia_recruit_done_at,omitempty"`
	MilitiaRecruitWarriors int        `json:"militia_recruit_warriors" gorm:"default:0"`
	MilitiaRecruitArchers  int        `json:"militia_recruit_archers" gorm:"default:0"`
	MilitiaRecruitGunners  int        `json:"militia_recruit_gunners" gorm:"default:0"`
}

// Deposit adds a resource to the island, respecting storage limits.
// Returns the actual amount deposited.
func (i *Island) Deposit(res ResourceType, amount float64) float64 {
	if i.Resources == nil {
		i.Resources = make(map[ResourceType]float64)
	}

	current := i.Resources[res]
	limit, ok := i.StorageLimits[res]
	if !ok {
		// If no limit is defined (e.g. non-production resource or error),
		// we default to a safe-ish large number or just accept it?
		// Actually, in our SSOT, engine always populates StorageLimits.
		// Fallback to current if limit is missing to avoid unlimited growth.
		limit = 999999999
	}

	canAccept := limit - current
	if canAccept <= 0 {
		return 0
	}

	toAdd := amount
	if toAdd > canAccept {
		toAdd = canAccept
	}

	i.Resources[res] += toAdd
	return toAdd
}

// Fleet represents a group of ships
type Fleet struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	IslandID     uuid.UUID `json:"island_id" gorm:"type:uuid;index"`
	Island       Island    `json:"-" gorm:"foreignKey:IslandID"`
	Name         string    `json:"name"`
	Ships        []Ship    `json:"ships,omitempty" gorm:"foreignKey:FleetID"`
	MoraleCruise *int      `json:"morale_cruise,omitempty" gorm:"column:morale_cruise"` // Morale during cruise (0-100), NULL means uninitialized (defaults to 50)

	// Fleet lock (anti-exploit: prevents captain swap during engagement)
	LockedUntil *time.Time `json:"locked_until,omitempty" gorm:"column:locked_until"` // Nullable, locks fleet until this time

	// Flagship selection (deterministic and explicit)
	FlagshipShipID *uuid.UUID `json:"flagship_ship_id,omitempty" gorm:"type:uuid;index"` // Nullable, explicit flagship ship ID

	// Fleet state and movement
	State   FleetState `json:"state" gorm:"default:'Idle'"` // Idle, Moving, Stationed, Returning, Traveling_To_Attack, Returning_From_Attack, SeaStationed
	FreeNav bool       `json:"free_nav" gorm:"default:false"`
	TargetX *int       `json:"target_x,omitempty"`
	TargetY *int       `json:"target_y,omitempty"`

	// PvP Interception Tracking
	ChasingFleetID          *uuid.UUID `json:"chasing_fleet_id,omitempty" gorm:"type:uuid;index"`
	ChasedByFleetID         *uuid.UUID `json:"chased_by_fleet_id,omitempty" gorm:"type:uuid;index"`
	InterceptStartedAt      *time.Time `json:"intercept_started_at,omitempty" gorm:"column:intercept_started_at"`
	InterceptTargetPlayerID *uuid.UUID `json:"intercept_target_player_id,omitempty" gorm:"type:uuid;index"`

	// PVP fields (STUB - no logic implemented)
	TargetIslandID *uuid.UUID               `json:"target_island_id,omitempty" gorm:"type:uuid"`
	TargetPveID    *uuid.UUID               `json:"target_pve_id,omitempty" gorm:"type:uuid"` // Track specific NPC target (Phase K)
	AttackLootJSON []byte                   `json:"-" gorm:"column:attack_loot"`
	AttackLoot     map[ResourceType]float64 `json:"attack_loot,omitempty" gorm:"-"`

	// Stationing fields (STUB - no logic implemented)
	// Cargo Inventory (New System)
	CargoJSON []byte                   `json:"-" gorm:"column:cargo"`
	Cargo     map[ResourceType]float64 `json:"cargo" gorm:"-"`

	// Transient Capacity Fields (Computed in AfterFind)
	CargoCapacity float64 `json:"cargo_capacity" gorm:"-"`
	CargoUsed     float64 `json:"cargo_used" gorm:"-"`
	CargoFree     float64 `json:"cargo_free" gorm:"-"`

	// Stationing fields (DEPRECATED - Use Cargo instead, kept for migration)
	StationedAt     *time.Time `json:"stationed_at,omitempty"`
	StationedNodeID *string    `json:"stationed_node_id,omitempty"`
	StoredAmount    float64    `json:"stored_amount" gorm:"default:0"`    // DEPRECATED
	StoredResource  string     `json:"stored_resource" gorm:"default:''"` // DEPRECATED
}

// Fleet hooks for JSON serialization
// Fleet hooks for JSON serialization
func (f *Fleet) BeforeSave(tx *gorm.DB) (err error) {
	if f.AttackLoot != nil {
		data, err := json.Marshal(f.AttackLoot)
		if err != nil {
			return err
		}
		f.AttackLootJSON = data
	}
	// Serialize Cargo
	if f.Cargo != nil {
		data, err := json.Marshal(f.Cargo)
		if err != nil {
			return err
		}
		f.CargoJSON = data
	}
	return nil
}

func (f *Fleet) AfterFind(tx *gorm.DB) (err error) {
	// Deserialize AttackLoot
	if len(f.AttackLootJSON) > 0 {
		if err := json.Unmarshal(f.AttackLootJSON, &f.AttackLoot); err != nil {
			return err
		}
	}
	if f.AttackLoot == nil {
		f.AttackLoot = make(map[ResourceType]float64)
	}

	// Deserialize Cargo
	if len(f.CargoJSON) > 0 {
		if err := json.Unmarshal(f.CargoJSON, &f.Cargo); err != nil {
			return err
		}
	}
	if f.Cargo == nil {
		f.Cargo = make(map[ResourceType]float64)
	}

	// Runtime Migration: Move StoredAmount to Cargo if present
	if f.StoredAmount > 0 && f.StoredResource != "" {
		resType := ResourceType(f.StoredResource)
		f.Cargo[resType] += f.StoredAmount

		// Mark legacy fields as empty in struct (will be persisted on next save)
		// We do NOT save immediately here to avoid write amplification on every read
		f.StoredAmount = 0
		f.StoredResource = ""
	}

	return nil
}

// ComputePayload calculates transient cargo stats.
// Logic moved here to avoid circular dependency with economy package.
func (f *Fleet) ComputePayload() {
	capacity := 0.0
	for _, s := range f.Ships {
		cap := 0.0
		switch s.Type {
		case "sloop":
			cap = 500
		case "brigantine":
			cap = 1500
		case "frigate":
			cap = 3000
		case "galleon":
			cap = 8000
		case "manowar":
			cap = 12000
		default:
			cap = 500 // Fallback
		}
		capacity += cap
	}

	used := 0.0
	if f.Cargo != nil {
		for _, amount := range f.Cargo {
			used += amount
		}
	}

	free := capacity - used
	if free < 0 {
		free = 0
	}

	f.CargoCapacity = capacity
	f.CargoUsed = used
	f.CargoFree = free
}

// CargoLoaded returns the total amount of resources currently in cargo.
func (f *Fleet) CargoLoaded() float64 {
	used := 0.0
	if f.Cargo != nil {
		for _, amount := range f.Cargo {
			used += amount
		}
	}
	return used
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

// IsAtSea returns true if the fleet is not docked/idle at island
func (f *Fleet) IsAtSea() bool {
	return f.State != FleetStateIdle && f.State != FleetStateStationed
}

// ConsumesRum returns true if the fleet consumes rum in this state
func (f *Fleet) ConsumesRum() bool {
	// Moving, Returning, PvP Travel, Chasing_PvE, SeaStationed consume rum
	return f.IsMoving() || f.State == FleetStateSeaStationed
}

// BlocksOrders returns true if the fleet cannot accept new manual orders
func (f *Fleet) BlocksOrders() bool {
	// Cannot change orders if Chasing? Or if Traveling to Attack (Locked flow)?
	// Assuming Attack Flows blocks orders.
	// Chasing PvE usually allows user to Cancel? (User said "Chasing_PvE -> Returning OK")
	// So Chasing does NOT strictly block, but might need "Cancel" command explicitly.
	// Traveling_To_Attack IS locked.
	return f.State == FleetStateTravelingToAttack ||
		f.State == FleetStateReturningFromAttack
}

// CanTransitionTo checks if a state transition is valid (SSOT Guard)
func (f *Fleet) CanTransitionTo(next FleetState) bool {
	current := f.State

	// Identity is always OK (no-op)
	if current == next {
		return true
	}

	switch current {
	case FleetStateIdle:
		// Idle can go anywhere except Returning (makes no sense to return if idle)
		// But maybe ReturningFromAttack if setting up for anim?
		return true

	case FleetStateMoving:
		// Can stop (Idle), Arrive (Idle/Stationed?), Return, Chase?
		return true

	case FleetStateStationed:
		// Can only go to Moving (if valid order) or Idle (Recall)
		// User rule: "Stationed -> Moving OK"
		// Stationed -> Idle matches "Recall" (instant in V1?) or "Recall" triggers Return?
		// Usually Stationed -> Moving (Recall trip)
		return next == FleetStateMoving || next == FleetStateIdle

	case FleetStateChasingPvE, FleetStateChasingPvP:
		// Can match "Returning" (Abort/Lost)
		// Can match "Idle" (Cancel immediate?)
		// Can match "Moving" (Redirect?)
		// Can match "SeaStationed" (FreeNav fallback)
		return true

	case FleetStateTravelingToAttack:
		// LOCKED state. Can only go to Returning_From_Attack (Result) or Idle (Reset/Bug)
		return next == FleetStateReturningFromAttack || next == FleetStateIdle

	case FleetStateReturningFromAttack:
		// Can only go to Idle (Arrival)
		return next == FleetStateIdle

	case FleetStateSeaStationed:
		// Can go back to Moving (FreeNav order)
		return next == FleetStateMoving || next == FleetStateIdle
	}

	return true
}

// Building represents a structure on an island
type Building struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	IslandID     uuid.UUID `json:"island_id" gorm:"type:uuid;index"`
	Type         string    `json:"type"`
	Level        int       `json:"level"`
	X            float64   `json:"x"`
	Y            float64   `json:"y"`
	Constructing bool      `json:"constructing"`
	FinishTime   time.Time `json:"finish_time"`
}

func (i *Island) BeforeSave(tx *gorm.DB) (err error) {
	if i.Resources != nil {
		data, err := json.Marshal(i.Resources)
		if err != nil {
			return err
		}
		i.ResourcesJSON = data
	}
	if i.Crew != nil {
		data, err := json.Marshal(i.Crew)
		if err != nil {
			return err
		}
		i.CrewJSON = data
	}
	return nil
}

func (i *Island) AfterFind(tx *gorm.DB) (err error) {
	if len(i.ResourcesJSON) > 0 {
		if err := json.Unmarshal(i.ResourcesJSON, &i.Resources); err != nil {
			return err
		}
	} else {
		i.Resources = make(map[ResourceType]float64)
	}

	if len(i.CrewJSON) > 0 {
		if err := json.Unmarshal(i.CrewJSON, &i.Crew); err != nil {
			return err
		}
	} else {
		i.Crew = make(map[UnitType]int)
	}

	return nil
}

// --- Naval & Combat Models ---

type UnitType string

const (
	Warrior UnitType = "warrior"
	Archer  UnitType = "archer"
	Gunner  UnitType = "gunner"
)

// CaptainRarity represents the rarity level of a captain
type CaptainRarity string

const (
	RarityCommon    CaptainRarity = "common"
	RarityRare      CaptainRarity = "rare"
	RarityLegendary CaptainRarity = "legendary"
)

// Captain represents a ship captain owned by a player
type Captain struct {
	ID             uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;"`
	PlayerID       uuid.UUID     `json:"player_id" gorm:"type:uuid;index"`
	TemplateID     string        `json:"template_id"` // e.g. "black_gale", "red_isabella"
	Name           string        `json:"name"`
	Rarity         CaptainRarity `json:"rarity"` // common, rare, legendary
	Level          int           `json:"level" gorm:"default:1"`
	XP             int           `json:"xp" gorm:"default:0"`
	Stars          int           `json:"stars" gorm:"default:0"`                              // 0-based, max depends on rarity
	SkillID        string        `json:"skill_id"`                                            // identifier for the main skill
	AssignedShipID *uuid.UUID    `json:"assigned_ship_id" gorm:"type:uuid;index"`             // nullable, indexed
	InjuredUntil   *time.Time    `json:"injured_until,omitempty" gorm:"column:injured_until"` // Nullable, captain injury until this time
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// CaptainShardWallet stores shards per player per captain template
type CaptainShardWallet struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	PlayerID   uuid.UUID `json:"player_id" gorm:"type:uuid;uniqueIndex:idx_player_template"`
	TemplateID string    `json:"template_id" gorm:"uniqueIndex:idx_player_template"`
	Shards     int       `json:"shards" gorm:"default:0"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Ship struct {
	ID       uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	PlayerID uuid.UUID  `json:"player_id" gorm:"type:uuid;index"`
	IslandID uuid.UUID  `json:"island_id" gorm:"type:uuid;index"`
	FleetID  *uuid.UUID `json:"fleet_id" gorm:"type:uuid;index"`

	Name string `json:"name"`
	Type string `json:"type"`

	Health    float64 `json:"health"`
	MaxHealth float64 `json:"max_health"`

	State string  `json:"state"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`

	FinishTime time.Time `json:"finish_time"` // For construction/repair

	CaptainID *uuid.UUID `json:"captain_id" gorm:"type:uuid;index"`

	// Militia composition per ship (for RPS combat system)
	MilitiaWarriors int `json:"militia_warriors" gorm:"default:0"`
	MilitiaArchers  int `json:"militia_archers" gorm:"default:0"`
	MilitiaGunners  int `json:"militia_gunners" gorm:"default:0"`
	MilitiaCapacity int `json:"militia_capacity" gorm:"default:50"`

	// Legacy crew map (kept for backward compatibility, but RPS uses MilitiaWarriors/Archers/Gunners)
	CrewJSON []byte           `json:"-" gorm:"column:crew"`
	Crew     map[UnitType]int `json:"-" gorm:"-"`
}

func (s *Ship) BeforeSave(tx *gorm.DB) (err error) {
	if s.Crew != nil {
		data, err := json.Marshal(s.Crew)
		if err != nil {
			return err
		}
		s.CrewJSON = data
	}
	return nil
}

func (s *Ship) AfterFind(tx *gorm.DB) (err error) {
	if len(s.CrewJSON) > 0 {
		return json.Unmarshal(s.CrewJSON, &s.Crew)
	}
	if s.Crew == nil {
		s.Crew = make(map[UnitType]int)
	}
	return nil
}

// PvPInterceptCooldown tracks anti-harassment cooldowns between players
type PvPInterceptCooldown struct {
	ID               uint      `gorm:"primaryKey"`
	AttackerPlayerID uuid.UUID `gorm:"type:uuid;index:idx_attacker_target,unique"`
	TargetPlayerID   uuid.UUID `gorm:"type:uuid;index:idx_attacker_target,unique"`
	BlockedUntil     time.Time
}

func (PvPInterceptCooldown) TableName() string {
	return "pvp_intercept_cooldowns"
}

// Friend represents a friend relationship (unidirectional)
type Friend struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	PlayerID uuid.UUID `json:"player_id" gorm:"type:uuid;index:idx_friends_unique,unique"`
	FriendID uuid.UUID `json:"friend_id" gorm:"type:uuid;index:idx_friends_unique,unique"`
	CreatedAt time.Time `json:"created_at"`
}

func (Friend) TableName() string {
	return "friends"
}

// Guild represents a player-created guild
type Guild struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name     string    `json:"name" gorm:"uniqueIndex"`
	OwnerID  uuid.UUID `json:"owner_id" gorm:"type:uuid;index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Guild) TableName() string {
	return "guilds"
}

// GuildMember tracks membership in guilds with join date
type GuildMember struct {
	GuildID  uuid.UUID `json:"guild_id" gorm:"type:uuid;primaryKey;index"`
	PlayerID uuid.UUID `json:"player_id" gorm:"type:uuid;primaryKey;index"`
	JoinedAt time.Time `json:"joined_at"`
}

func (GuildMember) TableName() string {
	return "guild_members"
}

// PvEVictory records a player's victory against a PvE target (monster/NPC)
type PvEVictory struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	PlayerID  uuid.UUID `json:"player_id" gorm:"type:uuid;index"`
	Tier      int       `json:"tier"` // 1, 2, or 3 - determines reputation gain
	ReputationGain int   `json:"reputation_gain" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
}

func (PvEVictory) TableName() string {
	return "pve_victories"
}

// PvPVictory records a player's victory against another player
type PvPVictory struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	WinnerID        uuid.UUID `json:"winner_id" gorm:"type:uuid;index"`
	LoserID         uuid.UUID `json:"loser_id" gorm:"type:uuid;index"`
	WinnerRepGain   int       `json:"winner_rep_gain" gorm:"default:0"`
	LoserRepLoss    int       `json:"loser_rep_loss" gorm:"default:0"`
	WinnerRepBefore int       `json:"winner_rep_before" gorm:"default:0"` // For tracking progression
	LoserRepBefore  int       `json:"loser_rep_before" gorm:"default:0"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PvPVictory) TableName() string {
	return "pvp_victories"
}
