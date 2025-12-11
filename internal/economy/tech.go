package economy

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TechEffect holds all possible bonuses from technologies
type TechEffect struct {
	ProdWood       float64 `json:"production_wood,omitempty"`
	ProdStone      float64 `json:"production_stone,omitempty"`
	ProdRum        float64 `json:"production_rum,omitempty"`
	ProdGold       float64 `json:"production_gold,omitempty"`
	LootBonus      float64 `json:"loot_bonus,omitempty"`
	StorageWood    float64 `json:"storage_wood,omitempty"`
	StorageStone   float64 `json:"storage_stone,omitempty"`
	StorageRum     float64 `json:"storage_rum,omitempty"`
	StorageGold    float64 `json:"storage_gold,omitempty"`
	SpeedBonus     float64 `json:"speed_bonus,omitempty"`
	WindBonus      float64 `json:"wind_bonus,omitempty"`
	CounterWind    float64 `json:"counter_wind,omitempty"`
	TravelTime     float64 `json:"travel_time,omitempty"`
	RepairWood     float64 `json:"repair_wood,omitempty"` // Reduction
	RepairGold     float64 `json:"repair_gold,omitempty"`
	RepairGlobal   float64 `json:"repair_global,omitempty"`
	ExtraShips     int     `json:"extra_ships,omitempty"`
	CrewHP         float64 `json:"crew_hp,omitempty"`
	CrewDamage     float64 `json:"crew_damage,omitempty"`
	GuerrierBonus  float64 `json:"guerrier_bonus,omitempty"`
	ArcherBonus    float64 `json:"archer_bonus,omitempty"`
	FusilierBonus  float64 `json:"fusilier_bonus,omitempty"`
	TriangleBonus  float64 `json:"triangle_bonus,omitempty"`
	TriangleMalus  float64 `json:"triangle_malus,omitempty"`
	CrewLossReduce float64 `json:"crew_loss_reduce,omitempty"`
	BuildReduce    float64 `json:"build_reduce,omitempty"`
	ResearchReduce float64 `json:"research_reduce,omitempty"`
	ExtraQueue     int     `json:"extra_queue,omitempty"`
}

type Cost struct {
	Wood  float64 `json:"wood,omitempty"`
	Stone float64 `json:"stone,omitempty"`
	Gold  float64 `json:"gold,omitempty"`
	Rum   float64 `json:"rum,omitempty"`
}

type Technology struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tree        string     `json:"tree"`
	Tier        int        `json:"tier"`
	ReqTH       int        `json:"required_townhall"`
	ReqAcad     int        `json:"required_academy"`
	Cost        Cost       `json:"cost"`
	TimeSec     int        `json:"research_time_sec"` // Using Int for JSON
	Effects     TechEffect `json:"effects"`
}

// TechRoot matches the JSON structure
type TechRoot struct {
	Economy   []Technology `json:"economy"`
	Naval     []Technology `json:"naval"`
	Combat    []Technology `json:"combat"`
	Logistics []Technology `json:"logistics"`
}

// TechBonuses aggregates bonuses for use in game logic
type TechBonuses struct {
	ProdWoodMult     float64
	ProdStoneMult    float64
	ProdRumMult      float64
	ProdGoldMult     float64
	LootBonus        float64
	StorageWoodMult  float64
	StorageStoneMult float64
	StorageRumMult   float64
	StorageGoldMult  float64

	// Naval
	SpeedBonus  float64
	WindBonus   float64
	CounterWind float64 // Factor to Reduce Malus (0.1 means 10% less malus)
	TravelTime  float64 // Reduction (0.1 means 10% faster)
	RepairWood  float64 // Reduction
	RepairGold  float64 // Reduction
	ExtraShips  int

	// Combat
	CrewHP         float64
	CrewDamage     float64
	GuerrierBonus  float64
	ArcherBonus    float64
	FusilierBonus  float64
	TriangleBonus  float64 // Add to 1.5
	TriangleMalus  float64 // Reduce from 0.5 (so positive value is good)
	CrewLossReduce float64

	// Logistics
	BuildTimeReduce    float64
	ResearchTimeReduce float64
	ExtraQueue         int
}

