package economy

import (
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
)

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

// BeginnerProtectionEndsAt renvoie la fin théorique de la protection (72 h
// après l'inscription). Utile pour l'affichage.
func BeginnerProtectionEndsAt(player *domain.Player) time.Time {
	return player.CreatedAt.Add(BeginnerProtectionDuration)
}

// ComputePvpLoot calcule le butin d'un pillage réussi : PvpLootRate des
// ressources au-dessus de la réserve protégée, le tout plafonné par la place
// libre dans la cale de l'attaquant (répartie au prorata entre ressources).
func ComputePvpLoot(defender map[domain.ResourceType]float64, defenderTownHall int, freeCargo float64) map[domain.ResourceType]float64 {
	loot := make(map[domain.ResourceType]float64)
	safe := float64(defenderTownHall) * PvpSafePerTownHall
	total := 0.0
	for _, res := range lootableResources {
		available := defender[res] - safe
		if available > 0 {
			stolen := available * PvpLootRate
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
