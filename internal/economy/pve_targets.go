package economy

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/gamedata"
	"github.com/google/uuid"
)

// PveTargetCacheEntry represents a cached PVE target entry
type PveTargetCacheEntry struct {
	Targets   []PveTarget
	ExpiresAt time.Time
	PlayerID  uuid.UUID
	IslandX   int
	IslandY   int
}

const (
	TargetsPerTier = 3 // Standard density
)

// PveTarget represents a PVE target (NPC fleet) on the world map
type PveTarget struct {
	ID   string `json:"id"`   // Stable ID: "npc-<playerID>-<slotIndex>"
	X    int    `json:"x"`    // Position X on world map (Legacy/Int)
	Y    int    `json:"y"`    // Position Y on world map (Legacy/Int)
	Tier int    `json:"tier"` // Tier 1, 2, or 3 (difficulty)
	Name string `json:"name"` // Display name (e.g., "Corsaires égarés")

	// Movement Simulation (New)
	RealX        float64        `json:"real_x"`                   // High-precision X
	RealY        float64        `json:"real_y"`                   // High-precision Y
	TargetX      float64        `json:"target_x"`                 // Destination X
	TargetY      float64        `json:"target_y"`                 // Destination Y
	Speed        float64        `json:"speed"`                    // Movement speed (units/sec)
	NextChangeAt time.Time      `json:"next_change_at,omitempty"` // Next trajectory change
	Captain      domain.Captain `json:"captain"`                  // Representative Captain
}

var (
	pveTargetCache = make(map[uuid.UUID]*PveTargetCacheEntry)
	pveCacheMutex  sync.RWMutex
	cacheTTL       = 10 * time.Minute
)

// GetPveTargets returns 3 PVE targets for a player's island
// Uses cache if available and not expired, otherwise generates new targets
func GetPveTargets(playerID uuid.UUID, islandX, islandY int) []PveTarget {
	pveCacheMutex.RLock()
	entry, exists := pveTargetCache[playerID]
	pveCacheMutex.RUnlock()

	// Check if cache is valid
	if exists && time.Now().Before(entry.ExpiresAt) && entry.IslandX == islandX && entry.IslandY == islandY {
		return entry.Targets
	}

	// Generate new targets
	targets := generatePveTargets(playerID, islandX, islandY)

	// Update cache
	pveCacheMutex.Lock()
	pveTargetCache[playerID] = &PveTargetCacheEntry{
		Targets:   targets,
		ExpiresAt: time.Now().Add(cacheTTL),
		PlayerID:  playerID,
		IslandX:   islandX,
		IslandY:   islandY,
	}
	pveCacheMutex.Unlock()

	return targets
}

// GetPveTargetByID returns a target by ID from cache (for tier lookup)
func GetPveTargetByID(playerID uuid.UUID, targetID string) *PveTarget {
	pveCacheMutex.RLock()
	defer pveCacheMutex.RUnlock()

	entry, exists := pveTargetCache[playerID]
	if !exists {
		return nil
	}

	for i := range entry.Targets {
		if entry.Targets[i].ID == targetID {
			return &entry.Targets[i]
		}
	}

	return nil
}

// ConsumePveTarget removes a target from cache (when engaged)
func ConsumePveTarget(playerID uuid.UUID, targetID string) {
	pveCacheMutex.Lock()
	defer pveCacheMutex.Unlock()

	entry, exists := pveTargetCache[playerID]
	if !exists {
		return
	}

	// Remove the target from the list
	newTargets := make([]PveTarget, 0, len(entry.Targets))
	for _, t := range entry.Targets {
		if t.ID != targetID {
			newTargets = append(newTargets, t)
		}
	}

	entry.Targets = newTargets
	// Cache expires immediately if all targets consumed, or keep remaining ones
	if len(newTargets) == 0 {
		delete(pveTargetCache, playerID)
	}
}

