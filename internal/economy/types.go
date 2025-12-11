package economy

import (
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Config represents the raw JSON structure
type Config struct {
	BuildTimeCurves map[string]CurvePreset    `json:"build_time_curves"`
	Buildings       map[string]BuildingConfig `json:"buildings"`
}

type CurvePreset struct {
	Pivot       int     `json:"pivot"`
	EarlyBase   float64 `json:"early_base"`
	EarlyFactor float64 `json:"early_factor"`
	LateBase    float64 `json:"late_base"`
	LateFactor  float64 `json:"late_factor"`
}

type BuildingConfig struct {
	Category         string             `json:"category"`
	BaseCost         map[string]float64 `json:"base_cost"`
	CostGrowth       float64            `json:"cost_growth"`
	BaseBuildTime    float64            `json:"base_build_time"` // Seconds
	TimeGrowth       float64            `json:"time_growth"`
	BaseProduction   float64            `json:"base_production"` // Per Hour
	ProductionGrowth float64            `json:"production_growth"`
	BaseStorage      map[string]float64 `json:"base_storage"`
	StorageGrowth    map[string]float64 `json:"storage_growth"`
	Prerequisites    []Requirement      `json:"prerequisites"`
}

type Requirement struct {
	Level       int    `json:"level"`
	ReqTownHall int    `json:"req_townhall"`
	ReqTech     string `json:"req_tech"`      // Tech ID
	ReqBuilding string `json:"req_building"`  // Building Type (min level 1 implied or specific?)
	ReqMinLevel int    `json:"req_min_level"` // For ReqBuilding
}

// LevelStats represents the calculated stats for a specific level
type LevelStats struct {
	Level      int
	Cost       map[domain.ResourceType]float64
	BuildTime  time.Duration
	Production float64 // Per Hour
	Storage    map[domain.ResourceType]float64
}
