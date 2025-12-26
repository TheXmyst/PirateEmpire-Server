package dto

import "time"

// PlayerDTO represents a player for API responses with string UUIDs
type PlayerDTO struct {
	ID                           string    `json:"id"`
	Username                     string    `json:"username"`
	Email                        string    `json:"email"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
	Role                         string    `json:"role"`
	IsAdmin                      bool      `json:"is_admin"`
	UnlockedTechs                []string  `json:"unlocked_techs"`
	ResearchingTechID            string    `json:"researching_tech_id"`
	ResearchFinishTime           time.Time `json:"research_finish_time"`
	CurrentResearchTotalDuration float64   `json:"current_research_total_duration_seconds"`
	PityLegendaryCount           int       `json:"pity_legendary_count"`
	PityRareCount                int       `json:"pity_rare_count"`
	LastResetAt                  *time.Time `json:"last_reset_at,omitempty"`
	DailyShardExchangeCount      int       `json:"daily_shard_exchange_count"`
	DailyShardExchangeDay        string    `json:"daily_shard_exchange_day"`
}

// StatusResponse represents the full response from GetStatus endpoint
type StatusResponse struct {
	Player   PlayerDTO    `json:"player"`
	Islands  []IslandDTO  `json:"islands,omitempty"`
	Captains []CaptainDTO `json:"captains,omitempty"`
}
