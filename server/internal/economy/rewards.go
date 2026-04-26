package economy

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

const (
	ScrapValueRatio = 0.20 // 20% of construction cost recovered
	LootVariance    = 0.10 // +/- 10% variance
)

// CalculateScrapReward calculates the resource reward for destroying a specific ship type
// Returns a map of resources (Wood, Gold, Rum)
func CalculateScrapReward(shipType string) (map[domain.ResourceType]float64, error) {
	stats, err := GetShipStats(shipType)
	if err != nil {
		return nil, fmt.Errorf("unknown ship type: %s", shipType)
	}

	rewards := make(map[domain.ResourceType]float64)

	// Base Value Calculation
	// We map construction costs to loot
	// Gold -> Gold
	// Wood -> Wood
	// Rum -> Rum (if any)
	// Stone -> Gold (Stone is construction material, converted to Gold value for loot simplicity? Or keep as Stone?)
	// Design doc said: Gold (40%), Wood (40%), Rum (20%) of TOTAL value.
	// But simpler implementation is: Take each cost component and multiply by 0.20.

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	variance := 1.0 + (rng.Float64()*2.0*LootVariance - LootVariance) // 0.9 to 1.1

	for res, amount := range stats.Cost {
		lootAmount := amount * ScrapValueRatio * variance

		// Map resources directly
		// Stone is rare, maybe convert it to Gold or keep it?
		// Let's keep it simple: what enters comes out.
		rewards[res] = lootAmount
	}

	// Ensure some Rum is always dropped for crew (if not in cost)
	// Some ships don't cost Rum (Sloop).
	if stats.Cost[domain.Rum] == 0 {
		// Bonus Rum based on ship HP (simulating supplies onboard)
		// e.g. 100 HP -> 20 Rum
		rewards[domain.Rum] = stats.MaxHealth * 0.2 * variance
	}

	return rewards, nil
}

// CalculateCombatRewards calculates total rewards for a list of destroyed ship types
func CalculateCombatRewards(destroyedShipTypes []string) map[domain.ResourceType]float64 {
	totalRewards := make(map[domain.ResourceType]float64)

	for _, shipType := range destroyedShipTypes {
		reward, err := CalculateScrapReward(shipType)
		if err != nil {
			fmt.Printf("[REWARDS] Error calculating reward for %s: %v\n", shipType, err)
			continue
		}

		for res, amount := range reward {
			totalRewards[res] += amount
		}
	}

	return totalRewards
}
