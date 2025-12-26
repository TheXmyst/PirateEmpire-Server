package dto

import "time"

// ResourceMap represents a map of resource types to amounts
type ResourceMap map[string]float64

// FleetDTO represents a fleet for API responses with string UUIDs
type FleetDTO struct {
	ID                      string                 `json:"id"`
	IslandID                string                 `json:"island_id"`
	Name                    string                 `json:"name"`
	Ships                   []ShipDTO              `json:"ships,omitempty"`
	MoraleCruise            *int                   `json:"morale_cruise,omitempty"`
	LockedUntil             *time.Time             `json:"locked_until,omitempty"`
	FlagshipShipID          *string                `json:"flagship_ship_id,omitempty"`
	State                   string                 `json:"state"`
	FreeNav                 bool                   `json:"free_nav"`
	TargetX                 *int                   `json:"target_x,omitempty"`
	TargetY                 *int                   `json:"target_y,omitempty"`
	ChasingFleetID          *string                `json:"chasing_fleet_id,omitempty"`
	ChasedByFleetID         *string                `json:"chased_by_fleet_id,omitempty"`
	InterceptStartedAt      *time.Time             `json:"intercept_started_at,omitempty"`
	InterceptTargetPlayerID *string                `json:"intercept_target_player_id,omitempty"`
	TargetIslandID          *string                `json:"target_island_id,omitempty"`
	TargetPveID             *string                `json:"target_pve_id,omitempty"`
	AttackLoot              ResourceMap            `json:"attack_loot,omitempty"`
	Cargo                   ResourceMap            `json:"cargo"`
	CargoCapacity           float64                `json:"cargo_capacity"`
	CargoUsed               float64                `json:"cargo_used"`
	CargoFree               float64                `json:"cargo_free"`
	StationedAt             *time.Time             `json:"stationed_at,omitempty"`
	StationedNodeID         *string                `json:"stationed_node_id,omitempty"`
	StoredAmount            float64                `json:"stored_amount"`
	StoredResource          string                 `json:"stored_resource"`
}
