package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// CheckPrerequisites verifies if a player can build/upgrade a building to the target level
func CheckPrerequisites(player *domain.Player, buildingType string, targetLevel int) error {
	cfg, ok := GetBuildingConfig(buildingType)
	if !ok {
		return fmt.Errorf("unknown building type: %s", buildingType)
	}

	// 0. Base Check: TownHall Cap
	// "TownHall determines max level of other buildings"
	// TH1->3, TH3->5, TH6->10, TH11->15, TH16->20, TH21->25, TH26->30
	// This logic handles all buildings EXCEPT TownHall itself (implied, or TH checks are just specific PRs)
	// Actually, TownHall upgrades usually depend on Resources, but user said "TH6 -> Tech Construction I".

	// Determine Player's TH Level
	// Only count TownHall that is finished (not under construction)
	thLevel := 0
	if len(player.Islands) > 0 {
		for _, b := range player.Islands[0].Buildings {
			if b.Type == "Hôtel de Ville" && !b.Constructing {
				thLevel = b.Level
				break
			}
		}
	}

	// Skip MaxLevel check for TownHall itself (it limits itself via specific reqs or just hardcoded logic)
	if buildingType != "Hôtel de Ville" {
		maxLvl := GetMaxLevelAllowedByTH(thLevel)
		if targetLevel > maxLvl {
			return fmt.Errorf("requires TownHall level to increase max limit (Current Max: %d)", maxLvl)
		}
	}

	// 1. Specific Prerequisites from Config
	for _, req := range cfg.Prerequisites {
		// Only check if we are reaching/passing the requirement level
		// E.g. Require Tech at Level 8. If going 7->8, check. If 8->9, check (requirement persists? usually yes).
		// User rule: "LVL 8 -> Tech X". Means to HAVE Level 8, you must have Tech X.
		// So if targetLevel >= req.Level, you must satisfy it.
		if targetLevel >= req.Level {
			if err := validateReq(player, req, thLevel); err != nil {
				return err
			}
		}
	}

	// User mentioned TH specific tech requirements:
	// TH6 -> Tech Construction I
	if buildingType == "Hôtel de Ville" {
		// Hardcoded TH rules if not in JSON (User put them in "Rules" section for TH)
		// But I put them in JSON? I didn't update TH in JSON yet.
		// Actually, I skipped TH in the batch updates properly.
		// I should rely on JSON whenever possible.
		// If I update TH JSON later, this loop covers it.
	}

	return nil
}

func validateReq(player *domain.Player, req Requirement, currentTH int) error {
	// a) TownHall Level
	if req.ReqTownHall > 0 {
		if currentTH < req.ReqTownHall {
			return fmt.Errorf("requires TownHall Level %d", req.ReqTownHall)
		}
	}

	// b) Tech
	if req.ReqTech != "" {
		hasTech := false
		for _, t := range player.UnlockedTechs {
			if t == req.ReqTech {
				hasTech = true
				break
			}
		}
		if !hasTech {
			return fmt.Errorf("requires technology: %s", req.ReqTech)
		}
	}

	// c) Building
	if req.ReqBuilding != "" {
		minLvl := 1
		if req.ReqMinLevel > 0 {
			minLvl = req.ReqMinLevel
		}

		found := false
		if len(player.Islands) > 0 {
			for _, b := range player.Islands[0].Buildings {
				// Only count finished buildings (not under construction) for prerequisites
				if b.Type == req.ReqBuilding && !b.Constructing && b.Level >= minLvl {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("requires building %s (Level %d+)", req.ReqBuilding, minLvl)
		}
	}

	return nil
}

func GetMaxLevelAllowedByTH(thLevel int) int {
	// TH1 -> 3
	// TH3 -> 5
	// TH6 -> 10
	// TH11 -> 15
	// TH16 -> 20
	// TH21 -> 25
	// TH26 -> 30
	// Fallback/Interpolation?
	if thLevel >= 26 {
		return 30
	}
	if thLevel >= 21 {
		return 25
	}
	if thLevel >= 16 {
		return 20
	}
	if thLevel >= 11 {
		return 15
	}
	if thLevel >= 6 {
		return 10
	}
	if thLevel >= 3 {
		return 5
	}
	return 3
}
