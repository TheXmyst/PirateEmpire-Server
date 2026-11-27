package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

type CTechEffect struct {
	SpeedBonus float64 `json:"speed_bonus"`
	ProdWood   float64 `json:"production_wood"`
	// Simplify: Just loose map or full struct? Struct is safer.
	// Repeating full struct from server/economy/tech.go for robust unmarshalling
	ProdStone    float64 `json:"production_stone"`
	ProdRum      float64 `json:"production_rum"`
	ProdGold     float64 `json:"production_gold"`
	StorageWood  float64 `json:"storage_wood"`
	StorageStone float64 `json:"storage_stone"`
	StorageRum   float64 `json:"storage_rum"`
	StorageGold  float64 `json:"storage_gold"`
}

type CLevelCost struct {
	Wood  float64 `json:"wood"`
	Stone float64 `json:"stone"`
	Gold  float64 `json:"gold"`
	Rum   float64 `json:"rum"`
}

type CTechnology struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Tier        int                `json:"tier"`
	ReqTH       int                `json:"required_townhall"`
	ReqAcad     int                `json:"required_academy"`
	Cost        CLevelCost         `json:"cost"`
	TimeSec     int                `json:"research_time_sec"`
	Effects     map[string]float64 `json:"effects"`
}

type CTechRoot struct {
	Economy   []CTechnology `json:"economy"`
	Naval     []CTechnology `json:"naval"`
	Combat    []CTechnology `json:"combat"`
	Logistics []CTechnology `json:"logistics"`
}

// TechUI manages the Technology Window state
type TechUI struct {
	Visible     bool
	Loaded      bool
	Data        CTechRoot
	SelectedTab string // "economy", "naval", "combat", "logistics"
	HoverID     string
	SelectedID  string
	ScrollY     float64 // défilement vertical de l'arbre (ui_scroll.go)

	// UI Metrics
	X, Y, W, H float64
}

func NewTechUI() *TechUI {
	return &TechUI{
		Visible:     false,
		SelectedTab: "economy",
		X:           100, Y: 100, W: 1000, H: 600,
	}
}

func (ui *TechUI) Load() error {
	bytes, err := readAsset("resources/data/tech.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, &ui.Data); err != nil {
		return err
	}
	ui.Loaded = true
	return nil
}

func (ui *TechUI) Update(g *Game) {
	if !ui.Visible {
		return
	}

	mx, my := cursorPosition()
	cmx, cmy := float64(mx), float64(my)

	// Handle Clicking Outside (Close)
	// Removed for click-through prevention, usually handled by main loop prioritizing UI

	// Onglets
	for i, t := range techTabs {
		tx, ty, tw, th := ui.tabRect(i)
		if cmx >= tx && cmx < tx+tw && cmy >= ty && cmy < ty+th && isMouseJustPressed(ebiten.MouseButtonLeft) {
			if ui.SelectedTab != t {
				ui.ScrollY = 0
			}
			ui.SelectedTab = t
		}
	}

	// Fermer
	if isKeyJustPressed(ebiten.KeyEscape) {
		ui.Visible = false
		g.ui.ShowTechUI = false
		return
	}

	// Technologies
	currentList := ui.GetVisibleListForTab(ui.SelectedTab, g)
	ui.HoverID = ""
	for i, tech := range currentList {
		nx, ny, ok := ui.nodePos(currentList, i)
		if !ok {
			continue
		}
		if cmy < ui.treeTop() || cmy > ui.listBottom() {
			break // pointeur hors de l'arbre (partie coupée)
		}
		if cmx >= nx && cmx < nx+techNodeW && cmy >= ny && cmy < ny+techNodeH {
			ui.HoverID = tech.ID
			if isMouseJustPressed(ebiten.MouseButtonLeft) {
				ui.SelectedID = tech.ID
			}
		}
	}
}

var techTabs = []string{"economy", "naval", "combat", "logistics"}
var techTabNames = map[string]string{"economy": "Économie", "naval": "Navale", "combat": "Combat", "logistics": "Logistique"}

