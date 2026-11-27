package economy

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
)

// =====================================================================
// Quêtes de progression
//
// Trois familles :
//   - "story"     (Histoire)  : le fil rouge, une quête à la fois, dans l'ordre ;
//   - "building"  (Bâtiments) : une quête par niveau de chaque bâtiment,
//                                générée à partir de configs/buildings.json ;
//   - "challenge" (Défis)     : objectifs annexes, débloqués par l'Hôtel de
//                                Ville ou par une autre quête.
//
// Le fichier configs/quests.json contient les quêtes Histoire et Défis et la
// règle de remboursement des quêtes Bâtiments. Ce fichier ne fait que DÉCRIRE
// les quêtes : la progression est calculée à partir de l'état réel du joueur
// (QuestPlayerState), donc une quête ne peut pas être « ratée ».
// =====================================================================

// Familles de quêtes.
const (
	QuestFamilyStory     = "story"
	QuestFamilyBuilding  = "building"
	QuestFamilyChallenge = "challenge"
)

// Compteurs d'événements (colonnes quest_<nom> de la table players).
const (
	QuestCounterCaptainPulls     = "captain_pulls"
	QuestCounterMilitiaRecruited = "militia_recruited"
	QuestCounterFleetsSent       = "fleets_sent"
	QuestCounterFleetsStationed  = "fleets_stationed"
	QuestCounterChatMessages     = "chat_messages"
	QuestCounterHarvested        = "harvested"
)

var knownQuestCounters = map[string]bool{
	QuestCounterCaptainPulls:     true,
	QuestCounterMilitiaRecruited: true,
	QuestCounterFleetsSent:       true,
	QuestCounterFleetsStationed:  true,
	QuestCounterChatMessages:     true,
	QuestCounterHarvested:        true,
}

// Types d'objectifs reconnus.
var knownObjectiveTypes = map[string]bool{
	"building_level":    true, // building + level
	"buildings_level":   true, // buildings (ou ["all"]) + level
	"buildings_built":   true, // count bâtiments construits (Hôtel de Ville compris)
	"tech_count":        true, // count recherches terminées
	"tech":              true, // tech (identifiant)
	"ships_owned":       true, // count navires terminés (ship_type optionnel)
	"fleet_size":        true, // count navires dans une même flotte
	"ship_crewed":       true, // count navires avec capitaine + matelots
	"militia_all_types": true, // au moins un matelot de chaque type
	"captains_owned":    true, // count capitaines
	"captain_rarity":    true, // count capitaines de rareté >= rarity
	"captain_stars":     true, // un capitaine avec au moins count étoiles
	"pve_wins":          true, // count victoires PvE (tier optionnel, 0 = tous)
	"counter":           true, // counter + count
	"guild":             true, // membre d'une guilde
}

// QuestReward : ce que rapporte une quête.
type QuestReward struct {
	Wood    float64 `json:"wood,omitempty"`
	Stone   float64 `json:"stone,omitempty"`
	Gold    float64 `json:"gold,omitempty"`
	Rum     float64 `json:"rum,omitempty"`
	Tickets int     `json:"tickets,omitempty"`
}

// Resources renvoie la récompense sous forme de ressources de l'île.
func (r QuestReward) Resources() map[domain.ResourceType]float64 {
	out := map[domain.ResourceType]float64{}
	add := func(res domain.ResourceType, v float64) {
		if v > 0 {
			out[res] = v
		}
	}
	add(domain.Wood, r.Wood)
	add(domain.Stone, r.Stone)
	add(domain.Gold, r.Gold)
	add(domain.Rum, r.Rum)
	if r.Tickets > 0 {
		out[domain.CaptainTicket] = float64(r.Tickets)
	}
	return out
}

// Add additionne deux récompenses.
func (r QuestReward) Add(o QuestReward) QuestReward {
	return QuestReward{r.Wood + o.Wood, r.Stone + o.Stone, r.Gold + o.Gold, r.Rum + o.Rum, r.Tickets + o.Tickets}
}

// QuestObjective : ce qu'il faut accomplir.
type QuestObjective struct {
	Type      string   `json:"type"`
	Counter   string   `json:"counter,omitempty"`
	Building  string   `json:"building,omitempty"`
	Buildings []string `json:"buildings,omitempty"`
	Level     int      `json:"level,omitempty"`
	Count     int      `json:"count,omitempty"`
	Tier      int      `json:"tier,omitempty"`
	Tech      string   `json:"tech,omitempty"`
	Rarity    string   `json:"rarity,omitempty"`
	ShipType  string   `json:"ship_type,omitempty"`
}

