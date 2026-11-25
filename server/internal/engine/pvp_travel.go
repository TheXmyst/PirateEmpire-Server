package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	CONTACT_RADIUS = 5.0
)

// isTravelState : états gérés par ce fichier (attaque, retour d'attaque,
// poursuites PvE/PvP).
func isTravelState(state domain.FleetState) bool {
	return state == domain.FleetStateTravelingToAttack ||
		state == domain.FleetStateReturningFromAttack ||
		state == domain.FleetStateChasingPvE ||
		state == domain.FleetStateChasingPvP
}

// processTravelFleets fait avancer les flottes en attaque/poursuite.
//
// On sépare deux choses très différentes :
//
//   - le DÉPLACEMENT (99,9 % des ticks) : quelques additions sur X/Y et un
//     peu de rhum en moins. Ça se fait en mémoire, et la flotte est juste
//     marquée "sale". Avant, chaque navire était écrit en base 10 fois par
//     seconde !
//
//   - les ÉVÉNEMENTS (arrivée, combat, pillage, dépôt du butin, poursuite
//     ratée) : rares mais critiques, car ils déplacent des ressources entre
//     joueurs. Ceux-là sont traités tout de suite, en base, avec le code de
//     combat existant (inchangé), via runWithDBLocked : sauvegarde de la
//     mémoire -> combat en base -> rechargement de la mémoire.
//
// Le verrou du monde doit être pris.
func (w *World) processTravelFleets() {
	type fleetRef struct{ island, fleet int }
	index := make(map[uuid.UUID]fleetRef)
	for i := range w.islands {
		for j := range w.islands[i].Fleets {
			index[w.islands[i].Fleets[j].ID] = fleetRef{i, j}
		}
	}
	findFleet := func(id uuid.UUID) *domain.Fleet {
		if ref, ok := index[id]; ok {
			return &w.islands[ref.island].Fleets[ref.fleet]
		}
		return nil
	}

	var events []uuid.UUID
	for i := range w.islands {
		island := &w.islands[i]
		for j := range island.Fleets {
			f := &island.Fleets[j]
			var needsEvent bool
			switch f.State {
			case domain.FleetStateTravelingToAttack:
				needsEvent = stepTowardFixedTarget(f, f.TargetIslandID == nil)
			case domain.FleetStateReturningFromAttack:
				needsEvent = stepTowardFixedTarget(f, false)
			case domain.FleetStateChasingPvE:
				needsEvent = stepChasingPvE(f, island.PlayerID)
			case domain.FleetStateChasingPvP:
				needsEvent = stepChasingPvP(f, findFleet)
			default:
				continue
			}
			if needsEvent {
				events = append(events, f.ID)
			} else {
				w.markDirty(island.ID, DirtyFleets)
			}
		}
	}

	if len(events) == 0 {
		return
	}

	// Événements : une seule passe "base de données" pour tous ceux du tick.
	err := w.runWithDBLocked(func() error {
		for _, id := range events {
			var fleet domain.Fleet
			if err := w.db.Preload("Ships").Preload("Island.Player.Captains").First(&fleet, "id = ?", id).Error; err != nil {
				logger.Warn("[PVP_TRAVEL] Error loading fleet for event", "fleet_id", id, "error", err)
				continue
			}
			if !isTravelState(fleet.State) {
				continue // déjà traité par un événement précédent (ex : poursuite)
			}
			handleTravelFleetDB(w.db, &fleet)
		}
		return nil
	})
	if err != nil {
		logger.Error("[PVP_TRAVEL] events postponed", "error", err)
	}
}

// handleTravelFleetDB : l'ancien traitement, basé sur la base de données.
// N'est plus appelé que pour les événements.
func handleTravelFleetDB(db *gorm.DB, fleet *domain.Fleet) {
	switch fleet.State {
	case domain.FleetStateTravelingToAttack:
		handleTravelingToAttack(db, fleet)
	case domain.FleetStateReturningFromAttack:
		handleReturningFromAttack(db, fleet)
	case domain.FleetStateChasingPvE:
		handleChasingPvE(db, fleet)
	case domain.FleetStateChasingPvP:
		handleChasingPvP(db, fleet)
	}
}

