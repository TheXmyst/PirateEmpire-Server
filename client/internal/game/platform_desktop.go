//go:build !js

package game

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// userFilePath : les fichiers du joueur vont à côté du .exe, même si le jeu
// est lancé depuis un autre dossier (raccourci, relance après mise à jour).
func userFilePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			dir := filepath.Dir(exe)
			// "go run" compile dans un dossier temporaire : là, on reste
			// dans le dossier courant.
			if !strings.HasPrefix(dir, os.TempDir()) {
				return filepath.Join(dir, name)
			}
		}
	}
	return name
}

// readUserFile lit un petit fichier sauvegardé par le joueur (session
// mémorisée, historique du chat...). Sur PC c'est un vrai fichier à côté du jeu.
func readUserFile(name string) ([]byte, error) {
	data, err := os.ReadFile(userFilePath(name))
	if err != nil && userFilePath(name) != name {
		// Ancien emplacement (dossier courant).
		return os.ReadFile(name)
	}
	return data, err
}

// removeUserFile efface un fichier du joueur (s'il existe).
func removeUserFile(name string) error {
	err := os.Remove(userFilePath(name))
	if p := userFilePath(name); p != name {
		_ = os.Remove(name)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// writeUserFile enregistre un petit fichier du joueur.
func writeUserFile(name string, data []byte) error {
	name = userFilePath(name)
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
