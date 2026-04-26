package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
)

type APIClient struct {
	BaseURL  string
	PlayerID uuid.UUID
	IslandID uuid.UUID     // Main island for now
	Token    string        // Auth token for Bearer authentication
	LastRTT  time.Duration // Latency
}

type ErrRequirementsNotMet struct {
	Title        string
	Subtitle     string
	Requirements []domain.Requirement
}

func (e *ErrRequirementsNotMet) Error() string {
	return e.Title
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{BaseURL: baseURL}
}

// doAuthorizedRequest performs an HTTP request with Bearer token authentication
// if a token is available. Returns the response and any error.
func (c *APIClient) doAuthorizedRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}

	// Set Content-Type for POST/PUT requests
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add Bearer token if available
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := &http.Client{}
	return client.Do(req)
}

func (c *APIClient) Register(username, password string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
		"email":    username + "@sea.com",
	})

	resp, err := http.Post(c.BaseURL+"/register", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register failed: %s", resp.Status)
	}

	var res struct {
		PlayerID uuid.UUID `json:"player_id"`
		IslandID uuid.UUID `json:"island_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	c.PlayerID = res.PlayerID
	c.IslandID = res.IslandID
	return nil
}

type LoginResponse struct {
	PlayerID uuid.UUID `json:"player_id"`
	IslandID uuid.UUID `json:"island_id"`
	Role     string    `json:"role"`
	IsAdmin  bool      `json:"is_admin"`
	Token    string    `json:"token,omitempty"` // Auth token for Bearer authentication
}

func (c *APIClient) Login(username, password string) (*LoginResponse, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := http.Post(c.BaseURL+"/login", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %s", resp.Status)
	}

	var res LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	c.PlayerID = res.PlayerID
	c.IslandID = res.IslandID
	c.Token = res.Token // Store token for authenticated requests
	return &res, nil
}

func (c *APIClient) GetStatus() (*domain.Player, error) {
	resp, err := http.Get(fmt.Sprintf("%s/status?player_id=%s", c.BaseURL, c.PlayerID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status failed: %s", resp.Status)
	}

	// The server returns a StatusResponse wrapper { player, islands, captains }
	var sr struct {
		Player   domain.Player    `json:"player"`
		Islands  []domain.Island  `json:"islands"`
		Captains []domain.Captain `json:"captains"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	// Reattach islands/captains to player for client use
	sr.Player.Islands = sr.Islands
	sr.Player.Captains = sr.Captains

	// Log fleets received from server
	if len(sr.Player.Islands) > 0 && len(sr.Player.Islands[0].Fleets) > 0 {
		fleets := sr.Player.Islands[0].Fleets
		fmt.Printf("[CLIENT] GetStatus received: fleets=%d\n", len(fleets))
		for i, fleet := range fleets {
			fmt.Printf("[CLIENT] Fleet[%d] id=%s name=%s ships=%d\n",
				i, fleet.ID.String(), fleet.Name, len(fleet.Ships))
		}
	} else {
		fmt.Printf("[CLIENT] GetStatus received: no fleets (islands=%d)\n", len(sr.Player.Islands))
	}
	return &sr.Player, nil
}

// SendChat posts a chat message to the server (requires auth).
func (c *APIClient) SendChat(message string) error {
	reqBody, _ := json.Marshal(map[string]string{"message": message})
	resp, err := c.doAuthorizedRequest("POST", "/chat/send", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chat send failed: %s", string(body))
	}
	return nil
}