// ProcessPvpTravelFleets : ancienne entrée 100 % base de données, gardée pour
// les outils/tests. Le moteur utilise processTravelFleets.
func ProcessPvpTravelFleets(db *gorm.DB) {
	var fleets []domain.Fleet
	states := []string{
		string(domain.FleetStateTravelingToAttack),
		string(domain.FleetStateReturningFromAttack),
		string(domain.FleetStateChasingPvE),
		string(domain.FleetStateChasingPvP),
	}
	if err := db.Preload("Ships").Preload("Island.Player.Captains").Where("state IN ?", states).Find(&fleets).Error; err != nil {
		logger.Warn("[PVP_TRAVEL] Error loading fleets", "error", err)
		return
	}
	for i := range fleets {
		handleTravelFleetDB(db, &fleets[i])
	}
}

// ---------------------------------------------------------------------------
// Déplacement en mémoire. Chaque fonction step* reproduit exactement les
// conditions de son équivalent handle* : si handle* ferait autre chose que
// "avancer", step* renvoie true (= événement à traiter en base).
// ---------------------------------------------------------------------------

// consumeTravelRum applique la pénalité de vitesse sans rhum et consomme le
// rhum d'un tick (0,1 s). Renvoie la vitesse par tick.
func consumeTravelRum(fleet *domain.Fleet) float64 {
	speed := economy.ComputeTravelSpeed(fleet, false) * 0.1 // isNPC=false, per tick (0.1s)

	hasRum := fleet.Cargo != nil && fleet.Cargo[domain.Rum] > 0
	if !hasRum {
		speed *= 0.8
	}

	consPerTick := economy.ComputeRumConsumption(len(fleet.Ships), false) * 0.1
	if fleet.Cargo == nil {
		fleet.Cargo = make(map[domain.ResourceType]float64)
	}
	if fleet.Cargo[domain.Rum] > 0 {
		fleet.Cargo[domain.Rum] -= consPerTick
		if fleet.Cargo[domain.Rum] < 0 {
			fleet.Cargo[domain.Rum] = 0
		}
	}
	return speed
}

// moveEachShipToward : chaque navire avance vers la cible (attaque / retour).
func moveEachShipToward(fleet *domain.Fleet, speed float64) {
	for i := range fleet.Ships {
		s := &fleet.Ships[i]
		dx := float64(*fleet.TargetX) - s.X
		dy := float64(*fleet.TargetY) - s.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > 0 {
			s.X += (dx / dist) * speed
			s.Y += (dy / dist) * speed
		}
	}
}

// stepTowardFixedTarget : attaque d'île ou retour au port.
func stepTowardFixedTarget(fleet *domain.Fleet, invalidTarget bool) bool {
	if fleet.TargetX == nil || fleet.TargetY == nil || invalidTarget || len(fleet.Ships) == 0 {
		return true
	}
	ship := fleet.Ships[0]
	dx := float64(*fleet.TargetX) - ship.X
	dy := float64(*fleet.TargetY) - ship.Y
	if math.Sqrt(dx*dx+dy*dy) <= 5 {
		return true // arrivé : combat ou dépôt du butin
	}
	speed := consumeTravelRum(fleet)
	moveEachShipToward(fleet, speed)
	return false
}

// stepChasingPvE : poursuite d'une cible PvE qui bouge.
func stepChasingPvE(fleet *domain.Fleet, playerID uuid.UUID) bool {
	if fleet.TargetPveID == nil {
		return true
	}
	target := economy.GetPveTargetByUUID(playerID, *fleet.TargetPveID)
	if target == nil {
		return true
	}
	tx := int(target.RealX)
	ty := int(target.RealY)
	fleet.TargetX = &tx
	fleet.TargetY = &ty
	if len(fleet.Ships) == 0 {
		return true
	}
	ship := fleet.Ships[0]
	dx := target.RealX - ship.X
	dy := target.RealY - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)
	if distance < 5.0 {
		return true // contact : combat
	}
	speed := consumeTravelRum(fleet)
	if distance > 0 {
		for i := range fleet.Ships {
			fleet.Ships[i].X += (dx / distance) * speed
			fleet.Ships[i].Y += (dy / distance) * speed
		}
	}
	return false
}

