package economy

// =============================================================================
//  EFFETS DES CAPITAINES  (captain_effects.go)
// =============================================================================
//
// Un capitaine a un talent (SkillID) dont la force monte du niveau 1 au
// niveau 80, plus des étoiles (doublons) qui donnent de petits bonus à toute
// la flotte. Ce fichier traduit tout ça en UNE structure, CaptainEffects, que
// le reste du serveur lit : combat, moral, vitesse, rhum, récolte, butin.
//
// Le capitaine qui compte pour une flotte est celui du navire amiral
// (voir FleetCaptain).
// =============================================================================

import (
	"fmt"
	"math"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
)

// CaptainEffects : tous les effets d'un capitaine, déjà calculés pour son
// niveau et ses étoiles. Les pourcentages sont des fractions (0.10 = 10 %).
type CaptainEffects struct {
	// Navigation
	SpeedPct                 float64 // vitesse en toutes circonstances
	FavorableWindSpeedPct    float64 // vitesse en plus par vent favorable
	UnfavorableWindReduction float64 // part de la pénalité de vent annulée
	WindNeverUnfavorable     bool
	PursuitSpeedPct          float64 // vitesse pendant une attaque/poursuite
	LowMoraleSpeedPct        float64 // vitesse si le moral est sous le seuil
	LowMoraleThreshold       int
	RumReductionPct          float64

	// Économie
	CargoPct     float64
	HarvestPct   float64
	LootPct      float64
	TicketChance float64 // chance d'un ticket bonus par victoire

	// Combat
	FleetDamagePct       float64
	FlagshipDamagePct    float64
	DamageTakenReduction float64 // ajoutée à la réduction de dégâts de chaque navire
	VsNpcDamagePct       float64
	RPSAdvantageBonus    float64            // ajouté au multiplicateur d'avantage (1,15)
	CrewTypeDamagePct    map[string]float64 // "warrior" / "archer" / "gunner"
	CrewSavePct          float64            // part des matelots sauvés quand un navire coule
	HPPct                float64            // PV en plus (étoiles)

	// Moral
	MoraleFloor               int     // moral au combat jamais sous cette valeur
	EnemyMoraleDamage         int     // moral ennemi en moins au début du combat
	MoraleBonusAmplify        float64 // ton bonus de moral est plus fort
	EnemyMoraleBonusReduction float64 // le bonus de moral ennemi est réduit
	SeaDecayReduction         float64 // le moral baisse moins vite en mer
	LowMoraleDecayReduction   float64 // idem, seulement sous 40 de moral
	NoSeaDecay                bool
	PortRecoveryPct           float64
	DepartureMorale           int // moral en plus à chaque départ du port
	VictoryMorale             int // moral en plus après une victoire
}

// Plafonds de sécurité (plusieurs sources peuvent s'additionner).
const (
	maxRumReduction       = 0.30
	maxDamageTakenBonus   = 0.40
	maxEngagementBonusPct = 0.60
)

func lerp(level int, base, max float64) float64 { return ScaleFloat(level, base, max) }
func lerpInt(level int, base, max int) int      { return ScaleInt(level, base, max) }