// FetchChat retrieves chat messages newer than `since`.
func (c *APIClient) FetchChat(since time.Time) ([]domain.ChatMessage, error) {
	url := fmt.Sprintf("%s/chat/feed", c.BaseURL)
	if !since.IsZero() {
		url += "?since=" + since.Format(time.RFC3339)
	}
	resp, err := c.doAuthorizedRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat fetch failed: %s", string(body))
	}
	var out struct {
		Messages   []domain.ChatMessage `json:"messages"`
		NextCursor string               `json:"next_cursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Messages, nil
}

// FetchChatBefore retrieves older chat messages before a timestamp with pagination.
func (c *APIClient) FetchChatBefore(before time.Time, limit int) ([]domain.ChatMessage, string, error) {
	url := fmt.Sprintf("%s/chat/feed", c.BaseURL)
	params := []string{}
	if !before.IsZero() {
		params = append(params, "before="+before.Format(time.RFC3339))
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}
	resp, err := c.doAuthorizedRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("chat fetch failed: %s", string(body))
	}
	var out struct {
		Messages   []domain.ChatMessage `json:"messages"`
		NextCursor string               `json:"next_cursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", err
	}
	return out.Messages, out.NextCursor, nil
}

func (c *APIClient) Build(buildingType string, x, y float64) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"island_id": c.IslandID,
		"type":      buildingType,
		"x":         x,
		"y":         y,
	})

	resp, err := c.doAuthorizedRequest("POST", "/build", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("erreur de connexion: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		// Try to parse error message from JSON response
		body, _ := io.ReadAll(resp.Body)
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != "" {
			return fmt.Errorf("%s", errorResp.Error)
		}
		// Fallback to generic error if JSON parsing fails
		return fmt.Errorf("construction impossible: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) ResetProgress() error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/reset", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reset failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) Upgrade(playerID, buildingID string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"player_id":   c.PlayerID.String(),
		"building_id": buildingID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/upgrade", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upgrade failed: %s - %s", resp.Status, string(body))
	}
	return nil
}

func (c *APIClient) AddResources() error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/add-resources", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add resources failed: %s", resp.Status)
	}
	return nil
}
func (c *APIClient) StartResearch(techID string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"tech_id":   techID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/research/start", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}
	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("research failed: %s", errRes.Error)
		}
		return fmt.Errorf("research failed: %s - %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *APIClient) BuildShip(shipType string) error {
	fmt.Printf("[DEBUG] BuildShip: PlayerID=%s, IslandID=%s, Type=%s\n", c.PlayerID, c.IslandID, shipType)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"island_id": c.IslandID,
		"ship_type": shipType,
	})

	resp, err := c.doAuthorizedRequest("POST", "/build-ship", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ship build failed: %s - %s", resp.Status, string(body))
	}
	return nil
}

// AddShipToFleet assigns a ship to a fleet
func (c *APIClient) AddShipToFleet(fleetID, shipID uuid.UUID) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"fleet_id":  fleetID,
		"ship_id":   shipID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/fleets/add-ship", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("erreur de connexion: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}
	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec assignation: %s - %s", resp.Status, string(bodyBytes))
	}
	return nil
}

