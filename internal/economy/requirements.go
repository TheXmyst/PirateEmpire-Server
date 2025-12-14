package economy

import (
	"encoding/json"
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// CheckBuildingLevel checks if a building meets the required level
// Returns (ok bool, req Requirement)
func CheckBuildingLevel(island *domain.Island, buildingType string, needed int) (ok bool, req domain.Requirement) {
	current := 0
	found := false
	constructing := false

	for _, b := range island.Buildings {
		if b.Type == buildingType {
			found = true
			current = b.Level
			constructing = b.Constructing
			break
		}
	}

	if !found {
		req = domain.Requirement{
			Kind:    "building_level",
			ID:      buildingType,
			Name:    DisplayNameBuilding(buildingType),
			Needed:  needed,
			Current: 0,
			Message: fmt.Sprintf("%s niveau %d requis (non construit)", DisplayNameBuilding(buildingType), needed),
		}
		return false, req
	}

	if constructing {
		req = domain.Requirement{
			Kind:    "building_level",
			ID:      buildingType,
			Name:    DisplayNameBuilding(buildingType),
			Needed:  needed,
			Current: current,
			Message: fmt.Sprintf("%s niveau %d requis (actuellement en construction)", DisplayNameBuilding(buildingType), needed),
		}
		return false, req
	}

	if current < needed {
		req = domain.Requirement{
			Kind:    "building_level",
			ID:      buildingType,
			Name:    DisplayNameBuilding(buildingType),
			Needed:  needed,
			Current: current,
			Message: fmt.Sprintf("%s niveau %d requis (actuel: %d)", DisplayNameBuilding(buildingType), needed, current),
		}
		return false, req
	}

	return true, domain.Requirement{}
}

// CheckTechUnlocked checks if a technology is unlocked
// Returns (ok bool, req Requirement)
func CheckTechUnlocked(player *domain.Player, techID string) (ok bool, req domain.Requirement) {
	var unlocked []string
	if len(player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(player.UnlockedTechsJSON, &unlocked)
	}

	hasTech := false
	for _, id := range unlocked {
		if id == techID {
			hasTech = true
			break
		}
	}

	if !hasTech {
		req = domain.Requirement{
			Kind:    "tech",
			ID:      techID,
			Name:    DisplayNameTech(techID),
			Needed:  1,
			Current: 0,
			Message: fmt.Sprintf("Technologie requise: %s (non recherchée)", DisplayNameTech(techID)),
		}
		return false, req
	}

	return true, domain.Requirement{}
}

// CheckResource checks if the island has enough of a resource
// Returns (ok bool, req Requirement)
func CheckResource(island *domain.Island, resType string, amount float64) (ok bool, req domain.Requirement) {
	current := island.Resources[domain.ResourceType(resType)]
	if current < amount {
		req = domain.Requirement{
			Kind:    "resource",
			ID:      resType,
			Name:    DisplayNameResource(resType),
			Needed:  int(amount),
			Current: int(current),
			Message: fmt.Sprintf("%s insuffisant: besoin de %.0f, avez %.0f", DisplayNameResource(resType), amount, current),
		}
		return false, req
	}
	return true, domain.Requirement{}
}

// ValidateBuildingPrerequisites validates all prerequisites for building/upgrading a building
func ValidateBuildingPrerequisites(player *domain.Player, island *domain.Island, buildingType string, targetLevel int) []domain.Requirement {
	var missing []domain.Requirement

	// Load building config
	config, ok := GetBuildingConfig(buildingType)
	if !ok {
		// Building type not found - return generic error
		missing = append(missing, domain.Requirement{
			Kind:    "other",
			ID:      buildingType,
			Name:    buildingType,
			Message: fmt.Sprintf("Type de bâtiment invalide: %s", buildingType),
		})
		return missing
	}

	// Check prerequisites from config
	for _, prereq := range config.Prerequisites {
		if prereq.Level == targetLevel {
		// Check TownHall level if required
		if prereq.ReqTownHall > 0 {
			ok, req := CheckBuildingLevel(island, "Hôtel de Ville", prereq.ReqTownHall)
			if !ok {
				missing = append(missing, req)
			}
		}

			// Check building level if required
			if prereq.ReqBuilding != "" && prereq.ReqMinLevel > 0 {
				ok, req := CheckBuildingLevel(island, prereq.ReqBuilding, prereq.ReqMinLevel)
				if !ok {
					missing = append(missing, req)
				}
			}

			// Check technology if required
			if prereq.ReqTech != "" {
				ok, req := CheckTechUnlocked(player, prereq.ReqTech)
				if !ok {
					missing = append(missing, req)
				}
			}
		}
	}

	return missing
}

// ValidateShipPrerequisites validates prerequisites for building a ship
func ValidateShipPrerequisites(player *domain.Player, island *domain.Island, shipType string) []domain.Requirement {
	var missing []domain.Requirement

	// Get ship config
	config, err := GetShipStats(shipType)
	if err != nil {
		missing = append(missing, domain.Requirement{
			Kind:    "other",
			ID:      shipType,
			Name:    DisplayNameShipType(shipType),
			Message: fmt.Sprintf("Type de navire invalide: %s", shipType),
		})
		return missing
	}

	// Check Shipyard level
	if config.RequiredShipyardLevel > 0 {
		ok, req := CheckBuildingLevel(island, "Chantier Naval", config.RequiredShipyardLevel)
		if !ok {
			missing = append(missing, req)
		}
	}

	// Check technology
	if config.RequiredTechID != "" {
		ok, req := CheckTechUnlocked(player, config.RequiredTechID)
		if !ok {
			missing = append(missing, req)
		}
	}

	return missing
}

// ValidateResearchPrerequisites validates prerequisites for starting a research
func ValidateResearchPrerequisites(player *domain.Player, island *domain.Island, techID string) []domain.Requirement {
	var missing []domain.Requirement

	// Get tech config
	tech, err := GetTech(techID)
	if err != nil {
		missing = append(missing, domain.Requirement{
			Kind:    "other",
			ID:      techID,
			Name:    techID,
			Message: fmt.Sprintf("Technologie invalide: %s", techID),
		})
		return missing
	}

	// Check TownHall level if required
	if tech.ReqTH > 0 {
		ok, req := CheckBuildingLevel(island, "Hôtel de Ville", tech.ReqTH)
		if !ok {
			missing = append(missing, req)
		}
	}

	// Check Academy level if required
	if tech.ReqAcad > 0 {
		ok, req := CheckBuildingLevel(island, "Académie", tech.ReqAcad)
		if !ok {
			missing = append(missing, req)
		}
	}

	// Note: Technology prerequisites are not stored in the Tech struct currently
	// If needed, they would be checked here

	return missing
}

// ValidateMilitiaRecruitPrerequisites validates prerequisites for recruiting crew
func ValidateMilitiaRecruitPrerequisites(island *domain.Island) []domain.Requirement {
	var missing []domain.Requirement

	// Check if Militia exists (level 1 minimum)
	ok, req := CheckBuildingLevel(island, "Milice", 1)
	if !ok {
		missing = append(missing, req)
	}

	return missing
}

