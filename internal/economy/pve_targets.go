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

// GenerateNpcCrewForShip generates crew composition for an NPC ship based on tier and ship type
// Returns: (warriors, archers, gunners)
// Uses deterministic RNG based on seed
func GenerateNpcCrewForShip(shipType string, tier int, seed int64) (int, int, int) {
	// Get max crew for this ship type
	maxCrew := MaxCrewForShipType(shipType)
	
	// Minimum crew (never zero)
	minCrew := 10
	if maxCrew < 10 {
		minCrew = maxCrew / 2
		if minCrew < 1 {
			minCrew = 1
		}
	}
	
	// Tier-based crew ratio (percentage of max capacity)
	var minRatio, maxRatio float64
	switch tier {
	case 1:
		minRatio = 0.35
		maxRatio = 0.50
	case 2:
		minRatio = 0.55
		maxRatio = 0.70
	case 3:
		minRatio = 0.75
		maxRatio = 0.90
	default:
		// Default to tier 1 if invalid
		minRatio = 0.35
		maxRatio = 0.50
	}
	
	// Calculate crew size range
	minTotal := int(float64(maxCrew) * minRatio)
	maxTotal := int(float64(maxCrew) * maxRatio)
	if minTotal < minCrew {
		minTotal = minCrew
	}
	if maxTotal > maxCrew {
		maxTotal = maxCrew
	}
	if maxTotal < minTotal {
		maxTotal = minTotal
	}
	
	// Deterministic random based on seed
	rng := rand.New(rand.NewSource(seed))
	totalCrew := minTotal + rng.Intn(maxTotal-minTotal+1)
	
	// Clamp to max
	if totalCrew > maxCrew {
		totalCrew = maxCrew
	}
	if totalCrew < minCrew {
		totalCrew = minCrew
	}
	
	// Composition based on tier
	var warriors, archers, gunners int
	
	if tier == 1 {
		// Tier 1: Neutral composition (~33/33/33)
		warriors = totalCrew / 3
		archers = totalCrew / 3
		gunners = totalCrew / 3
		remainder := totalCrew - (warriors + archers + gunners)
		// Distribute remainder evenly
		if remainder > 0 {
			warriors += remainder / 3
			archers += remainder / 3
			gunners += remainder / 3
			if remainder%3 > 0 {
				warriors++
			}
		}
	} else if tier == 2 {
		// Tier 2: Slight bias (random but bounded)
		// One type gets 40%, others get 30% each
		dominantType := rng.Intn(3) // 0=warrior, 1=archer, 2=gunner
		dominantCount := int(float64(totalCrew) * 0.40)
		otherCount := (totalCrew - dominantCount) / 2
		remainder := totalCrew - dominantCount - otherCount*2
		
		switch dominantType {
		case 0: // Warrior dominant
			warriors = dominantCount + remainder
			archers = otherCount
			gunners = otherCount
		case 1: // Archer dominant
			warriors = otherCount
			archers = dominantCount + remainder
			gunners = otherCount
		case 2: // Gunner dominant
			warriors = otherCount
			archers = otherCount
			gunners = dominantCount + remainder
		}
	} else { // tier == 3
		// Tier 3: Strong bias (for RPS to matter)
		// One type gets 50%, others get 25% each
		dominantType := rng.Intn(3)
		dominantCount := int(float64(totalCrew) * 0.50)
		otherCount := (totalCrew - dominantCount) / 2
		remainder := totalCrew - dominantCount - otherCount*2
		
		switch dominantType {
		case 0: // Warrior dominant
			warriors = dominantCount + remainder
			archers = otherCount
			gunners = otherCount
		case 1: // Archer dominant
			warriors = otherCount
			archers = dominantCount + remainder
			gunners = otherCount
		case 2: // Gunner dominant
			warriors = otherCount
			archers = otherCount
			gunners = dominantCount + remainder
		}
	}
	
	// Safety: ensure we don't exceed totalCrew
	total := warriors + archers + gunners
	if total > totalCrew {
		// Reduce proportionally
		scale := float64(totalCrew) / float64(total)
		warriors = int(float64(warriors) * scale)
		archers = int(float64(archers) * scale)
		gunners = totalCrew - warriors - archers
	} else if total < totalCrew {
		// Add remainder to first type
		warriors += totalCrew - total
	}
	
	// Final safety: ensure non-negative
	if warriors < 0 {
		warriors = 0
	}
	if archers < 0 {
		archers = 0
	}
	if gunners < 0 {
		gunners = 0
	}
	
	return warriors, archers, gunners
}

// PveTargetCacheEntry represents a cached PVE target entry
type PveTargetCacheEntry struct {
	Targets   []PveTarget
	ExpiresAt time.Time
	PlayerID  uuid.UUID
	IslandX   int
	IslandY   int
}

// PveTarget represents a PVE target (NPC fleet) on the world map
type PveTarget struct {
	ID   string `json:"id"`   // Stable ID: "npc-<playerID>-<slotIndex>"
	X    int    `json:"x"`    // Position X on world map
	Y    int    `json:"y"`    // Position Y on world map
	Tier int    `json:"tier"` // Tier 1, 2, or 3 (difficulty)
	Name string `json:"name"` // Display name (e.g., "Corsaires égarés")
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

// generatePveTargets generates 3 PVE targets around the player's island
func generatePveTargets(playerID uuid.UUID, islandX, islandY int) []PveTarget {
	targets := make([]PveTarget, 3)
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(playerID[0])))

	// Tier names
	tierNames := map[int][]string{
		1: {"Corsaires égarés", "Pirates solitaires", "Épaves errantes"},
		2: {"Chasseurs de primes", "Flibustiers aguerris", "Raiders expérimentés"},
		3: {"Escadre fantôme", "Armada redoutable", "Légion noire"},
	}

	// Generate 3 targets at different positions
	for i := 0; i < 3; i++ {
		tier := i + 1 // Tier 1, 2, 3

		// Generate position in radius 250-450 units from island
		angle := float64(i) * 2.0 * 3.14159 / 3.0 // 120 degrees apart
		radius := 250.0 + rng.Float64()*200.0     // 250-450 units
		x := islandX + int(radius*math.Cos(angle))
		y := islandY + int(radius*math.Sin(angle))

		// Clamp to world bounds (-1000 to 1000)
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

		// Select random name for tier
		names := tierNames[tier]
		name := names[rng.Intn(len(names))]

		targets[i] = PveTarget{
			ID:   fmt.Sprintf("npc-%s-%d", playerID.String(), i),
			X:    x,
			Y:    y,
			Tier: tier,
			Name: name,
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

		// Generate NPC crew based on tier and ship type (deterministic)
		// Use a deterministic seed based on targetID + ship index
		seed := int64(0)
		for _, char := range targetID {
			seed += int64(char)
		}
		seed += int64(i) * 1000 // Add ship index for variation
		
		warriors, archers, gunners := GenerateNpcCrewForShip(shipType, tier, seed)
		ship.CrewWarriors = warriors
		ship.CrewArchers = archers
		ship.CrewGunners = gunners

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