// Captain types and methods
type Captain struct {
	ID             uuid.UUID  `json:"id"`
	PlayerID       uuid.UUID  `json:"player_id"`
	TemplateID     string     `json:"template_id"`
	Name           string     `json:"name"`
	Rarity         string     `json:"rarity"`
	Level          int        `json:"level"`
	XP             int        `json:"xp"`
	Stars          int        `json:"stars"`
	SkillID        string     `json:"skill_id"`
	AssignedShipID *uuid.UUID `json:"assigned_ship_id,omitempty"`
	InjuredUntil   *time.Time `json:"injured_until,omitempty"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
	// Passive effect fields (computed server-side)
	PassiveID       string          `json:"passive_id,omitempty"`
	PassiveValue    float64         `json:"passive_value,omitempty"`
	PassiveIntValue int             `json:"passive_int_value,omitempty"`
	Threshold       int             `json:"threshold,omitempty"`
	DrainPerMinute  float64         `json:"drain_per_minute,omitempty"`
	Flags           map[string]bool `json:"flags,omitempty"`
	// Naval bonuses from stars (computed server-side)
	NavalHPBonusPct            float64 `json:"naval_hp_bonus_pct,omitempty"`
	NavalSpeedBonusPct         float64 `json:"naval_speed_bonus_pct,omitempty"`
	NavalDamageReductionPct    float64 `json:"naval_damage_reduction_pct,omitempty"`
	RumConsumptionReductionPct float64 `json:"rum_consumption_reduction_pct,omitempty"`
	// UI Convenience fields (from server)
	Shards       int  `json:"shards"`
	NextStarCost int  `json:"next_star_cost"`
	CanUpgrade   bool `json:"can_upgrade"`
}

func (c *APIClient) GetCaptains() ([]Captain, error) {
	resp, err := c.doAuthorizedRequest("GET", "/captains", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec récupération capitaines: %s", resp.Status)
	}

	var captains []Captain
	if err := json.NewDecoder(resp.Body).Decode(&captains); err != nil {
		return nil, fmt.Errorf("erreur parsing capitaines: %v", err)
	}

	return captains, nil
}

func (c *APIClient) AssignCaptain(captainID, shipID string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"captain_id": captainID,
		"ship_id":    shipID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/captains/assign", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec assignation capitaine: %s", resp.Status)
	}

	return nil
}

func (c *APIClient) UnassignCaptain(captainID string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"captain_id": captainID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/captains/unassign", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec retrait capitaine: %s", resp.Status)
	}

	return nil
}

// Dev Methods
func (c *APIClient) DevAddResources(amount float64) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"amount":    amount,
	})
	resp, err := c.doAuthorizedRequest("POST", "/dev/add-resources", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) DevFinishBuilding() error {
	reqBody, _ := json.Marshal(map[string]interface{}{"player_id": c.PlayerID})
	resp, err := c.doAuthorizedRequest("POST", "/dev/finish-building", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) DevFinishResearch() error {
	reqBody, _ := json.Marshal(map[string]interface{}{"player_id": c.PlayerID})
	resp, err := c.doAuthorizedRequest("POST", "/dev/finish-research", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) DevFinishShip() error {
	reqBody, _ := json.Marshal(map[string]interface{}{"player_id": c.PlayerID})
	resp, err := c.doAuthorizedRequest("POST", "/dev/finish-ship", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) DevTimeSkip(hours int) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"player_id": c.PlayerID,
		"hours":     hours,
	})
	resp, err := c.doAuthorizedRequest("POST", "/dev/time-skip", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %s", resp.Status)
	}
	return nil
}

// SimulateEngagement simulates an engagement between two fleets (admin only)
func (c *APIClient) SimulateEngagement(fleetAID, fleetBID string) (*domain.EngagementResult, error) {
	// Log the exact IDs being sent
	fmt.Printf("[CLIENT] SimulateEngagement: fleetAID='%s' fleetBID='%s'\n", fleetAID, fleetBID)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_a_id": fleetAID,
		"fleet_b_id": fleetBID,
	})
	fmt.Printf("[CLIENT] Request JSON: %s\n", string(reqBody))

	resp, err := c.doAuthorizedRequest("POST", "/dev/simulate-engagement", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode == http.StatusForbidden {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("admin requis: %s", errRes.Error)
		}
		return nil, fmt.Errorf("admin requis")
	}

	if resp.StatusCode == http.StatusNotFound {
		// Special handling for 404: dev routes are disabled
		return nil, fmt.Errorf("DEV_ROUTES_DISABLED: Dev routes are disabled. Set DEV_ROUTES_ENABLED=true and restart server.")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec simulation engagement: %s", resp.Status)
	}

	var result domain.EngagementResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("échec parsing réponse: %v", err)
	}

	return &result, nil
}

// SummonResult represents a single summon result
type SummonResult struct {
	Rarity        string          `json:"rarity"`
	TemplateID    string          `json:"template_id"`
	Name          string          `json:"name"`
	IsDuplicate   bool            `json:"is_duplicate"`
	ShardsGranted int             `json:"shards_granted,omitempty"`
	Captain       *domain.Captain `json:"captain,omitempty"`
}

// SummonCaptainResponse represents the response from summon endpoint (supports x1 and x10)
type SummonCaptainResponse struct {
	Results            []SummonResult `json:"results"`
	TicketsBefore      int            `json:"tickets_before"`
	TicketsAfter       int            `json:"tickets_after"`
	ShardsTotalGranted int            `json:"shards_total_granted,omitempty"`
	DuplicateCount     int            `json:"duplicate_count,omitempty"`
	Compensation       struct {
		RefundedTickets int `json:"refunded_tickets"`
	} `json:"compensation"`
	// Legacy fields for backward compatibility (x1 only)
	Duplicate     bool            `json:"duplicate,omitempty"`
	TicketBalance int             `json:"ticket_balance,omitempty"`
	Rarity        string          `json:"rarity,omitempty"`
	TemplateID    string          `json:"template_id,omitempty"`
	Captain       *domain.Captain `json:"captain,omitempty"`
}

func (c *APIClient) SummonCaptain(count int) (*SummonCaptainResponse, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"count": count,
	})
	resp, err := c.doAuthorizedRequest("POST", "/tavern/summon-captain", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec recrutement: %s", resp.Status)
	}

	var result SummonCaptainResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("échec parsing réponse: %v", err)
	}

	return &result, nil
}

// GrantTickets grants tickets for testing (admin only)
func (c *APIClient) GrantTickets(amount int) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"amount": amount,
	})
	resp, err := c.doAuthorizedRequest("POST", "/dev/grant-tickets", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec attribution tickets: %s", resp.Status)
	}

	return nil
}

// UpgradeCaptainStars upgrades a captain's stars by 1
func (c *APIClient) UpgradeCaptainStars(captainID uuid.UUID) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"captain_id": captainID.String(),
	})
	resp, err := c.doAuthorizedRequest("POST", "/captains/upgrade-stars", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec amélioration: %s", resp.Status)
	}

	return nil
}

// ExchangeShardsResponse represents the response from exchange shards endpoint
type ExchangeShardsResponse struct {
	Message       string                   `json:"message"`
	TicketsBefore int                      `json:"tickets_before"`
	TicketsAfter  int                      `json:"tickets_after"`
	ShardsSpent   int                      `json:"shards_spent"`
	Details       []map[string]interface{} `json:"details"`
}

// ExchangeShards exchanges shards for tickets
func (c *APIClient) ExchangeShards(rarity string, count int) (*ExchangeShardsResponse, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"rarity": rarity,
		"count":  count,
	})
	resp, err := c.doAuthorizedRequest("POST", "/tavern/exchange-shards", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec échange: %s", resp.Status)
	}

	var result ExchangeShardsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("échec parsing réponse: %v", err)
	}

	return &result, nil
}

// SetShipCrew sets the crew composition for a ship (dev only)
func (c *APIClient) SetShipCrew(shipID string, warriors, archers, gunners int) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"ship_id":  shipID,
		"warriors": warriors,
		"archers":  archers,
		"gunners":  gunners,
	})
	resp, err := c.doAuthorizedRequest("POST", "/dev/set-ship-crew", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("échec mise à jour équipage: %s", resp.Status)
	}

	return nil
}

// PVE Methods

// GetPveTargets returns the list of PVE targets for the player
func (c *APIClient) GetPveTargets() ([]domain.PveTarget, error) {
	resp, err := c.doAuthorizedRequest("GET", "/pve/targets", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec récupération cibles PVE: %s", resp.Status)
	}

	var response struct {
		Targets []domain.PveTarget `json:"targets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("erreur parsing cibles PVE: %v", err)
	}

	return response.Targets, nil
}