// generatePveTargets generates targets around the player's island with initial movement vectors
func generatePveTargets(playerID uuid.UUID, islandX, islandY int) []PveTarget {
	total := TargetsPerTier * 3
	targets := make([]PveTarget, 0, total)
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(playerID[0])))

	// Tier names
	tierNames := map[int][]string{
		1: {"Corsaires égarés", "Pirates solitaires", "Épaves errantes"},
		2: {"Chasseurs de primes", "Flibustiers aguerris", "Raiders expérimentés"},
		3: {"Escadre fantôme", "Armada redoutable", "Légion noire"},
	}

	// Generate targets for each tier
	count := 0
	for tier := 1; tier <= 3; tier++ {
		for j := 0; j < TargetsPerTier; j++ {
			// Random position 200-500 from island
			angle := rng.Float64() * 2.0 * math.Pi
			radius := 200.0 + rng.Float64()*300.0
			x := float64(islandX) + radius*math.Cos(angle)
			y := float64(islandY) + radius*math.Sin(angle)

			// Simple clamping
			if x < -1000 {
				x = -1000
			} else if x > 1000 {
				x = 1000
			}
			if y < -1000 {
				y = -1000
			} else if y > 1000 {
				y = 1000
			}

			// 1. Random initial destination
			tx := -1000.0 + rng.Float64()*2000.0
			ty := -1000.0 + rng.Float64()*2000.0

			// 2. Determine representative fleet composition and BASE SPEED
			var shipType string
			switch tier {
			case 1:
				shipType = "sloop"
			case 2:
				shipType = "brigantine"
			case 3:
				shipType = "frigate"
			}

			// Create a dummy fleet to calculate base speed via CalculateFleetSpeed
			dummyFleet := domain.Fleet{
				Ships: []domain.Ship{
					{Type: shipType},
				},
			}
			baseSpeed := CalculateFleetSpeed(&dummyFleet)

			// 3. Generate a representative captain based on tier
			names := tierNames[tier]
			captain := domain.Captain{
				ID:         uuid.New(),
				Name:       fmt.Sprintf("Capitaine %s", names[rng.Intn(len(names))]),
				Level:      1 + (tier-1)*20 + rng.Intn(10), // T1: 1-10, T2: 21-30, T3: 41-50
				TemplateID: "npc_captain",
			}
			// Assign a speed-related skill with some probability
			skills := []string{"wind_favorable_speed_bonus", "wind_never_unfavorable", "nav_morale_decay_reduction"}
			captain.SkillID = skills[rng.Intn(len(skills))]

			targets = append(targets, PveTarget{
				ID:           fmt.Sprintf("npc-%s-%d", playerID.String(), count),
				X:            int(x),
				Y:            int(y),
				RealX:        x,
				RealY:        y,
				TargetX:      tx,
				TargetY:      ty,
				Speed:        baseSpeed,
				Tier:         tier,
				Name:         names[rng.Intn(len(names))],
				Captain:      captain,
				NextChangeAt: time.Now().Add(time.Duration(30+rng.Intn(60)) * time.Second),
			})
			count++
		}
	}

	return targets
}

