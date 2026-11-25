package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// claimTicketRewards donne au joueur les tickets de gacha qui lui reviennent
// (connexion quotidienne + niveaux d'Hôtel de Ville atteints depuis la
// dernière réclamation). Appelée à la connexion et avant une invocation.
// Les écritures ciblent uniquement les colonnes concernées pour ne rien
// écraser d'autre.
func claimTicketRewards(db *gorm.DB, playerID uuid.UUID) (economy.TicketRewards, error) {
	var rewards economy.TicketRewards

	err := db.Transaction(func(tx *gorm.DB) error {
		var player domain.Player
		if err := tx.First(&player, "id = ?", playerID).Error; err != nil {
			return err
		}
		var island domain.Island
		if err := tx.Preload("Buildings").First(&island, "player_id = ?", playerID).Error; err != nil {
			return err
		}

		today := time.Now().Format("2006-01-02")
		rewards = economy.ComputeTicketRewards(player.LastDailyTicketDay, today, player.TownHallTicketsLevel, economy.TownHallLevel(&island))

		if err := tx.Model(&domain.Player{}).Where("id = ?", playerID).UpdateColumns(map[string]interface{}{
			"last_daily_ticket_day":   rewards.NewDailyDay,
			"town_hall_tickets_level": rewards.NewTownHallL,
		}).Error; err != nil {
			return err
		}

		if rewards.Total() == 0 {
			return nil
		}
		if island.Resources == nil {
			island.Resources = make(map[domain.ResourceType]float64)
		}
		island.Resources[domain.CaptainTicket] += float64(rewards.Total())
		data, err := json.Marshal(island.Resources)
		if err != nil {
			return err
		}
		return tx.Model(&domain.Island{}).Where("id = ?", island.ID).UpdateColumn("resources", data).Error
	})

	if err == nil && rewards.Total() > 0 {
		fmt.Printf("[TICKETS] player=%s daily=%d townhall=%d\n", playerID, rewards.Daily, rewards.TownHall)
	}
	return rewards, err
}

// checkPvpTargetable vérifie qu'une île peut être attaquée : ni bouclier de
// paix, ni protection débutant. targetIsland doit avoir Player et Buildings
// préchargés. Renvoie un code HTTP et un message si l'attaque est refusée.
func checkPvpTargetable(attacker *domain.Player, targetIsland *domain.Island) (int, string, bool) {
	if attacker != nil && attacker.IsAdmin {
		return 0, "", true // Les admins peuvent tester librement
	}
	now := time.Now()
	if targetIsland.ProtectedUntil != nil && now.Before(*targetIsland.ProtectedUntil) {
		return http.StatusConflict, "Ce joueur est sous protection de paix", false
	}
	if economy.IsBeginnerProtected(&targetIsland.Player, economy.TownHallLevel(targetIsland), now) {
		ends := economy.BeginnerProtectionEndsAt(&targetIsland.Player)
		return http.StatusConflict, fmt.Sprintf("Ce joueur bénéficie de la protection débutant (jusqu'au %s ou Hôtel de Ville niveau %d)",
			ends.Format("02/01 15:04"), economy.BeginnerProtectionMaxTownHall), false
	}
	return 0, "", true
}

// forfeitBeginnerProtection retire la protection débutant d'un joueur qui
// attaque quelqu'un d'autre.
func forfeitBeginnerProtection(db *gorm.DB, player *domain.Player) {
	if player == nil || player.BeginnerProtectionForfeited {
		return
	}
	if err := db.Model(&domain.Player{}).Where("id = ?", player.ID).UpdateColumn("beginner_protection_forfeited", true).Error; err != nil {
		fmt.Printf("[PVP] Impossible de retirer la protection débutant de %s: %v\n", player.ID, err)
		return
	}
	player.BeginnerProtectionForfeited = true
}