// EngagePve engages a PVE target with a fleet
func (c *APIClient) EngagePve(fleetID uuid.UUID, targetID string, seed *int64) (*domain.CombatResult, error) {
	reqBody := map[string]interface{}{
		"fleet_id":  fleetID.String(),
		"target_id": targetID,
	}
	if seed != nil {
		reqBody["seed"] = *seed
	}

	reqBodyBytes, _ := json.Marshal(reqBody)
	resp, err := c.doAuthorizedRequest("POST", "/pve/engage", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("session expirée, veuillez vous reconnecter: %s", errRes.Error)
		}
		return nil, fmt.Errorf("session expirée, veuillez vous reconnecter")
	}

	if resp.StatusCode == http.StatusConflict {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("flotte verrouillée")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec engagement PVE: %s", resp.Status)
	}

	var response struct {
		CombatResult domain.CombatResult `json:"combat_result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("erreur parsing résultat combat: %v", err)
	}

	return &response.CombatResult, nil
}

func (c *APIClient) AssignCrew(shipID uuid.UUID, warriors, archers, gunners int) error {
	types := []domain.UnitType{domain.Warrior, domain.Archer, domain.Gunner}
	qtys := []int{warriors, archers, gunners}

	for i, unitType := range types {
		qty := qtys[i]
		if qty <= 0 {
			continue
		}

		reqBody, _ := json.Marshal(map[string]interface{}{
			"ship_id":  shipID.String(),
			"type":     unitType,
			"quantity": qty,
		})
		resp, err := c.doAuthorizedRequest("POST", "/ship/militia/assign", bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errRes struct {
				Error string `json:"error"`
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
				return fmt.Errorf("%s: %s", unitType, errRes.Error)
			}
			return fmt.Errorf("échec assignation %s: %s", unitType, resp.Status)
		}
	}
	return nil
}

func (c *APIClient) UnassignCrew(shipID uuid.UUID, warriors, archers, gunners int) error {
	types := []domain.UnitType{domain.Warrior, domain.Archer, domain.Gunner}
	qtys := []int{warriors, archers, gunners}

	for i, unitType := range types {
		qty := qtys[i]
		if qty <= 0 {
			continue
		}

		reqBody, _ := json.Marshal(map[string]interface{}{
			"ship_id":  shipID.String(),
			"type":     unitType,
			"quantity": qty,
		})
		resp, err := c.doAuthorizedRequest("POST", "/ship/militia/unassign", bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errRes struct {
				Error string `json:"error"`
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
				return fmt.Errorf("%s: %s", unitType, errRes.Error)
			}
			return fmt.Errorf("échec retrait %s: %s", unitType, resp.Status)
		}
	}
	return nil
}

type RecruitResponse struct {
	DoneAt time.Time `json:"done_at"`
}

func (c *APIClient) MilitiaRecruit(islandID uuid.UUID, warriors, archers, gunners int) (*RecruitResponse, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"island_id": islandID.String(),
		"warriors":  warriors,
		"archers":   archers,
		"gunners":   gunners,
	})
	resp, err := c.doAuthorizedRequest("POST", "/ship/militia/recruit", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("échec recrutement: %s", resp.Status)
	}

	var res RecruitResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *APIClient) StationFleet(fleetID string, nodeID string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_id": fleetID,
		"node_id":  nodeID,
	})
	resp, err := c.doAuthorizedRequest("POST", "/fleets/station", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("station fleet failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) RecallFleet(fleetID string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_id": fleetID,
	})
	resp, err := c.doAuthorizedRequest("POST", "/fleets/recall", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("recall fleet failed: %s", resp.Status)
	}
	return nil
}

// NavigateFleet sends a fleet to arbitrary coordinates (Free Navigation)
func (c *APIClient) NavigateFleet(fleetID string, x, y int) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_id": fleetID,
		"target_x": x,
		"target_y": y,
	})

	resp, err := c.doAuthorizedRequest("POST", "/fleets/navigate", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("navigation failed: %s", resp.Status)
	}
	return nil
}

func (c *APIClient) GetWeather() (float64, time.Time, error) {
	resp, err := c.doAuthorizedRequest("GET", "/weather", nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, time.Time{}, fmt.Errorf("weather failed: %s", resp.Status)
	}

	var res struct {
		Direction  float64   `json:"direction"`
		NextChange time.Time `json:"next_change"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, time.Time{}, err
	}

	return res.Direction, res.NextChange, nil
}

