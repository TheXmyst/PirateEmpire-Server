package domain

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"

	"github.com/google/uuid"
)

// ToDTO converts a domain Fleet to a FleetDTO
func (f *Fleet) ToDTO() *dto.FleetDTO {
	if f == nil {
		return nil
	}

	// Convert ResourceType maps to string maps
	attackLootDTO := make(dto.ResourceMap)
	for k, v := range f.AttackLoot {
		attackLootDTO[string(k)] = v
	}

	cargoDTO := make(dto.ResourceMap)
	for k, v := range f.Cargo {
		cargoDTO[string(k)] = v
	}

	fleetDTO := &dto.FleetDTO{
		ID:                      f.ID.String(),
		IslandID:                f.IslandID.String(),
		Name:                    f.Name,
		MoraleCruise:            f.MoraleCruise,
		LockedUntil:             f.LockedUntil,
		State:                   string(f.State),
		FreeNav:                 f.FreeNav,
		TargetX:                 f.TargetX,
		TargetY:                 f.TargetY,
		InterceptStartedAt:      f.InterceptStartedAt,
		TargetIslandID:          nil,
		TargetPveID:             nil,
		AttackLoot:              attackLootDTO,
		Cargo:                   cargoDTO,
		CargoCapacity:           f.CargoCapacity,
		CargoUsed:               f.CargoUsed,
		CargoFree:               f.CargoFree,
		StationedAt:             f.StationedAt,
		StationedNodeID:         f.StationedNodeID,
		StoredAmount:            f.StoredAmount,
		StoredResource:          f.StoredResource,
	}

	// Convert ships to DTOs
	if f.Ships != nil && len(f.Ships) > 0 {
		shipDTOs := make([]dto.ShipDTO, len(f.Ships))
		for i, ship := range f.Ships {
			if shipDTO := ship.ToDTO(); shipDTO != nil {
				shipDTOs[i] = *shipDTO
			}
		}
		fleetDTO.Ships = shipDTOs
	}

	if f.FlagshipShipID != nil {
		flagshipStr := f.FlagshipShipID.String()
		fleetDTO.FlagshipShipID = &flagshipStr
	}

	if f.ChasingFleetID != nil {
		chasingStr := f.ChasingFleetID.String()
		fleetDTO.ChasingFleetID = &chasingStr
	}

	if f.ChasedByFleetID != nil {
		chasedStr := f.ChasedByFleetID.String()
		fleetDTO.ChasedByFleetID = &chasedStr
	}

	if f.InterceptTargetPlayerID != nil {
		targetStr := f.InterceptTargetPlayerID.String()
		fleetDTO.InterceptTargetPlayerID = &targetStr
	}

	if f.TargetIslandID != nil {
		targetIslandStr := f.TargetIslandID.String()
		fleetDTO.TargetIslandID = &targetIslandStr
	}

	if f.TargetPveID != nil {
		targetPveStr := f.TargetPveID.String()
		fleetDTO.TargetPveID = &targetPveStr
	}

	return fleetDTO
}

// FromDTO converts a FleetDTO to a domain Fleet
func (f *Fleet) FromDTO(fleetDTO *dto.FleetDTO) error {
	if fleetDTO == nil {
		return nil
	}

	id, err := uuid.Parse(fleetDTO.ID)
	if err != nil {
		return err
	}

	islandID, err := uuid.Parse(fleetDTO.IslandID)
	if err != nil {
		return err
	}

	// Convert string maps back to ResourceType maps
	attackLootDomain := make(map[ResourceType]float64)
	for k, v := range fleetDTO.AttackLoot {
		attackLootDomain[ResourceType(k)] = v
	}

	cargoDomain := make(map[ResourceType]float64)
	for k, v := range fleetDTO.Cargo {
		cargoDomain[ResourceType(k)] = v
	}

	f.ID = id
	f.IslandID = islandID
	f.Name = fleetDTO.Name
	f.MoraleCruise = fleetDTO.MoraleCruise
	f.LockedUntil = fleetDTO.LockedUntil
	f.State = FleetState(fleetDTO.State)
	f.FreeNav = fleetDTO.FreeNav
	f.TargetX = fleetDTO.TargetX
	f.TargetY = fleetDTO.TargetY
	f.InterceptStartedAt = fleetDTO.InterceptStartedAt
	f.AttackLoot = attackLootDomain
	f.Cargo = cargoDomain
	f.CargoCapacity = fleetDTO.CargoCapacity
	f.CargoUsed = fleetDTO.CargoUsed
	f.CargoFree = fleetDTO.CargoFree
	f.StationedAt = fleetDTO.StationedAt
	f.StationedNodeID = fleetDTO.StationedNodeID
	f.StoredAmount = fleetDTO.StoredAmount
	f.StoredResource = fleetDTO.StoredResource

	if fleetDTO.FlagshipShipID != nil {
		flagshipID, err := uuid.Parse(*fleetDTO.FlagshipShipID)
		if err != nil {
			return err
		}
		f.FlagshipShipID = &flagshipID
	}

	if fleetDTO.ChasingFleetID != nil {
		chasingID, err := uuid.Parse(*fleetDTO.ChasingFleetID)
		if err != nil {
			return err
		}
		f.ChasingFleetID = &chasingID
	}

	if fleetDTO.ChasedByFleetID != nil {
		chasedID, err := uuid.Parse(*fleetDTO.ChasedByFleetID)
		if err != nil {
			return err
		}
		f.ChasedByFleetID = &chasedID
	}

	if fleetDTO.InterceptTargetPlayerID != nil {
		targetID, err := uuid.Parse(*fleetDTO.InterceptTargetPlayerID)
		if err != nil {
			return err
		}
		f.InterceptTargetPlayerID = &targetID
	}

	if fleetDTO.TargetIslandID != nil {
		targetIslandID, err := uuid.Parse(*fleetDTO.TargetIslandID)
		if err != nil {
			return err
		}
		f.TargetIslandID = &targetIslandID
	}

	if fleetDTO.TargetPveID != nil {
		targetPveID, err := uuid.Parse(*fleetDTO.TargetPveID)
		if err != nil {
			return err
		}
		f.TargetPveID = &targetPveID
	}

	return nil
}
