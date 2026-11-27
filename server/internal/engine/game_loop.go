package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/google/uuid"
)

// Engine fait tourner la boucle de jeu (le "tick") sur le monde en mémoire.
type Engine struct {
	world        *World
	saveInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	stopOnce     sync.Once
}

// NewEngine crée le moteur autour d'un monde déjà chargé (voir LoadWorld).
func NewEngine(world *World, saveInterval time.Duration) *Engine {
	if saveInterval <= 0 {
		saveInterval = DefaultSaveInterval
	}
	return &Engine{
		world:        world,
		saveInterval: saveInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start lance deux goroutines :
//   - le tick (10 fois par seconde) qui fait avancer le jeu EN MÉMOIRE ;
//   - la sauvegarde périodique qui écrit ce qui a changé en base.
func (e *Engine) Start() {
	economy.InitWeather()                            // Initialize Weather System
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz tick for fluid movement/collision
	e.wg.Add(2)
	go func() {
		defer e.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.Tick()
			case <-e.stopCh:
				return
			}
		}
	}()
	go func() {
		defer e.wg.Done()
		e.world.RunAutosave(e.saveInterval, e.stopCh)
	}()
}

// Stop arrête le tick, attend qu'il soit vraiment fini, puis fait la
// sauvegarde finale. Après Stop, plus rien ne modifie la mémoire.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopCh)
		e.wg.Wait()
		if err := e.world.Close(); err != nil {
			logger.Error("[ENGINE] final save failed", "error", err)
		}
	})
}

// Tick fait avancer le jeu de 100 ms.
//
// Différence clé avec l'ancienne version : on ne relit PAS la base. On prend
// le verrou du monde, on modifie les îles en mémoire, on note ce qui a changé
// (markDirty), et c'est la sauvegarde périodique qui écrira.
func (e *Engine) Tick() {
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

	w := e.world
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	// Use 0.1s target delta, but could calculate real global delta if needed.
	// For now, 0.1s is safe with the 10Hz ticker.
	economy.UpdatePveTargetSimulation(0.1, w.islands)

	// Process PvP fleet travel (attacks and returns).
	// Le déplacement se fait en mémoire ; les événements rares (arrivée,
	// combat, pillage, dépôt du butin) passent par la base en transaction.
	w.processTravelFleets()

	for i := range w.islands {
		tickIsland(w, &w.islands[i], now)
	}
}

// tickIsland fait avancer une île (production, timers, flottes).
func tickIsland(w *World, island *domain.Island, now time.Time) {
	if island.LastUpdated.IsZero() {
		island.LastUpdated = now
		w.markDirty(island.ID, DirtyIsland)
		return
	}

	// LastUpdated est "l'heure jusqu'à laquelle la production a été comptée".
	// Comme la mémoire garde la valeur d'un tick à l'autre, chaque tick ajoute
	// exactement le temps écoulé depuis le précédent. Au démarrage, la valeur
	// vient de la base : le joueur récupère la production du temps d'arrêt.
	delta := now.Sub(island.LastUpdated)
	if delta < 0 {
		delta = 0
	}
	deltaSeconds := delta.Seconds()
	island.LastUpdated = now
	w.markDirty(island.ID, DirtyIsland)

	// Check Research Completion
	if completeResearchIfDue(island, now) {
		w.markDirty(island.ID, DirtyPlayer)
	}

	// Check Ship Construction Completion
	for j := range island.Ships {
		s := &island.Ships[j]
		if s.State == "UnderConstruction" && !s.FinishTime.IsZero() {
			if now.After(s.FinishTime) {
				s.State = "Ready"
				s.FinishTime = time.Time{} // Clear
				w.markDirty(island.ID, DirtyFleets)
				fmt.Printf("[GAME LOOP] Ship %s Construction Complete!\n", s.Name)
			}
		}
	}

	if calculateResources(island, delta) {
		w.markDirty(island.ID, DirtyBuildings)
	}

	// Sync ship coordinates (Fleet -> Island) to avoid stale data in JSON or flat list
	SyncIslandShipCoordinates(island)

	// Update Fleet Stationing Logic
	harvestedBefore := island.Player.QuestHarvested
	if UpdateFleetStationing(island, deltaSeconds) {
		w.markDirty(island.ID, DirtyFleets)
	}
	if island.Player.QuestHarvested != harvestedBefore {
		w.markDirty(island.ID, DirtyPlayer)
	}

	// Moral de croisière + XP des capitaines en mer
	updateFleetMoraleAndXP(w, island, deltaSeconds)

	// Sync again after movement to ensure the list we might save is fresh
	SyncIslandShipCoordinates(island)
}