func (c *APIClient) GetResourceNodes() ([]domain.ResourceNode, error) {
	resp, err := c.doAuthorizedRequest("GET", "/fleets/resource-nodes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get resource nodes failed: %s", resp.Status)
	}

	var res struct {
		Nodes []domain.ResourceNode `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Nodes, nil
}

func (c *APIClient) GetPvpTargets() ([]domain.PveTarget, error) {
	resp, err := c.doAuthorizedRequest("GET", "/pvp/targets", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pvp targets failed: %s", resp.Status)
	}

	var res struct {
		Targets []domain.PveTarget `json:"targets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Targets, nil
}

func (c *APIClient) SetActiveFleet(fleetID uuid.UUID) error {
	return fmt.Errorf("not implemented")
}

func (c *APIClient) DevGrantCaptain(playerID uuid.UUID, templateID string) error {
	return fmt.Errorf("not implemented")
}

func (c *APIClient) DevGrantTickets(playerID uuid.UUID, amount int) error {
	return fmt.Errorf("not implemented")
}

func (c *APIClient) DevSimulateEngagement(fleetAID string, tier int) (*domain.CombatResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// Cargo Transfer Response
type CargoTransferResponse struct {
	FleetCargo      map[domain.ResourceType]float64 `json:"fleet_cargo"`
	IslandResources map[domain.ResourceType]float64 `json:"island_resources"`
	CargoCapacity   float64                         `json:"cargo_capacity"`
	CargoUsed       float64                         `json:"cargo_used"`
	CargoFree       float64                         `json:"cargo_free"`
	Message         string                          `json:"message"`
}

// TransferToFleet transfers resources from Island to Fleet
func (c *APIClient) TransferToFleet(fleetID uuid.UUID, resource domain.ResourceType, amount float64) (*CargoTransferResponse, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_id": fleetID,
		"resource": resource,
		"amount":   amount,
	})

	url := fmt.Sprintf("/fleets/cargo/transfer-to-fleet?island_id=%s", c.IslandID)
	resp, err := c.doAuthorizedRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("session expirée")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("transfer failed: %s", resp.Status)
	}

	var res CargoTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	return &res, nil
}

