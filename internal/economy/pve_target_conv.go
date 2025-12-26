package economy

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/google/uuid"
)

// ToDTO converts a PveTarget to a PveTargetDTO with string UUIDs
func (pt *PveTarget) ToDTO() *dto.PveTargetDTO {
	if pt == nil {
		return nil
	}

	pveDTO := &dto.PveTargetDTO{
		ID:           pt.ID.String(),
		LegacyID:     pt.LegacyID,
		X:            pt.X,
		Y:            pt.Y,
		Tier:         pt.Tier,
		Name:         pt.Name,
		RealX:        pt.RealX,
		RealY:        pt.RealY,
		TargetX:      pt.TargetX,
		TargetY:      pt.TargetY,
		Speed:        pt.Speed,
		NextChangeAt: pt.NextChangeAt,
	}

	// Convert captain if present
	if pt.Captain.ID != (uuid.UUID{}) {
		captainDTO := pt.Captain.ToDTO()
		pveDTO.Captain = &captainDTO
	}

	return pveDTO
}
