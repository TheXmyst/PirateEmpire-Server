package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
)

// Affichage de la récolte des flottes stationnées sur un spot de ressources.
// Les chiffres viennent du serveur (harvest_rate_per_hour, etc.), qui fait
// exactement le même calcul que la boucle de jeu.

var harvestResNames = map[domain.ResourceType]string{
	domain.Wood:  "Bois",
	domain.Stone: "Pierre",
	domain.Gold:  "Or",
	domain.Rum:   "Rhum",
}

func harvestResName(res domain.ResourceType) string {
	if n, ok := harvestResNames[res]; ok {
		return n
	}
	return string(res)
}

// fleetHarvestPerHour : total récolté par heure, en mer, pour cette ressource
// (toutes flottes confondues). Ces ressources vont dans la cale et n'arrivent
// sur l'île qu'au retour de la flotte.
func (g *Game) fleetHarvestPerHour(res domain.ResourceType) (total float64, fleets int) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return 0, 0
	}
	for _, f := range g.player.Islands[0].Fleets {
		if f.HarvestRatePerHour > 0 && domain.ResourceType(f.StoredResource) == res {
			total += f.HarvestRatePerHour
			fleets++
		}
	}
	return total, fleets
}

// fmtMult : 1.5 -> "x1,5" (format français, deux décimales si besoin).
func fmtMult(m float64) string {
	s := fmt.Sprintf("%.2f", m)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return "x" + strings.ReplaceAll(s, ".", ",")
}

// fmtDurationShort : 2280 s -> "38 min", 5400 s -> "1 h 30".
func fmtDurationShort(sec float64) string {
	m := int(math.Ceil(sec / 60))
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d h %02d", m/60, m%60)
}

// fleetHarvestLine : « Récolte Or x1,5 : +612/h (durée x1,13) · pleine dans ~38 min ».
// Vide si la flotte ne récolte pas.
func fleetHarvestLine(f domain.Fleet) string {
	if f.HarvestRatePerHour <= 0 {
		return ""
	}
	res := domain.ResourceType(f.StoredResource)
	line := fmt.Sprintf("Récolte %s %s : +%s/h", harvestResName(res), fmtMult(f.NodeRichness), formatThousands(f.HarvestRatePerHour))
	if f.HarvestDurationMult > 1.005 {
		line += fmt.Sprintf(" (durée %s)", fmtMult(f.HarvestDurationMult))
	}
	if f.HarvestFullInSec > 0 {
		line += " · pleine dans ~" + fmtDurationShort(f.HarvestFullInSec)
	}
	return line
}