// QuestDef : une quête telle que décrite dans la configuration.
type QuestDef struct {
	ID          string         `json:"id"`
	Family      string         `json:"family"`
	Phase       int            `json:"phase"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Tutorial    string         `json:"tutorial,omitempty"`
	MinTownHall int            `json:"min_townhall,omitempty"`
	After       string         `json:"after,omitempty"`
	Objective   QuestObjective `json:"objective"`
	Reward      QuestReward    `json:"reward"`
}

// BuildingQuestRules : règle de génération des quêtes Bâtiments.
type BuildingQuestRules struct {
	MaxLevel              int      `json:"max_level"`
	EarlyMaxLevel         int      `json:"early_max_level"`
	TownHallEarlyMaxLevel int      `json:"townhall_early_max_level"`
	EarlyRefund           float64  `json:"early_refund"`
	LateRefund            float64  `json:"late_refund"`
	Order                 []string `json:"order"`
}

type questFile struct {
	BuildingQuests BuildingQuestRules `json:"building_quests"`
	Quests         []QuestDef         `json:"quests"`
}

var (
	questMu    sync.RWMutex
	questDefs  []QuestDef // ordre : Histoire, Bâtiments, Défis
	questByID  map[string]int
	questRules BuildingQuestRules
)

// buildingDisplayNames : noms affichés des bâtiments dont l'identifiant
// interne est resté en anglais.
var buildingDisplayNames = map[string]string{
	"Tavern":    "Taverne",
	"Infirmary": "Infirmerie",
}

// BuildingDisplayName renvoie le nom d'un bâtiment tel que le joueur le voit.
func BuildingDisplayName(t string) string {
	if n, ok := buildingDisplayNames[t]; ok {
		return n
	}
	return t
}

// LoadQuestConfig lit configs/quests.json. La configuration des bâtiments
// (LoadConfig) doit être chargée avant : les quêtes Bâtiments en dépendent.
func LoadQuestConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read quest config: %w", err)
	}
	return LoadQuestConfigData(data)
}

// LoadQuestConfigData : comme LoadQuestConfig, à partir du contenu du fichier.
func LoadQuestConfigData(data []byte) error {
	var f questFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("failed to parse quest config: %w", err)
	}
	defs, err := buildQuestDefs(f)
	if err != nil {
		return err
	}
	byID := make(map[string]int, len(defs))
	for i, d := range defs {
		byID[d.ID] = i
	}
	questMu.Lock()
	questDefs, questByID, questRules = defs, byID, f.BuildingQuests
	questMu.Unlock()
	logger.Info("Quest System: Config Loaded", "quests", len(defs))
	return nil
}

func buildQuestDefs(f questFile) ([]QuestDef, error) {
	var story, challenges []QuestDef
	for _, q := range f.Quests {
		switch q.Family {
		case QuestFamilyStory:
			story = append(story, q)
		case QuestFamilyChallenge:
			challenges = append(challenges, q)
		default:
			return nil, fmt.Errorf("quest %q: unknown family %q", q.ID, q.Family)
		}
	}
	buildings, err := generateBuildingQuests(f.BuildingQuests)
	if err != nil {
		return nil, err
	}
	defs := append(append(story, buildings...), challenges...)

	seen := map[string]bool{}
	for _, d := range defs {
		if d.ID == "" {
			return nil, fmt.Errorf("quest without id: %q", d.Title)
		}
		if seen[d.ID] {
			return nil, fmt.Errorf("duplicate quest id %q", d.ID)
		}
		seen[d.ID] = true
		o := d.Objective
		if !knownObjectiveTypes[o.Type] {
			return nil, fmt.Errorf("quest %q: unknown objective type %q", d.ID, o.Type)
		}
		if o.Type == "counter" && !knownQuestCounters[o.Counter] {
			return nil, fmt.Errorf("quest %q: unknown counter %q", d.ID, o.Counter)
		}
	}
	for _, d := range defs {
		if d.After != "" && !seen[d.After] {
			return nil, fmt.Errorf("quest %q: unknown 'after' quest %q", d.ID, d.After)
		}
	}
	return defs, nil
}

// generateBuildingQuests : une quête par niveau de chaque bâtiment.
// Récompense = un pourcentage du coût du niveau (arrondi à la dizaine).
func generateBuildingQuests(r BuildingQuestRules) ([]QuestDef, error) {
	var defs []QuestDef
	for _, btype := range r.Order {
		cfg, ok := GetBuildingConfig(btype)
		if !ok {
			return nil, fmt.Errorf("building quests: unknown building %q (buildings.json loaded?)", btype)
		}
		earlyMax := r.EarlyMaxLevel
		if btype == "Hôtel de Ville" {
			earlyMax = r.TownHallEarlyMaxLevel
		}
		for lvl := 1; lvl <= r.MaxLevel; lvl++ {
			rate, phase := r.EarlyRefund, 1
			if lvl > earlyMax {
				rate, phase = r.LateRefund, 2
			}
			cost := ComputeBuildingCost(cfg, lvl)
			rw := QuestReward{
				Wood:  roundTo10(cost[domain.Wood] * rate),
				Stone: roundTo10(cost[domain.Stone] * rate),
				Gold:  roundTo10(cost[domain.Gold] * rate),
				Rum:   roundTo10(cost[domain.Rum] * rate),
			}
			name := BuildingDisplayName(btype)
			title := fmt.Sprintf("%s niveau %d", name, lvl)
			desc := fmt.Sprintf("Monte le bâtiment %s au niveau %d.", name, lvl)
			if lvl == 1 {
				title = "Construire : " + name
				desc = fmt.Sprintf("Construis le bâtiment %s.", name)
			}
			defs = append(defs, QuestDef{
				ID:          buildingQuestID(btype, lvl),
				Family:      QuestFamilyBuilding,
				Phase:       phase,
				Title:       title,
				Description: desc,
				Objective:   QuestObjective{Type: "building_level", Building: btype, Level: lvl},
				Reward:      rw,
			})
		}
	}
	return defs, nil
}

func buildingQuestID(btype string, lvl int) string {
	return fmt.Sprintf("bld:%s:%d", btype, lvl)
}

func roundTo10(v float64) float64 { return math.Round(v/10) * 10 }

// QuestDefs renvoie toutes les quêtes (copie).
func QuestDefs() []QuestDef {
	questMu.RLock()
	defer questMu.RUnlock()
	return append([]QuestDef(nil), questDefs...)
}

// GetQuestDef renvoie une quête par son identifiant.
func GetQuestDef(id string) (QuestDef, bool) {
	questMu.RLock()
	defer questMu.RUnlock()
	i, ok := questByID[id]
	if !ok {
		return QuestDef{}, false
	}
	return questDefs[i], true
}

// =====================================================================
// État du joueur et évaluation
// =====================================================================

// QuestPlayerState : tout ce dont l'évaluation a besoin, déjà chargé.
type QuestPlayerState struct {
	Buildings []domain.Building
	Techs     []string
	Ships     []domain.Ship
	Fleets    []domain.Fleet
	Captains  []domain.Captain
	// MilitiaStock : matelots en réserve sur l'île.
	MilitiaStock map[domain.UnitType]int
	// PveWins : victoires PvE par palier.
	PveWins  map[int]int
	Counters map[string]float64
	InGuild  bool
	Claimed  map[string]bool
}

// QuestStatus : une quête telle qu'envoyée au client.
type QuestStatus struct {
	ID          string      `json:"id"`
	Family      string      `json:"family"`
	Phase       int         `json:"phase"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Tutorial    string      `json:"tutorial,omitempty"`
	Progress    float64     `json:"progress"`
	Target      float64     `json:"target"`
	Done        bool        `json:"done"`
	Locked      bool        `json:"locked,omitempty"`
	Hint        string      `json:"hint,omitempty"`
	Reward      QuestReward `json:"reward"`
}