// ComputeCaptainEffects calcule les effets d'un capitaine (nil = aucun effet).
func ComputeCaptainEffects(c *domain.Captain) CaptainEffects {
	e := CaptainEffects{CrewTypeDamagePct: map[string]float64{}}
	if c == nil {
		return e
	}
	l := ClampCaptainLevel(c.Level)

	switch c.SkillID {
	// ---------------- COMMUNS ----------------
	case "nav_morale_decay_reduction":
		e.SeaDecayReduction = lerp(l, 0.10, 0.40)
	case "rum_consumption_reduction":
		e.RumReductionPct = lerp(l, 0.05, 0.15)
	case "morale_floor":
		e.MoraleFloor = lerpInt(l, 15, 35)
	case "wind_favorable_speed_bonus":
		e.FavorableWindSpeedPct = lerp(l, 0.03, 0.10)
	case "port_morale_recovery_bonus":
		e.PortRecoveryPct = lerp(l, 0.20, 0.60)
	case "crew_loss_reduction":
		e.CrewSavePct = lerp(l, 0.03, 0.12)
	case "wind_unfavorable_penalty_reduction":
		e.UnfavorableWindReduction = lerp(l, 0.10, 0.35)
	case "low_morale_decay_slowdown":
		e.LowMoraleDecayReduction = lerp(l, 0.25, 0.60)
	case "departure_morale_bonus":
		e.DepartureMorale = lerpInt(l, 3, 10)
	case "small_damage_reduction":
		e.DamageTakenReduction = lerp(l, 0.02, 0.06)
	case "harvest_bonus":
		e.HarvestPct = lerp(l, 0.03, 0.12)
	case "warrior_damage_bonus":
		e.CrewTypeDamagePct["warrior"] = lerp(l, 0.03, 0.10)
	case "archer_damage_bonus":
		e.CrewTypeDamagePct["archer"] = lerp(l, 0.03, 0.10)
	case "gunner_damage_bonus":
		e.CrewTypeDamagePct["gunner"] = lerp(l, 0.03, 0.10)
	case "cargo_bonus":
		e.CargoPct = lerp(l, 0.04, 0.15)
	case "small_loot_bonus":
		e.LootPct = lerp(l, 0.03, 0.12)

	// ---------------- RARES ----------------
	case "interception_chance_bonus":
		e.PursuitSpeedPct = lerp(l, 0.06, 0.25)
	case "opening_enemy_morale_damage":
		e.EnemyMoraleDamage = lerpInt(l, 6, 25)
	case "enemy_morale_decay_multiplier":
		e.MoraleBonusAmplify = lerp(l, 0.15, 0.50)
	case "low_morale_speed_bonus":
		e.LowMoraleSpeedPct = lerp(l, 0.06, 0.22)
		e.LowMoraleThreshold = lerpInt(l, 35, 50)
	case "panic_immunity_threshold":
		e.EnemyMoraleBonusReduction = lerp(l, 0.35, 0.70)
	case "flagship_damage_bonus":
		e.FlagshipDamagePct = lerp(l, 0.06, 0.18)
	case "major_crew_loss_reduction":
		e.CrewSavePct = lerp(l, 0.06, 0.20)
	case "fleet_speed_bonus":
		e.SpeedPct = lerp(l, 0.03, 0.10)
	case "npc_damage_bonus":
		e.VsNpcDamagePct = lerp(l, 0.08, 0.20)
	case "rum_and_spirit":
		e.RumReductionPct = lerp(l, 0.06, 0.18)
		e.DepartureMorale = 5
	case "loot_bonus":
		e.LootPct = lerp(l, 0.08, 0.22)
	case "victory_morale_bonus":
		e.VictoryMorale = lerpInt(l, 5, 15)

	// ---------------- LÉGENDAIRES ----------------
	case "wind_never_unfavorable":
		e.WindNeverUnfavorable = true
		e.SpeedPct = lerp(l, 0.04, 0.12)
	case "terror_engagement":
		e.EnemyMoraleDamage = lerpInt(l, 10, 30)
	case "absolute_morale_floor":
		e.MoraleFloor = lerpInt(l, 25, 60)
	case "fleet_damage_bonus":
		e.FleetDamagePct = lerp(l, 0.06, 0.15)
	case "fleet_damage_reduction":
		e.DamageTakenReduction = lerp(l, 0.05, 0.12)
	case "golden_baron":
		e.LootPct = lerp(l, 0.15, 0.35)
		e.TicketChance = lerp(l, 0.05, 0.15)
	case "mist_shadow":
		e.SpeedPct = lerp(l, 0.08, 0.18)
		e.SeaDecayReduction = 0.25
	case "rps_mastery":
		e.RPSAdvantageBonus = lerp(l, 0.10, 0.25)

	// ---------------- UL (ULTRA RARES) ----------------
	case "ul_storm_king":
		e.SpeedPct = lerp(l, 0.15, 0.25)
		e.WindNeverUnfavorable = true
		e.NoSeaDecay = true
	case "ul_scarlet_widow":
		e.EnemyMoraleDamage = lerpInt(l, 20, 40)
		e.FleetDamagePct = lerp(l, 0.10, 0.20)
		e.VictoryMorale = 10
	case "ul_ghost_admiral":
		e.MoraleFloor = lerpInt(l, 50, 70)
		e.DamageTakenReduction = lerp(l, 0.10, 0.20)
		e.CrewSavePct = lerp(l, 0.15, 0.30)
	case "ul_abyss_merchant":
		e.LootPct = lerp(l, 0.30, 0.60)
		e.CargoPct = lerp(l, 0.20, 0.40)
		e.HarvestPct = lerp(l, 0.15, 0.30)
		e.TicketChance = lerp(l, 0.10, 0.25)
	}

	// Étoiles : bonus pour toute la flotte.
	stars := ComputeNavalBonuses(*c)
	e.HPPct += stars.NavalHPBonusPct
	e.SpeedPct += stars.NavalSpeedBonusPct
	e.DamageTakenReduction += stars.NavalDamageReductionPct
	e.RumReductionPct += stars.RumConsumptionReductionPct

	if e.RumReductionPct > maxRumReduction {
		e.RumReductionPct = maxRumReduction
	}
	if e.DamageTakenReduction > maxDamageTakenBonus {
		e.DamageTakenReduction = maxDamageTakenBonus
	}
	return e
}

