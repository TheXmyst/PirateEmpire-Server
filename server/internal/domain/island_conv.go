package domain

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"

	"github.com/google/uuid"
)

// ToDTO converts a domain Island to an IslandDTO
func (i *Island) ToDTO() *dto.IslandDTO {
	if i == nil {
		return nil
	}

	// Convert ResourceType maps to string maps
	resourcesDTO := make(dto.ResourceMap)
	for k, v := range i.Resources {
		resourcesDTO[string(k)] = v
	}

	storageLimitsDTO := make(dto.ResourceMap)
	for k, v := range i.StorageLimits {
		storageLimitsDTO[string(k)] = v
	}

	resourceGenDTO := make(dto.ResourceMap)
	for k, v := range i.ResourceGeneration {
		resourceGenDTO[string(k)] = v
	}

	resourceGenBaseDTO := make(dto.ResourceMap)
	for k, v := range i.ResourceGenerationBase {
		resourceGenBaseDTO[string(k)] = v
	}

	resourceGenBonusDTO := make(dto.ResourceMap)
	for k, v := range i.ResourceGenerationBonus {
		resourceGenBonusDTO[string(k)] = v
	}

	islandDTO := &dto.IslandDTO{
		ID:                     i.ID.String(),
		PlayerID:               i.PlayerID.String(),
		SeaID:                  i.SeaID.String(),
		Name:                   i.Name,
		Level:                  i.Level,
		X:                      i.X,
		Y:                      i.Y,
		Resources:              resourcesDTO,
		StorageLimits:          storageLimitsDTO,
		MilitiaStock:           convertCrewMap(i.Crew),
		LastUpdated:            i.LastUpdated,
		ProtectedUntil:         i.ProtectedUntil,
		ResourceGeneration:     resourceGenDTO,
		ResourceGenerationBase: resourceGenBaseDTO,
		ResourceGenerationBonus: resourceGenBonusDTO,
		MilitiaRecruiting:      i.MilitiaRecruiting,
		MilitiaRecruitDoneAt:   i.MilitiaRecruitDoneAt,
		MilitiaRecruitWarriors: i.MilitiaRecruitWarriors,
		MilitiaRecruitArchers:  i.MilitiaRecruitArchers,
		MilitiaRecruitGunners:  i.MilitiaRecruitGunners,
	}

	// Convert buildings to DTOs
	if i.Buildings != nil && len(i.Buildings) > 0 {
		buildingDTOs := make([]dto.BuildingDTO, len(i.Buildings))
		for j, building := range i.Buildings {
			buildingDTOs[j] = building.ToDTO()
		}
		islandDTO.Buildings = buildingDTOs
	}

	// Convert fleets to DTOs
	if i.Fleets != nil && len(i.Fleets) > 0 {
		fleetDTOs := make([]dto.FleetDTO, len(i.Fleets))
		for j, fleet := range i.Fleets {
			if fleetDTO := fleet.ToDTO(); fleetDTO != nil {
				fleetDTOs[j] = *fleetDTO
			}
		}
		islandDTO.Fleets = fleetDTOs
	}

	// Convert ships to DTOs
	if i.Ships != nil && len(i.Ships) > 0 {
		shipDTOs := make([]dto.ShipDTO, len(i.Ships))
		for j, ship := range i.Ships {
			if shipDTO := ship.ToDTO(); shipDTO != nil {
				shipDTOs[j] = *shipDTO
			}
		}
		islandDTO.Ships = shipDTOs
	}

	if i.ActiveFleetID != nil {
		activeFleetStr := i.ActiveFleetID.String()
		islandDTO.ActiveFleetID = &activeFleetStr
	}

	return islandDTO
}

// FromDTO converts an IslandDTO to a domain Island
func (i *Island) FromDTO(islandDTO *dto.IslandDTO) error {
	if islandDTO == nil {
		return nil
	}

	id, err := uuid.Parse(islandDTO.ID)
	if err != nil {
		return err
	}

	playerID, err := uuid.Parse(islandDTO.PlayerID)
	if err != nil {
		return err
	}

	seaID, err := uuid.Parse(islandDTO.SeaID)
	if err != nil {
		return err
	}

	i.ID = id
	i.PlayerID = playerID
	i.SeaID = seaID
	i.Name = islandDTO.Name
	i.Level = islandDTO.Level
	i.X = islandDTO.X
	i.Y = islandDTO.Y

	// Convert string maps back to ResourceType maps
	resourcesDomain := make(map[ResourceType]float64)
	for k, v := range islandDTO.Resources {
		resourcesDomain[ResourceType(k)] = v
	}

	storageLimitsDomain := make(map[ResourceType]float64)
	for k, v := range islandDTO.StorageLimits {
		storageLimitsDomain[ResourceType(k)] = v
	}

	resourceGenDomain := make(map[ResourceType]float64)
	for k, v := range islandDTO.ResourceGeneration {
		resourceGenDomain[ResourceType(k)] = v
	}

	resourceGenBaseDomain := make(map[ResourceType]float64)
	for k, v := range islandDTO.ResourceGenerationBase {
		resourceGenBaseDomain[ResourceType(k)] = v
	}

	resourceGenBonusDomain := make(map[ResourceType]float64)
	for k, v := range islandDTO.ResourceGenerationBonus {
		resourceGenBonusDomain[ResourceType(k)] = v
	}

	i.Resources = resourcesDomain
	i.StorageLimits = storageLimitsDomain
	i.Crew = convertStringMapToCrewMap(islandDTO.MilitiaStock)
	i.LastUpdated = islandDTO.LastUpdated
	i.ProtectedUntil = islandDTO.ProtectedUntil
	i.ResourceGeneration = resourceGenDomain
	i.ResourceGenerationBase = resourceGenBaseDomain
	i.ResourceGenerationBonus = resourceGenBonusDomain
	i.MilitiaRecruiting = islandDTO.MilitiaRecruiting
	i.MilitiaRecruitDoneAt = islandDTO.MilitiaRecruitDoneAt
	i.MilitiaRecruitWarriors = islandDTO.MilitiaRecruitWarriors
	i.MilitiaRecruitArchers = islandDTO.MilitiaRecruitArchers
	i.MilitiaRecruitGunners = islandDTO.MilitiaRecruitGunners

	if islandDTO.ActiveFleetID != nil {
		activeFleetID, err := uuid.Parse(*islandDTO.ActiveFleetID)
		if err != nil {
			return err
		}
		i.ActiveFleetID = &activeFleetID
	}

	return nil
}

// ToDTO converts a domain Building to a BuildingDTO
func (b *Building) ToDTO() dto.BuildingDTO {
	return dto.BuildingDTO{
		ID:           b.ID.String(),
		IslandID:     b.IslandID.String(),
		Type:         b.Type,
		Level:        b.Level,
		X:            b.X,
		Y:            b.Y,
		Constructing: b.Constructing,
		FinishTime:   b.FinishTime,
	}
}

// Helper function to convert UnitType map to string map
func convertCrewMap(crewMap map[UnitType]int) map[string]int {
	result := make(map[string]int)
	for unitType, count := range crewMap {
		result[string(unitType)] = count
	}
	return result
}

// Helper function to convert string map to UnitType map
func convertStringMapToCrewMap(stringMap map[string]int) map[UnitType]int {
	result := make(map[UnitType]int)
	for unitStr, count := range stringMap {
		result[UnitType(unitStr)] = count
	}
	return result
}
