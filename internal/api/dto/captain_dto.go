package dto

import (
    "time"
)

// CaptainDTO is the API representation sent to the client.
// All UUIDs are serialized as strings to match client expectations.
type CaptainDTO struct {
    ID             string     `json:"id"`
    PlayerID       string     `json:"player_id"`
    TemplateID     string     `json:"template_id"`
    Name           string     `json:"name"`
    Rarity         string     `json:"rarity"`
    Level          int        `json:"level"`
    XP             int        `json:"xp"`
    Stars          int        `json:"stars"`
    SkillID        string     `json:"skill_id"`
    AssignedShipID *string    `json:"assigned_ship_id,omitempty"`
    InjuredUntil   *time.Time `json:"injured_until,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at"`
}