const (
	techNodeW   = 126.0
	techNodeH   = 80.0
	techColW    = 136.0
	techRowH    = 92.0
	techDetailW = 290.0
)

// tabRect : position de l'onglet i, dans l'en-tête de la fenêtre.
func (ui *TechUI) tabRect(i int) (x, y, w, h float64) {
	return ui.X + 190 + float64(i)*124, ui.Y + 14, 116, 32
}

// listBottom : bas de la zone des technologies (au-dessus de la barre d'amélioration).
func (ui *TechUI) listBottom() float64 { return ui.Y + ui.H - 110 }

// nodePos : position de la i-ème technologie (une colonne par palier).
func (ui *TechUI) nodePos(list []CTechnology, i int) (x, y float64, ok bool) {
	yIdx := 0
	for j := 0; j < i; j++ {
		if list[j].Tier == list[i].Tier {
			yIdx++
		}
	}
	x = ui.X + 20 + float64(list[i].Tier-1)*techColW
	y = ui.Y + 66 + float64(yIdx)*techRowH - ui.ScrollY
	return x, y, y+techNodeH > ui.treeTop() && y < ui.listBottom()
}

// treeTop : haut de la zone de l'arbre (sous les onglets).
func (ui *TechUI) treeTop() float64 { return ui.Y + 60 }

// treeContentH : hauteur de l'arbre (la colonne la plus longue).
func treeContentH(list []CTechnology) float64 {
	rows := map[int]int{}
	maxRows := 0
	for _, t := range list {
		rows[t.Tier]++
		if rows[t.Tier] > maxRows {
			maxRows = rows[t.Tier]
		}
	}
	return 6 + float64(maxRows)*techRowH
}

func (ui *TechUI) GetListForTab(tab string) []CTechnology {
	switch tab {
	case "economy":
		return ui.Data.Economy
	case "naval":
		return ui.Data.Naval
	case "combat":
		return ui.Data.Combat
	case "logistics":
		return ui.Data.Logistics
	}
	return nil
}

func (ui *TechUI) GetTech(id string) *CTechnology {
	lists := [][]CTechnology{ui.Data.Economy, ui.Data.Naval, ui.Data.Combat, ui.Data.Logistics}
	for _, list := range lists {
		for _, t := range list {
			if t.ID == id {
				return &t
			}
		}
	}
	return nil
}

func (ui *TechUI) GetVisibleListForTab(tab string, g *Game) []CTechnology {
	fullList := ui.GetListForTab(tab)

	thLevel := 0
	acadLevel := 0
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			if b.Type == "Hôtel de Ville" {
				thLevel = b.Level
			}
			if b.Type == "Académie" {
				acadLevel = b.Level
			}
		}
	}

	// Check Unlocked Cache
	unlockedMap := make(map[string]bool)
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			unlockedMap[id] = true
		}
	}

	var visible []CTechnology
	for _, t := range fullList {
		// Show if unlocked OR requirements met
		if unlockedMap[t.ID] {
			visible = append(visible, t)
			continue
		}

		if t.ReqTH <= thLevel && t.ReqAcad <= acadLevel {
			visible = append(visible, t)
		}
	}
	return visible
}

