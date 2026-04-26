package api

import (
	"fmt"
	"net/http"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ShipMilitiaRequest struct {
	ShipID   string          `json:"ship_id"`
	Type     domain.UnitType `json:"type"`
	Quantity int             `json:"quantity"`
}

type ShipMilitiaResponse struct {
	Ship            dto.ShipDTO             `json:"ship"`
	MilitiaStock    map[domain.UnitType]int `json:"militia_stock"`
	MilitiaCapacity int                     `json:"militia_capacity"`
}

// GetShipTier returns a temporary tier for capacity calculation
func GetShipTier(shipType string) int {
	switch shipType {
	case "sloop":
		return 1
	case "brigantine":
		return 2
	case "frigate":
		return 3
	case "galleon":
		return 4
	case "manowar":
		return 5
	default:
		return 1
	}
}

// AssignShipMilitia transfers militia from island to ship
func AssignShipMilitia(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	req := new(ShipMilitiaRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	if req.Quantity <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La quantité doit être positive"})
	}

	shipID, err := uuid.Parse(req.ShipID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID de navire invalide"})
	}

	db := repository.GetDB()

	// Load ship
	var ship domain.Ship
	if err := db.First(&ship, "id = ?", shipID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}

	// Load island to check ownership and stock
	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", ship.IslandID, player.ID).Error; err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé ou île introuvable"})
	}

	// Check island stock
	if island.Crew[req.Type] < req.Quantity {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Stock de l'île insuffisant"})
	}

	// Check ship capacity (Temporary rule: 50 * Tier)
	tier := GetShipTier(ship.Type)
	capacity := 50 * tier
	currentTotal := ship.MilitiaWarriors + ship.MilitiaArchers + ship.MilitiaGunners
	if currentTotal+req.Quantity > capacity {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Capacité du navire atteinte (%d/%d)", currentTotal, capacity)})
	}

	// Update counts
	switch req.Type {
	case domain.Warrior:
		ship.MilitiaWarriors += req.Quantity
	case domain.Archer:
		ship.MilitiaArchers += req.Quantity
	case domain.Gunner:
		ship.MilitiaGunners += req.Quantity
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Type de milice inconnu"})
	}

	island.Crew[req.Type] -= req.Quantity
	ship.MilitiaCapacity = capacity // Persist the computed capacity for the UI

	// Save within transaction
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&ship).Error; err != nil {
			return err
		}
		if err := tx.Save(&island).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	fmt.Printf("[MILITIA_ASSIGN] ship=%s type=%s qty=%d (NewTotal: %d/%d)\n", ship.ID, req.Type, req.Quantity, currentTotal+req.Quantity, capacity)

	return c.JSON(http.StatusOK, ShipMilitiaResponse{
		Ship:            *ship.ToDTO(),
		MilitiaStock:    island.Crew,
		MilitiaCapacity: capacity,
	})
}

// UnassignShipMilitia transfers militia from ship to island
func UnassignShipMilitia(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	req := new(ShipMilitiaRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	if req.Quantity <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La quantité doit être positive"})
	}

	shipID, err := uuid.Parse(req.ShipID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID de navire invalide"})
	}

	db := repository.GetDB()

	// Load ship
	var ship domain.Ship
	if err := db.First(&ship, "id = ?", shipID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}

	// Check if ship has enough of this type
	var currentQty int
	switch req.Type {
	case domain.Warrior:
		currentQty = ship.MilitiaWarriors
	case domain.Archer:
		currentQty = ship.MilitiaArchers
	case domain.Gunner:
		currentQty = ship.MilitiaGunners
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Type de milice inconnu"})
	}

	if currentQty < req.Quantity {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Le navire n'a pas assez d'unités de ce type"})
	}

	// Load island
	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", ship.IslandID, player.ID).Error; err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé ou île introuvable"})
	}

	// Update counts
	switch req.Type {
	case domain.Warrior:
		ship.MilitiaWarriors -= req.Quantity
	case domain.Archer:
		ship.MilitiaArchers -= req.Quantity
	case domain.Gunner:
		ship.MilitiaGunners -= req.Quantity
	}

	island.Crew[req.Type] += req.Quantity

	tier := GetShipTier(ship.Type)
	capacity := 50 * tier
	ship.MilitiaCapacity = capacity

	// Save within transaction
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&ship).Error; err != nil {
			return err
		}
		if err := tx.Save(&island).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	fmt.Printf("[MILITIA_UNASSIGN] ship=%s type=%s qty=%d\n", ship.ID, req.Type, req.Quantity)

	return c.JSON(http.StatusOK, ShipMilitiaResponse{
		Ship:            *ship.ToDTO(),
		MilitiaStock:    island.Crew,
		MilitiaCapacity: capacity,
	})
}