// QuestFamilySummary : avancement d'une famille (pour le journal).
type QuestFamilySummary struct {
	Claimed int `json:"claimed"`
	Total   int `json:"total"`
}

func (s *QuestPlayerState) levels() map[string]int {
	lv := map[string]int{}
	for _, b := range s.Buildings {
		if b.Level > lv[b.Type] {
			lv[b.Type] = b.Level // Level = dernier niveau terminé
		}
	}
	return lv
}

func (s *QuestPlayerState) townHall() int { return s.levels()["Hôtel de Ville"] }

// shipFinished : un navire terminé et encore à flot.
func shipFinished(sh domain.Ship) bool {
	return sh.State != "UnderConstruction" && sh.State != "Destroyed" && sh.State != "Sunk"
}

// QuestProgress renvoie l'avancement et l'objectif chiffré d'une quête.
func QuestProgress(d QuestDef, s *QuestPlayerState) (progress, target float64) {
	o := d.Objective
	count := float64(o.Count)
	if count <= 0 {
		count = 1
	}
	levels := s.levels()
	switch o.Type {
	case "building_level":
		return math.Min(float64(levels[o.Building]), float64(o.Level)), float64(o.Level)
	case "buildings_level":
		names := o.Buildings
		if len(names) == 1 && names[0] == "all" {
			questMu.RLock()
			names = questRules.Order
			questMu.RUnlock()
		}
		ok := 0
		for _, n := range names {
			if levels[n] >= o.Level {
				ok++
			}
		}
		return float64(ok), float64(len(names))
	case "buildings_built":
		n := 0
		for _, l := range levels {
			if l >= 1 {
				n++
			}
		}
		return math.Min(float64(n), count), count
	case "tech_count":
		return math.Min(float64(len(s.Techs)), count), count
	case "tech":
		for _, t := range s.Techs {
			if t == o.Tech {
				return 1, 1
			}
		}
		return 0, 1
	case "ships_owned":
		n := 0
		for _, sh := range s.Ships {
			if shipFinished(sh) && (o.ShipType == "" || sh.Type == o.ShipType) {
				n++
			}
		}
		return math.Min(float64(n), count), count
	case "fleet_size":
		best := 0
		for _, f := range s.Fleets {
			n := 0
			for _, sh := range f.Ships {
				if shipFinished(sh) {
					n++
				}
			}
			if n > best {
				best = n
			}
		}
		return math.Min(float64(best), count), count
	case "ship_crewed":
		hasCaptain := map[string]bool{}
		for _, c := range s.Captains {
			if c.AssignedShipID != nil {
				hasCaptain[c.AssignedShipID.String()] = true
			}
		}
		n := 0
		for _, sh := range s.Ships {
			crew := sh.MilitiaWarriors + sh.MilitiaArchers + sh.MilitiaGunners
			if crew > 0 && (sh.CaptainID != nil || hasCaptain[sh.ID.String()]) {
				n++
			}
		}
		return math.Min(float64(n), count), count
	case "militia_all_types":
		w, a, g := s.MilitiaStock[domain.Warrior], s.MilitiaStock[domain.Archer], s.MilitiaStock[domain.Gunner]
		for _, sh := range s.Ships {
			w += sh.MilitiaWarriors
			a += sh.MilitiaArchers
			g += sh.MilitiaGunners
		}
		n := 0
		for _, v := range []int{w, a, g} {
			if v > 0 {
				n++
			}
		}
		return float64(n), 3
	case "captains_owned":
		return math.Min(float64(len(s.Captains)), count), count
	case "captain_rarity":
		n := 0
		for _, c := range s.Captains {
			if rarityRank(string(c.Rarity)) >= rarityRank(o.Rarity) {
				n++
			}
		}
		return math.Min(float64(n), count), count
	case "captain_stars":
		best := 0
		for _, c := range s.Captains {
			if c.Stars > best {
				best = c.Stars
			}
		}
		return math.Min(float64(best), count), count
	case "pve_wins":
		n := 0
		for tier, wins := range s.PveWins {
			if o.Tier == 0 || tier == o.Tier {
				n += wins
			}
		}
		return math.Min(float64(n), count), count
	case "counter":
		return math.Min(math.Floor(s.Counters[o.Counter]), count), count
	case "guild":
		if s.InGuild {
			return 1, 1
		}
		return 0, 1
	}
	return 0, 1
}