func (ui *TechUI) Draw(screen *ebiten.Image, g *Game) {
	if !ui.Visible {
		return
	}

	// Defensive guard: check if player/islands/buildings are ready
	if g.player == nil {
		// Draw UI skeleton but skip building-dependent parts
		uiFillRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, vecAA)
		drawButton(screen, float64(ui.X), float64(ui.Y), float64(ui.W), float64(ui.H), "Chargement données joueur…", kindFromColor(color.RGBA{40, 30, 20, 255}))
		g.Log("[TECH UI] skip draw: player=nil")
		return
	}

	if len(g.player.Islands) == 0 {
		// Draw UI skeleton but skip building-dependent parts
		uiFillRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, vecAA)
		drawButton(screen, float64(ui.X), float64(ui.Y), float64(ui.W), float64(ui.H), "Chargement île…", kindFromColor(color.RGBA{40, 30, 20, 255}))
		g.Log("[TECH UI] skip draw: islands=0")
		return
	}

	// Check if Academy exists
	hasAcademy := false
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type == "Académie" {
			hasAcademy = true
			break
		}
	}

	if !hasAcademy {
		// Draw UI skeleton but skip building-dependent parts
		uiFillRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, vecAA)
		drawButton(screen, float64(ui.X), float64(ui.Y), float64(ui.W), float64(ui.H), "Académie indisponible", kindFromColor(color.RGBA{40, 30, 20, 255}))
		g.Log("[TECH UI] skip draw: academy missing")
		return
	}

	drawDimBackground(screen, 150)
	scrollBlocker(ui.X, ui.Y, ui.W, ui.H)
	drawThemePanel(screen, ui.X, ui.Y, ui.W, ui.H, 18)
	drawText(screen, "Académie", titleFace(28), ui.X+22, ui.Y+12, themeGold, alignLeft)
	for i, t := range techTabs {
		tx, ty, tw, th := ui.tabRect(i)
		drawTab(screen, tx, ty, tw, th, techTabNames[t], ui.SelectedTab == t)
	}
	drawCloseButton(screen, ui.X+ui.W-30, ui.Y+30, 15)

	unlockedMap := make(map[string]bool)
	for _, id := range g.player.UnlockedTechs {
		unlockedMap[id] = true
	}

	list := ui.GetVisibleListForTab(ui.SelectedTab, g)
	if len(list) == 0 {
		drawText(screen, "Aucune recherche disponible pour l'instant.", bodyFace(15, true), ui.X+24, ui.Y+80, themeText, alignLeft)
		drawText(screen, "Améliore l'Hôtel de Ville et l'Académie pour en débloquer.", bodyFace(14, false), ui.X+24, ui.Y+104, themeMuted, alignLeft)
	}
	treeW := ui.W - techDetailW - 40
	scrollRegion(screen, &ui.ScrollY, ui.X+10, ui.treeTop(), treeW, ui.listBottom()-ui.treeTop(), treeContentH(list))
	full := screen
	for i, tech := range list {
		nx, ny, ok := ui.nodePos(list, i)
		if !ok {
			continue
		}
		screen := clipTo(full, ui.X+10, ui.treeTop(), treeW, ui.listBottom()-ui.treeTop())
		isUnlocked := unlockedMap[tech.ID]
		isResearching := g.player.ResearchingTechID == tech.ID
		drawCard(screen, nx, ny, techNodeW, techNodeH, ui.HoverID == tech.ID || ui.SelectedID == tech.ID)
		lines := wrapText(tech.Name, bodyFace(13, true), techNodeW-16)
		if len(lines) > 2 {
			lines = lines[:2]
		}
		for k, l := range lines {
			drawText(screen, l, bodyFace(13, true), nx+8, ny+6+float64(k)*16, themeText, alignLeft)
		}
		status, clr := "Disponible", color.Color(themeGold)
		if isUnlocked {
			status, clr = "Apprise", themeGreen
		} else if isResearching {
			status, clr = "En cours", themeOrange
		}
		fillRoundRect(screen, nx+8, ny+techNodeH-17, 7, 7, 3.5, clr)
		drawText(screen, status, bodyFace(12, false), nx+20, ny+techNodeH-22, clr, alignLeft)
	}

	// Détail de la technologie choisie
	var selTech CTechnology
	for _, t := range list {
		if t.ID == ui.SelectedID {
			selTech = t
			break
		}
	}
	if selTech.ID != "" {
		ui.drawTechDetail(screen, g, selTech, unlockedMap[selTech.ID])
	}

	// Barre d'amélioration de l'Académie
	g.DrawAcademyUpgradeBar(screen, ui.X, ui.Y+ui.H-100, ui.W)

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(mx) >= ui.X+ui.W-47 && float64(mx) <= ui.X+ui.W-13 && float64(my) >= ui.Y+13 && float64(my) <= ui.Y+47 {
			ui.Visible = false
			g.ui.ShowTechUI = false
		}
	}
}

