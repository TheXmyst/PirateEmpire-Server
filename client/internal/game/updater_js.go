//go:build js

package game

import "errors"

// Dans le navigateur, il n'y a rien à mettre à jour : recharger la page
// suffit pour avoir la dernière version.

func CheckForUpdates(apiURL string) (bool, string, error) {
	return false, "", nil
}

func PerformUpdate(downloadURL string) error {
	return errors.New("mise à jour automatique indisponible dans le navigateur")
}