var (
	techMap    map[string]Technology // Flattened map for ID lookup
	techTrees  TechRoot              // Original tree structure
	techLoaded bool
	techMu     sync.RWMutex
)

// LoadTechConfig loads the tech.json file
func LoadTechConfig(path string) error {
	techMu.Lock()
	defer techMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read tech config: %w", err)
	}

	if err := json.Unmarshal(data, &techTrees); err != nil {
		return fmt.Errorf("failed to parse tech config: %w", err)
	}

	// Flatten into Map
	techMap = make(map[string]Technology)

	for _, t := range techTrees.Economy {
		techMap[t.ID] = t
	}
	for _, t := range techTrees.Naval {
		techMap[t.ID] = t
	}
	for _, t := range techTrees.Combat {
		techMap[t.ID] = t
	}
	for _, t := range techTrees.Logistics {
		techMap[t.ID] = t
	}

	techLoaded = true
	fmt.Printf("Technology System: Loaded %d technologies.\n", len(techMap))
	return nil
}

// GetTech returns a specific technology config
func GetTech(id string) (*Technology, error) {
	techMu.RLock()
	defer techMu.RUnlock()

	if !techLoaded {
		return nil, fmt.Errorf("tech config not loaded")
	}

	if t, ok := techMap[id]; ok {
		return &t, nil
	}
	return nil, fmt.Errorf("tech not found: %s", id)
}

// GetTechDuration returns Duration for timers
func (t *Technology) GetDuration() time.Duration {
	return time.Duration(t.TimeSec) * time.Second
}

// CalculateTechBonuses aggregates effects from a list of unlocked tech IDs
func CalculateTechBonuses(unlockedTechs []string) TechBonuses {
	techMu.RLock()
	defer techMu.RUnlock()

	b := TechBonuses{
		// Default Multipliers (Base 1.0 logic handled in calculator,
		// here we return ADDITIVE bonuses usually, e.g. 0.05)
		// Or should we return totals?
		// User prompt said "prodFinal = base * (1 + sum(ProdWood...))"
		// So we return the SUM of bonuses (0.05 + 0.05 = 0.10)
	}

	for _, id := range unlockedTechs {
		tech, ok := techMap[id]
		if !ok {
			continue // Should not happen if data integrity is kept
		}
		e := tech.Effects

		// Economy
		b.ProdWoodMult += e.ProdWood
		b.ProdStoneMult += e.ProdStone
		b.ProdRumMult += e.ProdRum
		b.ProdGoldMult += e.ProdGold
		b.LootBonus += e.LootBonus
		b.StorageWoodMult += e.StorageWood
		b.StorageStoneMult += e.StorageStone
		b.StorageRumMult += e.StorageRum
		b.StorageGoldMult += e.StorageGold

		// Naval
		b.SpeedBonus += e.SpeedBonus
		b.WindBonus += e.WindBonus
		b.CounterWind += e.CounterWind
		b.TravelTime += e.TravelTime
		b.RepairWood += e.RepairWood
		b.RepairGold += e.RepairGold
		b.RepairGold += e.RepairGlobal // Global applies to both
		b.RepairWood += e.RepairGlobal
		b.ExtraShips += e.ExtraShips

		// Combat
		b.CrewHP += e.CrewHP
		b.CrewDamage += e.CrewDamage
		b.GuerrierBonus += e.GuerrierBonus
		b.ArcherBonus += e.ArcherBonus
		b.FusilierBonus += e.FusilierBonus
		b.TriangleBonus += e.TriangleBonus
		b.TriangleMalus += e.TriangleMalus
		b.CrewLossReduce += e.CrewLossReduce

		// Logistics
		b.BuildTimeReduce += e.BuildReduce
		b.ResearchTimeReduce += e.ResearchReduce
		b.ExtraQueue += e.ExtraQueue
	}

	return b
}

// CalculateAcademyResearchBonus calculates the research time reduction bonus from Academy level
// Formula: If AcademyLevel <= 5 → 0, else (AcademyLevel - 5) * 0.0075
// Returns a value between 0.0 and 0.1875 (0% to 18.75% reduction)
func CalculateAcademyResearchBonus(academyLevel int) float64 {
	if academyLevel <= 5 {
		return 0.0
	}
	return float64(academyLevel-5) * 0.0075
}