// techEffectNames : libellés des effets de recherche.
var techEffectNames = map[string]string{
	"production_wood": "Production de bois", "production_stone": "Production de pierre",
	"production_gold": "Production d'or", "production_rum": "Production de rhum",
	"storage_wood": "Stockage de bois", "storage_stone": "Stockage de pierre",
	"storage_gold": "Stockage d'or", "storage_rum": "Stockage de rhum",
	"speed_bonus": "Vitesse des navires", "travel_time": "Temps de trajet",
	"research_reduce": "Durée des recherches", "build_reduce": "Durée des constructions",
	"wind_bonus": "Bonus de vent", "counter_wind": "Vent contraire", "crew_damage": "Dégâts de l'équipage",
	"crew_hp": "Vie de l'équipage", "extra_ships": "Navires par flotte", "extra_queue": "File de construction",
	"guerrier_bonus": "Bonus guerriers", "archer_bonus": "Bonus archers", "fusilier_bonus": "Bonus fusiliers",
	"triangle_bonus": "Avantage d'unités", "triangle_malus": "Désavantage d'unités",
	"crew_loss_reduce": "Pertes d'équipage", "loot_bonus": "Butin",
	"repair_wood": "Coût de réparation (bois)", "repair_gold": "Coût de réparation (or)", "repair_global": "Coût de réparation",
}

func formatTechEffect(k string, v float64) string {
	name := techEffectNames[k]
	if name == "" {
		name = k
	}
	val := fmt.Sprintf("%+.0f %%", v*100)
	if k == "extra_ships" || k == "extra_queue" {
		val = fmt.Sprintf("%+.0f", v)
	}
	return name + " " + val
}

