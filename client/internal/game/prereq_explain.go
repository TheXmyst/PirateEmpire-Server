package game

import (
	"fmt"
	"sort"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
)

// prereq_explain.go : dire au joueur exactement ce qui lui manque (quelle
// recherche, quel bâtiment à quel niveau), en cochant ce qui est déjà bon.
// Même règles que le serveur (economy/prereq_list.go).

// techInfo : la techno et l'onglet de l'arbre où elle se trouve.
func (g *Game) techInfo(id string) (*CTechnology, string) {
	if g.techUI == nil {
		return nil, ""
	}
	for _, tab := range techTabs {
		list := g.techUI.GetListForTab(tab)
		for i := range list {
			if list[i].ID == id {
				return &list[i], tab
			}
		}
	}
	return nil, ""
}

func (g *Game) techName(id string) string {
	if t, _ := g.techInfo(id); t != nil && t.Name != "" {
		return t.Name
	}
	return id
}

func (g *Game) techUnlocked(id string) bool {
	if g.player == nil {
		return false
	}
	for _, t := range g.player.UnlockedTechs {
		if t == id {
			return true
		}
	}
	return false
}

// finishedBuildingLevel : niveau du bâtiment (0 s'il n'est pas construit ou
// encore en chantier pour la première fois).
func (g *Game) finishedBuildingLevel(btype string) int {
	if g.player == nil || len(g.player.Islands) == 0 {
		return 0
	}
	lvl := 0
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type != btype {
			continue
		}
		l := b.Level
		if b.Constructing {
			l = b.Level - 1
		}
		if l > lvl {
			lvl = l
		}
	}
	return lvl
}

type prereqBuilder struct {
	g    *Game
	rows []domain.Requirement
}

func (p *prereqBuilder) building(btype string, lvl int) {
	if lvl <= 0 {
		return
	}
	for i := range p.rows {
		if p.rows[i].Kind == "building_level" && p.rows[i].ID == btype {
			if lvl > p.rows[i].Needed {
				p.rows[i].Needed = lvl
			}
			return
		}
	}
	p.rows = append(p.rows, domain.Requirement{Kind: "building_level", ID: btype, Name: buildingDisplayName(btype), Needed: lvl})
}

func (p *prereqBuilder) tech(id string) {
	if id == "" {
		return
	}
	name := p.g.techName(id)
	unlocked := p.g.techUnlocked(id)
	msg := fmt.Sprintf("Recherche « %s » (à l'Académie)", name)
	if !unlocked && p.g.player != nil && p.g.player.ResearchingTechID == id {
		msg = fmt.Sprintf("Recherche « %s » : en cours à l'Académie", name)
	}
	p.rows = append(p.rows, domain.Requirement{Kind: "tech", ID: id, Name: name, Message: msg, Met: unlocked})
	if unlocked {
		return
	}
	if t, _ := p.g.techInfo(id); t != nil {
		acad := t.ReqAcad
		if acad < 1 {
			acad = 1
		}
		p.building("Académie", acad)
		p.building("Hôtel de Ville", t.ReqTH)
	}
}

func (p *prereqBuilder) done() []domain.Requirement {
	for i := range p.rows {
		r := &p.rows[i]
		if r.Kind != "building_level" {
			continue
		}
		r.Current = p.g.finishedBuildingLevel(r.ID)
		r.Met = r.Current >= r.Needed
		if r.Current == 0 {
			r.Message = fmt.Sprintf("%s niveau %d (pas encore construit)", r.Name, r.Needed)
		} else {
			r.Message = fmt.Sprintf("%s niveau %d (tu as le niveau %d)", r.Name, r.Needed, r.Current)
		}
	}
	return p.rows
}

// shipPrerequisites : ce qu'il faut pour construire un navire.
func (g *Game) shipPrerequisites(shipyardLevel int, techID string) []domain.Requirement {
	p := &prereqBuilder{g: g}
	p.building("Chantier Naval", shipyardLevel)
	p.tech(techID)
	return p.done()
}

