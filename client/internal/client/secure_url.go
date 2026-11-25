package client

import (
	"fmt"
	"net/url"
)

// CheckSecureURL renvoie une erreur si l'adresse n'est pas en HTTPS.
//
// En HTTP, tout circule en clair : mot de passe, jeton de connexion... et
// n'importe qui sur le même réseau (wifi public, box...) peut les lire ou les
// modifier. Le HTTP reste accepté pour un serveur sur ta propre machine
// (localhost), pratique pour développer.
func CheckSecureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("adresse invalide %q: %w", raw, err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLocalHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("%q n'est pas en HTTPS, les échanges ne sont pas chiffrés", raw)
}

func isLocalHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}
