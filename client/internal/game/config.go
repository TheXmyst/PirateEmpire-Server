package game

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
)

// ClientVersion : version du jeu. Les builds publiés la reçoivent à la
// compilation (deploy/update.sh : -ldflags "-X ...ClientVersion=<commit>").
// "dev" = compilé à la main : pas de mise à jour automatique.
var ClientVersion = "dev"

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

	// PC : lit config.json. Navigateur : utilise l'adresse du site.
	loadPlatformConfig(&cfg)
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}

	if err := client.CheckSecureURL(cfg.APIURL); err != nil {
		fmt.Printf("[SECURITE] Attention : %v\n", err)
	}

	return cfg
}
