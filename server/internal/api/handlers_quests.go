package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// handlers_quests.go : le journal de quêtes.
//   GET  /quests        → les quêtes proposées et leur avancement
//   POST /quests/claim  → réclamer une quête ({"id": "..."}) ou tout ({"all": true})
//
// La progression est recalculée à chaque appel à partir de l'état du joueur
// (economy.EvaluateQuests). Seuls quelques événements sont comptés à part
// (tirages, recrutements, sorties en mer…) : voir BumpQuestCounter.

// QuestsResponse : ce que reçoit le client.
type QuestsResponse struct {
	Quests    []economy.QuestStatus                 `json:"quests"`
	Summary   map[string]economy.QuestFamilySummary `json:"summary"`
	Claimable int                                   `json:"claimable"`
}

// ClaimQuestRequest : une quête précise, ou toutes celles qui sont terminées.
type ClaimQuestRequest struct {
	ID  string `json:"id"`
	All bool   `json:"all"`
}

// ClaimQuestResponse : récompense reçue + journal à jour.
type ClaimQuestResponse struct {
	QuestsResponse
	Claimed []string            `json:"claimed"`
	Granted economy.QuestReward `json:"granted"`
	Message string              `json:"message,omitempty"`
}

// loadQuestState charge tout ce qu'il faut pour évaluer les quêtes d'un joueur.
func loadQuestState(db *gorm.DB, playerID uuid.UUID) (*domain.Island, *economy.QuestPlayerState, error) {
	var island domain.Island
	if err := db.Preload("Player").Preload("Buildings").Preload("Ships").Preload("Fleets.Ships").
		First(&island, "player_id = ?", playerID).Error; err != nil {
		return nil, nil, err
	}
	var captains []domain.Captain
	if err := db.Where("player_id = ?", playerID).Find(&captains).Error; err != nil {
		return nil, nil, err
	}
	var wins []struct {
		Tier  int
		Count int
	}
	if err := db.Model(&domain.PvEVictory{}).Select("tier, count(*) as count").
		Where("player_id = ?", playerID).Group("tier").Scan(&wins).Error; err != nil {
		return nil, nil, err
	}
	var claims []domain.PlayerQuestClaim
	if err := db.Where("player_id = ?", playerID).Find(&claims).Error; err != nil {
		return nil, nil, err
	}

	p := island.Player
	st := &economy.QuestPlayerState{
		Buildings:    island.Buildings,
		Techs:        p.UnlockedTechs,
		Ships:        island.Ships,
		Fleets:       island.Fleets,
		Captains:     captains,
		MilitiaStock: island.Crew,
		PveWins:      map[int]int{},
		Counters: map[string]float64{
			economy.QuestCounterCaptainPulls:     float64(p.QuestCaptainPulls),
			economy.QuestCounterMilitiaRecruited: float64(p.QuestMilitiaRecruited),
			economy.QuestCounterFleetsSent:       float64(p.QuestFleetsSent),
			economy.QuestCounterFleetsStationed:  float64(p.QuestFleetsStationed),
			economy.QuestCounterChatMessages:     float64(p.QuestChatMessages),
			economy.QuestCounterHarvested:        p.QuestHarvested,
		},
		InGuild: p.GuildID != nil,
		Claimed: map[string]bool{},
	}
	for _, w := range wins {
		st.PveWins[w.Tier] = w.Count
	}
	for _, c := range claims {
		st.Claimed[c.QuestID] = true
	}
	return &island, st, nil
}

func buildQuestsResponse(st *economy.QuestPlayerState) QuestsResponse {
	quests, summary := economy.EvaluateQuests(st)
	n := 0
	for _, q := range quests {
		if q.Done {
			n++
		}
	}
	if quests == nil {
		quests = []economy.QuestStatus{}
	}
	return QuestsResponse{Quests: quests, Summary: summary, Claimable: n}
}

// GetQuests : GET /quests
func GetQuests(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	_, st, err := loadQuestState(repository.GetDB(), player.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}
	return c.JSON(http.StatusOK, buildQuestsResponse(st))
}

var errQuestNotReady = errors.New("quest not ready")

// fitsInStorage : la récompense tient-elle dans l'entrepôt ? (Le moteur
// rabote tout ce qui dépasse la capacité : on refuse plutôt que de perdre.)
func fitsInStorage(res map[domain.ResourceType]float64, have map[domain.ResourceType]float64, limits map[domain.ResourceType]float64) (bool, domain.ResourceType) {
	for r, v := range res {
		limit, capped := limits[r]
		if !capped || v <= 0 {
			continue
		}
		if have[r]+v > limit {
			return false, r
		}
	}
	return true, ""
}

var resourceNamesFR = map[domain.ResourceType]string{
	domain.Wood: "bois", domain.Stone: "pierre", domain.Gold: "or", domain.Rum: "rhum",
}

