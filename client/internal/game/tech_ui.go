package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	file, err := os.Open("assets/tech.json")
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, _ := io.ReadAll(file)
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

	mx, my := ebiten.CursorPosition()
	cmx, cmy := float64(mx), float64(my)

	// Handle Clicking Outside (Close)
	// Removed for click-through prevention, usually handled by main loop prioritizing UI

	// Tabs
	tabs := []string{"economy", "naval", "combat", "logistics"}
	tabW := 150.0
	for i, t := range tabs {
		tx := ui.X + float64(i)*tabW
		ty := ui.Y - 30
		if cmx >= tx && cmx < tx+tabW && cmy >= ty && cmy < ui.Y {
			if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				ui.SelectedTab = t
			}
		}
	}

	// Tech Nodes
	currentList := ui.GetVisibleListForTab(ui.SelectedTab, g)
	ui.HoverID = ""

	colW := 180.0
	rowH := 100.0
	baseX := ui.X + 20
	baseY := ui.Y + 40

	for i, tech := range currentList {
		// Layout: Tier = Column, Index in Tier = Row?
		// We don't have "Index", so standard grid 5 columns
		col := float64(tech.Tier - 1)
		// Rudimentary sorting display: stack based on index in list?
		// Ideally group by tier.

		// Simple layout: just fit them in tier columns, stacking Y.
		// Calculate Y index based on how many previous items had same tier.
		yIdx := 0
		for j := 0; j < i; j++ {
			if currentList[j].Tier == tech.Tier {
				yIdx++
			}
		}

		nx := baseX + col*colW
		ny := baseY + float64(yIdx)*rowH

		if cmx >= nx && cmx < nx+160 && cmy >= ny && cmy < ny+80 {
			ui.HoverID = tech.ID
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				ui.SelectedID = tech.ID
				// Trigger Action? Or just Select?
				// Let's make click = Start Research if available
			}
		}
	}
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
		vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, true)
		vector.DrawFilledRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), color.RGBA{40, 30, 20, 255}, true)
		vector.StrokeRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), 2, color.RGBA{200, 180, 100, 255}, true)
		ebitenutil.DebugPrintAt(screen, "Chargement données joueur…", int(ui.X)+20, int(ui.Y)+40)
		g.Log("[TECH UI] skip draw: player=nil")
		return
	}

	if len(g.player.Islands) == 0 {
		// Draw UI skeleton but skip building-dependent parts
		vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, true)
		vector.DrawFilledRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), color.RGBA{40, 30, 20, 255}, true)
		vector.StrokeRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), 2, color.RGBA{200, 180, 100, 255}, true)
		ebitenutil.DebugPrintAt(screen, "Chargement île…", int(ui.X)+20, int(ui.Y)+40)
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
		vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, true)
		vector.DrawFilledRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), color.RGBA{40, 30, 20, 255}, true)
		vector.StrokeRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), 2, color.RGBA{200, 180, 100, 255}, true)
		ebitenutil.DebugPrintAt(screen, "Académie indisponible", int(ui.X)+20, int(ui.Y)+40)
		g.Log("[TECH UI] skip draw: academy missing")
		return
	}

	// 1. Overlay (Dim Background)
	vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 150}, true)

	// 2. Window Background
	vector.DrawFilledRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), color.RGBA{40, 30, 20, 255}, true)
	// Border
	vector.StrokeRect(screen, float32(ui.X), float32(ui.Y), float32(ui.W), float32(ui.H), 2, color.RGBA{200, 180, 100, 255}, true)

	// 3. Tabs
	tabs := []string{"economy", "naval", "combat", "logistics"}
	tabW := 150.0
	for i, t := range tabs {
		tx := float32(ui.X) + float32(i)*float32(tabW)
		ty := float32(ui.Y) - 30
		col := color.RGBA{60, 50, 40, 255}
		if ui.SelectedTab == t {
			col = color.RGBA{80, 70, 60, 255}
		}
		vector.DrawFilledRect(screen, tx, ty, float32(tabW), 30, col, true)
		vector.StrokeRect(screen, tx, ty, float32(tabW), 30, 1, color.White, true)
		ebitenutil.DebugPrintAt(screen, strings.ToUpper(t), int(tx)+10, int(ty)+5)
	}

	// 4. Tech Nodes (adjusted to leave space for upgrade bar at bottom)
	list := ui.GetVisibleListForTab(ui.SelectedTab, g)
	colW := 180.0
	rowH := 100.0
	baseX := ui.X + 20
	baseY := ui.Y + 40
	// Reserve space for upgrade bar at bottom (100px)
	contentHeight := ui.H - 140 // 40 (top margin) + 100 (upgrade bar)

	unlockedMap := make(map[string]bool)
	if g.player != nil {
		for _, id := range g.player.UnlockedTechs {
			unlockedMap[id] = true
		}
	}

	for i, tech := range list {
		col := float64(tech.Tier - 1)
		yIdx := 0
		for j := 0; j < i; j++ {
			if list[j].Tier == tech.Tier {
				yIdx++
			}
		}

		nx := baseX + col*colW
		ny := baseY + float64(yIdx)*rowH

		// Skip if node would be below the upgrade bar
		if ny+80 > ui.Y+contentHeight {
			continue
		}

		// Status Color
		var statusColor color.Color
		status := "LOCKED"

		// Logic - defensive check for player
		isUnlocked := unlockedMap[tech.ID]
		isResearching := false
		if g.player != nil {
			isResearching = (g.player.ResearchingTechID == tech.ID)
		}

		if isUnlocked {
			statusColor = color.RGBA{50, 150, 50, 255} // Green
			status = "DONE"
		} else if isResearching {
			statusColor = color.RGBA{200, 200, 50, 255} // Yellow
			status = "BUSY"
		} else {
			// Check Requirements
			// Simplified: Check standard TH logic?
			// We can check if previous tiers are unlocked? Use JSON rules.
			// Assume Client doesn't valid fully, just visual cue.
			// If Tier 1 -> Available. If Tier > 1, assume simple check?
			// Let's just make it Blue for now to indicate "Clickable".
			statusColor = color.RGBA{50, 50, 150, 255} // Blue
			status = "Avail"
		}

		vector.DrawFilledRect(screen, float32(nx), float32(ny), 160, 80, statusColor, true)
		vector.StrokeRect(screen, float32(nx), float32(ny), 160, 80, 2, color.White, true)

		ebitenutil.DebugPrintAt(screen, tech.Name, int(nx)+5, int(ny)+5)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Tier %d", tech.Tier), int(nx)+5, int(ny)+25)
		ebitenutil.DebugPrintAt(screen, status, int(nx)+5, int(ny)+60)

		// Cost (small)
		// Draw cost text with opaque background to prevent ghosting
		costText := fmt.Sprintf("%.0f Or", tech.Cost.Gold)
		drawTextWithBackground(screen, costText, int(nx)+5, int(ny)+40, color.RGBA{20, 30, 40, 255})

		// Tooltip / Selection Overlay
		if ui.HoverID == tech.ID {
			vector.StrokeRect(screen, float32(nx), float32(ny), 160, 80, 4, color.RGBA{255, 255, 0, 255}, true)
		}
	}

	// 5. Selected Detail Panel (Right Side or Bottom)
	if ui.SelectedID != "" {
		detailX := ui.X + ui.W - 300
		detailY := ui.Y
		vector.DrawFilledRect(screen, float32(detailX), float32(detailY), 300, float32(ui.H), color.RGBA{20, 20, 20, 250}, true)

		// Find Tech Data
		var selTech CTechnology
		for _, t := range list {
			if t.ID == ui.SelectedID {
				selTech = t
				break
			}
		}
		if selTech.ID == "" {
			// Might be in other tab? Search full.
			// Simplify: Just don't draw if not found in current tab
		} else {
			// Draw Details
			ebitenutil.DebugPrintAt(screen, selTech.Name, int(detailX)+10, int(detailY)+20)
			ebitenutil.DebugPrintAt(screen, selTech.ID, int(detailX)+10, int(detailY)+40)

			y := int(detailY) + 80
			// Draw cost text with opaque background to prevent ghosting
			costLine1 := fmt.Sprintf("Cout: %.0f Bois, %.0f Pierre", selTech.Cost.Wood, selTech.Cost.Stone)
			drawTextWithBackground(screen, costLine1, int(detailX)+10, y, color.RGBA{20, 30, 40, 255})
			y += 20
			costLine2 := fmt.Sprintf("      %.0f Or, %.0f Rhum", selTech.Cost.Gold, selTech.Cost.Rum)
			drawTextWithBackground(screen, costLine2, int(detailX)+10, y, color.RGBA{20, 30, 40, 255})
			y += 30

			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Temps: %d sec", selTech.TimeSec), int(detailX)+10, y)
			y += 30

			// Requirements
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TownHall: %d", selTech.ReqTH), int(detailX)+10, y)
			y += 20
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Academy: %d", selTech.ReqAcad), int(detailX)+10, y)
			y += 40

			// 6. Draw Effects
			y = int(detailY) + 200
			if len(selTech.Effects) > 0 {
				ebitenutil.DebugPrintAt(screen, "Effets:", int(detailX)+10, y)
				y += 20
				for k, v := range selTech.Effects {
					valStr := fmt.Sprintf("+%.0f%%", v*100)
					keyName := k
					// Simple translations
					switch k {
					case "production_wood":
						keyName = "Prod Bois"
					case "production_stone":
						keyName = "Prod Pierre"
					case "production_gold":
						keyName = "Prod Or"
					case "production_rum":
						keyName = "Prod Rhum"
					}
					ebitenutil.DebugPrintAt(screen, fmt.Sprintf("- %s : %s", keyName, valStr), int(detailX)+20, y)
					y += 15
				}
			}

			// Action Button
			btnX := float32(detailX) + 20
			btnY := float32(detailY) + float32(ui.H) - 80

			// Logic: Is Researching? Unlocked?
			label := "RECHERCHER"
			btnColor := color.RGBA{0, 200, 0, 255}

			// Pulse Animation for Search Button
			pulse := 0.0
			if label == "RECHERCHER" {
				pulse = 5.0 * math.Sin(float64(time.Now().UnixMilli())/200.0)
			}

			// Force check map again to be sure
			isUnlocked := false
			if g.player != nil {
				for _, unlockedID := range g.player.UnlockedTechs {
					if unlockedID == selTech.ID {
						isUnlocked = true
						break
					}
				}
			}

			if g.player != nil && g.player.ResearchingTechID != "" {
				if g.player.ResearchingTechID == selTech.ID {
					// SHOW PROGRESS BAR
					label = "EN COURS..."
					// btnColor is unused in this branch as we draw custom progress bar
					// ... (Keep existing progress bar logic) ...
					// We need to keep the existing progress bar drawing code here or copy it back.
					// Since we are replacing the block, we must re-include it.

					// Calculate Progress using server-provided total duration
					remaining := time.Until(g.player.ResearchFinishTime)

					// Use server-provided total duration (includes all bonuses: tech + academy)
					totalDuration := g.player.ResearchTotalDurationSeconds
					if totalDuration <= 0 {
						// Fallback to base time if server hasn't provided duration yet
						totalDuration = float64(selTech.TimeSec)
					}
					totalDur := time.Duration(totalDuration) * time.Second
					prog := 1.0 - (float64(remaining) / float64(totalDur))
					if prog < 0 {
						prog = 0
					}
					if prog > 1 {
						prog = 1
					}

					vector.DrawFilledRect(screen, btnX, btnY, 200, 50, color.RGBA{50, 50, 50, 255}, true)
					vector.DrawFilledRect(screen, btnX, btnY, 200*float32(prog), 50, color.RGBA{0, 150, 255, 255}, true)
					ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%.0f%%)", label, prog*100), int(btnX)+40, int(btnY)+15)

				} else {
					label = "OCCUPE"
					btnColor = color.RGBA{100, 100, 100, 255}
					vector.DrawFilledRect(screen, btnX, btnY, 200, 50, btnColor, true)
					ebitenutil.DebugPrintAt(screen, label, int(btnX)+50, int(btnY)+15)
				}
			} else if isUnlocked {
				label = "APPRIS !"
				btnColor = color.RGBA{100, 100, 100, 255} // Grey
				vector.DrawFilledRect(screen, btnX, btnY, 200, 50, btnColor, true)
				ebitenutil.DebugPrintAt(screen, label, int(btnX)+50, int(btnY)+15)
			} else {
				// Standard Button (Animated)
				effX := btnX - float32(pulse/2)
				effY := btnY - float32(pulse/2)
				effW := 200.0 + float32(pulse)
				effH := 50.0 + float32(pulse)

				vector.DrawFilledRect(screen, effX, effY, effW, effH, btnColor, true)
				vector.StrokeRect(screen, effX, effY, effW, effH, 2, color.White, true)
				ebitenutil.DebugPrintAt(screen, label, int(btnX)+50, int(btnY)+15)
			}

			// Check Click on Research
			mx, my := ebiten.CursorPosition()
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if float64(mx) >= float64(btnX) && float64(mx) < float64(btnX)+200 && float64(my) >= float64(btnY) && float64(my) < float64(btnY)+50 {
					if label == "RECHERCHER" {
						g.StartResearch(selTech.ID)
					}
				}
			}

			// Debug / Error Feedback
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Req: TH %d | Acad %d", selTech.ReqTH, selTech.ReqAcad), int(detailX)+10, int(detailY)+60)
		}
	}

	// Academy Upgrade Bar at bottom (drawn last to ensure it's on top)
	upgradeBarY := ui.Y + ui.H - 100
	g.DrawAcademyUpgradeBar(screen, ui.X, upgradeBarY, ui.W)

	// Close Button
	ebitenutil.DebugPrintAt(screen, "[X] FERMER", int(ui.X+ui.W)-80, int(ui.Y)+10)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) > ui.X+ui.W-90 && float64(mx) < ui.X+ui.W-10 && float64(my) > ui.Y && float64(my) < ui.Y+30 {
			ui.Visible = false
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
				g.errorMessage = err.Error()
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

	// Background with golden border
	vector.DrawFilledRect(screen, float32(x+20), float32(y), float32(w-40), float32(barH), color.RGBA{30, 40, 50, 255}, true)
	vector.StrokeRect(screen, float32(x+20), float32(y), float32(w-40), float32(barH), 2, color.RGBA{200, 150, 50, 255}, true)

	// Defensive guards - panic-proof
	if g.player == nil {
		textX := int(x) + 30
		textY := int(y) + 10
		ebitenutil.DebugPrintAt(screen, "Chargement île…", textX, textY)
		return
	}

	if len(g.player.Islands) == 0 {
		textX := int(x) + 30
		textY := int(y) + 10
		ebitenutil.DebugPrintAt(screen, "Chargement île…", textX, textY)
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
		textX := int(x) + 30
		textY := int(y) + 10
		ebitenutil.DebugPrintAt(screen, "Académie indisponible", textX, textY)
		return
	}

	// Get current level
	currentLevel := g.getAcademyLevel()

	// Display current level
	textX := int(x) + 30
	textY := int(y) + 10
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Niveau Actuel: %d", currentLevel), textX, textY)

	if currentLevel >= 30 {
		ebitenutil.DebugPrintAt(screen, "NIVEAU MAX ATTEINT", textX, textY+20)
		return
	}

	// Get upgrade cost
	upgradeCost := g.getBuildingCost("Académie", currentLevel)
	costY := textY + 25
	// Draw cost text with opaque background to prevent ghosting
	// Use fixed order to prevent flickering
	costStr := buildCostString(upgradeCost, false, nil)
	costText := fmt.Sprintf("Cout Niv Suivant: %s", costStr)
	drawTextWithBackground(screen, costText, textX, costY, color.RGBA{30, 40, 50, 255})

	// Build time
	buildTime := g.getBuildingDuration("Académie", currentLevel+1)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Temps Construction: %ds", int(buildTime.Seconds())), textX, costY+15)

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

	btnCol := color.RGBA{0, 150, 0, 255} // Green
	btnText := "AMELIORER"
	if isBusy {
		btnCol = color.RGBA{80, 80, 80, 255}
		btnText = "OCCUPE"
		canAfford = false
	} else if !canAfford {
		btnCol = color.RGBA{150, 50, 50, 255}
		btnText = "MANQUE"
	}

	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
	ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+30, int(btnY)+12)

	// Handle upgrade button click
	if canAfford && !isBusy && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
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
							g.errorMessage = err.Error()
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
