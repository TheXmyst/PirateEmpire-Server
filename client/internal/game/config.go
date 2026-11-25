package game

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
)

const ClientVersion = "1.0.0" // Manually increment this for each release

// DefaultAPIURL est l'adresse utilisée si config.json est absent : le serveur en ligne.
// Pour développer en local, mettre "http://localhost:8080" dans config.json.
const DefaultAPIURL = "https://seadogs.duckdns.org"

// Config holds the client configuration
type Config struct {
	APIURL string `json:"api_url"`
}

// LoadConfig loads the configuration from config.json
// Returns a default config if the file doesn't exist or errors
func LoadConfig() Config {
	cfg := Config{
		APIURL: DefaultAPIURL, // Default
	}

	file, err := os.Open("config.json")
	if err != nil {
		// File doesn't exist or can't be opened, return default
		return cfg
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		// Failed to decode, return default (or partially loaded)
		return cfg
	}

	if err := client.CheckSecureURL(cfg.APIURL); err != nil {
		fmt.Printf("[SECURITE] Attention : %v\n", err)
	}

	return cfg
}
