package economy

import (
	"fmt"
	"math/rand"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Gacha rates (sum must equal 1.0)
const (
	GachaRateCommon    = 0.62  // 62 %
	GachaRateRare      = 0.28  // 28 %
	GachaRateLegendary = 0.085 // 8,5 %
	GachaRateUltra     = 0.015 // 1,5 % (UL)
)

// Shards granted per duplicate rarity
const (
	ShardsPerCommonDup    = 10
	ShardsPerRareDup      = 20
	ShardsPerLegendaryDup = 40
	ShardsPerUltraDup     = 80
)

// Pity thresholds (tirages sans la rareté avant qu'elle soit garantie)
const (
	PityUltraThreshold     = 120
	PityLegendaryThreshold = 50
	PityRareThreshold      = 10
	PityEnabled            = true
)

// ShardsPerDuplicate : fragments reçus pour un doublon de cette rareté.
func ShardsPerDuplicate(r domain.CaptainRarity) int {
	switch r {
	case domain.RarityRare:
		return ShardsPerRareDup
	case domain.RarityLegendary:
		return ShardsPerLegendaryDup
	case domain.RarityUltra:
		return ShardsPerUltraDup
	}
	return ShardsPerCommonDup
}

// CaptainTemplate represents a captain template that can be rolled
type CaptainTemplate struct {
	TemplateID string
	Name       string
	Rarity     domain.CaptainRarity
	SkillID    string
}

// Les 40 capitaines. Les 16 premiers existaient déjà : leur TemplateID et leur
// SkillID ne changent pas (les joueurs les gardent). Effets : captain_effects.go.
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
		{"cook_marie", "Marie la Cambuse", domain.RarityCommon, "departure_morale_bonus"},
		{"carpenter", "Le Charpentier", domain.RarityCommon, "small_damage_reduction"},
		{"lynx_eye", "Œil-de-Lynx", domain.RarityCommon, "harvest_bonus"},
		{"iron_fists", "Poings d'Acier", domain.RarityCommon, "warrior_damage_bonus"},
		{"tom_archer", "Tom l'Archer", domain.RarityCommon, "archer_damage_bonus"},
		{"short_fuse", "Mèche Courte", domain.RarityCommon, "gunner_damage_bonus"},
		{"accountant", "Le Comptable", domain.RarityCommon, "cargo_bonus"},
		{"hangover", "Gueule-de-Bois", domain.RarityCommon, "small_loot_bonus"},
	},
	domain.RarityRare: {
		{"black_gale", "Gale Noir", domain.RarityRare, "interception_chance_bonus"},
		{"morale_breaker", "Briseur de Moral", domain.RarityRare, "opening_enemy_morale_damage"},
		{"fear_monger", "Semeur de Peur", domain.RarityRare, "enemy_morale_decay_multiplier"},
		{"desperate_speed", "Vitesse Désespérée", domain.RarityRare, "low_morale_speed_bonus"},
		{"unbreakable", "Inébranlable", domain.RarityRare, "panic_immunity_threshold"},
		{"anne_two_blades", "Anne Deux-Lames", domain.RarityRare, "flagship_damage_bonus"},
		{"sea_doctor", "Le Docteur des Mers", domain.RarityRare, "major_crew_loss_reduction"},
		{"greybeard", "Barbe-Grise", domain.RarityRare, "fleet_speed_bonus"},
		{"moray", "La Murène", domain.RarityRare, "npc_damage_bonus"},
		{"jack_double_barrel", "Jack Double-Fût", domain.RarityRare, "rum_and_spirit"},
		{"the_fence", "Le Receleur", domain.RarityRare, "loot_bonus"},
		{"victory_song", "Chant de Victoire", domain.RarityRare, "victory_morale_bonus"},
	},
	domain.RarityLegendary: {
		{"wind_master", "Maître du Vent", domain.RarityLegendary, "wind_never_unfavorable"},
		{"red_isabella", "Isabella la Rouge", domain.RarityLegendary, "terror_engagement"},
		{"immortal_captain", "Capitaine Immortel", domain.RarityLegendary, "absolute_morale_floor"},
		{"grey_kraken", "Le Kraken Gris", domain.RarityLegendary, "fleet_damage_bonus"},
		{"lady_coral", "Dame Corail", domain.RarityLegendary, "fleet_damage_reduction"},
		{"golden_baron", "Le Baron Doré", domain.RarityLegendary, "golden_baron"},
		{"mist_shadow", "Ombre-Brume", domain.RarityLegendary, "mist_shadow"},
		{"silas_redhand", "Silas Main-Rouge", domain.RarityLegendary, "rps_mastery"},
	},
	domain.RarityUltra: {
		{"storm_king", "Barbe-d'Orage, Roi des Tempêtes", domain.RarityUltra, "ul_storm_king"},
		{"scarlet_widow", "La Veuve Écarlate", domain.RarityUltra, "ul_scarlet_widow"},
		{"ghost_admiral", "L'Amiral Fantôme", domain.RarityUltra, "ul_ghost_admiral"},
		{"abyss_merchant", "Le Marchand des Abysses", domain.RarityUltra, "ul_abyss_merchant"},
	},
}

