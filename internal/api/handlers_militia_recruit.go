package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type RecruitMilitiaRequest struct {
	IslandID string `json:"island_id"`
	Warriors int    `json:"warriors"`
	Archers  int    `json:"archers"`
	Gunners  int    `json:"gunners"`
}

type RecruitMilitiaResponse struct {
	IslandID     string                  `json:"island_id"`
	MilitiaStock map[domain.UnitType]int `json:"militia_stock"`
	DoneAt       time.Time               `json:"done_at"`
}

// RecruitMilitia handles the militia recruitment request
// NOTE: Due to "no DB migration" constraint, we implement this as INSTANT recruitment for now.
func RecruitMilitia(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	req := new(RecruitMilitiaRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	if req.Warriors < 0 || req.Archers < 0 || req.Gunners < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Quantités invalides"})
	}
	totalUnits := req.Warriors + req.Archers + req.Gunners
	if totalUnits <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Aucune unité à recruter"})
	}

	islandID, err := uuid.Parse(req.IslandID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID d'île invalide"})
	}

	db := repository.GetDB()

	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", islandID, player.ID).Error; err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Île introuvable ou accès refusé"})
	}

	// Calculate costs (Must match client UI logic: W=100G/20R, A=120G/20R, G=150G/30R ???)
	// UI says:
	// goldCost := Warrior*10 + Archer*12 + Gunner*15
	// rumCost := Warrior*2 + Archer*2 + Gunner*3
	goldCost := float64(req.Warriors*10 + req.Archers*12 + req.Gunners*15)
	rumCost := float64(req.Warriors*2 + req.Archers*2 + req.Gunners*3)

	if island.Resources[domain.Gold] < goldCost {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Pas assez d'Or"})
	}
	if island.Resources[domain.Rum] < rumCost {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Pas assez de Rhum"})
	}

	// Calculate duration (base 15s + 3s per unit, with bonus)
	// Same logic as client for consistency
	// TODO: Get Militia Building Level for bonus (assuming 0 for now or fetch building)
	baseDuration := 15.0
	perUnitDuration := 3.0
	rawDuration := baseDuration + float64(totalUnits)*perUnitDuration

	// Loading building for bonus would require another query or preload
	// For now, simplify or do the query if needed.
	// Let's do a quick check for Militia building level
	militiaLevel := 0
	for _, b := range island.Buildings {
		if b.Type == "Milice" {
			militiaLevel = b.Level
			break
		}
	}

	bonusPct := float64(militiaLevel) * 0.005
	if bonusPct > 0.30 {
		bonusPct = 0.30
	}
	durationSeconds := rawDuration * (1.0 - bonusPct)
	if durationSeconds < 10.0 {
		durationSeconds = 10.0
	}

	// Deduct resources
	island.Resources[domain.Gold] -= goldCost
	island.Resources[domain.Rum] -= rumCost

	// Start recruitment timer
	doneAt := time.Now().Add(time.Duration(durationSeconds) * time.Second)
	island.MilitiaRecruiting = true
	island.MilitiaRecruitDoneAt = &doneAt
	island.MilitiaRecruitWarriors = req.Warriors
	island.MilitiaRecruitArchers = req.Archers
	island.MilitiaRecruitGunners = req.Gunners

	if err := db.Save(&island).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur sauvegarde"})
	}

	fmt.Printf("[MILITIA_RECRUIT] player=%s island=%s qty=%d duration=%.1fs -> started\n", player.ID, island.ID, totalUnits, durationSeconds)

	return c.JSON(http.StatusOK, RecruitMilitiaResponse{
		IslandID:     island.ID.String(),
		MilitiaStock: island.Crew, // Stock hasn't changed yet
		DoneAt:       doneAt,
	})
}

// CheckIslandRecruitment checks if recruitment is finished and applies changes
// Returns true if changes were made and saved
func CheckIslandRecruitment(db *gorm.DB, island *domain.Island) bool {
	if !island.MilitiaRecruiting || island.MilitiaRecruitDoneAt == nil {
		return false
	}

	if time.Now().After(*island.MilitiaRecruitDoneAt) {
		// Apply recruitment
		island.Crew[domain.Warrior] += island.MilitiaRecruitWarriors
		island.Crew[domain.Archer] += island.MilitiaRecruitArchers
		island.Crew[domain.Gunner] += island.MilitiaRecruitGunners

		// Reset state
		island.MilitiaRecruiting = false
		island.MilitiaRecruitDoneAt = nil
		island.MilitiaRecruitWarriors = 0
		island.MilitiaRecruitArchers = 0
		island.MilitiaRecruitGunners = 0

		if err := db.Save(island).Error; err != nil {
			fmt.Printf("[CheckIslandRecruitment] Failed to save island: %v\n", err)
			return false
		}

		fmt.Printf("[MILITIA_RECRUIT] COMPLETED for island %s\n", island.ID)
		return true
	}
	return false
}
