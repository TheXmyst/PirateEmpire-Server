package domain

import (
	"encoding/json"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
)

// ToDTO converts a domain Player to a PlayerDTO
func (p *Player) ToDTO() *dto.PlayerDTO {
	if p == nil {
		return nil
	}

	// Ensure UnlockedTechs are populated; fallback to JSON if hook didn't hydrate.
	unlockedTechs := p.UnlockedTechs
	if len(unlockedTechs) == 0 && len(p.UnlockedTechsJSON) > 0 {
		var techs []string
		if err := json.Unmarshal(p.UnlockedTechsJSON, &techs); err == nil {
			unlockedTechs = techs
		}
	}
	if unlockedTechs == nil {
		unlockedTechs = []string{}
	}

	return &dto.PlayerDTO{
		ID:                           p.ID.String(),
		Username:                     p.Username,
		Email:                        p.Email,
		CreatedAt:                    p.CreatedAt,
		UpdatedAt:                    p.UpdatedAt,
		Role:                         p.Role,
		IsAdmin:                      p.IsAdmin,
		UnlockedTechs:                unlockedTechs,
		ResearchingTechID:            p.ResearchingTechID,
		ResearchFinishTime:           p.ResearchFinishTime,
		CurrentResearchTotalDuration: p.ResearchTotalDurationSeconds,
		PityLegendaryCount:           p.PityLegendaryCount,
		PityRareCount:                p.PityRareCount,
		LastResetAt:                  p.LastResetAt,
		DailyShardExchangeCount:      p.DailyShardExchangeCount,
		DailyShardExchangeDay:        p.DailyShardExchangeDay,
	}
}
