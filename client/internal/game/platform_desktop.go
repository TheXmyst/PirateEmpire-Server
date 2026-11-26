//go:build !js

package game

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

// platform_desktop.go : tout ce qui change entre la version PC et la version
// navigateur (voir platform_js.go pour l'autre moitié).
//
// Sur PC, toutes les ressources (images, musiques...) sont collées dans le .exe.

//go:embed resources
var assetsFS embed.FS

// IsWebBuild vaut true seulement dans la version navigateur.
const IsWebBuild = false

// readAsset lit une ressource du jeu (chemin "resources/...").
func readAsset(path string) ([]byte, error) {
	return fs.ReadFile(assetsFS, path)
}

// readUserFile lit un petit fichier sauvegardé par le joueur (identifiants,
// historique du chat...). Sur PC c'est un vrai fichier à côté du jeu.
func readUserFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// writeUserFile enregistre un petit fichier du joueur.
func writeUserFile(name string, data []byte) error {
	if dir := filepath.Dir(name); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(name, data, 0o644)
}

// loadPlatformConfig lit config.json à côté du jeu, s'il existe.
func loadPlatformConfig(cfg *Config) {
	file, err := os.Open("config.json")
	if err != nil {
		return // pas de fichier : on garde l'adresse par défaut
	}
	defer file.Close()
	_ = json.NewDecoder(file).Decode(cfg)
}
