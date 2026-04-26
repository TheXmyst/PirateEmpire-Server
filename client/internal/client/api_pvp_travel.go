package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SendPvpAttack sends a fleet to attack an enemy island
func (c *APIClient) SendPvpAttack(fleetID, targetIslandID string) (travelTimeMinutes float64, distance float64, err error) {
	payload := map[string]string{
		"fleet_id":         fleetID,
		"target_island_id": targetIslandID,
	}

	payloadBytes, _ := json.Marshal(payload)
	resp, err := c.doAuthorizedRequest("POST", "/pvp/send-attack", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes struct {
			Error string `json:"error"`
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &errRes); err == nil && errRes.Error != "" {
			return 0, 0, fmt.Errorf("%s", errRes.Error)
		}
		return 0, 0, fmt.Errorf("échec envoi attaque: %s", resp.Status)
	}

	var response struct {
		TravelTimeMinutes float64 `json:"travel_time_minutes"`
		Distance          float64 `json:"distance"`
		Message           string  `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, 0, fmt.Errorf("erreur parsing résultat: %v", err)
	}

	return response.TravelTimeMinutes, response.Distance, nil
}

// SendPveAttack sends a fleet to chase a PvE target
func (c *APIClient) SendPveAttack(fleetID, targetPveID string) error {
	payload := map[string]string{
		"fleet_id":      fleetID,
		"target_pve_id": targetPveID,
	}

	payloadBytes, _ := json.Marshal(payload)
	resp, err := c.doAuthorizedRequest("POST", "/pve/attack", bytes.NewBuffer(payloadBytes))
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
		return fmt.Errorf("échec envoi attaque: %s", resp.Status)
	}

	return nil
}
