package gamedata

import (
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

type BuildingStats struct {
	Cost           map[domain.ResourceType]float64
	BuildTime      time.Duration
	ProductionType domain.ResourceType
	BaseProduction float64
}

// Stats for Level 1 buildings
var Buildings = map[string]BuildingStats{
	"Hôtel de Ville": {
		Cost:      map[domain.ResourceType]float64{domain.Wood: 100, domain.Gold: 50, domain.Stone: 20},
		BuildTime: 30 * time.Second,
	},
	"Scierie": {
		Cost:           map[domain.ResourceType]float64{domain.Wood: 50, domain.Gold: 10},
		BuildTime:      10 * time.Second,
		ProductionType: domain.Wood,
		BaseProduction: 0.2,
	},
	"Carrière": {
		Cost:           map[domain.ResourceType]float64{domain.Wood: 100, domain.Gold: 20},
		BuildTime:      10 * time.Second,
		ProductionType: domain.Stone,
		BaseProduction: 0.2,
	},
	"Mine d'Or": {
		Cost:           map[domain.ResourceType]float64{domain.Wood: 200, domain.Stone: 50},
		BuildTime:      15 * time.Second,
		ProductionType: domain.Gold,
		BaseProduction: 0.2,
	},
	"Distillerie": {
		Cost:           map[domain.ResourceType]float64{domain.Wood: 150, domain.Gold: 50, domain.Stone: 20},
		BuildTime:      20 * time.Second,
		ProductionType: domain.Rum,
		BaseProduction: 0.2,
	},
	"Chantier Naval": {
		Cost:      map[domain.ResourceType]float64{domain.Wood: 300, domain.Gold: 100, domain.Stone: 100},
		BuildTime: 45 * time.Second,
	},
	"Entrepôt": {
		Cost:      map[domain.ResourceType]float64{domain.Wood: 100, domain.Stone: 50},
		BuildTime: 10 * time.Second,
	},
	"Le Nid du Flibustier": {
		Cost:      map[domain.ResourceType]float64{domain.Wood: 500, domain.Gold: 500, domain.Stone: 200, domain.Rum: 100},
		BuildTime: 60 * time.Second,
	},
	"La Chambre des Plans": {
		Cost:      map[domain.ResourceType]float64{domain.Wood: 200, domain.Gold: 200},
		BuildTime: 30 * time.Second,
	},
}
