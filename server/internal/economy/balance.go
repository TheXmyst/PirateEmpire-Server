package economy

import (
	"fmt"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// =====================================================================
// Réglages d'équilibrage (rééquilibrage de novembre 2026)
// Toutes les valeurs « de game design » ajoutées par ce rééquilibrage
// sont regroupées ici pour pouvoir les retoucher facilement.
// =====================================================================

const (
	// --- Protection débutant ---

	// BeginnerProtectionDuration : durée de la protection après l'inscription.
	BeginnerProtectionDuration = 72 * time.Hour
	// BeginnerProtectionMaxTownHall : la protection tombe dès que l'Hôtel de
	// Ville atteint ce niveau (même si les 72 h ne sont pas écoulées).
	BeginnerProtectionMaxTownHall = 8

	// --- Pillage PvP ---

	// PvpLootRate : part des ressources pillables qui est volée.
	PvpLootRate = 0.25
	// PvpSafePerTownHall : réserve protégée par niveau d'Hôtel de Ville.
	PvpSafePerTownHall = 2000.0

	// --- Tickets de gacha ---

	// StartingCaptainTickets : tickets donnés à l'inscription.
	StartingCaptainTickets = 3
	// DailyLoginTickets : tickets donnés une fois par jour à la connexion.
	DailyLoginTickets = 1
	// TicketsPerTownHallLevel : tickets donnés pour chaque niveau d'Hôtel de
	// Ville atteint à partir du niveau 2.
	TicketsPerTownHallLevel = 2

	// --- Nœuds de ressources (récolte par une flotte stationnée) ---

	// NodeRateWoodStone, NodeRateRum, NodeRateGold : débit de base (par seconde).
	NodeRateWoodStone = 0.3
	NodeRateRum       = 0.2
	NodeRateGold      = 0.1
	// NodeDurationBonusPerMinute : bonus de débit par minute passée sur le nœud,
	// plafonné à NodeMaxDurationBonus (x2 après 100 min).
	NodeDurationBonusPerMinute = 0.01
	NodeMaxDurationBonus       = 2.0
)

// PveTicketDropChance : chance de gagner 1 ticket de gacha en battant une
// cible PvE, selon son palier.
var PveTicketDropChance = map[int]float64{1: 0.10, 2: 0.20, 3: 0.35}

// NodeHarvestRatePerSec renvoie le débit de récolte d'un nœud de ressources :
// débit de base de la ressource x richesse du nœud x bonus de durée.
func NodeHarvestRatePerSec(res domain.ResourceType, richness, minutesStationed float64) float64 {
	base := NodeRateWoodStone
	switch res {
	case domain.Rum:
		base = NodeRateRum
	case domain.Gold:
		base = NodeRateGold
	}
	if richness <= 0 {
		richness = 1.0
	}
	bonus := 1.0 + minutesStationed*NodeDurationBonusPerMinute
	if bonus > NodeMaxDurationBonus {
		bonus = NodeMaxDurationBonus
	}
	if bonus < 1.0 {
		bonus = 1.0
	}
	return base * richness * bonus
}

// FleetCargoCapacity renvoie la capacité totale de cale d'une flotte.
func FleetCargoCapacity(f *domain.Fleet) float64 {
	capacity := 0.0
	for _, s := range f.Ships {
		capacity += ShipCargoCapacity(s.Type)
	}
	return capacity
}

// lootableResources : ressources qu'un pillage peut voler (jamais les tickets).
var lootableResources = []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum}

// TownHallLevel renvoie le niveau atteint de l'Hôtel de Ville.
// Contrairement à GetBuildingLevel, un Hôtel de Ville en cours d'amélioration
// compte pour son niveau actuel (et non 0).
func TownHallLevel(island *domain.Island) int {
	for _, b := range island.Buildings {
		if b.Type == "Hôtel de Ville" {
			return b.Level
		}
	}
	return 0
}

// IsBeginnerProtected indique si un joueur bénéficie encore de la protection
// débutant : compte de moins de 72 h, Hôtel de Ville sous le niveau 8, et pas
// d'attaque lancée contre un autre joueur.
func IsBeginnerProtected(player *domain.Player, townHallLevel int, now time.Time) bool {
	if player == nil || player.BeginnerProtectionForfeited {
		return false
	}
	if townHallLevel >= BeginnerProtectionMaxTownHall {
		return false
	}
	if player.CreatedAt.IsZero() {
		return false
	}
	return now.Sub(player.CreatedAt) < BeginnerProtectionDuration
}

