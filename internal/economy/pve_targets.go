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
		minRatio = 0.10 // Tier 1 reduced from 0.35 to 0.10
		maxRatio = 0.25 // Tier 1 reduced from 0.50 to 0.25
	case 2:
		minRatio = 0.55
		maxRatio = 0.70
	case 3:
		minRatio = 0.75
		maxRatio = 0.90
	default:
		// Default to tier 1 if invalid
		minRatio = 0.10
		maxRatio = 0.25
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
	X    int    `json:"x"`    // Current Position X
	Y    int    `json:"y"`    // Current Position Y
	Tier int    `json:"tier"` // Tier 1, 2, or 3
	Name string `json:"name"` // Display name

	// Random Waypoint Movement Parameters
	OrbitCenterX int `json:"orbit_center_x"` // Keep for reference (island center)
	OrbitCenterY int `json:"orbit_center_y"` // Keep for reference (island center)

	TargetX float64 `json:"target_x"` // Destination X
	TargetY float64 `json:"target_y"` // Destination Y
	Speed   float64 `json:"speed"`    // Speed in units/sec

	// Deprecated Orbit Fields (Removed)
	// RadiusX, RadiusY, SpeedX, SpeedY, OrbitAngle
}

// UpdatePosition updates the ship's position towards its target
// Returns true if position changed
func (t *PveTarget) UpdatePosition(deltaSeconds float64, islands []domain.Island) bool {
	// Calculate vector to target
	dx := t.TargetX - float64(t.X)
	dy := t.TargetY - float64(t.Y)
	dist := math.Sqrt(dx*dx + dy*dy)

	// If reached target (within small threshold), pick new target
	if dist < 5.0 {
		// Pick new random target in the annulus (250-600)

		// Attempt to find a clear path (up to 10 tries)
		foundValid := false
		for attempt := 0; attempt < 10; attempt++ {
			angle := rand.Float64() * 2 * math.Pi
			radius := 250.0 + rand.Float64()*350.0 // 250 to 600

			candX := float64(t.OrbitCenterX) + radius*math.Cos(angle)
			candY := float64(t.OrbitCenterY) + radius*math.Sin(angle)

			// Check if path from Current(X,Y) to Candidate is clear
			if IsPathClear(float64(t.X), float64(t.Y), candX, candY, islands) {
				t.TargetX = candX
				t.TargetY = candY
				foundValid = true
				break
			}
		}

		// If no valid path found after retries, just pick the last one (avoid stuck loop)
		// or stay put? Let's just pick strictly the last attempt to keep moving.
		if !foundValid {
			// Fallback: Do NOT update TargetX/TargetY.
			// The ship will aim for the same spot (already reached), effectively idling/waiting.
			// Next frame/tick will retry finding a path.
			// Ideally we could slightly jitter or wait, but keeping same target effectively stops it.
			// Explicitly set target to current position to FORCE STOP
			t.TargetX = float64(t.X)
			t.TargetY = float64(t.Y)
			fmt.Printf("[PVE] Force Stop for %s at (%d,%d) due to collision risk\n", t.ID, t.X, t.Y)
		}

		return true
	}

	// Move towards target
	moveDist := t.Speed * deltaSeconds
	if moveDist > dist {
		moveDist = dist
	}

	// Normalized direction * moveDist
	t.X += int((dx / dist) * moveDist)
	t.Y += int((dy / dist) * moveDist)

	return true
}

// UpdatePveTargetSimulation updates all cached targets
// Includes Global Collision Avoidance against all player islands
func UpdatePveTargetSimulation(deltaSeconds float64, islands []domain.Island) {
	pveCacheMutex.Lock()
	defer pveCacheMutex.Unlock()

	for _, entry := range pveTargetCache {
		// Only update if cache is valid (not expired)
		if time.Now().Before(entry.ExpiresAt) {
			for i := range entry.Targets {
				entry.Targets[i].UpdatePosition(deltaSeconds, islands)
			}
		}
	}
}

