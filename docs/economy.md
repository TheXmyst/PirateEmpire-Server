# Economy System Documentation

## Overview
The Economy System manages building costs, build times, and production rates using a data-driven approach with exponential growth formulas.
It supports levels 1 to 30.

## Configuration
**File:** `server/configs/buildings.json`

Format:
```json
{
  "buildings": {
    "BuildingName": {
      "base_cost": { "wood": 100, "stone": 50 },
      "cost_growth": 1.15,
      "base_build_time": 60,
      "time_growth": 1.14,
      "base_production": 60, // Per Hour
      "production_growth": 1.12
    }
  }
}
```

## Formulas
Level `n` stats are calculated relative to Base (Level 1) values:

- **Cost**: `BaseCost * (CostGrowth ^ (n-1))`
- **Time**: `BaseTime * (TimeGrowth ^ (n-1))`
- **Production**: `BaseProduction * (ProductionGrowth ^ (n-1))`

## Code Integration
The system is implemented in `server/internal/economy`.

### Usage
```go
import "github.com/TheXmyst/Sea-Dogs/server/internal/economy"

// Load Config at startup
economy.LoadConfig("configs/buildings.json")

// Get Stats for Level 5
stats, err := economy.GetBuildingStats("Scierie", 5)
if err != nil { ... }

fmt.Println(stats.Cost)
fmt.Println(stats.BuildTime)
fmt.Println(stats.Production) // Per Hour
```

## Legacy Replaced
This system replaces the hardcoded values in `server/internal/gamedata`.
The server logic in `handlers.go` (Construction) and `game_loop.go` (Production) now uses this system.
