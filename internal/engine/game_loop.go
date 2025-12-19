package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
)

// Engine handles the game loop and logic
type Engine struct {
	stopCh chan struct{}
	// Checkpoint tracking: map island ID -> last checkpoint time
	islandCheckpoints map[uuid.UUID]time.Time
	islandLastTicks   map[uuid.UUID]time.Time
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
		islandLastTicks:   make(map[uuid.UUID]time.Time),
	}
}

func (e *Engine) Start() {
	economy.InitWeather()                            // Initialize Weather System
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz tick for fluid movement/collision
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
		logger.Error("Tick: DB is nil")
		return
	}

	// Update Weather System
	if economy.GlobalWeather != nil {
		economy.GlobalWeather.Update()
	}

	// Loads Tech Config if not loaded
	if err := economy.LoadTechConfig("configs/tech.json"); err != nil {
		// Ignored for now
	}
	// Loads Ship Config
	if err := economy.LoadShipConfig("configs/ships.json"); err != nil {
		logger.Error("Error loading ship config", "error", err)
	}

	// Update PvE Target Simulation (Random Walk)
	// Assume 1.0s delta since ticker is 1s
	var islands []domain.Island
	// Preload everything needed
	if err := db.Preload("Buildings").Preload("Player").Preload("Ships").Preload("Fleets.Ships").Find(&islands).Error; err != nil {
		fmt.Println("Tick: Error listing islands:", err)
		return
	}

	now := time.Now()
	// Use 0.1s target delta, but could calculate real global delta if needed.
	// For now, 0.1s is safe with the 10Hz ticker.
	economy.UpdatePveTargetSimulation(0.1, islands)

	// Process PvP fleet travel (attacks and returns)
	ProcessPvpTravelFleets(db)

	for i := range islands {
		island := &islands[i]

		if island.LastUpdated.IsZero() {
			island.LastUpdated = now
			e.islandCheckpoints[island.ID] = now // Initialize checkpoint tracking
			db.Save(island)
			continue
		}

		lastTick, hasLast := e.islandLastTicks[island.ID]
		if !hasLast {
			lastTick = island.LastUpdated
		}
		delta := now.Sub(lastTick)
		deltaSeconds := delta.Seconds()
		e.islandLastTicks[island.ID] = now

		// Check Research Completion
		if island.Player.ResearchingTechID != "" && !island.Player.ResearchFinishTime.IsZero() {
			if now.After(island.Player.ResearchFinishTime) {
				// 1. Refetch Player from DB to ensure we have the absolute latest state
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
						freshPlayer.ResearchTotalDurationSeconds = 0

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

		// Sync ship coordinates (Fleet -> Island) to avoid stale data in JSON or flat list
		SyncIslandShipCoordinates(island)

		// Update Fleet Stationing Logic
		UpdateFleetStationing(island, deltaSeconds)

		// Sync again after movement to ensure the list we might save is fresh
		SyncIslandShipCoordinates(island)

		// Checkpoint-based persistence
		lastCheckpoint, exists := e.islandCheckpoints[island.ID]
		shouldCheckpoint := !exists || now.Sub(lastCheckpoint) >= IslandCheckpointInterval

		if shouldCheckpoint {
			island.LastUpdated = now
			e.islandCheckpoints[island.ID] = now

			// Save changes - OMIT Player to prevent reverting the Manual Save above
			island.Player = domain.Player{}

			// Save Island ONLY (Omit associations to avoid overriding movement with stale data)
			if err := db.Omit("Player", "Ships", "Fleets").Save(island).Error; err != nil {
				fmt.Printf("Error saving island %s: %v\n", island.Name, err)
			} else {
				// Save fleets and THEIR ships (the ones moving)
				shipsProcessed := make(map[uuid.UUID]bool)
				for i := range island.Fleets {
					f := &island.Fleets[i]
					db.Save(f)
					for j := range f.Ships {
						s := &f.Ships[j]
						db.Save(s)
						shipsProcessed[s.ID] = true
					}
				}
				// Save unassigned island ships (construction timers, etc)
				// SKIP ships already saved via Fleets to avoid overwriting movement
				for i := range island.Ships {
					s := &island.Ships[i]
					if !shipsProcessed[s.ID] {
						db.Save(s)
					}
				}
			}
		} else {
			island.LastUpdated = now
		}

		// EXTRA: High-frequency save for moving fleets
		// This ensures clients polling every 1s see movement, even if the island checkpoint is 5s.
		for i := range island.Fleets {
			f := &island.Fleets[i]
			if f.State == "Moving" || f.State == "Returning" {
				db.Save(f)
				for j := range f.Ships {
					db.Save(&f.Ships[j])
				}
			}
		}
	}
}

// UpdateFleetStationing handles movement and gathering for stationed fleets
func UpdateFleetStationing(island *domain.Island, deltaSeconds float64) {
	for i := range island.Fleets {
		f := &island.Fleets[i]

		// Fleet Speed: Calculate based on ship composition (weighted average)
		baseSpeed := economy.CalculateFleetSpeed(f)

		// Apply Tech Bonuses on top of ship-type speed
		if island.Player.ID != uuid.Nil {
			var techs []string
			if len(island.Player.UnlockedTechsJSON) > 0 {
				if err := json.Unmarshal(island.Player.UnlockedTechsJSON, &techs); err == nil {
					mods := economy.ComputeTechModifiers(techs)
					baseSpeed *= (1.0 + mods.ShipSpeedMultiplier)
				}
			}
		}

		if f.State == "Moving" && f.TargetX != nil && f.TargetY != nil {
			// Move towards target
			var refShip *domain.Ship
			if len(f.Ships) > 0 {
				refShip = &f.Ships[0]
			}

			if refShip != nil {
				dx := float64(*f.TargetX) - refShip.X
				dy := float64(*f.TargetY) - refShip.Y
				dist := math.Sqrt(dx*dx + dy*dy)

				if dist < 10.0 {
					// Arrived
					f.State = "Stationed"
					now := time.Now()
					f.StationedAt = &now
					// Move all ships to exact target
					for s := range f.Ships {
						f.Ships[s].X = float64(*f.TargetX)
						f.Ships[s].Y = float64(*f.TargetY)
					}
					fmt.Printf("[STATIONING] Fleet %s arrived at station.\n", f.Name)
				} else {
					// Calculate Wind Angle
					angleRad := math.Atan2(dy, dx)
					angleDeg := angleRad * (180 / math.Pi)
					if angleDeg < 0 {
						angleDeg += 360
					}

					// Apply Wind
					windMod := 1.0
					if economy.GlobalWeather != nil {
						windMod = economy.GlobalWeather.GetWindFactor(angleDeg)
					}

					// Move
					currentSpeed := baseSpeed * windMod
					move := currentSpeed * deltaSeconds
					if move > dist {
						move = dist
					}

					ratio := move / dist
					moveX := dx * ratio
					moveY := dy * ratio

					// Update all ships
					for s := range f.Ships {
						f.Ships[s].X += moveX
						f.Ships[s].Y += moveY
					}
				}
			}
		} else if f.State == "Stationed" && f.StationedAt != nil {
			// Gathering Logic
			// 1. Calculate Capacity
			capacity := 0.0
			for _, s := range f.Ships {
				cap := 500.0
				switch s.Type {
				case "sloop":
					cap = 500
				case "brigantine":
					cap = 1500
				case "frigate":
					cap = 3000
				case "galleon":
					cap = 8000
				case "manowar":
					cap = 12000
				}
				capacity += cap
			}

			// 2. Calculate Rate
			minutesStationed := time.Since(*f.StationedAt).Minutes()
			bonusMultiplier := 1.0 + (minutesStationed * 0.01)
			if bonusMultiplier > 2.0 {
				bonusMultiplier = 2.0
			}

			ratePerSec := 2.0 * bonusMultiplier

			// Add to StoredAmount
			f.StoredAmount += ratePerSec * deltaSeconds

			// Check Capacity
			if f.StoredAmount >= capacity {
				f.StoredAmount = capacity
				f.State = "Returning"
				f.StationedAt = nil
				f.StationedNodeID = nil
				homeX := island.X
				homeY := island.Y
				f.TargetX = &homeX
				f.TargetY = &homeY
				fmt.Printf("[STATIONING] Fleet %s full (%.0f). Returning home.\n", f.Name, f.StoredAmount)
			}
		} else if f.State == "Returning" && f.TargetX != nil {
			// Move Home
			var refShip *domain.Ship
			if len(f.Ships) > 0 {
				refShip = &f.Ships[0]
			}

			if refShip != nil {
				dx := float64(*f.TargetX) - refShip.X
				dy := float64(*f.TargetY) - refShip.Y
				dist := math.Sqrt(dx*dx + dy*dy)

				if dist < 10.0 {
					// Arrived Home
					f.State = "Idle"
					f.TargetX = nil
					f.TargetY = nil

					// Deposit Resources
					resType := domain.ResourceType(f.StoredResource)
					island.Resources[resType] += f.StoredAmount

					msg := fmt.Sprintf("Fleet %s returned with %.0f %s", f.Name, f.StoredAmount, resType)
					fmt.Printf("[STATIONING] %s\n", msg)

					f.StoredAmount = 0
					f.StoredResource = ""
				} else {
					// Calculate Wind Angle (Same as Moving)
					angleRad := math.Atan2(dy, dx)
					angleDeg := angleRad * (180 / math.Pi)
					if angleDeg < 0 {
						angleDeg += 360
					}

					// Apply Wind
					windMod := 1.0
					if economy.GlobalWeather != nil {
						windMod = economy.GlobalWeather.GetWindFactor(angleDeg)
					}

					// Move
					currentSpeed := baseSpeed * windMod
					move := currentSpeed * deltaSeconds
					if move > dist {
						move = dist
					}

					ratio := move / dist
					moveX := dx * ratio
					moveY := dy * ratio

					for s := range f.Ships {
						f.Ships[s].X += moveX
						f.Ships[s].Y += moveY
					}
				}
			}
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

// SyncIslandShipCoordinates ensures s.X/Y in island.Ships matches those in island.Fleets.Ships
func SyncIslandShipCoordinates(island *domain.Island) {
	fleetShipMap := make(map[uuid.UUID]*domain.Ship)
	for i := range island.Fleets {
		for j := range island.Fleets[i].Ships {
			s := &island.Fleets[i].Ships[j]
			fleetShipMap[s.ID] = s
		}
	}

	for i := range island.Ships {
		s := &island.Ships[i]
		if fs, ok := fleetShipMap[s.ID]; ok {
			s.X = fs.X
			s.Y = fs.Y
		}
	}
}