// stepChasingPvP : interception de la flotte d'un autre joueur.
func stepChasingPvP(fleet *domain.Fleet, findFleet func(uuid.UUID) *domain.Fleet) bool {
	if fleet.ChasingFleetID == nil {
		return true
	}
	target := findFleet(*fleet.ChasingFleetID)
	if target == nil {
		return true
	}
	if target.State == domain.FleetStateIdle || target.State == domain.FleetStateStationed {
		return true // safe harbor
	}
	if fleet.InterceptStartedAt != nil && time.Since(*fleet.InterceptStartedAt) > 2*time.Minute {
		return true // timeout
	}
	if len(target.Ships) == 0 || len(fleet.Ships) == 0 {
		return true
	}
	targetShip := target.Ships[0]
	tx := int(targetShip.X)
	ty := int(targetShip.Y)
	fleet.TargetX = &tx
	fleet.TargetY = &ty

	ship := fleet.Ships[0]
	dx := targetShip.X - ship.X
	dy := targetShip.Y - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)
	if distance <= CONTACT_RADIUS {
		return true // contact : combat
	}
	applyPvpMovement(fleet, dx, dy, distance)
	return false
}

// applyPvpMovement : déplacement d'une poursuite PvP (sans écrire en base).
func applyPvpMovement(fleet *domain.Fleet, dx, dy, distance float64) {
	speed := consumeTravelRum(fleet)
	if distance > 0 {
		ratio := speed / distance
		if ratio > 1 {
			ratio = 1
		}
		for i := range fleet.Ships {
			fleet.Ships[i].X += dx * ratio
			fleet.Ships[i].Y += dy * ratio
		}
	}
}

// handleChasingPvE handles tracking of moving PvE targets
func handleChasingPvE(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetPveID == nil {
		fmt.Printf("[PVE_TRACK] abort fleet=%s reason=no_target_id\n", fleet.Name)
		returnHome(db, fleet)
		return
	}

	// Load PvE Target
	target := economy.GetPveTargetByUUID(fleet.IslandID, *fleet.TargetPveID) // Use Fleet IslandID as PlayerID proxy if needed?
	// Wait, GetPveTargetByUUID needs PlayerID. Fleet.IslandID is Island UUID.
	// We loaded fleet.Island.Player.Captains but not fleet.Island.Player?
	// Actually ProcessPvpTravelFleets preloads Island.Player.Captains, so Island.PlayerID is available.
	// We need to pass the PlayerID to GetPveTargetByUUID.
	// The fleets query preloads Island.Player...

	// Refetch PlayerID if missing (safety)
	var playerID uuid.UUID
	if fleet.Island.PlayerID != uuid.Nil {
		playerID = fleet.Island.PlayerID
	} else {
		// Fallback lookup
		var island domain.Island
		if err := db.Select("player_id").First(&island, "id = ?", fleet.IslandID).Error; err == nil {
			playerID = island.PlayerID
		}
	}

	// Try to get target
	target = economy.GetPveTargetByUUID(playerID, *fleet.TargetPveID)

	if target == nil {
		fmt.Printf("[PVE_TRACK] abort fleet=%s reason=target_lost_or_expired\n", fleet.Name)
		returnHome(db, fleet)
		return
	}

	// Update Fleet Target Coords to match Moving Target (Tracking)
	tx := int(target.RealX)
	ty := int(target.RealY)
	fleet.TargetX = &tx
	fleet.TargetY = &ty

	// Get Fleet Position
	if len(fleet.Ships) == 0 {
		returnHome(db, fleet)
		return
	}
	ship := fleet.Ships[0]

	// Calculate Distance
	dx := target.RealX - ship.X
	dy := target.RealY - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Check Engage Radius (5.0)
	if distance < 5.0 {
		fmt.Printf("[PVE_TRACK] engage fleet=%s target=%s dist=%.2f\n", fleet.Name, target.Name, distance)

		// Trigger Combat
		combatResult, loot, err := executePveCombat(db, fleet, target)
		if err != nil {
			logger.Error("[PVE_TRACK] Combat error", "error", err)
			returnHome(db, fleet)
			return
		}

		// Handle Loot
		if combatResult.Winner == "fleet_a" && len(loot) > 0 {
			fleet.AttackLoot = loot
			lootJSON, _ := json.Marshal(loot)
			fleet.AttackLootJSON = lootJSON

			if fleet.Cargo == nil {
				fleet.Cargo = make(map[domain.ResourceType]float64)
			}
			for res, amount := range loot {
				fleet.Cargo[res] += amount
			}
		}

		// Consume Target (Remove from cache)
		economy.ConsumePveTarget(playerID, target.ID.String())

		// Return Home
		returnHome(db, fleet)
		return
	}

	// Move Fleet (normalement fait en mémoire par stepChasingPvE)
	speed := consumeTravelRum(fleet)
	if distance > 0 {
		for i := range fleet.Ships {
			s := &fleet.Ships[i]
			s.X += (dx / distance) * speed
			s.Y += (dy / distance) * speed
			db.Save(s)
		}
	}

	db.Save(fleet)
}

