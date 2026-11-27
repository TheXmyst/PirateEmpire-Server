package api

import (
	"net/http"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type setActiveFleetRequest struct {
	FleetID string `json:"fleet_id"`
}

// SetActiveFleet choisit la flotte active de l'île (celle qui défend en PvP
// et qui est utilisée par défaut en PvE). Avant, le bouton « Activer » du
// client n'appelait rien : aucune flotte n'était jamais active.
func SetActiveFleet(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	req := new(setActiveFleetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}
	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID de flotte invalide"})
	}

	db := repository.GetDB()

	var fleet domain.Fleet
	if err := db.First(&fleet, "id = ?", fleetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable"})
	}

	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", fleet.IslandID, player.ID).Error; err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé ou île introuvable"})
	}

	if !isFleetUnlocked(fleet.Name, player.UnlockedTechs) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cette flotte est verrouillée (technologie requise)"})
	}

	if err := db.Model(&domain.Island{}).Where("id = ?", island.ID).Update("active_fleet_id", fleet.ID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	return c.JSON(http.StatusOK, map[string]string{"active_fleet_id": fleet.ID.String()})
}
