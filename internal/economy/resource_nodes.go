package economy

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
)

// ResourceNodeCacheEntry represents a cached resource node entry
type ResourceNodeCacheEntry struct {
	Nodes     []domain.ResourceNode
	ExpiresAt time.Time
	PlayerID  uuid.UUID
	IslandX   int
	IslandY   int
}

var (
	nodeCache      = make(map[uuid.UUID]*ResourceNodeCacheEntry)
	nodeCacheMutex sync.RWMutex
	nodeCacheTTL   = 60 * time.Minute // Stable for 1 hour to avoid nodes jumping around
)

// GetResourceNodes returns resource nodes for a player's island
func GetResourceNodes(playerID uuid.UUID, islandX, islandY int, allIslands []domain.Island) []domain.ResourceNode {
	nodeCacheMutex.RLock()
	entry, exists := nodeCache[playerID]
	nodeCacheMutex.RUnlock()

	// Check if cache is valid
	if exists && time.Now().Before(entry.ExpiresAt) && entry.IslandX == islandX && entry.IslandY == islandY {
		return entry.Nodes
	}

	// Generate new nodes
	nodes := generateResourceNodes(playerID, islandX, islandY, allIslands)

	// Update cache
	nodeCacheMutex.Lock()
	nodeCache[playerID] = &ResourceNodeCacheEntry{
		Nodes:     nodes,
		ExpiresAt: time.Now().Add(nodeCacheTTL),
		PlayerID:  playerID,
		IslandX:   islandX,
		IslandY:   islandY,
	}
	nodeCacheMutex.Unlock()

	return nodes
}

// GetResourceNodeByID checks if a node exists and is valid for the player
func GetResourceNodeByID(playerID uuid.UUID, nodeID string, islandX, islandY int) *domain.ResourceNode {
	nodes := GetResourceNodes(playerID, islandX, islandY, nil) // Assume nil is okay if cached, otherwise might be risky without full island list. Ideally we pass full list or rely on cache.
	for _, n := range nodes {
		if n.ID == nodeID {
			return &n
		}
	}
	return nil
}

// generateResourceNodes generates static resource spots
func generateResourceNodes(playerID uuid.UUID, islandX, islandY int, allIslands []domain.Island) []domain.ResourceNode {
	count := 4 // One of each type
	nodes := make([]domain.ResourceNode, 0, count)

	// Deterministic RNG based on Player ID + Day (rotates daily)
	seed := int64(playerID[0]) + time.Now().Unix()/86400
	rng := rand.New(rand.NewSource(seed))

	types := []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum}

	for i, resType := range types {
		var x, y float64
		valid := false

		// Try to find a valid spot
		for attempt := 0; attempt < 15; attempt++ {
			angle := rng.Float64() * 2 * math.Pi
			radius := 300.0 + rng.Float64()*400.0 // 300-700 units away

			cx := float64(islandX) + radius*math.Cos(angle)
			cy := float64(islandY) + radius*math.Sin(angle)

			// Use simulate PVE collision checking (IsPositionClear)
			if IsPositionClear(cx, cy, allIslands) {
				x, y = cx, cy
				valid = true
				break
			}
		}

		if valid {
			nodes = append(nodes, domain.ResourceNode{
				ID:       fmt.Sprintf("res-%s-%d", playerID.String()[:8], i),
				X:        int(x),
				Y:        int(y),
				Type:     resType,
				Richness: 1.0 + rng.Float64()*0.5, // 1.0x to 1.5x richness
			})
		}
	}

	return nodes
}
