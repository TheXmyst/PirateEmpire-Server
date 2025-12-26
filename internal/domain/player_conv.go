package domain

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
)

// ToDTO converts a domain Player to a PlayerDTO
func (p *Player) ToDTO() *dto.PlayerDTO {
	if p == nil {
		return nil
	}

	return &dto.PlayerDTO{
		ID:                           p.ID.String(),
		Username:                     p.Username,
		Email:                        p.Email,
		CreatedAt:                    p.CreatedAt,
		UpdatedAt:                    p.UpdatedAt,
		Role:                         p.Role,
		IsAdmin:                      p.IsAdmin,
		UnlockedTechs:                p.UnlockedTechs,
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
