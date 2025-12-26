package dto

import "time"

// IslandDTO represents an island for API responses with string UUIDs
type IslandDTO struct {
	ID                    string                   `json:"id"`
	PlayerID              string                   `json:"player_id"`
	SeaID                 string                   `json:"sea_id"`
	Name                  string                   `json:"name"`
	Level                 int                      `json:"level"`
	X                     int                      `json:"x"`
	Y                     int                      `json:"y"`
	Resources             ResourceMap              `json:"resources"`
	StorageLimits         ResourceMap              `json:"storage_limits"`
	MilitiaStock          map[string]int           `json:"militia_stock"`
	Buildings             []BuildingDTO            `json:"buildings,omitempty"`
	Fleets                []FleetDTO               `json:"fleets,omitempty"`
	Ships                 []ShipDTO                `json:"ships,omitempty"`
	LastUpdated           time.Time                `json:"last_updated"`
	ProtectedUntil        *time.Time               `json:"protected_until,omitempty"`
	ActiveFleetID         *string                  `json:"active_fleet_id,omitempty"`
	ResourceGeneration    ResourceMap              `json:"resource_generation,omitempty"`
	ResourceGenerationBase ResourceMap             `json:"resource_generation_base,omitempty"`
	ResourceGenerationBonus ResourceMap            `json:"resource_generation_bonus,omitempty"`
	MilitiaRecruiting     bool                     `json:"militia_recruiting"`
	MilitiaRecruitDoneAt  *time.Time               `json:"militia_recruit_done_at,omitempty"`
	MilitiaRecruitWarriors int                    `json:"militia_recruit_warriors"`
	MilitiaRecruitArchers int                     `json:"militia_recruit_archers"`
	MilitiaRecruitGunners int                     `json:"militia_recruit_gunners"`
}

// BuildingDTO represents a building for API responses with string UUIDs
type BuildingDTO struct {
	ID           string    `json:"id"`
	IslandID     string    `json:"island_id"`
	Type         string    `json:"type"`
	Level        int       `json:"level"`
	X            float64   `json:"x"`
	Y            float64   `json:"y"`
	Constructing bool      `json:"constructing"`
	FinishTime   time.Time `json:"finish_time"`
}