// completeResearchIfDue débloque la techno en cours si son timer est fini.
// Renvoie true si le joueur a changé.
//
// Pas besoin de l'écrire en base immédiatement : si le serveur plante avant
// la sauvegarde, au redémarrage le timer sera toujours écoulé et la recherche
// se terminera à nouveau. Un événement qui dépend seulement d'une heure de
// fin est "rejouable", donc il peut attendre la sauvegarde périodique.
func completeResearchIfDue(island *domain.Island, now time.Time) bool {
	p := &island.Player
	if p.ResearchingTechID == "" || p.ResearchFinishTime.IsZero() || !now.After(p.ResearchFinishTime) {
		return false
	}
	techID := p.ResearchingTechID

	var unlocked []string
	if len(p.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(p.UnlockedTechsJSON, &unlocked)
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

	p.UnlockedTechs = unlocked
	newJSON, _ := json.Marshal(unlocked)
	p.UnlockedTechsJSON = newJSON
	p.ResearchingTechID = ""
	p.ResearchFinishTime = time.Time{}
	p.ResearchTotalDurationSeconds = 0
	fmt.Printf("[GAME LOOP] Research Complete: %s for Player %s\n", techID, p.Username)
	return true
}

// UpdateFleetStationing handles movement and gathering for stationed fleets.
// Renvoie true si au moins une flotte a changé (pour la marquer "sale").
func UpdateFleetStationing(island *domain.Island, deltaSeconds float64) bool {
	changed := false
	for i := range island.Fleets {
		f := &island.Fleets[i]

		// Les flottes en attaque/poursuite sont gérées par processTravelFleets
		// (pvp_travel.go), qui compte déjà leur rhum et leur déplacement.
		// Avant, les deux comptaient le rhum, mais la copie périmée de la base
		// écrasait l'une des deux consommations : en mémoire on ne garde que
		// la bonne, pour ne pas leur faire boire le double.
		if isTravelState(f.State) {
			continue
		}
		if f.ConsumesRum() || f.State == domain.FleetStateStationed {
			changed = true
		}

		// Capitaine de la flotte (navire amiral) et ses effets
		captain := economy.FleetCaptain(f, island.Player.Captains)
		capEff := economy.ComputeCaptainEffects(captain)

		// Fleet Speed: Calculate based on ship composition (weighted average)
		baseSpeed := economy.CalculateFleetSpeed(f)
		baseSpeed *= economy.CaptainSpeedMultiplier(capEff, economy.FleetMorale(f), false)

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

		// RUM MECHANIC: Speed Penalty & Consumption (Movement Only)
		// Phase B & C Impl
		hasRum := false
		if f.Cargo != nil && f.Cargo[domain.Rum] > 0 {
			hasRum = true
		}

		// 1. Speed Penalty if Out of Rum
		if !hasRum {
			baseSpeed *= 0.8
		}

		// 2. Consumption (Only when Moving/Returning/SeaStationed)
		if f.ConsumesRum() {
			// Consumption (SSOT w/ Multiplier)
			// Tick is 100ms (0.1s) BUT UpdateFleetStationing is called with deltaSeconds.
			// deltaSeconds matches the actual wall-clock time passed.
			// CalculateFleetRumConsumption returns rate Per Second.
			consPerSec := economy.ComputeRumConsumption(len(f.Ships), false) // isNPC=false
			consumption := consPerSec * deltaSeconds

			// Captain Bonus
			if captain != nil {
				reduc := capEff.RumReductionPct
				if reduc > 0 {
					consumption *= (1.0 - reduc)
					// Log (Throttled ~1%)
					if time.Now().UnixNano()/int64(time.Millisecond)%10000 < 100 { // ~10s interval
						fmt.Printf("[RUM] fleet=%s ships=%d baseSec=%.4f reduc=%.2f final=%.4f\n", f.Name, len(f.Ships), consPerSec, reduc, consumption)
					}
				}
			}

			if f.Cargo == nil {
				f.Cargo = make(map[domain.ResourceType]float64)
			}

			if f.Cargo[domain.Rum] > 0 {
				f.Cargo[domain.Rum] -= consumption
				if f.Cargo[domain.Rum] < 0 {
					f.Cargo[domain.Rum] = 0
					fmt.Printf("[RUM] Fleet %s ran out of Rum!\n", f.Name)
				}
			}

			// Log Speed (Throttled)
			if f.IsMoving() && time.Now().UnixNano()/int64(time.Millisecond)%10000 < 100 {
				finalSpeed := economy.ComputeTravelSpeed(f, false)
				fmt.Printf("[NAV] fleet=%s speed=%.4f/sec\n", f.Name, finalSpeed)
			}

			// SPECIAL LOG for SeaStationed (User Request)
			if f.State == domain.FleetStateSeaStationed && time.Now().UnixNano()/int64(time.Millisecond)%10000 < 100 {
				fmt.Printf("[RUM_SEA] fleet=%s cons=%.4f remaining=%.1f\n", f.ID, consumption, f.Cargo[domain.Rum])
			}
		}

		if f.State == domain.FleetStateMoving && f.TargetX != nil && f.TargetY != nil {
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
					// Snap all ships to exact target before clearing TargetX/Y
					if f.TargetX != nil && f.TargetY != nil {
						for s := range f.Ships {
							f.Ships[s].X = float64(*f.TargetX)
							f.Ships[s].Y = float64(*f.TargetY)
						}
					}

					isHome := f.TargetX != nil && f.TargetY != nil && *f.TargetX == island.X && *f.TargetY == island.Y
					if isHome {
						// Arrived Home
						f.State = domain.FleetStateIdle
						f.TargetX = nil
						f.TargetY = nil
						f.FreeNav = false
						f.StationedNodeID = nil
						f.StationedAt = nil

						// Deposit Resources from Cargo
						if island.Resources == nil {
							island.Resources = make(map[domain.ResourceType]float64)
						}
						depositedLog := ""
						for res, amount := range f.Cargo {
							if amount > 0 {
								deposited := island.Deposit(res, amount)
								if deposited > 0 {
									depositedLog += fmt.Sprintf("%.0f %s, ", deposited, res)
									f.Cargo[res] -= deposited
								}
							}
						}
						if depositedLog != "" {
							fmt.Printf("[STATIONING] Fleet %s arrived HOME and deposited: %s\n", f.Name, depositedLog)
						} else {
							fmt.Printf("[STATIONING] Fleet %s arrived HOME.\n", f.Name)
						}
					} else if f.StationedNodeID != nil {
						f.State = domain.FleetStateStationed
						now := time.Now()
						f.StationedAt = &now
						fmt.Printf("[STATIONING] Fleet %s arrived at station.\n", f.Name)
					} else {
						// Free Navigation arrival
						f.State = domain.FleetStateSeaStationed
						fmt.Printf("[NAV_FREE] Fleet %s arrived at destination (FreeNav).\n", f.Name)
					}
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
					windMod = economy.ApplyWindWithCaptain(capEff, windMod)

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
		} else if f.State == domain.FleetStateStationed && f.StationedAt != nil {
			// Gathering Logic
			// 1. Capacité de cale (configs/ships.json)
			capacity := economy.FleetCargoCapacity(f) * (1 + capEff.CargoPct)

			// 2. Débit de récolte (ressource, richesse du nœud, durée)
			resType := domain.ResourceType(f.StoredResource)
			if resType == "" {
				resType = domain.Wood // Fallback
			}
			minutesStationed := time.Since(*f.StationedAt).Minutes()
			ratePerSec := economy.NodeHarvestRatePerSec(resType, f.NodeRichness, minutesStationed) * (1 + capEff.HarvestPct)
			amountToAdd := ratePerSec * deltaSeconds

			// 3. Add to Cargo (Initialize if nil)
			if f.Cargo == nil {
				f.Cargo = make(map[domain.ResourceType]float64)
			}
			f.Cargo[resType] += amountToAdd

			// 4. Check Total Load vs Capacity
			totalLoad := 0.0
			for _, amount := range f.Cargo {
				totalLoad += amount
			}

			harvested := amountToAdd
			if totalLoad >= capacity {
				// Clamp
				diff := totalLoad - capacity
				if diff > 0 {
					f.Cargo[resType] -= diff
					harvested -= diff
				}

				f.State = domain.FleetStateReturning
				f.StationedAt = nil
				f.StationedNodeID = nil
				homeX := island.X
				homeY := island.Y
				f.TargetX = &homeX
				f.TargetY = &homeY
				fmt.Printf("[STATIONING] Fleet %s full (%.0f/%.0f). Returning home.\n", f.Name, totalLoad, capacity)
			}
			// Quêtes : total récolté sur les spots.
			if harvested > 0 {
				island.Player.QuestHarvested += harvested
			}
		} else if f.State == domain.FleetStateReturning && f.TargetX != nil {
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
					f.State = domain.FleetStateIdle
					f.TargetX = nil
					f.TargetY = nil

					// Deposit Resources from Cargo
					if island.Resources == nil {
						island.Resources = make(map[domain.ResourceType]float64)
					}
					depositedLog := ""
					for res, amount := range f.Cargo {
						if amount > 0 {
							island.Resources[res] += amount
							depositedLog += fmt.Sprintf("%.0f %s, ", amount, res)
							f.Cargo[res] = 0 // Clear cargo
						}
					}

					// Clear deprecated fields just in case
					f.StoredAmount = 0
					f.StoredResource = ""

					if depositedLog != "" {
						fmt.Printf("[STATIONING] Fleet %s returned and deposited: %s\n", f.Name, depositedLog)
					} else {
						fmt.Printf("[STATIONING] Fleet %s returned empty.\n", f.Name)
					}
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
					windMod = economy.ApplyWindWithCaptain(capEff, windMod)

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
	return changed
}

// calculateResources updates island resources based on buildings and techs.
// Renvoie true si un bâtiment a fini sa construction (bâtiments à sauvegarder).
func calculateResources(island *domain.Island, delta time.Duration) bool {
	buildingsChanged := false
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
				buildingsChanged = true
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
	return buildingsChanged
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

// updateFleetMoraleAndXP fait vivre le moral des flottes (remonte au port,
// baisse en mer), donne le moral de départ du capitaine, et l'XP de navigation
// (1 par minute en mer, versée au retour au port).
func updateFleetMoraleAndXP(w *World, island *domain.Island, deltaSeconds float64) {
	if w.moraleFrac == nil {
		w.moraleFrac = make(map[uuid.UUID]float64)
		w.seaSeconds = make(map[uuid.UUID]float64)
		w.prevState = make(map[uuid.UUID]domain.FleetState)
	}
	for i := range island.Fleets {
		f := &island.Fleets[i]
		captain := economy.FleetCaptain(f, island.Player.Captains)
		eff := economy.ComputeCaptainEffects(captain)
		before := economy.FleetMorale(f)
		morale := before

		prev, known := w.prevState[f.ID]
		w.prevState[f.ID] = f.State
		atPort := f.State == domain.FleetStateIdle

		// Départ du port : bonus de moral du capitaine
		if known && prev == domain.FleetStateIdle && !atPort && eff.DepartureMorale > 0 {
			morale += eff.DepartureMorale
		}

		// Moral continu
		frac := w.moraleFrac[f.ID] + economy.MoraleRatePerSecond(f, eff)*deltaSeconds
		whole := math.Trunc(frac)
		morale += int(whole)
		w.moraleFrac[f.ID] = frac - whole
		if morale >= 100 || morale <= 0 {
			w.moraleFrac[f.ID] = 0
		}

		if morale != before || f.MoraleCruise == nil {
			economy.SetFleetMorale(f, morale)
			w.markDirty(island.ID, DirtyFleets)
		}

		// XP de navigation
		if !atPort {
			w.seaSeconds[f.ID] += deltaSeconds
		} else if secs := w.seaSeconds[f.ID]; secs > 0 {
			w.seaSeconds[f.ID] = 0
			xp := int(secs / 60 * economy.CaptainXPPerMinuteAtSea)
			if captain != nil && xp > 0 {
				economy.AddCaptainXP(captain, xp)
				w.markDirty(island.ID, DirtyCaptains)
			}
		}
	}
}