// -----------------------------------------------------------------------------
// Quel capitaine commande une flotte ?
// -----------------------------------------------------------------------------

// FleetCaptain renvoie le capitaine du navire amiral (voir SelectFlagshipShip),
// ou nil. captains = les capitaines du joueur.
func FleetCaptain(fleet *domain.Fleet, captains []domain.Captain) *domain.Captain {
	if fleet == nil || len(captains) == 0 {
		return nil
	}
	flagship, _, _ := SelectFlagshipShip(fleet)
	if flagship == nil {
		return nil
	}
	for i := range captains {
		c := &captains[i]
		if flagship.CaptainID != nil && *flagship.CaptainID == c.ID {
			return c
		}
	}
	for i := range captains {
		c := &captains[i]
		if c.AssignedShipID != nil && *c.AssignedShipID == flagship.ID {
			return c
		}
	}
	return nil
}

// IsNpcFleet : les flottes PNJ sont générées à la volée, sans île.
func IsNpcFleet(f *domain.Fleet) bool {
	return f.IslandID == uuid.Nil
}

// -----------------------------------------------------------------------------
// Vitesse
// -----------------------------------------------------------------------------

// ApplyWindWithCaptain transforme le facteur de vent (0,7 à 1,3) selon le
// capitaine.
func ApplyWindWithCaptain(e CaptainEffects, windFactor float64) float64 {
	if windFactor < 1 {
		if e.WindNeverUnfavorable {
			return 1
		}
		return 1 - (1-windFactor)*(1-e.UnfavorableWindReduction)
	}
	if windFactor > 1 && e.FavorableWindSpeedPct > 0 {
		return windFactor * (1 + e.FavorableWindSpeedPct)
	}
	return windFactor
}

// CaptainSpeedMultiplier : multiplicateur de vitesse hors vent.
func CaptainSpeedMultiplier(e CaptainEffects, morale int, pursuing bool) float64 {
	m := 1 + e.SpeedPct
	if pursuing {
		m += e.PursuitSpeedPct
	}
	if e.LowMoraleSpeedPct > 0 && morale < e.LowMoraleThreshold {
		m += e.LowMoraleSpeedPct
	}
	return m
}

// FleetMorale lit le moral de croisière (50 si jamais initialisé).
func FleetMorale(f *domain.Fleet) int {
	if f == nil || f.MoraleCruise == nil {
		return DefaultFleetMorale
	}
	return *f.MoraleCruise
}

// -----------------------------------------------------------------------------
// Description lisible (fiche du capitaine)
// -----------------------------------------------------------------------------

func pct(v float64) string { return fmt.Sprintf("%.0f %%", math.Round(v*100)) }

