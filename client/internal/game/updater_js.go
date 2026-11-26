//go:build js

package game

import "errors"

// Dans le navigateur, il n'y a rien à mettre à jour : recharger la page
// suffit pour avoir la dernière version.

type UpdateInfo struct {
	Version string
	URL     string
	SHA256  string
}

func CheckForUpdates(apiURL string) (bool, UpdateInfo, error) {
	return false, UpdateInfo{}, nil
}

func PerformUpdate(info UpdateInfo, progress func(float64)) error {
	return errors.New("mise à jour automatique indisponible dans le navigateur")
}

func CleanupOldUpdate() {}
