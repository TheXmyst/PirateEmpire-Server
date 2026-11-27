package engine

// Suites d'un combat pour une flotte de joueur : moral, XP du capitaine,
// matelots sauvés des navires coulés. Appelé depuis executePveCombat et
// executePvpCombat (chemin base de données : la mémoire est rechargée après).

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// combatSide : une flotte de joueur engagée dans un combat.
type combatSide struct {
	fleet     *domain.Fleet
	captain   *domain.Captain
	destroyed []uuid.UUID
	won, lost bool
	winXP     int // XP du capitaine en cas de victoire
}

// loadPlayerCaptains charge les capitaines d'un joueur (vide si erreur).
func loadPlayerCaptains(db *gorm.DB, playerID uuid.UUID) []domain.Captain {
	var caps []domain.Captain
	if playerID == uuid.Nil {
		return caps
	}
	_ = db.Where("player_id = ?", playerID).Find(&caps).Error
	return caps
}

// fleetPlayerCaptains : capitaines du propriétaire d'une flotte (préchargés si
// possible, sinon lus en base).
func fleetPlayerCaptains(db *gorm.DB, fleet *domain.Fleet) []domain.Captain {
	if len(fleet.Island.Player.Captains) > 0 {
		return fleet.Island.Player.Captains
	}
	var island domain.Island
	if err := db.Select("id", "player_id").First(&island, "id = ?", fleet.IslandID).Error; err != nil {
		return nil
	}
	return loadPlayerCaptains(db, island.PlayerID)
}

// applyCombatOutcome : à appeler APRÈS le combat, AVANT de couler les navires
// (on lit les matelots à bord). Ne sauvegarde pas la flotte : l'appelant le fait.
func applyCombatOutcome(db *gorm.DB, side combatSide) {
	eff := economy.ComputeCaptainEffects(side.captain)

	// 1. Moral de croisière
	economy.ApplyCombatMorale(side.fleet, side.won, side.lost, eff)

	// 2. Matelots sauvés des navires coulés (ils rentrent à l'île)
	if eff.CrewSavePct > 0 && len(side.destroyed) > 0 {
		saved := map[domain.UnitType]int{}
		for _, id := range side.destroyed {
			for _, s := range side.fleet.Ships {
				if s.ID != id {
					continue
				}
				saved[domain.Warrior] += int(math.Floor(float64(s.MilitiaWarriors) * eff.CrewSavePct))
				saved[domain.Archer] += int(math.Floor(float64(s.MilitiaArchers) * eff.CrewSavePct))
				saved[domain.Gunner] += int(math.Floor(float64(s.MilitiaGunners) * eff.CrewSavePct))
			}
		}
		total := saved[domain.Warrior] + saved[domain.Archer] + saved[domain.Gunner]
		if total > 0 {
			var island domain.Island
			if err := db.First(&island, "id = ?", side.fleet.IslandID).Error; err == nil {
				if island.Crew == nil {
					island.Crew = map[domain.UnitType]int{}
				}
				for k, v := range saved {
					island.Crew[k] += v
				}
				_ = island.BeforeSave(db)
				db.Model(&domain.Island{}).Where("id = ?", island.ID).Update("crew", island.CrewJSON)
				fmt.Printf("[CAPTAIN] crew saved fleet=%s total=%d\n", side.fleet.ID, total)
			}
		}
	}

	// 3. XP du capitaine
	if side.captain != nil {
		xp := economy.CaptainXPDefeat
		if side.won {
			xp = side.winXP
		}
		grantCaptainXP(db, side.captain.ID, xp)
	}
}

// grantCaptainXP ajoute de l'XP à un capitaine directement en base.
func grantCaptainXP(db *gorm.DB, captainID uuid.UUID, xp int) {
	if xp <= 0 {
		return
	}
	var c domain.Captain
	if err := db.First(&c, "id = ?", captainID).Error; err != nil {
		return
	}
	levels := economy.AddCaptainXP(&c, xp)
	db.Model(&domain.Captain{}).Where("id = ?", c.ID).Updates(map[string]interface{}{"level": c.Level, "xp": c.XP})
	if levels > 0 {
		fmt.Printf("[CAPTAIN] %s level up -> %d (+%d xp)\n", c.Name, c.Level, xp)
	}
}

// applyLootBonus : butin des victoires augmenté par le capitaine, plus une
// chance de ticket bonus. Les tickets ne sont pas multipliés.
func applyLootBonus(loot map[domain.ResourceType]float64, captain *domain.Captain) {
	eff := economy.ComputeCaptainEffects(captain)
	if eff.LootPct > 0 {
		for res, amount := range loot {
			if res == domain.CaptainTicket {
				continue
			}
			loot[res] = math.Floor(amount * (1 + eff.LootPct))
		}
	}
	if eff.TicketChance > 0 && rand.Float64() < eff.TicketChance {
		loot[domain.CaptainTicket] += 1
	}
}