// AllRarities : de la plus courante à la plus rare.
var AllRarities = []domain.CaptainRarity{domain.RarityCommon, domain.RarityRare, domain.RarityLegendary, domain.RarityUltra}

// IsValidRarity : la rareté existe.
func IsValidRarity(r domain.CaptainRarity) bool {
	for _, x := range AllRarities {
		if x == r {
			return true
		}
	}
	return false
}

// rarityFromRoll : tirage normal, x dans [0, 1).
func rarityFromRoll(x float64) domain.CaptainRarity {
	switch {
	case x < GachaRateCommon:
		return domain.RarityCommon
	case x < GachaRateCommon+GachaRateRare:
		return domain.RarityRare
	case x < GachaRateCommon+GachaRateRare+GachaRateLegendary:
		return domain.RarityLegendary
	}
	return domain.RarityUltra
}

// RollCaptainRarity rolls a random rarity based on gacha rates
func RollCaptainRarity() domain.CaptainRarity {
	return rarityFromRoll(rand.Float64())
}

// RollCaptainRarityWithPity : tirage avec pitié. Les compteurs sont les
// tirages depuis la dernière rareté de ce niveau (ou mieux). Renvoie la rareté
// et si elle a été forcée par la pitié.
func RollCaptainRarityWithPity(pityUltra, pityLegendary, pityRare int) (domain.CaptainRarity, bool) {
	if PityEnabled {
		if pityUltra >= PityUltraThreshold {
			return domain.RarityUltra, true
		}
		if pityLegendary >= PityLegendaryThreshold {
			return domain.RarityLegendary, true
		}
		if pityRare >= PityRareThreshold {
			r := RollCaptainRarity()
			if r == domain.RarityCommon {
				return domain.RarityRare, true
			}
			return r, false
		}
	}
	return RollCaptainRarity(), false
}

// UpdatePityCounters met à jour les compteurs après un tirage.
func UpdatePityCounters(r domain.CaptainRarity, pityUltra, pityLegendary, pityRare *int) {
	*pityUltra++
	*pityLegendary++
	*pityRare++
	switch r {
	case domain.RarityUltra:
		*pityUltra, *pityLegendary, *pityRare = 0, 0, 0
	case domain.RarityLegendary:
		*pityLegendary, *pityRare = 0, 0
	case domain.RarityRare:
		*pityRare = 0
	}
}

// PickCaptainTemplateByRarity randomly selects a template from the given rarity pool
func PickCaptainTemplateByRarity(rarity domain.CaptainRarity) (CaptainTemplate, error) {
	templates, exists := captainTemplates[rarity]
	if !exists || len(templates) == 0 {
		return CaptainTemplate{}, fmt.Errorf("no templates available for rarity: %s", rarity)
	}
	return templates[rand.Intn(len(templates))], nil
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