func rarityRank(r string) int {
	switch r {
	case string(domain.RarityUltra):
		return 4
	case string(domain.RarityLegendary):
		return 3
	case string(domain.RarityRare):
		return 2
	case string(domain.RarityCommon):
		return 1
	}
	return 0
}

// QuestDone : l'objectif est-il atteint ?
func QuestDone(d QuestDef, s *QuestPlayerState) bool {
	p, t := QuestProgress(d, s)
	return p >= t
}

// QuestUnlocked : la quête est-elle proposée au joueur ? (Histoire : la
// première non réclamée ; Bâtiments : niveaux atteints et niveau suivant ;
// Défis : Hôtel de Ville et quête précédente.)
func QuestUnlocked(d QuestDef, s *QuestPlayerState) bool {
	if s.Claimed[d.ID] {
		return false
	}
	switch d.Family {
	case QuestFamilyStory:
		for _, q := range QuestDefs() {
			if q.Family == QuestFamilyStory && !s.Claimed[q.ID] {
				return q.ID == d.ID
			}
		}
		return false
	case QuestFamilyBuilding:
		lvl := s.levels()[d.Objective.Building]
		return d.Objective.Level <= lvl+1
	default:
		if d.MinTownHall > 0 && s.townHall() < d.MinTownHall {
			return false
		}
		if d.After != "" && !s.Claimed[d.After] {
			return false
		}
		return true
	}
}

