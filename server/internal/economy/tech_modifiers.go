package economy

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// TechModifiers centralizes all computed bonuses from technologies.
// Unlike the legacy TechBonuses, this uses Maps for better scalability.
type TechModifiers struct {
	// Economy
	ResourceProductionMultiplier map[domain.ResourceType]float64
	StorageCapacityMultiplier    map[domain.ResourceType]float64
	LootBonus                    float64

	// Naval
	ShipSpeedMultiplier   float64
	WindEfficiency        float64 // 0.0 = base, 0.1 = +10%
	CounterWindMitigation float64 // 0.0 = base, 0.5 = 50% less penalty
	TravelTimeReduction   float64
	RepairCostMultiplier  map[domain.ResourceType]float64
	FleetSizeBonus        int

	// Combat
	CrewStatsMultiplier   map[string]float64 // "hp", "damage"
	UnitTypeBonus         map[domain.UnitType]float64
	TriangleBonus         float64
	CrewLossReduction     float64
	DamageReductionGlobal float64

	// Logistics
	BuildTimeReduction    float64
	ResearchTimeReduction float64
	ExtraQueue            int
}

// NewTechModifiers creates an initialized TechModifiers struct
func NewTechModifiers() TechModifiers {
	return TechModifiers{
		ResourceProductionMultiplier: make(map[domain.ResourceType]float64),
		StorageCapacityMultiplier:    make(map[domain.ResourceType]float64),
		RepairCostMultiplier:         make(map[domain.ResourceType]float64),
		CrewStatsMultiplier:          make(map[string]float64),
		UnitTypeBonus:                make(map[domain.UnitType]float64),
	}
}

// ComputeTechModifiers purely calculates modifiers from a list of unlocked tech IDs.
// It maps the flat TechEffect fields (legacy) to the new Map-based structure.
func ComputeTechModifiers(unlockedTechs []string) TechModifiers {
	techMu.RLock()
	defer techMu.RUnlock()

	mods := NewTechModifiers()

	// Helper to safely get map value
	addMap := func(m map[domain.ResourceType]float64, key domain.ResourceType, val float64) {
		m[key] += val
	}

	for _, id := range unlockedTechs {
		tech, ok := techMap[id]
		if !ok {
			continue
		}
		e := tech.Effects

		// --- Economy ---
		if e.ProdWood > 0 {
			addMap(mods.ResourceProductionMultiplier, domain.Wood, e.ProdWood)
		}
		if e.ProdStone > 0 {
			addMap(mods.ResourceProductionMultiplier, domain.Stone, e.ProdStone)
		}
		if e.ProdGold > 0 {
			addMap(mods.ResourceProductionMultiplier, domain.Gold, e.ProdGold)
		}
		if e.ProdRum > 0 {
			addMap(mods.ResourceProductionMultiplier, domain.Rum, e.ProdRum)
		}

		if e.StorageWood > 0 {
			addMap(mods.StorageCapacityMultiplier, domain.Wood, e.StorageWood)
		}
		if e.StorageStone > 0 {
			addMap(mods.StorageCapacityMultiplier, domain.Stone, e.StorageStone)
		}
		if e.StorageGold > 0 {
			addMap(mods.StorageCapacityMultiplier, domain.Gold, e.StorageGold)
		}
		if e.StorageRum > 0 {
			addMap(mods.StorageCapacityMultiplier, domain.Rum, e.StorageRum)
		}

		mods.LootBonus += e.LootBonus

		// --- Naval ---
		mods.ShipSpeedMultiplier += e.SpeedBonus
		mods.WindEfficiency += e.WindBonus
		mods.CounterWindMitigation += e.CounterWind
		mods.TravelTimeReduction += e.TravelTime
		mods.FleetSizeBonus += e.ExtraShips

		if e.RepairWood > 0 {
			// RepairTech in JSON is usually positive (reduction), so we sum it.
			// Logic handling the subtraction happens in usage.
			addMap(mods.RepairCostMultiplier, domain.Wood, e.RepairWood)
		}
		if e.RepairGold > 0 {
			addMap(mods.RepairCostMultiplier, domain.Gold, e.RepairGold)
		}
		if e.RepairGlobal > 0 {
			addMap(mods.RepairCostMultiplier, domain.Wood, e.RepairGlobal)
			addMap(mods.RepairCostMultiplier, domain.Gold, e.RepairGlobal)
		}

		// --- Combat ---
		if e.CrewHP > 0 {
			mods.CrewStatsMultiplier["hp"] += e.CrewHP
		}
		if e.CrewDamage > 0 {
			mods.CrewStatsMultiplier["damage"] += e.CrewDamage
		}

		if e.GuerrierBonus > 0 {
			mods.UnitTypeBonus[domain.Warrior] += e.GuerrierBonus
		}
		if e.ArcherBonus > 0 {
			mods.UnitTypeBonus[domain.Archer] += e.ArcherBonus
		}
		if e.FusilierBonus > 0 {
			mods.UnitTypeBonus[domain.Gunner] += e.FusilierBonus
		}

		mods.TriangleBonus += e.TriangleBonus
		mods.CrewLossReduction += e.CrewLossReduce

		// --- Logistics ---
		mods.BuildTimeReduction += e.BuildReduce
		mods.ResearchTimeReduction += e.ResearchReduce
		mods.ExtraQueue += e.ExtraQueue
	}

	return mods
}