// TransferToIsland transfers resources from Fleet to Island
func (c *APIClient) TransferToIsland(fleetID uuid.UUID, resource domain.ResourceType, amount float64) (*CargoTransferResponse, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"fleet_id": fleetID,
		"resource": resource,
		"amount":   amount,
	})

	url := fmt.Sprintf("/fleets/cargo/transfer-to-island?island_id=%s", c.IslandID)
	resp, err := c.doAuthorizedRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("session expirée")
	}

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf("%s", errRes.Error)
		}
		return nil, fmt.Errorf("transfer failed: %s", resp.Status)
	}

	var res CargoTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	return &res, nil
}

// StartIntercept initiates a PvP interception pursuit
func (c *APIClient) StartIntercept(attackerFleetID, targetFleetID string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"attacker_fleet_id": attackerFleetID,
		"target_fleet_id":   targetFleetID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/pvp/intercept/start", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf("%s", errRes.Error)
		}
		return fmt.Errorf("intercept failed: %s", resp.Status)
	}
	return nil
}

// AbortIntercept stops a PvP interception pursuit
func (c *APIClient) AbortIntercept(fleetID string) error {
	reqBody, _ := json.Marshal(map[string]string{
		"attacker_fleet_id": fleetID,
	})

	resp, err := c.doAuthorizedRequest("POST", "/pvp/intercept/abort", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("abort failed: %s", resp.Status)
	}
	return nil
}