// drawTechDetail : panneau de droite pour la technologie sélectionnée.
func (ui *TechUI) drawTechDetail(screen *ebiten.Image, g *Game, t CTechnology, isUnlocked bool) {
	x := ui.X + ui.W - techDetailW - 20
	y := ui.Y + 60
	w := techDetailW
	h := ui.listBottom() - y
	drawThemePanel(screen, x, y, w, h, 14)

	px := x + 16
	cy := y + 12
	for _, l := range wrapText(t.Name, titleFace(22), w-32) {
		drawText(screen, l, titleFace(22), px, cy, themeGold, alignLeft)
		cy += 26
	}
	if t.Description != "" {
		for _, l := range wrapText(t.Description, bodyFace(13, false), w-32) {
			drawText(screen, l, bodyFace(13, false), px, cy, themeMuted, alignLeft)
			cy += 17
		}
	}
	cy += 6

	island := g.player.Islands[0]
	costs := map[domain.ResourceType]float64{domain.Wood: t.Cost.Wood, domain.Stone: t.Cost.Stone, domain.Gold: t.Cost.Gold, domain.Rum: t.Cost.Rum}
	drawText(screen, "Coût", bodyFace(12, true), px, cy, themeMuted, alignLeft)
	cy += 18
	row1 := map[domain.ResourceType]float64{domain.Wood: costs[domain.Wood], domain.Stone: costs[domain.Stone]}
	row2 := map[domain.ResourceType]float64{domain.Gold: costs[domain.Gold], domain.Rum: costs[domain.Rum]}
	if drawCost(screen, row1, island.Resources, px, cy) > 0 {
		cy += 18
	}
	if drawCost(screen, row2, island.Resources, px, cy) > 0 {
		cy += 18
	}
	drawText(screen, "Durée "+formatDuration(t.TimeSec), bodyFace(13, false), px, cy+2, themeMuted, alignLeft)
	cy += 24

	thOK := false
	for _, b := range island.Buildings {
		if b.Type == "Hôtel de Ville" && b.Level >= t.ReqTH {
			thOK = true
		}
	}
	reqClr := func(ok bool) color.Color {
		if ok {
			return themeText
		}
		return color.RGBA{235, 90, 70, 255}
	}
	drawText(screen, fmt.Sprintf("Hôtel de Ville niv. %d", t.ReqTH), bodyFace(13, false), px, cy, reqClr(thOK), alignLeft)
	cy += 17
	drawText(screen, fmt.Sprintf("Académie niv. %d", t.ReqAcad), bodyFace(13, false), px, cy, reqClr(g.getAcademyLevel() >= t.ReqAcad), alignLeft)
	cy += 24

	if len(t.Effects) > 0 {
		drawText(screen, "Effets", bodyFace(12, true), px, cy, themeMuted, alignLeft)
		cy += 18
		keys := make([]string, 0, len(t.Effects))
		for k := range t.Effects {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			drawText(screen, formatTechEffect(k, t.Effects[k]), bodyFace(13, true), px, cy, themeGreen, alignLeft)
			cy += 18
		}
	}

	// Bouton
	btnW, btnH := w-40, 42.0
	btnX, btnY := x+20, y+h-btnH-16
	researching := g.player.ResearchingTechID
	switch {
	case researching == t.ID:
		remaining := time.Until(g.player.ResearchFinishTime)
		total := g.player.ResearchTotalDurationSeconds
		if total <= 0 {
			total = float64(t.TimeSec)
		}
		prog := 1 - remaining.Seconds()/total
		if prog < 0 {
			prog = 0
		}
		if prog > 1 {
			prog = 1
		}
		drawProgressBar(screen, btnX, btnY+8, btnW, 12, prog, themeGold)
		drawText(screen, fmt.Sprintf("Recherche en cours · %s", formatDuration(int(math.Max(0, remaining.Seconds())))), bodyFace(13, true), btnX+btnW/2, btnY+26, themeText, alignCenter)
	case researching != "":
		drawButton(screen, btnX, btnY, btnW, btnH, "Une recherche est en cours", btnDisabled)
	case isUnlocked:
		drawButton(screen, btnX, btnY, btnW, btnH, "Déjà apprise", btnDisabled)
	default:
		drawButton(screen, btnX, btnY, btnW, btnH, "Rechercher", btnPrimary)
		if isMouseJustPressed(ebiten.MouseButtonLeft) && isHovered(btnX, btnY, btnW, btnH) {
			g.StartResearch(t.ID)
		}
	}
}

func (g *Game) StartResearch(techID string) {
	go func() {
		err := g.api.StartResearch(techID)
		if err != nil {
			g.Log("Research Failed: %v", err)
			// Check if it's an auth error
			if strings.Contains(err.Error(), "session expirée") || strings.Contains(err.Error(), "Authentication required") {
				g.showError = true
				g.errorMessage = "Session expirée, veuillez vous reconnecter"
				g.errorDebounce = 60
				// Return to login screen
				g.state = StateLogin
				g.api.Token = "" // Clear token
				g.player = nil
				return
			}
			// Try to handle as prerequisites error
			if !g.handleAPIError(err, fmt.Sprintf("Recherche %s", techID)) {
				// Not a prerequisites error - show standard error
				g.showError = true
				g.errorMessage = friendlyError(err)
				g.errorDebounce = 60
			}
			return
		}

		g.Log("Research Started: %s", techID)
		// Update will be fetched on next status poll
		g.player.ResearchingTechID = techID
		g.lastUpdate = time.Time{} // Force refresh on next poll
	}()
}

