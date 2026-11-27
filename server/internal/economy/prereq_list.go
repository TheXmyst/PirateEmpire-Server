package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// ReqStatus : une ligne de la liste des prérequis montrée au joueur
// (fenêtre « Prérequis manquants »), remplie ou non.
type ReqStatus struct {
	Kind    string `json:"kind"` // "building_level" ou "tech"
	ID      string `json:"id"`   // type de bâtiment ou identifiant de la techno
	Name    string `json:"name"`
	Message string `json:"message"`
	Needed  int    `json:"needed,omitempty"`
	Current int    `json:"current,omitempty"`
	Met     bool   `json:"met"`
}

// finishedLevels : niveau de chaque bâtiment terminé de la première île.
func finishedLevels(player *domain.Player) map[string]int {
	levels := map[string]int{}
	if len(player.Islands) > 0 {
		for _, b := range player.Islands[0].Buildings {
			if !b.Constructing && b.Level > levels[b.Type] {
				levels[b.Type] = b.Level
			}
		}
	}
	return levels
}

type reqList struct {
	rows   []ReqStatus
	levels map[string]int
}

// needBuilding ajoute (ou relève) une ligne « bâtiment niveau N ».
func (l *reqList) needBuilding(btype string, lvl int) {
	if lvl <= 0 {
		return
	}
	for i := range l.rows {
		if l.rows[i].Kind == "building_level" && l.rows[i].ID == btype {
			if lvl > l.rows[i].Needed {
				l.rows[i].Needed = lvl
			}
			return
		}
	}
	l.rows = append(l.rows, ReqStatus{Kind: "building_level", ID: btype, Name: btype, Needed: lvl})
}

func (l *reqList) needTech(player *domain.Player, techID string) {
	for _, r := range l.rows {
		if r.Kind == "tech" && r.ID == techID {
			return
		}
	}
	unlocked := false
	for _, t := range player.UnlockedTechs {
		if t == techID {
			unlocked = true
			break
		}
	}
	name := TechNameFR(techID)
	msg := fmt.Sprintf("Recherche « %s » (à l'Académie)", name)
	if !unlocked && player.ResearchingTechID == techID {
		msg = fmt.Sprintf("Recherche « %s » : en cours à l'Académie", name)
	}
	l.rows = append(l.rows, ReqStatus{Kind: "tech", ID: techID, Name: name, Message: msg, Met: unlocked})
	if unlocked {
		return
	}
	// Ce qu'il faut pour pouvoir lancer cette recherche
	if t, err := GetTech(techID); err == nil && t != nil {
		acad := t.ReqAcad
		if acad < 1 {
			acad = 1
		}
		l.needBuilding("Académie", acad)
		l.needBuilding("Hôtel de Ville", t.ReqTH)
	}
}

func (l *reqList) finish() []ReqStatus {
	for i := range l.rows {
		r := &l.rows[i]
		if r.Kind != "building_level" {
			continue
		}
		r.Current = l.levels[r.ID]
		r.Met = r.Current >= r.Needed
		if r.Current == 0 {
			r.Message = fmt.Sprintf("%s niveau %d (pas encore construit)", r.Name, r.Needed)
		} else {
			r.Message = fmt.Sprintf("%s niveau %d (tu as le niveau %d)", r.Name, r.Needed, r.Current)
		}
	}
	return l.rows
}

// ListBuildingPrerequisites : tous les prérequis pour construire / monter un
// bâtiment au niveau voulu, remplis ou non (dans l'ordre : techno d'abord).
func ListBuildingPrerequisites(player *domain.Player, buildingType string, targetLevel int) []ReqStatus {
	l := &reqList{levels: finishedLevels(player)}
	cfg, ok := GetBuildingConfig(buildingType)
	if !ok {
		return nil
	}
	if buildingType != "Hôtel de Ville" {
		th := l.levels["Hôtel de Ville"]
		if targetLevel > GetMaxLevelAllowedByTH(th) {
			l.needBuilding("Hôtel de Ville", RequiredTHForLevel(targetLevel))
		}
	}
	var techs []string
	for _, req := range cfg.Prerequisites {
		if targetLevel < req.Level {
			continue
		}
		l.needBuilding("Hôtel de Ville", req.ReqTownHall)
		if req.ReqBuilding != "" {
			minLvl := req.ReqMinLevel
			if minLvl < 1 {
				minLvl = 1
			}
			l.needBuilding(req.ReqBuilding, minLvl)
		}
		if req.ReqTech != "" {
			techs = append(techs, req.ReqTech)
		}
	}
	for _, t := range techs {
		l.needTech(player, t)
	}
	return l.finish()
}

// ListShipPrerequisites : prérequis pour construire un type de navire.
func ListShipPrerequisites(player *domain.Player, shipyardLevel int, techID string) []ReqStatus {
	l := &reqList{levels: finishedLevels(player)}
	l.needBuilding("Chantier Naval", shipyardLevel)
	if techID != "" {
		l.needTech(player, techID)
	}
	return l.finish()
}