// ==================== SOCIAL METHODS ====================

type PlayerSummary struct {
	ID       uuid.UUID  `json:"id"`
	Username string     `json:"username"`
	GuildID  *uuid.UUID `json:"guild_id,omitempty"`
}

type GuildInfo struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	OwnerID     uuid.UUID      `json:"owner_id"`
	Members     []PlayerSummary `json:"members"`
	MemberCount int            `json:"member_count"`
	CreatedAt   time.Time      `json:"created_at"`
}

type LeaderboardEntry struct {
	Rank     int       `json:"rank"`
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Reputation int    `json:"reputation"`
	Members  int       `json:"members,omitempty"`
	Username string    `json:"username,omitempty"`
}

// SearchFriends searches for players by username
func (c *APIClient) SearchFriends(query string) ([]PlayerSummary, error) {
	reqBody, _ := json.Marshal(map[string]string{"query": query})

	resp, err := c.doAuthorizedRequest("POST", "/social/friends/search", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: %s", resp.Status)
	}

	var result []PlayerSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// AddFriend adds a friend
func (c *APIClient) AddFriend(friendID uuid.UUID) error {
	reqBody, _ := json.Marshal(map[string]interface{}{"friend_id": friendID})

	resp, err := c.doAuthorizedRequest("POST", "/social/friends/add", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf(errRes.Error)
		}
		return fmt.Errorf("add friend failed: %s", resp.Status)
	}

	return nil
}

// RemoveFriend removes a friend
func (c *APIClient) RemoveFriend(friendID uuid.UUID) error {
	reqBody, _ := json.Marshal(map[string]interface{}{"friend_id": friendID})

	resp, err := c.doAuthorizedRequest("POST", "/social/friends/remove", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remove friend failed: %s", resp.Status)
	}

	return nil
}

// ListFriends returns the player's friend list
func (c *APIClient) ListFriends() ([]PlayerSummary, error) {
	resp, err := c.doAuthorizedRequest("GET", "/social/friends", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list friends failed: %s", resp.Status)
	}

	var result []PlayerSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// CreateGuild creates a new guild
func (c *APIClient) CreateGuild(name string) (*GuildInfo, error) {
	reqBody, _ := json.Marshal(map[string]string{"name": name})

	resp, err := c.doAuthorizedRequest("POST", "/guild/create", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf(errRes.Error)
		}
		return nil, fmt.Errorf("create guild failed: %s", resp.Status)
	}

	var result GuildInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetGuild returns the player's current guild info
func (c *APIClient) GetGuild() (*GuildInfo, error) {
	resp, err := c.doAuthorizedRequest("GET", "/guild/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err == nil && errRes.Error != "" {
			return nil, fmt.Errorf(errRes.Error)
		}
		return nil, fmt.Errorf("get guild failed: %s", resp.Status)
	}

	var result GuildInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// LeaveGuild leaves the current guild
func (c *APIClient) LeaveGuild() error {
	resp, err := c.doAuthorizedRequest("POST", "/guild/leave", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err == nil && errRes.Error != "" {
			return fmt.Errorf(errRes.Error)
		}
		return fmt.Errorf("leave guild failed: %s", resp.Status)
	}

	return nil
}

// GetLeaderboardPlayers returns the top players
func (c *APIClient) GetLeaderboardPlayers() ([]LeaderboardEntry, error) {
	resp, err := c.doAuthorizedRequest("GET", "/leaderboard/players", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get leaderboard failed: %s", resp.Status)
	}

	var result []LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetLeaderboardGuilds returns the top guilds
func (c *APIClient) GetLeaderboardGuilds() ([]LeaderboardEntry, error) {
	resp, err := c.doAuthorizedRequest("GET", "/leaderboard/guilds", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get leaderboard failed: %s", resp.Status)
	}

	var result []LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