func returnHome(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.FreeNav {
		// In FreeNav mode, we don't auto-return. We stay at sea.
		fleet.State = domain.FleetStateSeaStationed
		fleet.TargetX = nil
		fleet.TargetY = nil
		fleet.TargetPveID = nil
		fleet.TargetIslandID = nil
		fmt.Printf("[NAV_FREE] Combat end: fleet=%s staying SeaStationed (FreeNav)\n", fleet.ID)
		db.Save(fleet)
		return
	}

	fleet.State = domain.FleetStateReturningFromAttack
	fleet.TargetPveID = nil
	fleet.TargetIslandID = nil

	var homeIsland domain.Island
	if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
		fleet.TargetX = &homeIsland.X
		fleet.TargetY = &homeIsland.Y
	}
	db.Save(fleet)
}

// executePveCombat triggers PvE combat
func executePveCombat(db *gorm.DB, attackerFleet *domain.Fleet, target *economy.PveTarget) (*economy.CombatResult, map[domain.ResourceType]float64, error) {
	// Generate NPC Fleet from Target
	npcFleet := economy.GenerateNpcFleet(target.Tier, target.ID.String())

	// Captain
	// We need to use the target.Captain
	// But ExecuteNavalCombat expects *domain.Captain
	// We can use target.Captain directly

	var attackerCaptain *domain.Captain
	// Load attacker captain
	// Simple lookup for now
	// TODO: Proper captain assignment lookup

	// Engage
	engResult := economy.ComputeEngagementMorale(*attackerFleet, npcFleet, attackerCaptain, &target.Captain)

	// Execute
	combatRes, err := economy.ExecuteNavalCombat(attackerFleet, &npcFleet, attackerCaptain, &target.Captain, engResult, time.Now().UnixNano())
	if err != nil {
		return nil, nil, err
	}

	// Loot Calculation (Simplified for PvE)
	loot := make(map[domain.ResourceType]float64)
	if combatRes.Winner == "fleet_a" {
		// Tier based loot
		baseLoot := float64(target.Tier * 1000)
		loot[domain.Gold] = baseLoot
		loot[domain.Wood] = baseLoot * 0.5
		if target.Tier >= 2 {
			loot[domain.Rum] = baseLoot * 0.2
		}

		// Rééquilibrage : chance de gagner un ticket de gacha selon le palier
		if rand.Float64() < economy.PveTicketDropChance[target.Tier] {
			loot[domain.CaptainTicket] = 1
		}
	}

	// Destroy Attacker Ships (if any)
	for _, id := range combatRes.ShipsDestroyedA {
		economy.DestroyShipHard(db, id)
	}

	return &combatRes, loot, nil
}

