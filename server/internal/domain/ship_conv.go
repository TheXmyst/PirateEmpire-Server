package domain

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"

	"github.com/google/uuid"
)

// ToDTO converts a domain Ship to a ShipDTO
func (s *Ship) ToDTO() *dto.ShipDTO {
	if s == nil {
		return nil
	}

	shipDTO := &dto.ShipDTO{
		ID:              s.ID.String(),
		PlayerID:        s.PlayerID.String(),
		IslandID:        s.IslandID.String(),
		Name:            s.Name,
		Type:            s.Type,
		Health:          s.Health,
		MaxHealth:       s.MaxHealth,
		State:           s.State,
		X:               s.X,
		Y:               s.Y,
		FinishTime:      s.FinishTime,
		MilitiaWarriors: s.MilitiaWarriors,
		MilitiaArchers:  s.MilitiaArchers,
		MilitiaGunners:  s.MilitiaGunners,
		MilitiaCapacity: s.MilitiaCapacity,
	}

	if s.FleetID != nil {
		fleetIDStr := s.FleetID.String()
		shipDTO.FleetID = &fleetIDStr
	}

	if s.CaptainID != nil {
		captainIDStr := s.CaptainID.String()
		shipDTO.CaptainID = &captainIDStr
	}

	return shipDTO
}

// FromDTO converts a ShipDTO to a domain Ship
func (s *Ship) FromDTO(shipDTO *dto.ShipDTO) error {
	if shipDTO == nil {
		return nil
	}

	id, err := uuid.Parse(shipDTO.ID)
	if err != nil {
		return err
	}

	playerID, err := uuid.Parse(shipDTO.PlayerID)
	if err != nil {
		return err
	}

	islandID, err := uuid.Parse(shipDTO.IslandID)
	if err != nil {
		return err
	}

	s.ID = id
	s.PlayerID = playerID
	s.IslandID = islandID
	s.Name = shipDTO.Name
	s.Type = shipDTO.Type
	s.Health = shipDTO.Health
	s.MaxHealth = shipDTO.MaxHealth
	s.State = shipDTO.State
	s.X = shipDTO.X
	s.Y = shipDTO.Y
	s.FinishTime = shipDTO.FinishTime
	s.MilitiaWarriors = shipDTO.MilitiaWarriors
	s.MilitiaArchers = shipDTO.MilitiaArchers
	s.MilitiaGunners = shipDTO.MilitiaGunners
	s.MilitiaCapacity = shipDTO.MilitiaCapacity

	if shipDTO.FleetID != nil {
		fleetID, err := uuid.Parse(*shipDTO.FleetID)
		if err != nil {
			return err
		}
		s.FleetID = &fleetID
	}

	if shipDTO.CaptainID != nil {
		captainID, err := uuid.Parse(*shipDTO.CaptainID)
		if err != nil {
			return err
		}
		s.CaptainID = &captainID
	}

	return nil
}
