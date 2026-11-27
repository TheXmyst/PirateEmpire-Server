package api

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
)

// costToDTO convertit un coût du moteur en ResourceMap pour l'API.
func costToDTO(cost map[domain.ResourceType]float64) dto.ResourceMap {
	out := make(dto.ResourceMap, len(cost))
	for res, amount := range cost {
		out[string(res)] = amount
	}
	return out
}

// fillBuildingCosts ajoute à l'île les vrais coûts calculés par le serveur
// (même formule que celle qui débite les ressources) : coût du niveau
// suivant de chaque bâtiment et coût de construction de chaque type.
// Le client les affiche au lieu de les recalculer de son côté.
func fillBuildingCosts(island *dto.IslandDTO) {
	if island == nil {
		return
	}
	for i := range island.Buildings {
		b := &island.Buildings[i]
		if stats, err := economy.GetBuildingStats(b.Type, b.Level+1); err == nil {
			b.NextLevelCost = costToDTO(stats.Cost)
		}
	}
	island.BuildCosts = make(map[string]dto.ResourceMap)
	for _, bType := range economy.BuildingTypes() {
		if stats, err := economy.GetBuildingStats(bType, 1); err == nil {
			island.BuildCosts[bType] = costToDTO(stats.Cost)
		}
	}
}
