package economy

import (
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
)

// PvP Constants
const (
	MinTownHallForPvP = 3             // Beginner protection until TH 3
	PvpSearchRadius   = 500           // Search radius in map units
	PvpPeaceDuration  = 4 * time.Hour // Shield duration after defeat
)

// PvpTarget represents an attackable player
type PvpTarget struct {
	PlayerID    uuid.UUID `json:"player_id"`
	IslandID    uuid.UUID `json:"island_id"`
	PlayerName  string    `json:"player_name"`
	IslandName  string    `json:"island_name"`
	X           int       `json:"x"`
	Y           int       `json:"y"`
	Distance    float64   `json:"distance"`
	ResourceEst string    `json:"resource_est"` // Low/Medium/High estimate (simulating spy report)
	TownHallLvl int       `json:"town_hall_lvl"`
}

// GetPvpTargets finds attackable players near the given coordinates
func GetPvpTargets(playerID uuid.UUID, x, y int) ([]PvpTarget, error) {
	db := repository.GetDB()
	var targets []PvpTarget
	var islands []domain.Island

	// Find islands within "square" radius first for DB speed, then filter by circle
	minX, maxX := x-PvpSearchRadius, x+PvpSearchRadius
	minY, maxY := y-PvpSearchRadius, y+PvpSearchRadius

	// Query: distinct players, not self, within bounds, TH >= MinTownHallForPvP
	// We scan islands.
	err := db.Preload("Player").
		Where("player_id != ? AND x BETWEEN ? AND ? AND y BETWEEN ? AND ?", playerID, minX, maxX, minY, maxY).
		Find(&islands).Error

	if err != nil {
		return nil, err
	}

	for _, island := range islands {
		// 1. Beginner Protection Check
		townHallLevel := GetBuildingLevel(&island, "Hôtel de Ville")
		if townHallLevel < MinTownHallForPvP {
			continue
		}

		// 2. Distance Check (Euclidean)
		dist := math.Sqrt(math.Pow(float64(island.X-x), 2) + math.Pow(float64(island.Y-y), 2))
		if dist > PvpSearchRadius {
			continue
		}

		// 3. Estimate Resources (Spy Report Lite)
		totalRes := 0.0
		for _, amount := range island.Resources {
			totalRes += amount
		}
		est := "Faible"
		if totalRes > 50000 {
			est = "Élevée"
		} else if totalRes > 10000 {
			est = "Moyenne"
		}

		targets = append(targets, PvpTarget{
			PlayerID:    island.PlayerID,
			IslandID:    island.ID,
			PlayerName:  island.Player.Username,
			IslandName:  island.Name,
			X:           island.X,
			Y:           island.Y,
			Distance:    math.Round(dist*10) / 10,
			ResourceEst: est,
			TownHallLvl: townHallLevel,
		})

		// Limit to 10 targets
		if len(targets) >= 10 {
			break
		}
	}

	return targets, nil
}

// GetBuildingLevel returns the level of a specific building type on an island
func GetBuildingLevel(island *domain.Island, buildingType string) int {
	for _, b := range island.Buildings {
		if b.Type == buildingType {
			return b.Level
		}
	}
	return 0 // Should not happen if data integrity is kept (TH is mandatory)
}
