package economy

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

var (
	config   Config
	loaded   bool
	mu       sync.RWMutex
	maxLevel = 30
)

// LoadConfig loads the buildings.json file
func LoadConfig(path string) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read building config: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse building config: %w", err)
	}

	loaded = true
	fmt.Println("Economy System: Config Loaded Successfully")
	return nil
}

// GetBuildingStats calculates stats for a specific building type and level
func GetBuildingStats(buildingType string, level int) (*LevelStats, error) {
	mu.RLock()
	defer mu.RUnlock()

	if !loaded {
		return nil, fmt.Errorf("economy config not loaded")
	}

	if level < 1 || level > maxLevel {
		return nil, fmt.Errorf("invalid level: %d (max %d)", level, maxLevel)
	}

	cfg, exists := config.Buildings[buildingType]
	if !exists {
		return nil, fmt.Errorf("unknown building type: %s", buildingType)
	}

	stats := &LevelStats{
		Level:   level,
		Cost:    make(map[domain.ResourceType]float64),
		Storage: make(map[domain.ResourceType]float64),
	}

	// 1. Calculate Cost (New System)
	stats.Cost = ComputeBuildingCost(cfg, level)

	// 2. Calculate Build Time (New System)
	stats.BuildTime = ComputeBuildTime(buildingType, level, cfg)

	/* OLD SYSTEM
	timeMult := math.Pow(cfg.TimeGrowth, float64(level-1))
	seconds := cfg.BaseBuildTime * timeMult
	stats.BuildTime = time.Duration(seconds) * time.Second
	*/

	// 3. Calculate Production (Per Hour)
	// Prod_n = BaseProd * (ProdGrowth ^ (n-1))
	// 3. Calculate Production (Per Hour)
	// Prod_n = BaseProd * (ProdGrowth ^ (n-1))
	if cfg.BaseProduction > 0 {
		stats.Production = ComputeProductionPerHour(cfg, level, 0.0)
	}

	// 4. Calculate Storage (New System)
	if len(cfg.BaseStorage) > 0 {
		stats.Storage = ComputeWarehouseStorage(cfg, level)
	}

	return stats, nil
}

// GetMaxLevel returns the hardcoded max level
func GetMaxLevel() int {
	return maxLevel
}

// GetBuildingConfig safely retrieves the configuration for a building type
func GetBuildingConfig(buildingType string) (BuildingConfig, bool) {
	mu.RLock()
	defer mu.RUnlock()

	if !loaded {
		return BuildingConfig{}, false
	}
	cfg, ok := config.Buildings[buildingType]
	return cfg, ok
}

// ComputeBuildTime calculates construction time using the new 2-phase system
func ComputeBuildTime(buildingType string, level int, bCfg BuildingConfig) time.Duration {
	// Debug Hook (Env var check could be cached, but for now strict check)
	debug := os.Getenv("DEBUG_BUILD_TIME") == "true"

	// 1. Resolve Category and Curve
	category := bCfg.Category
	if category == "" {
		// Fallback to old system if no category
		if debug {
			fmt.Printf("[BuildTime] type=%s level=%d msg='No Category, using legacy'\n", buildingType, level)
		}
		return legacyBuildTime(level, bCfg)
	}

	curve, ok := config.BuildTimeCurves[category]
	if !ok {
		// Fallback if category exists but no curve defined
		if debug {
			fmt.Printf("[BuildTime] type=%s level=%d category=%s msg='Curve not found, using legacy'\n", buildingType, level, category)
		}
		return legacyBuildTime(level, bCfg)
	}

	// 2. Apply Formula
	var seconds float64

	if level <= curve.Pivot {
		// Early Phase
		// time = early_base * (early_factor ^ (level-1))
		seconds = curve.EarlyBase * math.Pow(curve.EarlyFactor, float64(level-1))
	} else {
		// Late Phase
		// time = late_base * (late_factor ^ (level-pivot))
		seconds = curve.LateBase * math.Pow(curve.LateFactor, float64(level-curve.Pivot))
	}

	if debug {
		fmt.Printf("[BuildTime] type=%s level=%d category=%s phase=%s time=%.2fs\n",
			buildingType, level, category,
			map[bool]string{true: "Early", false: "Late"}[level <= curve.Pivot],
			seconds)
	}

	return time.Duration(seconds) * time.Second
}

func legacyBuildTime(level int, bCfg BuildingConfig) time.Duration {
	timeMult := math.Pow(bCfg.TimeGrowth, float64(level-1))
	seconds := bCfg.BaseBuildTime * timeMult
	return time.Duration(seconds) * time.Second
}

// ComputeBuildingCost calculates the cost for a specific level based on F2P formulas
func ComputeBuildingCost(cfg BuildingConfig, level int) map[domain.ResourceType]float64 {
	costs := make(map[domain.ResourceType]float64)
	// Cost = Base * (Growth ^ (Level - 1))
	growthMult := math.Pow(cfg.CostGrowth, float64(level-1))

	for res, baseVal := range cfg.BaseCost {
		costs[domain.ResourceType(res)] = math.Round(baseVal * growthMult)
	}
	return costs
}

// ComputeWarehouseStorage calculates storage capacity for a specific level
func ComputeWarehouseStorage(cfg BuildingConfig, level int) map[domain.ResourceType]float64 {
	storage := make(map[domain.ResourceType]float64)

	for res, baseVal := range cfg.BaseStorage {
		// Determine growth for this specific resource if available, otherwise 1.0 (no growth)
		// Assuming config.StorageGrowth maps resource names to growth factors equivalent to base_storage keys
		growth := 1.0
		if g, ok := cfg.StorageGrowth[res]; ok {
			growth = g
		}

		// Storage = Base * (Growth ^ (Level - 1))
		mult := math.Pow(growth, float64(level-1))
		val := math.Round(baseVal * mult)
		storage[domain.ResourceType(res)] = val

		// Debug Log for Warehouse (Entrepôt) to verify exponential curve
		if cfg.BaseStorage["gold"] == 50000 && (level == 2 || level == 3) {
			fmt.Printf("[DebugStorage] level=%d res=%s base=%.0f growth=%.2f result=%.0f\n", level, res, baseVal, growth, val)
		}
	}
	return storage
}

// ComputeProductionPerHour calculates hourly production with optional percentage bonuses
// bonusPercent: 0.05 for +5%, 0.50 for +50%
func ComputeProductionPerHour(cfg BuildingConfig, level int, bonusPercent float64) float64 {
	if cfg.BaseProduction <= 0 {
		return 0
	}
	prodMult := math.Pow(cfg.ProductionGrowth, float64(level-1))
	base := cfg.BaseProduction * prodMult
	return math.Round(base * (1.0 + bonusPercent))
}
