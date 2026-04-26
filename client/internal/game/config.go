package game

import (
	"encoding/json"
	"os"
)

const ClientVersion = "1.0.0" // Manually increment this for each release

// DefaultAPIURL is the fallback URL if config is missing
const DefaultAPIURL = "http://localhost:8080"

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

	return cfg
}
