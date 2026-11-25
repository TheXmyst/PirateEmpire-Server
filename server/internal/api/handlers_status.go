package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/engine"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// gameWorld est l'état du jeu en mémoire, branché au démarrage par main.go.
var gameWorld *engine.World

// SetWorld branche le monde en mémoire sur les handlers.
func SetWorld(w *engine.World) { gameWorld = w }

// GetStatus renvoie l'état complet d'un joueur.
//
// C'est l'endpoint que le client appelle en boucle (toutes les 1-2 s). Avant,
// chaque appel relisait le joueur et son île en base, recalculait la
// production et réécrivait l'île. Maintenant il lit directement la mémoire :
// le tick a déjà calculé les ressources et la position des flottes, il n'y a
// plus qu'à les copier dans la réponse. Zéro requête SQL dans le cas normal.
func GetStatus(c echo.Context) error {
	playerIDStr := c.QueryParam("player_id")
	if playerIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "player_id required"})
	}

	playerID, err := uuid.Parse(playerIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid player_id"})
	}

	var response dto.StatusResponse
	found := false

	gameWorld.WithPlayerIsland(playerID, func(island *domain.Island, tools *engine.IslandTools) {
		if island == nil {
			return
		}
		found = true
		db := repository.GetDB()

		// 1. Corrections rares qui écrivent en base (comptes anciens, fin de
		//    recrutement). Elles passent par la passerelle : sauvegarde de la
		//    mémoire, écriture en base, puis rechargement de la mémoire.
		if needsFleetRepair(island) {
			islandID := island.ID
			island, _ = tools.RunWithDB(func() error {
				if err := ensurePlayerFleets(db, &domain.Island{ID: islandID}); err != nil {
					fmt.Printf("[FLEET] GetStatus: Failed to ensure fleets: %v\n", err)
				}
				return nil
			})
		}
		if island != nil && island.MilitiaRecruiting && island.MilitiaRecruitDoneAt != nil && time.Now().After(*island.MilitiaRecruitDoneAt) {
			target := island
			island, _ = tools.RunWithDB(func() error {
				CheckIslandRecruitment(db, target)
				return nil
			})
		}
		if island == nil {
			found = false
			return
		}

		// 2. Le reste se fait en mémoire, et sera sauvegardé par la sauvegarde
		//    périodique.
		tools.CompleteResearchIfDue(time.Now()) // LAZY UPDATE (le tick le fait aussi)

		// HOTFIX: Update Townhall position
		for j := range island.Buildings {
			b := &island.Buildings[j]
			if b.Type == "Hôtel de Ville" && (b.X != -40 || b.Y != -144) {
				b.X = -40
				b.Y = -144
				tools.MarkDirty(engine.DirtyBuildings)
			}
		}

		// SSOT Capacity Calculation (champs calculés, non sauvegardés)
		for j := range island.Fleets {
			island.Fleets[j].ComputePayload()
		}

		// Log fleets being sent to client
		fleetInfo := make([]string, 0, len(island.Fleets))
		for _, f := range island.Fleets {
			fleetInfo = append(fleetInfo, fmt.Sprintf("(%s,%s)", f.ID.String(), f.Name))
		}
		fmt.Printf("[STATUS] player=%s island=%s fleets=%d ids=%v\n",
			playerID.String(), island.ID.String(), len(island.Fleets), fleetInfo)

		if os.Getenv("DEBUG") != "" || os.Getenv("CAPTAIN_DEBUG") != "" {
			if time.Now().Unix()%10 == 0 {
				fmt.Printf("[CAPTAINS] status player=%s captains=%d\n", playerID, len(island.Player.Captains))
			}
		}

		// 3. On construit la réponse PENDANT qu'on a le verrou. Les DTO sont
		//    des copies : une fois le verrou rendu, le tick peut modifier l'île
		//    sans risque pour la réponse en cours d'envoi.
		islandDTO := island.ToDTO()
		captainDTOs := make([]dto.CaptainDTO, len(island.Player.Captains))
		for i, captain := range island.Player.Captains {
			captainDTOs[i] = captain.ToDTO()
		}
		response = dto.StatusResponse{
			Player:   *island.Player.ToDTO(),
			Islands:  []dto.IslandDTO{*islandDTO},
			Captains: captainDTOs,
		}
	})

	if found {
		return c.JSON(http.StatusOK, response)
	}
	return getStatusWithoutIsland(c, playerID)
}

// needsFleetRepair : le joueur n'a pas ses 3 flottes, ou une flotte avec des
// navires n'a pas de navire amiral (ce que ensurePlayerFleets corrige).
func needsFleetRepair(island *domain.Island) bool {
	names := map[string]bool{}
	for _, f := range island.Fleets {
		names[f.Name] = true
		if f.FlagshipShipID == nil && len(f.Ships) > 0 {
			return true
		}
	}
	return !names["Flotte 1"] || !names["Flotte 2"] || !names["Flotte 3"]
}

// getStatusWithoutIsland : cas rare d'un joueur sans île (ou inconnu).
// Il n'est pas dans le monde en mémoire, donc on le lit en base.
func getStatusWithoutIsland(c echo.Context, playerID uuid.UUID) error {
	var player domain.Player
	var notFound bool
	_ = gameWorld.RunWithDB(func() error {
		if err := repository.GetDB().Preload("Captains").First(&player, "id = ?", playerID).Error; err != nil {
			notFound = true
		}
		return nil
	})
	if notFound {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Player not found"})
	}
	captainDTOs := make([]dto.CaptainDTO, len(player.Captains))
	for i, captain := range player.Captains {
		captainDTOs[i] = captain.ToDTO()
	}
	return c.JSON(http.StatusOK, dto.StatusResponse{
		Player:   *player.ToDTO(),
		Islands:  []dto.IslandDTO{},
		Captains: captainDTOs,
	})
}
