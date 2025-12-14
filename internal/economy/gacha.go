package economy

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Gacha rates (sum must equal 1.0)
const (
	GachaRateCommon    = 0.70 // 70%
	GachaRateRare      = 0.25 // 25%
	GachaRateLegendary = 0.05 // 5%
)

// Shards granted per duplicate rarity
const (
	ShardsPerCommonDup    = 10
	ShardsPerRareDup      = 20
	ShardsPerLegendaryDup = 40
)

// Pity thresholds
const (
	PityLegendaryThreshold = 80 // Guaranteed legendary at 80 pulls
	PityRareThreshold       = 10 // Guaranteed rare at 10 pulls (optional)
	PityEnabled             = true // Toggle pity system
)

// CaptainTemplate represents a captain template that can be rolled
type CaptainTemplate struct {
	TemplateID string
	Name       string
	Rarity     domain.CaptainRarity
	SkillID    string
}

// Hardcoded captain templates based on existing skill IDs
var captainTemplates = map[domain.CaptainRarity][]CaptainTemplate{
	domain.RarityCommon: {
		{"sailor_john", "John le Matelot", domain.RarityCommon, "nav_morale_decay_reduction"},
		{"old_salt", "Vieux Salé", domain.RarityCommon, "rum_consumption_reduction"},
		{"steady_hand", "Main Ferme", domain.RarityCommon, "morale_floor"},
		{"wind_catcher", "Attrape-Vent", domain.RarityCommon, "wind_favorable_speed_bonus"},
		{"harbor_master", "Maître du Port", domain.RarityCommon, "port_morale_recovery_bonus"},
		{"survivor", "Survivant", domain.RarityCommon, "crew_loss_reduction"},
		{"storm_rider", "Cavalier de Tempête", domain.RarityCommon, "wind_unfavorable_penalty_reduction"},
		{"calm_sea", "Mer Calme", domain.RarityCommon, "low_morale_decay_slowdown"},
	},
	domain.RarityRare: {
		{"black_gale", "Gale Noir", domain.RarityRare, "interception_chance_bonus"},
		{"morale_breaker", "Briseur de Moral", domain.RarityRare, "opening_enemy_morale_damage"},
		{"fear_monger", "Semeur de Peur", domain.RarityRare, "enemy_morale_decay_multiplier"},
		{"desperate_speed", "Vitesse Désespérée", domain.RarityRare, "low_morale_speed_bonus"},
		{"unbreakable", "Inébranlable", domain.RarityRare, "panic_immunity_threshold"},
	},
	domain.RarityLegendary: {
		{"wind_master", "Maître du Vent", domain.RarityLegendary, "wind_never_unfavorable"},
		{"red_isabella", "Isabella la Rouge", domain.RarityLegendary, "terror_engagement"},
		{"immortal_captain", "Capitaine Immortel", domain.RarityLegendary, "absolute_morale_floor"},
	},
}

// RollCaptainRarity rolls a random rarity based on gacha rates
func RollCaptainRarity() domain.CaptainRarity {
	rng := rand.Float64()

	if rng < GachaRateCommon {
		return domain.RarityCommon
	} else if rng < GachaRateCommon+GachaRateRare {
		return domain.RarityRare
	} else {
		return domain.RarityLegendary
	}
}

// RollCaptainRarityWithPity rolls a rarity with pity system consideration
// Returns the rolled rarity and whether it was forced by pity
func RollCaptainRarityWithPity(pityLegendary, pityRare int) (domain.CaptainRarity, bool) {
	// Check pity guarantees (if enabled)
	if PityEnabled {
		if pityLegendary >= PityLegendaryThreshold {
			return domain.RarityLegendary, true
		}
		if pityRare >= PityRareThreshold {
			return domain.RarityRare, true
		}
	}
	
	// Normal roll
	rarity := RollCaptainRarity()
	return rarity, false
}

// PickCaptainTemplateByRarity randomly selects a template from the given rarity pool
func PickCaptainTemplateByRarity(rarity domain.CaptainRarity) (CaptainTemplate, error) {
	templates, exists := captainTemplates[rarity]
	if !exists || len(templates) == 0 {
		return CaptainTemplate{}, fmt.Errorf("no templates available for rarity: %s", rarity)
	}

	// Use time-based seed for randomness (called from handler, so each request gets different seed)
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(templates))
	return templates[index], nil
}

// GetCaptainTemplateByID retrieves a template by its TemplateID (for duplicate checking)
func GetCaptainTemplateByID(templateID string) (CaptainTemplate, bool) {
	for _, templates := range captainTemplates {
		for _, t := range templates {
			if t.TemplateID == templateID {
				return t, true
			}
		}
	}
	return CaptainTemplate{}, false
}

// GetTemplatesByRarity returns all templates for a given rarity
func GetTemplatesByRarity(rarity domain.CaptainRarity) []CaptainTemplate {
	templates, exists := captainTemplates[rarity]
	if !exists {
		return []CaptainTemplate{}
	}
	return templates
}