// IsPathClear checks if a direct line between Start and End intersects any island safety zone
// Safety zone = Island Radius (~100 units?)
// Simple check: Distance from IslandCenter to LineSegment < SafetyRadius
func IsPathClear(sx, sy, ex, ey float64, islands []domain.Island) bool {
	// Const for safety radius around an island (Visual size is approx 50-80, let's use 250 for VERY safe buffer)
	// User reported clipping, so we increased this from 120 to 250.
	const IslandSafetyRadius = 250.0

	for _, isl := range islands {
		ix := float64(isl.X)
		iy := float64(isl.Y)

		// Check distance from Point (ix,iy) to Segment (sx,sy)-(ex,ey)
		// Vector AB (Start to End)
		abx := ex - sx
		aby := ey - sy

		// Vector AP (Start to Point)
		apx := ix - sx
		apy := iy - sy

		// Project AP onto AB to find "t" (closest point projected on infinite line)
		// t = (AP . AB) / (AB . AB)
		abLenSq := abx*abx + aby*aby
		if abLenSq == 0 {
			// Start and End are same point - check distance to point
			dist := math.Sqrt((sx-ix)*(sx-ix) + (sy-iy)*(sy-iy))
			if dist < IslandSafetyRadius {
				return false
			}
			continue
		}

		t := (apx*abx + apy*aby) / abLenSq

		// Clamp t to segment [0, 1]
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}

		// Closest point on segment
		cx := sx + t*abx
		cy := sy + t*aby

		// Distance from Island Center to Closest Point
		distSq := (cx-ix)*(cx-ix) + (cy-iy)*(cy-iy)

		if distSq < IslandSafetyRadius*IslandSafetyRadius {
			return false // Collision detected
		}
	}
	return true
}

var (
	pveTargetCache = make(map[uuid.UUID]*PveTargetCacheEntry)
	pveCacheMutex  sync.RWMutex
	cacheTTL       = 10 * time.Minute
)