// PvpTargetBlockReason renvoie pourquoi une île ne peut pas être attaquée
// (bouclier de paix ou protection débutant), ou "" si l'attaque est permise.
// targetIsland doit avoir Player et Buildings préchargés. Les admins passent.
func PvpTargetBlockReason(attacker *domain.Player, targetIsland *domain.Island, now time.Time) string {
	if attacker != nil && attacker.IsAdmin {
		return ""
	}
	if targetIsland.ProtectedUntil != nil && now.Before(*targetIsland.ProtectedUntil) {
		return "Ce joueur est sous protection de paix"
	}
	if IsBeginnerProtected(&targetIsland.Player, TownHallLevel(targetIsland), now) {
		return fmt.Sprintf("Ce joueur bénéficie de la protection débutant (jusqu'au %s ou Hôtel de Ville niveau %d)",
			BeginnerProtectionEndsAt(&targetIsland.Player).Format("02/01 15:04"), BeginnerProtectionMaxTownHall)
	}
	return ""
}

// BeginnerProtectionEndsAt renvoie la fin théorique de la protection (72 h
// après l'inscription). Utile pour l'affichage.
func BeginnerProtectionEndsAt(player *domain.Player) time.Time {
	return player.CreatedAt.Add(BeginnerProtectionDuration)
}

// ComputePvpLoot calcule le butin d'un pillage réussi : PvpLootRate des
// ressources au-dessus de la réserve protégée, le tout plafonné par la place
// libre dans la cale de l'attaquant (répartie au prorata entre ressources).
func ComputePvpLoot(defender map[domain.ResourceType]float64, defenderTownHall int, freeCargo float64) map[domain.ResourceType]float64 {
	return ComputePvpLootWithBonus(defender, defenderTownHall, freeCargo, 0)
}

// ComputePvpLootWithBonus : comme ComputePvpLoot, avec un bonus de butin du
// capitaine (0.20 = 25 % × 1,2 des ressources pillables, jamais plus que tout).
func ComputePvpLootWithBonus(defender map[domain.ResourceType]float64, defenderTownHall int, freeCargo, lootBonus float64) map[domain.ResourceType]float64 {
	rate := math.Min(1, PvpLootRate*(1+lootBonus))
	loot := make(map[domain.ResourceType]float64)
	safe := float64(defenderTownHall) * PvpSafePerTownHall
	total := 0.0
	for _, res := range lootableResources {
		available := defender[res] - safe
		if available > 0 {
			stolen := available * rate
			loot[res] = stolen
			total += stolen
		}
	}
	if freeCargo < 0 {
		freeCargo = 0
	}
	if total > freeCargo && total > 0 {
		ratio := freeCargo / total
		for res, amount := range loot {
			loot[res] = amount * ratio
		}
	}
	for res, amount := range loot {
		amount = math.Floor(amount)
		if amount <= 0 {
			delete(loot, res)
			continue
		}
		loot[res] = amount
	}
	return loot
}

// ShipCargoCapacity renvoie la capacité de cale d'un type de navire, lue dans
// configs/ships.json (source unique de vérité).
func ShipCargoCapacity(shipType string) float64 {
	if cfg, err := GetShipStats(shipType); err == nil && cfg.CargoCapacity > 0 {
		return cfg.CargoCapacity
	}
	return 500 // Valeur de secours (capacité du Sloop)
}

// TicketRewards décrit les tickets gagnés lors d'une réclamation.
type TicketRewards struct {
	Daily        int    // Tickets de connexion quotidienne
	TownHall     int    // Tickets de montée de niveau d'Hôtel de Ville
	NewDailyDay  string // Nouvelle valeur de Player.LastDailyTicketDay
	NewTownHallL int    // Nouvelle valeur de Player.TownHallTicketsLevel
}

// Total renvoie le nombre total de tickets gagnés.
func (r TicketRewards) Total() int { return r.Daily + r.TownHall }

// ComputeTicketRewards calcule les tickets à donner (fonction pure, testable).
// - today : jour courant au format AAAA-MM-JJ.
// - townHall : niveau actuel de l'Hôtel de Ville.
func ComputeTicketRewards(lastDailyDay, today string, grantedTownHall, townHall int) TicketRewards {
	r := TicketRewards{NewDailyDay: lastDailyDay, NewTownHallL: grantedTownHall}
	if lastDailyDay != today {
		r.Daily = DailyLoginTickets
		r.NewDailyDay = today
	}
	// Les tickets d'HdV commencent au niveau 2 : on considère le niveau 1
	// comme déjà « payé ».
	from := grantedTownHall
	if from < 1 {
		from = 1
	}
	if townHall > from {
		r.TownHall = (townHall - from) * TicketsPerTownHallLevel
		r.NewTownHallL = townHall
	} else if grantedTownHall < 1 {
		r.NewTownHallL = 1
	}
	return r
}
