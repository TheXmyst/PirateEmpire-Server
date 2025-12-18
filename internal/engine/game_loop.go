package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
)

// Engine handles the game loop and logic
type Engine struct {
	stopCh chan struct{}
	// Checkpoint tracking: map island ID -> last checkpoint time
	islandCheckpoints map[uuid.UUID]time.Time
}

// IslandCheckpointInterval defines how often islands are persisted to DB
// This reduces DB writes by ~80% while maintaining correctness:
// - Timers use absolute timestamps (FinishTime) → no dependency on save frequency
// - Resources are recalculated from LastUpdated on read → max loss = checkpoint interval
const IslandCheckpointInterval = 5 * time.Second

func NewEngine() *Engine {
	return &Engine{
		stopCh:            make(chan struct{}),
		islandCheckpoints: make(map[uuid.UUID]time.Time),
	}
}

func (e *Engine) Start() {
	ticker := time.NewTicker(1 * time.Second) // Faster tick for smooth experience
	go func() {
		for {
			select {
			case <-ticker.C:
				e.Tick()
			case <-e.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

// Tick processes one game cycle
func (e *Engine) Tick() {
	db := repository.GetDB()
	if db == nil {
		fmt.Println("Tick: DB is nil")
		return
	}

	// Loads Tech Config if not loaded
	if err := economy.LoadTechConfig("configs/tech.json"); err != nil {
		// Ignored for now
	}
	// Loads Ship Config
	if err := economy.LoadShipConfig("configs/ships.json"); err != nil {
		fmt.Println("Error loading ship config:", err)
	}

	// Update PvE Target Simulation (Random Walk)
	// Assume 1.0s delta since ticker is 1s
	var islands []domain.Island
	// Preload buildings AND Player AND Ships
	if err := db.Preload("Buildings").Preload("Player").Preload("Ships").Find(&islands).Error; err != nil {
		fmt.Println("Tick: Error listing islands:", err)
		return
	}

	// Update PvE Target Simulation (Random Walk)
	// Moved here to access 'islands' for collision avoidance
	economy.UpdatePveTargetSimulation(1.0, islands)

	now := time.Now()

	for i := range islands {
		island := &islands[i]

		if island.LastUpdated.IsZero() {
			island.LastUpdated = now
			e.islandCheckpoints[island.ID] = now // Initialize checkpoint tracking
			db.Save(island)
			continue
		}

		delta := now.Sub(island.LastUpdated)

		// Check Research Completion
		if island.Player.ResearchingTechID != "" && !island.Player.ResearchFinishTime.IsZero() {
			// Debug
			// fmt.Printf("[GAME LOOP] Check Research: %s. Fin: %v, Now: %v\n", island.Player.ResearchingTechID, island.Player.ResearchFinishTime, now)

			if now.After(island.Player.ResearchFinishTime) {
				// 1. Refetch Player from DB to ensure we have the absolute latest state
				// This prevents overwriting concurrent changes or using stale GameLoop data
				var freshPlayer domain.Player
				if err := db.First(&freshPlayer, "id = ?", island.Player.ID).Error; err == nil {

					// 2. Modify the FRESH player record
					techID := freshPlayer.ResearchingTechID
					if techID != "" {
						var unlocked []string
						if len(freshPlayer.UnlockedTechsJSON) > 0 {
							_ = json.Unmarshal(freshPlayer.UnlockedTechsJSON, &unlocked)
						}

						exists := false
						for _, id := range unlocked {
							if id == techID {
								exists = true
								break
							}
						}
						if !exists {
							unlocked = append(unlocked, techID)
						}

						freshPlayer.UnlockedTechs = unlocked
						newJSON, _ := json.Marshal(unlocked)
						freshPlayer.UnlockedTechsJSON = newJSON
						freshPlayer.ResearchingTechID = ""
						freshPlayer.ResearchFinishTime = time.Time{}

						// 3. Save the FRESH player
						if err := db.Save(&freshPlayer).Error; err != nil {
							fmt.Println("Error saving fresh player research:", err)
						} else {
							fmt.Printf("[GAME LOOP] Research Complete: %s for Player %s\n", techID, freshPlayer.Username)
							// Update local island.Player so future logic in this tick isn't stale
							island.Player = freshPlayer
						}
					}
				}
			}
		}

		// Check Ship Construction Completion
		for j := range island.Ships {
			s := &island.Ships[j]
			if s.State == "UnderConstruction" && !s.FinishTime.IsZero() {
				if now.After(s.FinishTime) {
					s.State = "Ready"
					s.FinishTime = time.Time{} // Clear
					if err := db.Save(s).Error; err != nil {
						fmt.Printf("Error saving ship %s: %v\n", s.ID, err)
					}
					fmt.Printf("[GAME LOOP] Ship %s Construction Complete!\n", s.Name)
				}
			}
		}

		CalculateResources(island, delta)

		// Checkpoint-based persistence: Save Island only every IslandCheckpointInterval
		// This reduces DB writes by ~80% while maintaining correctness:
		// - Building/Research/Ship timers use absolute timestamps (FinishTime) → no dependency on save frequency
		// - Resources are recalculated from LastUpdated on read (GetStatus lazy update) → max loss = checkpoint interval on crash
		// - Event-based writes (Build, Upgrade, StartResearch, StartShip) remain immediate and transactional
		lastCheckpoint, exists := e.islandCheckpoints[island.ID]
		shouldCheckpoint := !exists || now.Sub(lastCheckpoint) >= IslandCheckpointInterval

		if shouldCheckpoint {
			island.LastUpdated = now
			e.islandCheckpoints[island.ID] = now

			// Save changes - OMIT Player to prevent reverting the Manual Save above
			// Double safety: Clear the Player struct from the island object before saving
			// This ensures GORM has no data to try and cascade update
			island.Player = domain.Player{}
			if err := db.Omit("Player").Save(island).Error; err != nil {
				fmt.Printf("Error saving island %s: %v\n", island.Name, err)
			} else {
				fmt.Printf("[ECONOMY] Island checkpoint saved id=%s\n", island.ID)
			}
		} else {
			// Update LastUpdated in memory only (not persisted yet)
			// This ensures resource calculation uses correct delta on next tick
			// But we don't persist until checkpoint to reduce DB writes
			island.LastUpdated = now
		}
	}
}

// CalculateResources updates island resources based on buildings and techs
func CalculateResources(island *domain.Island, delta time.Duration) {
	if island.Resources == nil {
		island.Resources = make(map[domain.ResourceType]float64)
	}
	// Reset Generation Maps (Transient)
	island.ResourceGeneration = make(map[domain.ResourceType]float64)
	island.ResourceGenerationBase = make(map[domain.ResourceType]float64)
	island.ResourceGenerationBonus = make(map[domain.ResourceType]float64)

	// 1. Calculate Tech Bonuses (New System: TechModifiers)
	var mods economy.TechModifiers
	if island.Player.ID != uuid.Nil {
		var techs []string
		if len(island.Player.UnlockedTechsJSON) > 0 {
			if err := json.Unmarshal(island.Player.UnlockedTechsJSON, &techs); err == nil {
				mods = economy.ComputeTechModifiers(techs)
			}
		} else {
			mods = economy.ComputeTechModifiers(nil)
		}
	} else {
		mods = economy.ComputeTechModifiers(nil)
	}

	// 2. Base Limits
	limits := map[domain.ResourceType]float64{
		domain.Wood:  5000.0,
		domain.Stone: 5000.0,
		domain.Rum:   3000.0,
		domain.Gold:  10000.0,
	}

	// 3. Process Buildings
	for i := range island.Buildings {
		b := &island.Buildings[i]

		// Construction Logic
		if b.Constructing {
			if time.Now().After(b.FinishTime) {
				b.Constructing = false
				b.Level++
				fmt.Printf("[GAME LOOP] Building %s Finished Construction! New Level: %d\n", b.ID, b.Level)
				if db := repository.GetDB(); db != nil {
					db.Save(b)
				}
			} else {
				continue
			}
		}

		stats, err := economy.GetBuildingStats(b.Type, b.Level)
		if err != nil {
			continue
		}

		// Production with Bonus
		if stats.Production > 0 {
			var resType domain.ResourceType
			switch b.Type {
			case "Scierie":
				resType = domain.Wood
			case "Carrière":
				resType = domain.Stone
			case "Mine d'Or":
				resType = domain.Gold
			case "Distillerie":
				resType = domain.Rum
			}

			if resType != "" {
				// Use new Map-based lookup
				prodBonus := mods.ResourceProductionMultiplier[resType]

				// PRODUCTION CALCULATION (Per Building)
				// Base production for this building
				baseCalc := stats.Production
				// Bonus amount for this building
				bonusCalc := stats.Production * prodBonus
				// Total for this building
				finalProd := baseCalc + bonusCalc

				// Update Island Totals (Totals are per second, but we store per Hour in Base/Bonus for UI?)
				// NO, `stats.Production` is usually per Hour in config? Let's check.
				// game_loop.go:262: amount := (finalProd / 3600.0) * delta.Seconds()
				// This implies stats.Production is Per Hour.

				// Aggregate Generation Rates (Per Hour) onto Island struct for Tooltips
				island.ResourceGeneration[resType] += finalProd
				island.ResourceGenerationBase[resType] += baseCalc
				island.ResourceGenerationBonus[resType] += bonusCalc

				// Apply to actual resources (using delta)
				amount := (finalProd / 3600.0) * delta.Seconds()
				island.Resources[resType] += amount
			}
		}

		// Storage with Bonus
		// Logic: Building storage REPLACES base storage if its higher (Max Logic).
		// This ensures Warehouse (20k) replaces Base (5k) instead of adding to it.
		// Now applying Specific Tech Bonuses to Storage.
		if len(stats.Storage) > 0 {
			for res, amount := range stats.Storage {
				// Use new Map-based lookup
				storageBonus := mods.StorageCapacityMultiplier[res]

				finalStorage := math.Round(amount * (1.0 + storageBonus))
				if finalStorage > limits[res] {
					limits[res] = finalStorage
				}
			}
		}
	}

	// 4. Cap Resources
	for res, val := range island.Resources {
		if limit, ok := limits[res]; ok {
			if val > limit {
				island.Resources[res] = limit
			}
		}
	}
	island.StorageLimits = limits
}