// GetPveTargets returns 3 PVE targets for a player's island
// Uses cache if available and not expired, otherwise generates new targets
// Accepts allIslands for safe spawn generation
func GetPveTargets(playerID uuid.UUID, islandX, islandY int, allIslands []domain.Island) []PveTarget {
	pveCacheMutex.RLock()
	entry, exists := pveTargetCache[playerID]
	pveCacheMutex.RUnlock()

	// Check if cache is valid
	if exists && time.Now().Before(entry.ExpiresAt) && entry.IslandX == islandX && entry.IslandY == islandY {
		return entry.Targets
	}

	// Generate new targets with collision check
	targets := generatePveTargets(playerID, islandX, islandY, allIslands)

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

// IsPositionClear checks if a point (x,y) is safely away from all islands
func IsPositionClear(x, y float64, islands []domain.Island) bool {
	// Safety radius for spawning (keep them well clear)
	const SpawnSafetyRadius = 400.0 // Match path safety

	for _, isl := range islands {
		dx := x - float64(isl.X)
		dy := y - float64(isl.Y)
		distSq := dx*dx + dy*dy

		if distSq < SpawnSafetyRadius*SpawnSafetyRadius {
			return false
		}
	}
	return true
}

// generatePveTargets generates 3 PVE targets around the player's island
// Accepts allIslands to avoiding spawning on neighbors
func generatePveTargets(playerID uuid.UUID, islandX, islandY int, allIslands []domain.Island) []PveTarget {
	// Generate 5 targets at different positions
	targetCount := 5
	targets := make([]PveTarget, targetCount)
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(playerID[0])))

	// Tier names
	tierNames := map[int][]string{
		1: {"Corsaires égarés", "Pirates solitaires", "Épaves errantes"},
		2: {"Chasseurs de primes", "Flibustiers aguerris", "Raiders expérimentés"},
		3: {"Escadre fantôme", "Armada redoutable", "Légion noire"},
	}

	for i := 0; i < targetCount; i++ {
		tier := i%3 + 1 // Tier 1, 2, 3, 1, 2...

		var startX, startY, targetX, targetY, speed float64
		// Initial Placement Retry Loop
		// Maximum 10 tries to reasonable spawn position
		validSpawn := false
		for attempt := 0; attempt < 10; attempt++ {
			// Random Start Position
			angle := rng.Float64() * 2 * math.Pi
			radius := 250.0 + rng.Float64()*350.0 // 250-600

			startX = float64(islandX) + radius*math.Cos(angle)
			startY = float64(islandY) + radius*math.Sin(angle)

			// Random Initial Target
			targetAngle := rng.Float64() * 2 * math.Pi
			targetRadius := 250.0 + rng.Float64()*350.0
			targetX = float64(islandX) + targetRadius*math.Cos(targetAngle)
			targetY = float64(islandY) + targetRadius*math.Sin(targetAngle)

			// 1. Check if Start Position is clear
			if !IsPositionClear(startX, startY, allIslands) {
				continue
			}

			// 2. Check if INITIAL PATH (Start -> Target) is clear
			// This was the missing link causing clipping traverse on spawn
			if !IsPathClear(startX, startY, targetX, targetY, allIslands) {
				continue
			}

			validSpawn = true
			break
		}

		if !validSpawn {
			fmt.Printf("[PVE] Warning: Could not find clear spawn/path for target %d after 10 attempts\n", i)
		}

		// If fails after 10 tries, we accept the last generated one to avoid infinite loop
		// But it's highly likely to be safe if map isn't saturated.
		// If map IS saturated, well, pirates have to go somewhere.

		// Speed (Pixels per second)
		// User requested visual movement.
		// Reduced to 10-18 units/sec (Third Reduction)
		speed = 10.0 + rng.Float64()*8.0

		// Select random name for tier
		names := tierNames[tier]
		name := names[rng.Intn(len(names))]

		targets[i] = PveTarget{
			ID:           fmt.Sprintf("npc-%s-%d", playerID.String(), i),
			X:            int(startX),
			Y:            int(startY),
			Tier:         tier,
			Name:         name,
			OrbitCenterX: islandX,
			OrbitCenterY: islandY,
			TargetX:      targetX,
			TargetY:      targetY,
			Speed:        speed,
		}

		fmt.Printf("[PVE] Generated Target %d (Attempted Clear Spawn): ID=%s Tier=%d Speed=%.1f Start=(%d,%d)\n",
			i, targets[i].ID, tier, speed, int(startX), int(startY))
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
		// Tier 1: 1 weak ship (Sloop only) - Tutorial Difficulty
		shipTypes = []string{"sloop"}
		shipCount = 1
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
		shipTypes = []string{"sloop"}
		shipCount = 1
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

// GenerateNpcCaptain generates a transient captain for NPC fleets
// Tier 1: Common
// Tier 2: Rare
// Tier 3: Legendary
func GenerateNpcCaptain(tier int) *domain.Captain {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var rarity domain.CaptainRarity
	var level int
	var stars int

	switch tier {
	case 1:
		rarity = domain.RarityCommon
		level = 1 + rng.Intn(5) // Lvl 1-5
		stars = 0
	case 2:
		rarity = domain.RarityRare
		level = 10 + rng.Intn(20) // Lvl 10-29
		stars = 1
	case 3:
		rarity = domain.RarityLegendary
		level = 30 + rng.Intn(20) // Lvl 30-49
		stars = 3
	default:
		rarity = domain.RarityCommon
		level = 1
		stars = 0
	}

	// Pick a random skill valid for this rarity
	var skillID string
	if rarity == domain.RarityCommon {
		skills := []string{"nav_morale_decay_reduction", "rum_consumption_reduction", "morale_floor"}
		skillID = skills[rng.Intn(len(skills))]
	} else if rarity == domain.RarityRare {
		skills := []string{"interception_chance_bonus", "opening_enemy_morale_damage", "low_morale_speed_bonus"}
		skillID = skills[rng.Intn(len(skills))]
	} else {
		// Legendary
		skills := []string{"wind_never_unfavorable", "terror_engagement", "absolute_morale_floor"}
		skillID = skills[rng.Intn(len(skills))]
	}

	return &domain.Captain{
		ID:         uuid.New(),
		PlayerID:   uuid.Nil, // NPC
		TemplateID: "npc_captain",
		Name:       fmt.Sprintf("Capitaine T%d", tier),
		Rarity:     rarity,
		Level:      level,
		Stars:      stars,
		SkillID:    skillID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