// buildingPrerequisites : ce qu'il faut pour construire un nouveau bâtiment
// (données du menu Construction ; le serveur reste l'arbitre).
func (g *Game) buildingPrerequisites(item ConstructionItem) []domain.Requirement {
	p := &prereqBuilder{g: g}
	p.building("Hôtel de Ville", item.ReqThLevel)
	if item.ReqBuilding != "" {
		lvl := item.ReqBuildingLvl
		if lvl < 1 {
			lvl = 1
		}
		p.building(item.ReqBuilding, lvl)
	}
	p.tech(item.ReqTech)
	return p.done()
}

// missingTechLine : « Recherche requise : X » si une techno manque, sinon "".
func (g *Game) missingTechLine(techID string) string {
	if techID == "" || g.techUnlocked(techID) {
		return ""
	}
	if g.player != nil && g.player.ResearchingTechID == techID {
		return fmt.Sprintf("Recherche en cours : %s", g.techName(techID))
	}
	return fmt.Sprintf("Recherche requise : %s", g.techName(techID))
}

// sortPrereqs : ce qui manque d'abord, ce qui est bon ensuite.
func sortPrereqs(reqs []domain.Requirement) {
	sort.SliceStable(reqs, func(i, j int) bool { return !reqs[i].Met && reqs[j].Met })
}

// firstMissingTech : la première recherche qui manque (pour le raccourci).
func firstMissingTech(reqs []domain.Requirement) string {
	for _, r := range reqs {
		if r.Kind == "tech" && !r.Met {
			return r.ID
		}
	}
	return ""
}

// goToAcademy ferme les fenêtres et ouvre l'arbre des recherches sur la
// bonne techno (ou le menu Construction s'il n'y a pas encore d'Académie).
func (g *Game) goToAcademy(techID string) {
	g.closePrereqModal()
	g.ui.ShowShipyard = false
	g.ui.ShowConstruction = false
	g.ui.ShowFleetUI = false
	if g.finishedBuildingLevel("Académie") == 0 {
		g.ui.ShowConstruction = true
		g.showError = true
		g.errorMessage = "Construis d'abord l'Académie pour lancer des recherches"
		g.errorDebounce = 150
		return
	}
	if g.techUI == nil {
		return
	}
	tech, tab := g.techInfo(techID)
	if tab != "" {
		g.techUI.SelectedTab = tab
	}
	// L'arbre cache les recherches pas encore accessibles : on dit pourquoi.
	if tech != nil {
		acad, th := g.finishedBuildingLevel("Académie"), g.finishedBuildingLevel("Hôtel de Ville")
		msg := ""
		switch {
		case tech.ReqAcad > acad:
			msg = fmt.Sprintf("Améliore l'Académie au niveau %d pour débloquer « %s »", tech.ReqAcad, tech.Name)
		case tech.ReqTH > th:
			msg = fmt.Sprintf("Améliore l'Hôtel de Ville au niveau %d pour débloquer « %s »", tech.ReqTH, tech.Name)
		}
		if msg != "" {
			g.showError = true
			g.errorMessage = msg
			g.errorDebounce = 240
		}
	}
	g.techUI.SelectedID = techID
	g.techUI.Visible = true
	g.ui.ShowTechUI = true
}

func (g *Game) hasBuilding(btype string) bool {
	if g.player == nil || len(g.player.Islands) == 0 {
		return false
	}
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type == btype {
			return true
		}
	}
	return false
}

func hasMissing(reqs []domain.Requirement) bool {
	for _, r := range reqs {
		if !r.Met {
			return true
		}
	}
	return false
}

// firstMissingLine : une phrase courte sur ce qui bloque (recherche d'abord).
func (g *Game) firstMissingLine(reqs []domain.Requirement) string {
	for _, r := range reqs {
		if r.Kind == "tech" && !r.Met {
			if g.player != nil && g.player.ResearchingTechID == r.ID {
				return "Recherche en cours : " + r.Name
			}
			return "Recherche requise : " + r.Name
		}
	}
	for _, r := range reqs {
		if !r.Met {
			return fmt.Sprintf("Requiert : %s niv. %d", r.Name, r.Needed)
		}
	}
	return ""
}