// CaptainEffectLines décrit le talent d'un capitaine en français, avec les
// valeurs de son niveau actuel (sans les étoiles, affichées à part).
func CaptainEffectLines(c *domain.Captain) []string {
	if c == nil {
		return nil
	}
	tmp := *c
	tmp.Stars = 0
	e := ComputeCaptainEffects(&tmp)
	var out []string
	add := func(s string) { out = append(out, s) }

	if e.SpeedPct > 0 {
		add("Vitesse de la flotte +" + pct(e.SpeedPct))
	}
	if e.WindNeverUnfavorable {
		add("Le vent n'est jamais défavorable")
	}
	if e.FavorableWindSpeedPct > 0 {
		add("Vitesse +" + pct(e.FavorableWindSpeedPct) + " par vent favorable")
	}
	if e.UnfavorableWindReduction > 0 {
		add("Pénalité de vent défavorable −" + pct(e.UnfavorableWindReduction))
	}
	if e.PursuitSpeedPct > 0 {
		add("Vitesse de poursuite +" + pct(e.PursuitSpeedPct) + " (attaques)")
	}
	if e.LowMoraleSpeedPct > 0 {
		add(fmt.Sprintf("Sous %d de moral, vitesse +%s", e.LowMoraleThreshold, pct(e.LowMoraleSpeedPct)))
	}
	if e.RumReductionPct > 0 {
		add("Rhum consommé −" + pct(e.RumReductionPct))
	}
	if e.FleetDamagePct > 0 {
		add("Dégâts de toute la flotte +" + pct(e.FleetDamagePct))
	}
	if e.FlagshipDamagePct > 0 {
		add("Dégâts du navire amiral +" + pct(e.FlagshipDamagePct))
	}
	for _, k := range []string{"warrior", "archer", "gunner"} {
		if v := e.CrewTypeDamagePct[k]; v > 0 {
			name := map[string]string{"warrior": "Guerriers", "archer": "Archers", "gunner": "Artilleurs"}[k]
			add(fmt.Sprintf("Navires de %s : dégâts +%s", name, pct(v)))
		}
	}
	if e.VsNpcDamagePct > 0 {
		add("Dégâts contre les PNJ +" + pct(e.VsNpcDamagePct))
	}
	if e.RPSAdvantageBonus > 0 {
		add("Avantage du triangle Guerriers/Archers/Artilleurs +" + pct(e.RPSAdvantageBonus))
	}
	if e.DamageTakenReduction > 0 {
		add("Dégâts subis par la flotte −" + pct(e.DamageTakenReduction))
	}
	if e.CrewSavePct > 0 {
		add("Matelots perdus au combat −" + pct(e.CrewSavePct) + " (ils rentrent à l'île)")
	}
	if e.MoraleFloor > 0 {
		add(fmt.Sprintf("Moral au combat jamais sous %d", e.MoraleFloor))
	}
	if e.EnemyMoraleDamage > 0 {
		add(fmt.Sprintf("Moral ennemi −%d au début du combat", e.EnemyMoraleDamage))
	}
	if e.MoraleBonusAmplify > 0 {
		add("Ton bonus de moral au combat est " + pct(e.MoraleBonusAmplify) + " plus fort")
	}
	if e.EnemyMoraleBonusReduction > 0 {
		add("Le bonus de moral de l'ennemi est réduit de " + pct(e.EnemyMoraleBonusReduction))
	}
	if e.NoSeaDecay {
		add("Le moral ne baisse jamais en mer")
	} else if e.SeaDecayReduction > 0 {
		add("Moral en mer : baisse " + pct(e.SeaDecayReduction) + " moins vite")
	}
	if e.LowMoraleDecayReduction > 0 {
		add("Sous 40 de moral, il baisse " + pct(e.LowMoraleDecayReduction) + " moins vite")
	}
	if e.PortRecoveryPct > 0 {
		add("Au port, le moral remonte " + pct(e.PortRecoveryPct) + " plus vite")
	}
	if e.DepartureMorale > 0 {
		add(fmt.Sprintf("Moral +%d à chaque départ du port", e.DepartureMorale))
	}
	if e.VictoryMorale > 0 {
		add(fmt.Sprintf("Moral +%d de plus après chaque victoire", e.VictoryMorale))
	}
	if e.CargoPct > 0 {
		add("Cargaison +" + pct(e.CargoPct))
	}
	if e.HarvestPct > 0 {
		add("Récolte sur les spots +" + pct(e.HarvestPct))
	}
	if e.LootPct > 0 {
		add("Butin des victoires +" + pct(e.LootPct))
	}
	if e.TicketChance > 0 {
		add(pct(e.TicketChance) + " de chance de ticket bonus par victoire")
	}
	if len(out) == 0 {
		add("Aucun effet")
	}
	return out
}

// CaptainEffectSummary : les lignes jointes (pour un affichage sur une ligne).
func CaptainEffectSummary(c *domain.Captain) string {
	return strings.Join(CaptainEffectLines(c), " · ")
}