// GenerateNpcFleet generates a fleet NPC runtime based on tier
// Returns a domain.Fleet with ships but no player/island association
func GenerateNpcFleet(tier int, targetID string) domain.Fleet {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Fleet composition based on tier
	var shipTypes []string
	var shipCount int

	switch tier {
	case 1:
		// Tier 1: 2-3 weak ships
		shipTypes = []string{"sloop", "sloop", "brigantine"}
		shipCount = 2 + rng.Intn(2) // 2-3 ships
	case 2:
		// Tier 2: 3-4 medium ships
		shipTypes = []string{"brigantine", "brigantine", "frigate", "frigate"}
		shipCount = 3 + rng.Intn(2) // 3-4 ships
	case 3:
		// Tier 3: 4-5 strong ships
		shipTypes = []string{"frigate", "frigate", "galleon", "galleon", "manowar"}
		shipCount = 4 + rng.Intn(2) // 4-5 ships
	default:
		// Fallback to tier 1
		shipTypes = []string{"sloop", "sloop"}
		shipCount = 2
	}

	// Create fleet
	fleet := domain.Fleet{
		ID:    uuid.New(),
		Name:  fmt.Sprintf("NPC Fleet %s", targetID),
		Ships: make([]domain.Ship, 0, shipCount),
	}

	// Generate ships
	for i := 0; i < shipCount && i < len(shipTypes); i++ {
		shipType := shipTypes[i]
		baseStats, err := gamedata.GetShipBaseStats(shipType)
		if err != nil {
			// Skip invalid ship type
			continue
		}

		// Create ship with full health
		ship := domain.Ship{
			ID:        uuid.New(),
			Name:      fmt.Sprintf("NPC %s %d", shipType, i+1),
			Type:      shipType,
			Health:    baseStats.HP,
			MaxHealth: baseStats.HP,
			State:     "Ready",
			X:         0,
			Y:         0,
			// No PlayerID, IslandID, FleetID for NPC ships (runtime only)
		}

		// Add basic crew (balanced RPS)
		crewSize := int(baseStats.HP / 10) // Rough crew size based on HP
		ship.MilitiaWarriors = crewSize / 3
		ship.MilitiaArchers = crewSize / 3
		ship.MilitiaGunners = crewSize / 3
		// Distribute remainder
		remainder := crewSize - (ship.MilitiaWarriors + ship.MilitiaArchers + ship.MilitiaGunners)
		if remainder > 0 {
			ship.MilitiaWarriors += remainder
		}

		fleet.Ships = append(fleet.Ships, ship)
	}

	// Set flagship (first ship)
	if len(fleet.Ships) > 0 {
		fleet.FlagshipShipID = &fleet.Ships[0].ID
	}

	// Set morale (NPC fleets have fixed morale based on tier)
	morale := 40 + tier*10 // Tier 1: 50, Tier 2: 60, Tier 3: 70
	fleet.MoraleCruise = &morale
	return fleet
}

// UpdatePveTargetSimulation updates PVE target positions (Random Walk)
// It iterates through the global cache and moves all active targets, applying speed modifiers.
func UpdatePveTargetSimulation(delta float64, islands []domain.Island) {
	pveCacheMutex.Lock()
	defer pveCacheMutex.Unlock()

	now := time.Now()

	for playerID, entry := range pveTargetCache {
		// Skip expired entries
		if now.After(entry.ExpiresAt) {
			continue
		}

		// NPCs do NOT benefit from player technologies.
		techMultiplier := 1.0

		for i := range entry.Targets {
			t := &entry.Targets[i]

			// Direction vector
			dx := t.TargetX - t.RealX
			dy := t.TargetY - t.RealY
			dist := math.Sqrt(dx*dx + dy*dy)

			// 1. Destination Update (Random Walk)
			reached := dist < 5.0
			timeout := !t.NextChangeAt.IsZero() && now.After(t.NextChangeAt)

			if reached || timeout {
				seed := int64(playerID[0]) + int64(i)*1337
				rng := rand.New(rand.NewSource(now.UnixNano()/(1000*1000*100) + seed))

				t.TargetX = -1000.0 + rng.Float64()*2000.0
				t.TargetY = -1000.0 + rng.Float64()*2000.0
				t.NextChangeAt = now.Add(time.Duration(30+rng.Intn(90)) * time.Second)

				// Recalculate vector
				dx = t.TargetX - t.RealX
				dy = t.TargetY - t.RealY
				dist = math.Sqrt(dx*dx + dy*dy)
			}

			if dist > 0 {
				// 2. Calculate Wind Multiplier
				angleRad := math.Atan2(dy, dx)
				angleDeg := angleRad * (180 / math.Pi)
				if angleDeg < 0 {
					angleDeg += 360
				}

				windMultiplier := 1.0
				if GlobalWeather != nil {
					windMultiplier = GlobalWeather.GetWindFactor(angleDeg)
				}

				// 3. Calculate Captain Multiplier
				isFavorable := windMultiplier > 1.0
				captainMultiplier := CalculateCaptainSpeedMultiplier(t.Captain, isFavorable)

				// 4. Combine all modifiers: base * tech * captain * wind
				effectiveSpeed := t.Speed * techMultiplier * captainMultiplier * windMultiplier

				// 5. Execute Move
				move := effectiveSpeed * delta
				if move > dist {
					move = dist
				}

				ratio := move / dist
				t.RealX += dx * ratio
				t.RealY += dy * ratio

				// Sync legacy fields
				t.X = int(t.RealX)
				t.Y = int(t.RealY)
			}
		}
	}
}