// executePvpCombat triggers the PvP combat logic (simplified version)
// Returns combat result and loot
func executePvpCombat(db *gorm.DB, attackerFleet *domain.Fleet, targetIslandID uuid.UUID) (*economy.CombatResult, map[domain.ResourceType]float64, error) {
	// Load target island with buildings
	var targetIsland domain.Island
	if err := db.Preload("Buildings").Preload("Player").First(&targetIsland, "id = ?", targetIslandID).Error; err != nil {
		return nil, nil, fmt.Errorf("target island not found: %v", err)
	}

	// Load attacker island
	var attackerIsland domain.Island
	if err := db.First(&attackerIsland, "id = ?", attackerFleet.IslandID).Error; err != nil {
		return nil, nil, fmt.Errorf("attacker island not found: %v", err)
	}

	// Get defender fleet (use active fleet or militia)
	var defenderFleet domain.Fleet
	hasDefenderFleet := false

	if targetIsland.ActiveFleetID != nil {
		if err := db.Preload("Ships").Where("id = ?", *targetIsland.ActiveFleetID).First(&defenderFleet).Error; err == nil {
			hasDefenderFleet = true
		}
	}

	// If no active fleet, create militia fleet (simplified)
	if !hasDefenderFleet {
		defenderFleet = domain.Fleet{
			ID:       uuid.New(),
			IslandID: targetIsland.ID,
			Name:     "Militia",
			State:    domain.FleetStateIdle,
			Ships:    []domain.Ship{}, // Empty fleet = militia uses island garrison
		}
	}

	// Get captains
	var attackerCaptain, defenderCaptain *domain.Captain
	// TODO: Load actual captains if needed

	// Compute engagement
	engResult := economy.ComputeEngagementMorale(*attackerFleet, defenderFleet, attackerCaptain, defenderCaptain)

	// Execute combat
	combatRes, err := economy.ExecuteNavalCombat(attackerFleet, &defenderFleet, attackerCaptain, defenderCaptain, engResult, time.Now().UnixNano())
	if err != nil {
		return nil, nil, err
	}

	// Calculate loot if attacker won
	loot := make(map[domain.ResourceType]float64)
	if combatRes.Winner == "fleet_a" {
		defenderTH := economy.TownHallLevel(&targetIsland)

		// Deserialize target island resources
		if len(targetIsland.ResourcesJSON) > 0 {
			json.Unmarshal(targetIsland.ResourcesJSON, &targetIsland.Resources)
		}

		// Butin : 25 % au-dessus de la réserve, plafonné par la cale libre
		_, _, freeCargo := economy.ComputeFleetPayload(attackerFleet)
		loot = economy.ComputePvpLoot(targetIsland.Resources, defenderTH, freeCargo)
		for res, stolen := range loot {
			targetIsland.Resources[res] -= stolen
		}

		// Apply peace shield to defender
		shieldEnd := time.Now().Add(4 * time.Hour)
		targetIsland.ProtectedUntil = &shieldEnd

		// Save target island
		resJSON, _ := json.Marshal(targetIsland.Resources)
		targetIsland.ResourcesJSON = resJSON
		db.Save(&targetIsland)
	}

	// Destroy ships (attacker only in this simplified version)
	for _, id := range combatRes.ShipsDestroyedA {
		economy.DestroyShipHard(db, id)
	}

	return &combatRes, loot, nil
}

// pvpArrivalBlockReason renvoie pourquoi la cible d'un raid ne peut plus être
// attaquée à l'arrivée de la flotte, ou "" si le combat peut avoir lieu.
func pvpArrivalBlockReason(db *gorm.DB, fleet *domain.Fleet) string {
	var target domain.Island
	if err := db.Preload("Player").Preload("Buildings").First(&target, "id = ?", *fleet.TargetIslandID).Error; err != nil {
		return "" // Cible introuvable : executePvpCombat gère l'erreur
	}
	var home domain.Island
	if err := db.Preload("Player").First(&home, "id = ?", fleet.IslandID).Error; err != nil {
		return ""
	}
	return economy.PvpTargetBlockReason(&home.Player, &target, time.Now())
}