// ClaimQuest : POST /quests/claim
func ClaimQuest(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	req := new(ClaimQuestRequest)
	if err := c.Bind(req); err != nil || (req.ID == "" && !req.All) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	var resp ClaimQuestResponse
	var blockedBy domain.ResourceType
	err := repository.GetDB().Transaction(func(tx *gorm.DB) error {
		island, st, err := loadQuestState(tx, player.ID)
		if err != nil {
			return err
		}
		var todo []economy.QuestDef
		if req.All {
			todo = economy.ClaimableQuests(st)
		} else {
			d, ok := economy.GetQuestDef(req.ID)
			if !ok || !economy.QuestUnlocked(d, st) || !economy.QuestDone(d, st) {
				return errQuestNotReady
			}
			todo = []economy.QuestDef{d}
		}

		if island.Resources == nil {
			island.Resources = map[domain.ResourceType]float64{}
		}
		limits := economy.ComputeStorageLimits(island.Buildings, st.Techs)
		now := time.Now()
		for _, d := range todo {
			res := d.Reward.Resources()
			if ok, r := fitsInStorage(res, island.Resources, limits); !ok {
				blockedBy = r
				continue
			}
			claim := domain.PlayerQuestClaim{PlayerID: player.ID, QuestID: d.ID, ClaimedAt: now}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue // déjà réclamée (double clic)
			}
			for r, v := range res {
				island.Resources[r] += v
			}
			st.Claimed[d.ID] = true
			resp.Claimed = append(resp.Claimed, d.ID)
			resp.Granted = resp.Granted.Add(d.Reward)
		}
		if len(resp.Claimed) > 0 {
			_ = island.BeforeSave(tx) // Resources -> colonne JSON
			if err := tx.Model(&domain.Island{}).Where("id = ?", island.ID).
				Update("resources", island.ResourcesJSON).Error; err != nil {
				return err
			}
		}
		// En réclamant, de nouvelles quêtes peuvent s'ouvrir (Histoire, Défis).
		resp.QuestsResponse = buildQuestsResponse(st)
		return nil
	})
	if errors.Is(err, errQuestNotReady) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cette quête n'est pas encore terminée"})
	}
	if err != nil {
		fmt.Printf("[QUESTS] claim failed for %s: %v\n", player.ID, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur du serveur, réessaie"})
	}
	if blockedBy != "" {
		name := resourceNamesFR[blockedBy]
		if len(resp.Claimed) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("Ton entrepôt est plein (%s) : dépense un peu ou améliore l'Entrepôt avant de réclamer.", name),
			})
		}
		resp.Message = fmt.Sprintf("Certaines récompenses attendent : ton entrepôt est plein (%s).", name)
	}
	if resp.Claimed == nil {
		resp.Claimed = []string{}
	}
	return c.JSON(http.StatusOK, resp)
}

// BumpQuestCounter ajoute n à un compteur de quêtes du joueur. À appeler
// seulement après une action réussie. Une erreur est juste journalisée : les
// quêtes ne doivent jamais faire échouer une action de jeu.
func BumpQuestCounter(db *gorm.DB, playerID uuid.UUID, counter string, n float64) {
	if n <= 0 || playerID == uuid.Nil {
		return
	}
	col, ok := questCounterColumns[counter]
	if !ok {
		return
	}
	if err := db.Model(&domain.Player{}).Where("id = ?", playerID).
		UpdateColumn(col, gorm.Expr(col+" + ?", n)).Error; err != nil {
		fmt.Printf("[QUESTS] counter %s not updated for %s: %v\n", counter, playerID, err)
	}
}

// questCounterColumns : compteur → colonne (liste fermée, jamais de texte venu
// du client dans la requête SQL).
var questCounterColumns = map[string]string{
	economy.QuestCounterCaptainPulls:     "quest_captain_pulls",
	economy.QuestCounterMilitiaRecruited: "quest_militia_recruited",
	economy.QuestCounterFleetsSent:       "quest_fleets_sent",
	economy.QuestCounterFleetsStationed:  "quest_fleets_stationed",
	economy.QuestCounterChatMessages:     "quest_chat_messages",
	economy.QuestCounterHarvested:        "quest_harvested",
}

// CountsForQuests : middleware qui ajoute 1 à chaque compteur donné quand la
// route a réussi (réponse 2xx). Pour les actions simples (envoyer une flotte,
// écrire dans le chat…).
func CountsForQuests(counters ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err != nil {
				return err
			}
			if s := c.Response().Status; s < 200 || s >= 300 {
				return nil
			}
			if p := auth.GetAuthenticatedPlayer(c); p != nil {
				for _, ctr := range counters {
					BumpQuestCounter(repository.GetDB(), p.ID, ctr, 1)
				}
			}
			return nil
		}
	}
}
