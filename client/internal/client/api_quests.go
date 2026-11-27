package client

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Quêtes de progression (voir server/internal/api/handlers_quests.go).

// QuestReward : récompense d'une quête.
type QuestReward struct {
	Wood    float64 `json:"wood"`
	Stone   float64 `json:"stone"`
	Gold    float64 `json:"gold"`
	Rum     float64 `json:"rum"`
	Tickets int     `json:"tickets"`
}

// Quest : une quête proposée au joueur.
type Quest struct {
	ID          string      `json:"id"`
	Family      string      `json:"family"` // story, building, challenge
	Phase       int         `json:"phase"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Tutorial    string      `json:"tutorial"`
	Progress    float64     `json:"progress"`
	Target      float64     `json:"target"`
	Done        bool        `json:"done"`
	Locked      bool        `json:"locked"`
	Hint        string      `json:"hint"`
	Reward      QuestReward `json:"reward"`
}

// QuestFamilySummary : quêtes réclamées / total d'une famille.
type QuestFamilySummary struct {
	Claimed int `json:"claimed"`
	Total   int `json:"total"`
}

// QuestsResponse : le journal de quêtes.
type QuestsResponse struct {
	Quests    []Quest                       `json:"quests"`
	Summary   map[string]QuestFamilySummary `json:"summary"`
	Claimable int                           `json:"claimable"`
	// Seulement après une réclamation :
	Claimed []string    `json:"claimed"`
	Granted QuestReward `json:"granted"`
	Message string      `json:"message"`
}

// GetQuests : le journal de quêtes du joueur.
func (c *APIClient) GetQuests() (*QuestsResponse, error) {
	resp, err := c.doAuthorizedRequest("GET", "/quests", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, serverError(resp, nil)
	}
	var out QuestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClaimQuest réclame une quête (id) ou toutes les quêtes terminées (id vide).
func (c *APIClient) ClaimQuest(id string) (*QuestsResponse, error) {
	body := map[string]interface{}{"id": id}
	if id == "" {
		body = map[string]interface{}{"all": true}
	}
	b, _ := json.Marshal(body)
	resp, err := c.doAuthorizedRequest("POST", "/quests/claim", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, serverError(resp, nil)
	}
	var out QuestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
