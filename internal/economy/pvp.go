package economy

import (
	"fmt"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
)

// PvP Constants
const (
	MinTownHallForPvP = 4             // Beginner protection until TH 3 completed (TH4 unlocks PvP)
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

// GetPvpTargets finds attackable players near the given coordinates.
// If all is true, search radius and TH level protection are bypassed.
func GetPvpTargets(playerID uuid.UUID, x, y int, all bool) ([]PvpTarget, error) {
	db := repository.GetDB()
	var targets []PvpTarget
	var islands []domain.Island

	// Find islands within "square" radius first for DB speed, then filter by circle
	minX, maxX := x-PvpSearchRadius, x+PvpSearchRadius
	minY, maxY := y-PvpSearchRadius, y+PvpSearchRadius

	// Query: distinct players, not self, within bounds (if not "all")
	query := db.Preload("Player").Preload("Buildings")
	if !all {
		query = query.Where("x BETWEEN ? AND ? AND y BETWEEN ? AND ?", minX, maxX, minY, maxY)
	}
	err := query.Where("player_id != ?", playerID).Find(&islands).Error

	if err != nil {
		fmt.Printf("[PVP_DEBUG] DB Error: %v\n", err)
		return nil, err
	}

	fmt.Printf("[PVP_DEBUG] Searching near X:%d Y:%d (Bounds: %d-%d, %d-%d)\n", x, y, minX, maxX, minY, maxY)
	fmt.Printf("[PVP_DEBUG] Found %d potential islands in square radius.\n", len(islands))

	for _, island := range islands {
		// 1. Beginner Protection Check (Skip if Admin/All)
		townHallLevel := GetBuildingLevel(&island, "Hôtel de Ville")
		if !all && townHallLevel < MinTownHallForPvP {
			fmt.Printf("[PVP_DEBUG] SKIP %s (TH: %d < %d)\n", island.Player.Username, townHallLevel, MinTownHallForPvP)
			continue
		}

		// 2. Distance Check (Euclidean) (Skip if Admin/All)
		dist := math.Sqrt(math.Pow(float64(island.X-x), 2) + math.Pow(float64(island.Y-y), 2))
		if !all && dist > PvpSearchRadius {
			fmt.Printf("[PVP_DEBUG] SKIP %s (Dist: %.1f > %d)\n", island.Player.Username, dist, PvpSearchRadius)
			continue
		}

		fmt.Printf("[PVP_DEBUG] KEEP %s (ID: %s, Dist: %.1f, TH: %d)\n", island.Player.Username, island.ID.String(), dist, townHallLevel)

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
// Only counts completed buildings (not under construction)
func GetBuildingLevel(island *domain.Island, buildingType string) int {
	for _, b := range island.Buildings {
		if b.Type == buildingType && !b.Constructing {
			return b.Level
		}
	}
	return 0 // Should not happen if data integrity is kept (TH is mandatory)
}
