package dto

import "time"

// ShipDTO represents a ship for API responses with string UUIDs
type ShipDTO struct {
	ID              string     `json:"id"`
	PlayerID        string     `json:"player_id"`
	IslandID        string     `json:"island_id"`
	FleetID         *string    `json:"fleet_id,omitempty"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Health          float64    `json:"health"`
	MaxHealth       float64    `json:"max_health"`
	State           string     `json:"state"`
	X               float64    `json:"x"`
	Y               float64    `json:"y"`
	FinishTime      time.Time  `json:"finish_time"`
	CaptainID       *string    `json:"captain_id,omitempty"`
	MilitiaWarriors int        `json:"militia_warriors"`
	MilitiaArchers  int        `json:"militia_archers"`
	MilitiaGunners  int        `json:"militia_gunners"`
	MilitiaCapacity int        `json:"militia_capacity"`
}