// DrawAcademyUpgradeBar renders upgrade information and button at bottom (similar to Shipyard)
func (g *Game) DrawAcademyUpgradeBar(screen *ebiten.Image, x, y, w float64) {
	barH := 90.0

	drawCard(screen, x+20, y, w-40, barH, false)
	textX := x + 36

	// Defensive guards - panic-proof
	if g.player == nil {
		uiPrint(screen, "Chargement île…", int(textX), int(y)+10)
		return
	}

	if len(g.player.Islands) == 0 {
		uiPrint(screen, "Chargement île…", int(textX), int(y)+10)
		return
	}

	// Check if Academy exists
	hasAcademy := false
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type == "Académie" {
			hasAcademy = true
			break
		}
	}

	if !hasAcademy {
		uiPrint(screen, "Académie indisponible", int(textX), int(y)+10)
		return
	}

	// Get current level
	currentLevel := g.getAcademyLevel()

	drawText(screen, fmt.Sprintf("Académie · niveau %d", currentLevel), titleFace(22), textX, y+10, themeGold, alignLeft)
	if currentLevel >= 30 {
		drawText(screen, "Niveau maximum atteint", bodyFace(14, true), textX, y+44, themeGreen, alignLeft)
		return
	}
	upgradeCost := g.getBuildingCost("Académie", currentLevel)
	drawText(screen, "Niveau suivant :", bodyFace(13, false), textX, y+44, themeMuted, alignLeft)
	drawCost(screen, upgradeCost, g.player.Islands[0].Resources, textX+textWidth("Niveau suivant : ", bodyFace(13, false)), y+44)
	buildTime := g.getBuildingDuration("Académie", currentLevel+1)
	drawText(screen, "Durée "+formatDuration(int(buildTime.Seconds())), bodyFace(13, false), textX, y+64, themeMuted, alignLeft)

	// Upgrade button
	btnW, btnH := 150.0, 40.0
	btnX := x + w - btnW - 40
	btnY := y + barH/2 - btnH/2

	// Check if can afford - defensive check with Resources map
	canAfford := true
	if g.player.Islands[0].Resources == nil {
		canAfford = false
	} else {
		for res, amt := range upgradeCost {
			if g.player.Islands[0].Resources[res] < amt {
				canAfford = false
				break
			}
		}
	}

	// Check if busy
	isBusy := false
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			if b.Constructing {
				isBusy = true
				break
			}
		}
	}

	kind, btnText := btnPrimary, "Améliorer"
	if isBusy {
		kind, btnText = btnDisabled, "Occupé"
		canAfford = false
	} else if !canAfford {
		kind, btnText = btnDisabled, "Ressources"
	}
	drawButton(screen, btnX, btnY, btnW, btnH, btnText, kind)

	// Handle upgrade button click
	if canAfford && !isBusy && isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		if float64(mx) >= btnX && float64(mx) <= btnX+btnW && float64(my) >= btnY && float64(my) <= btnY+btnH {
			// Find academy building ID
			var academyID string
			if g.player != nil && len(g.player.Islands) > 0 {
				for _, b := range g.player.Islands[0].Buildings {
					if b.Type == "Académie" {
						academyID = b.ID.String()
						break
					}
				}
			}
			if academyID != "" {
				g.Log("Ordering Upgrade: Académie")
				go func(pid, bid string) {
					err := g.api.Upgrade(pid, bid)
					if err != nil {
						g.Log("Academy upgrade failed: %v", err)
						// Try to handle as prerequisites error
						if !g.handleAPIError(err, "Amélioration Académie") {
							// Not a prerequisites error - show standard error
							g.showError = true
							g.errorMessage = friendlyError(err)
							g.errorDebounce = 60
						}
					} else {
						time.Sleep(100 * time.Millisecond)
						g.api.GetStatus()
					}
				}(g.player.ID.String(), academyID)
			}
		}
	}
}

// Helper: Get academy level
func (g *Game) getAcademyLevel() int {
	if g.player == nil || len(g.player.Islands) == 0 {
		return 0
	}
	for _, b := range g.player.Islands[0].Buildings {
		if b.Type == "Académie" {
			return b.Level
		}
	}
	return 0
}