// handleTravelingToAttack checks if fleet arrived at target and triggers combat
func handleTravelingToAttack(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetX == nil || fleet.TargetY == nil || fleet.TargetIslandID == nil {
		logger.Warn("[PVP_TRAVEL] Fleet has invalid target, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = domain.FleetStateIdle
		db.Save(fleet)
		return
	}

	// Get fleet's current position (use first ship's position)
	if len(fleet.Ships) == 0 {
		logger.Warn("[PVP_TRAVEL] Fleet has no ships, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Calculate distance to target (Float Precision!)
	ship := fleet.Ships[0]
	// distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	dx := float64(*fleet.TargetX) - ship.X
	dy := float64(*fleet.TargetY) - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Debug Log (Throttled)
	if time.Now().UnixNano()/int64(time.Millisecond)%5000 < 100 { // ~5s interval
		fmt.Printf("[PVP_TRAVEL] Fleet %s dist=%.4f target=(%d,%d) ship=(%.2f,%.2f)\n", fleet.Name, distance, *fleet.TargetX, *fleet.TargetY, ship.X, ship.Y)
	}

	// Move ships toward target if not arrived
	if distance > 5 {
		// (Le moteur fait normalement ce déplacement en mémoire, voir
		// stepTowardFixedTarget. Ce chemin ne sert plus qu'à l'entrée base.)
		speed := consumeTravelRum(fleet)
		moveEachShipToward(fleet, speed)
		for i := range fleet.Ships {
			db.Save(&fleet.Ships[i])
		}
		db.Save(fleet)
		return
	}

	// Check if arrived (distance < 5 units)
	if distance < 5 {
		logger.Info("[PVP] Fleet arrived at target, triggering combat", "fleet_id", fleet.ID)

		// Protections revérifiées à l'arrivée (bouclier de paix posé entre-temps,
		// etc.). Si la cible est protégée, la flotte rentre sans combattre.
		if reason := pvpArrivalBlockReason(db, fleet); reason != "" {
			logger.Info("[PVP_TRAVEL] Target protected on arrival, fleet returns home", "fleet_id", fleet.ID, "reason", reason)
			fleet.TargetIslandID = nil
			fleet.State = domain.FleetStateReturningFromAttack
			var homeIsland domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
				fleet.TargetX = &homeIsland.X
				fleet.TargetY = &homeIsland.Y
			}
			db.Save(fleet)
			return
		}

		// Trigger combat
		combatResult, loot, err := executePvpCombat(db, fleet, *fleet.TargetIslandID)
		if err != nil {
			logger.Error("[PVP_TRAVEL] Combat error", "error", err)
			fleet.State = domain.FleetStateIdle
			db.Save(fleet)
			return
		}

		// Store loot in fleet
		if combatResult.Winner == "fleet_a" && len(loot) > 0 {
			// 1. Keep AttackLoot for "Battle Report" UI
			fleet.AttackLoot = loot
			lootJSON, _ := json.Marshal(loot)
			fleet.AttackLootJSON = lootJSON

			// 2. Add to Cargo for physical transport
			if fleet.Cargo == nil {
				fleet.Cargo = make(map[domain.ResourceType]float64)
			}
			for res, amount := range loot {
				fleet.Cargo[res] += amount
			}
		}

		// Set returning state or SeaStationed if FreeNav
		if fleet.FreeNav {
			fleet.State = domain.FleetStateSeaStationed
			fleet.TargetX = nil
			fleet.TargetY = nil
			fmt.Printf("[NAV_FREE] PvP Combat end: fleet=%s staying SeaStationed (FreeNav)\n", fleet.ID)
		} else {
			fleet.State = domain.FleetStateReturningFromAttack
			// Set target to home island
			var homeIsland domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
				fleet.TargetX = &homeIsland.X
				fleet.TargetY = &homeIsland.Y
			}
		}

		fleet.TargetIslandID = nil
		db.Save(fleet)
		logger.Info("[PVP_TRAVEL] Combat finished", "fleet_id", fleet.ID, "free_nav", fleet.FreeNav)
	} else {
		// Continue moving (no log spam)
	}
}

// handleReturningFromAttack checks if fleet arrived home and deposits loot
func handleReturningFromAttack(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetX == nil || fleet.TargetY == nil {
		logger.Warn("[PVP_TRAVEL] Fleet has invalid return target, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = domain.FleetStateIdle
		db.Save(fleet)
		return
	}

	// Get fleet's current position
	if len(fleet.Ships) == 0 {
		logger.Warn("[PVP_TRAVEL] Fleet has no ships, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Calculate distance to home (Float Precision!)
	ship := fleet.Ships[0]
	// distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	dx := float64(*fleet.TargetX) - ship.X
	dy := float64(*fleet.TargetY) - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Debug Log (Throttled)
	if time.Now().UnixNano()/int64(time.Millisecond)%5000 < 100 {
		fmt.Printf("[PVP_TRAVEL] Fleet %s (returning) dist=%.4f\n", fleet.Name, distance)
	}

	// Move ships toward home if not arrived
	if distance > 5 {
		// (Le moteur fait normalement ce déplacement en mémoire, voir
		// stepTowardFixedTarget. Ce chemin ne sert plus qu'à l'entrée base.)
		speed := consumeTravelRum(fleet)
		moveEachShipToward(fleet, speed)
		for i := range fleet.Ships {
			db.Save(&fleet.Ships[i])
		}
		db.Save(fleet)
		return
	}

	// Check if arrived home (distance < 5 units)
	if distance < 5 {
		logger.Info("[PVP_TRAVEL] Fleet arrived home, depositing loot", "fleet_id", fleet.ID)

		// Deposit loot (From Cargo now, covering both Loot and any other stock)
		depositedLog := ""
		if len(fleet.Cargo) > 0 {
			var island domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&island).Error; err == nil {
				// Deserialize resources if needed
				if island.Resources == nil {
					island.Resources = make(map[domain.ResourceType]float64)
				}

				// Add all Cargo to island
				for resType, amount := range fleet.Cargo {
					if amount > 0 {
						island.Resources[resType] += amount
						depositedLog += fmt.Sprintf("%.0f %s, ", amount, resType)
					}
				}

				// Serialize resources
				resJSON, _ := json.Marshal(island.Resources)
				island.ResourcesJSON = resJSON

				db.Save(&island)
			}
		}

		// Fallback: If Cargo was empty but AttackLoot existed (migration case?), deposit AttackLoot
		// This handles in-flight fleets during update
		if len(fleet.Cargo) == 0 && len(fleet.AttackLoot) > 0 {
			var island domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&island).Error; err == nil {
				// Deserialize resources if needed
				if island.Resources == nil {
					island.Resources = make(map[domain.ResourceType]float64)
				}
				for resType, amount := range fleet.AttackLoot {
					island.Resources[resType] += amount
					depositedLog += fmt.Sprintf("%.0f %s (legacy), ", amount, resType)
				}
				resJSON, _ := json.Marshal(island.Resources)
				island.ResourcesJSON = resJSON
				db.Save(&island)
			}
		}

		// Reset fleet
		fleet.State = domain.FleetStateIdle
		fleet.TargetX = nil
		fleet.TargetY = nil
		fleet.AttackLoot = nil
		fleet.AttackLootJSON = nil
		fleet.Cargo = nil // Clear cargo after deposit

		db.Save(fleet)
		logger.Info("[PVP_TRAVEL] Fleet reset to Idle", "fleet_id", fleet.ID)
	} else {
		// Continue moving (no log spam)
	}
}

// handleChasingPvP handles real-time pursuit of another player's fleet
func handleChasingPvP(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.ChasingFleetID == nil {
		failIntercept(db, fleet, "no_target_id", false)
		return
	}

	// 1. Load Target Fleet
	var target domain.Fleet
	if err := db.Preload("Ships").First(&target, "id = ?", *fleet.ChasingFleetID).Error; err != nil {
		failIntercept(db, fleet, "target_lost", false)
		return
	}

	// 2. Safe Harbor Check (Is the target at its island?)
	// If target is Docked or Idle at island, it's safe.
	if target.State == domain.FleetStateIdle || target.State == domain.FleetStateStationed {
		failIntercept(db, fleet, "safe_harbor", true)
		return
	}

	// 3. Timeout Check (2 minutes)
	if fleet.InterceptStartedAt != nil && time.Since(*fleet.InterceptStartedAt) > 2*time.Minute {
		failIntercept(db, fleet, "timeout", true)
		return
	}

	// 4. Update Destination (Real-time Tracking)
	if len(target.Ships) == 0 {
		failIntercept(db, fleet, "target_no_ships", false)
		return
	}
	targetShip := target.Ships[0]
	tx := int(targetShip.X)
	ty := int(targetShip.Y)
	fleet.TargetX = &tx
	fleet.TargetY = &ty

	// 5. Calculate Distance
	if len(fleet.Ships) == 0 {
		returnHome(db, fleet)
		return
	}
	ship := fleet.Ships[0]
	dx := targetShip.X - ship.X
	dy := targetShip.Y - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// 6. Contact Detection
	if distance <= CONTACT_RADIUS {
		fmt.Printf("[PVP_INTERCEPT] contact! trigger combat attacker=%s target=%s dist=%.2f\n", fleet.ID, target.ID, distance)

		// CLEAR CHASE FIELDS on both sides first to avoid infinite loops if combat logic fails
		targetID := target.ID
		db.Model(&domain.Fleet{}).Where("id = ?", targetID).Update("chased_by_fleet_id", nil)
		fleet.ChasingFleetID = nil
		fleet.InterceptStartedAt = nil
		fleet.InterceptTargetPlayerID = nil

		// Load target island ID for loot/shield logic (ExecutePvpCombat requires it)
		// Trigger combat using existing SSOT path
		combatRes, loot, err := executePvpCombat(db, fleet, target.IslandID)
		if err != nil {
			fmt.Printf("[PVP_INTERCEPT] combat error: %v\n", err)
			returnHome(db, fleet)
			return
		}

		// Apply results (similar to handleTravelingToAttack)
		if combatRes.Winner == "fleet_a" && len(loot) > 0 {
			fleet.AttackLoot = loot
			lootJSON, _ := json.Marshal(loot)
			fleet.AttackLootJSON = lootJSON

			if fleet.Cargo == nil {
				fleet.Cargo = make(map[domain.ResourceType]float64)
			}
			for res, amount := range loot {
				fleet.Cargo[res] += amount
			}
		}

		if fleet.FreeNav {
			fleet.State = domain.FleetStateSeaStationed
			fleet.TargetX = nil
			fleet.TargetY = nil
		} else {
			fleet.State = domain.FleetStateReturningFromAttack
			// Set target to home island
			var homeIsland domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
				fleet.TargetX = &homeIsland.X
				fleet.TargetY = &homeIsland.Y
			}
		}
		db.Save(fleet)
		return
	}

	// 7. Movement & Rhum Consumption (Logic shared with TravelingToAttack helper)
	handlePvpMovement(db, fleet, dx, dy, distance)
}

// handlePvpMovement : déplacement + sauvegarde (entrée base uniquement).
func handlePvpMovement(db *gorm.DB, fleet *domain.Fleet, dx, dy, distance float64) {
	applyPvpMovement(fleet, dx, dy, distance)
	for i := range fleet.Ships {
		db.Save(&fleet.Ships[i])
	}
	db.Save(fleet)
}

// failIntercept handles interception failure cases (timeout, safe harbor)
func failIntercept(db *gorm.DB, fleet *domain.Fleet, reason string, applyCooldown bool) {
	fmt.Printf("[PVP_INTERCEPT] fail reason=%s attacker_fleet=%s\n", reason, fleet.ID)

	targetFleetID := fleet.ChasingFleetID
	targetPlayerID := fleet.InterceptTargetPlayerID

	// 1. Clear fields on attacker
	if fleet.FreeNav {
		fleet.State = domain.FleetStateSeaStationed
		fleet.TargetX = nil
		fleet.TargetY = nil
	} else {
		returnHome(db, fleet)
	}
	fleet.ChasingFleetID = nil
	fleet.InterceptStartedAt = nil
	fleet.InterceptTargetPlayerID = nil
	db.Save(fleet)

	// 2. Clear fields on target if known
	if targetFleetID != nil {
		db.Model(&domain.Fleet{}).Where("id = ?", *targetFleetID).Update("chased_by_fleet_id", nil)
	}

	// 3. Apply Cooldown if applicable
	if applyCooldown && targetPlayerID != nil {
		blockedUntil := time.Now().Add(10 * time.Minute)

		// Get attacker player ID from island
		var island domain.Island
		if err := db.Select("player_id").First(&island, "id = ?", fleet.IslandID).Error; err == nil {
			// Upsert cooldown
			var cooldown domain.PvPInterceptCooldown
			if err := db.Where("attacker_player_id = ? AND target_player_id = ?", island.PlayerID, *targetPlayerID).First(&cooldown).Error; err == nil {
				db.Model(&cooldown).Update("blocked_until", blockedUntil)
			} else {
				db.Create(&domain.PvPInterceptCooldown{
					AttackerPlayerID: island.PlayerID,
					TargetPlayerID:   *targetPlayerID,
					BlockedUntil:     blockedUntil,
				})
			}
			fmt.Printf("[PVP_INTERCEPT] cooldown applied: attacker=%s target=%s until=%v\n", island.PlayerID, *targetPlayerID, blockedUntil)
		}
	}
}