// EvaluateQuests renvoie les quêtes proposées au joueur, avec leur avancement,
// et l'avancement de chaque famille.
func EvaluateQuests(s *QuestPlayerState) ([]QuestStatus, map[string]QuestFamilySummary) {
	defs := QuestDefs()
	summary := map[string]QuestFamilySummary{}
	var out []QuestStatus
	// Pour les indices « il te manque… » des quêtes Bâtiments verrouillées.
	hintPlayer := &domain.Player{UnlockedTechs: s.Techs, Islands: []domain.Island{{Buildings: s.Buildings}}}
	levels := s.levels()
	for _, d := range defs {
		fs := summary[d.Family]
		fs.Total++
		if s.Claimed[d.ID] {
			fs.Claimed++
		}
		summary[d.Family] = fs
		if !QuestUnlocked(d, s) {
			continue
		}
		p, t := QuestProgress(d, s)
		st := QuestStatus{
			ID: d.ID, Family: d.Family, Phase: d.Phase, Title: d.Title,
			Description: d.Description, Tutorial: d.Tutorial,
			Progress: p, Target: t, Done: p >= t, Reward: d.Reward,
		}
		if d.Family == QuestFamilyBuilding && !st.Done && d.Objective.Level == levels[d.Objective.Building]+1 {
			for _, r := range ListBuildingPrerequisites(hintPlayer, d.Objective.Building, d.Objective.Level) {
				if !r.Met {
					st.Locked = true
					st.Hint = r.Message
					break
				}
			}
		}
		out = append(out, st)
	}
	return out, summary
}

// ClaimableQuests renvoie, dans l'ordre, les quêtes terminées et pas encore
// réclamées.
func ClaimableQuests(s *QuestPlayerState) []QuestDef {
	var out []QuestDef
	for _, d := range QuestDefs() {
		if QuestUnlocked(d, s) && QuestDone(d, s) {
			out = append(out, d)
		}
	}
	return out
}

// =====================================================================
// Capacité de stockage (pour ne pas perdre une récompense)
// =====================================================================

// BaseStorageLimits : capacité d'une île sans Entrepôt (même valeurs que le
// moteur, voir engine.calculateResources).
var BaseStorageLimits = map[domain.ResourceType]float64{
	domain.Wood:  5000.0,
	domain.Stone: 5000.0,
	domain.Rum:   3000.0,
	domain.Gold:  10000.0,
}

// ComputeStorageLimits calcule la capacité de stockage d'une île comme le fait
// le moteur : capacité de base, relevée par les bâtiments terminés qui stockent
// (Entrepôt), bonus de recherche compris.
func ComputeStorageLimits(buildings []domain.Building, techs []string) map[domain.ResourceType]float64 {
	mods := ComputeTechModifiers(techs)
	limits := map[domain.ResourceType]float64{}
	for k, v := range BaseStorageLimits {
		limits[k] = v
	}
	for _, b := range buildings {
		if b.Constructing || b.Level < 1 {
			continue
		}
		stats, err := GetBuildingStats(b.Type, b.Level)
		if err != nil {
			continue
		}
		for res, amount := range stats.Storage {
			v := math.Round(amount * (1.0 + mods.StorageCapacityMultiplier[res]))
			if v > limits[res] {
				limits[res] = v
			}
		}
	}
	return limits
}
