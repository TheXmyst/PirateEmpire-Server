package economy

import "github.com/TheXmyst/Sea-Dogs/server/internal/domain"

// Petits outils pour écrire des messages d'erreur lisibles par les joueurs.

// ResourceNameFR : nom d'une ressource en français, précédé de « de » ou « d' ».
func ResourceNameFR(res domain.ResourceType) string {
	switch res {
	case domain.Wood:
		return "de bois"
	case domain.Stone:
		return "de pierre"
	case domain.Gold:
		return "d'or"
	case domain.Rum:
		return "de rhum"
	}
	return "de " + string(res)
}

// TechNameFR : nom affiché d'une technologie (son identifiant si inconnue).
func TechNameFR(id string) string {
	if t, err := GetTech(id); err == nil && t != nil && t.Name != "" {
		return t.Name
	}
	return id
}

// RequiredTHForLevel : niveau d'Hôtel de Ville qui autorise un bâtiment au niveau voulu.
func RequiredTHForLevel(target int) int {
	for th := 1; th <= 30; th++ {
		if GetMaxLevelAllowedByTH(th) >= target {
			return th
		}
	}
	return 30
}
