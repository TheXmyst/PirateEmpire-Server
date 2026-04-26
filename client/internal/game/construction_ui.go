package game

import (
	"fmt"
	"image/color"

	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Construction Menu Logic

type ConstructionItem struct {
	Type           string
	Name           string
	Desc           string
	Cost           map[domain.ResourceType]float64
	Icon           *ebiten.Image
	ReqThLevel     int
	ReqBuilding    string
	ReqBuildingLvl int
	ReqTech        string
}

func (g *Game) getConstructionItems() []ConstructionItem {
	items := []ConstructionItem{
		{
			Type: "Hôtel de Ville", Name: "Hôtel de Ville", Desc: "Centre administratif.",
			Cost: map[domain.ResourceType]float64{domain.Wood: 500, domain.Stone: 500, domain.Gold: 1000},
			Icon: g.iconTownhall,
		},
		{
			Type: "Scierie", Name: "Scierie", Desc: "Produit du bois.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 100, domain.Gold: 50},
			Icon:       g.iconSawmill,
			ReqThLevel: 1,
		},
		{
			Type: "Carrière", Name: "Carrière", Desc: "Produit de la pierre.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 150, domain.Gold: 100},
			Icon:       g.iconStoneQuarry,
			ReqThLevel: 2, ReqBuilding: "Scierie", ReqBuildingLvl: 1,
		},
		{
			Type: "Mine d'Or", Name: "Mine d'Or", Desc: "Produit de l'or.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 100, domain.Stone: 100},
			Icon:       g.iconGoldMine,
			ReqThLevel: 3,
		},
		{
			Type: "Distillerie", Name: "Distillerie", Desc: "Produit du rhum.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 300, domain.Stone: 100, domain.Gold: 200},
			Icon:       g.iconDistillery,
			ReqThLevel: 2,
		},
		{
			Type: "Entrepôt", Name: "Entrepôt", Desc: "Augmente le stockage.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 200, domain.Stone: 50},
			Icon:       g.iconWarehouse,
			ReqThLevel: 1,
		},
		{
			Type: "Académie", Name: "Académie", Desc: "Permet les recherches.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 400, domain.Stone: 300, domain.Gold: 500},
			Icon:       g.iconAcademy,
			ReqThLevel: 3,
		},
		{
			Type: "Chantier Naval", Name: "Chantier Naval", Desc: "Construction de navires.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 150, domain.Stone: 150, domain.Gold: 400},
			Icon:       g.iconShipyard, // Ensure iconShipyard is loaded
			ReqThLevel: 4, ReqBuilding: "Scierie", ReqBuildingLvl: 2, ReqTech: "tech_naval_1",
		},
		{
			Type: "Tavern", Name: "Tavern", Desc: "Recrute des capitaines.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 300, domain.Stone: 200, domain.Gold: 500},
			Icon:       g.iconTavern,
			ReqThLevel: 3,
		},
		{
			Type: "Milice", Name: "Milice", Desc: "Recrute des matelots.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 250, domain.Stone: 200, domain.Gold: 400},
			Icon:       g.iconMilitia,
			ReqThLevel: 4, ReqBuilding: "Académie", ReqBuildingLvl: 2,
		},
		{
			Type: "Infirmary", Name: "Infirmerie", Desc: "Soigne les capitaines.",
			Cost:       map[domain.ResourceType]float64{domain.Wood: 200, domain.Stone: 200, domain.Gold: 300},
			Icon:       g.iconInfirmary,
			ReqThLevel: 3,
		},
	}

	// Debug: Log building count and check for gold mine
	g.Log("Construction Items: %d buildings loaded", len(items))
	hasGoldMine := false
	for _, item := range items {
		if item.Type == "Mine d'Or" {
			hasGoldMine = true
			g.Log("Gold Mine found in construction list: Icon=%v, ReqThLevel=%d", item.Icon != nil, item.ReqThLevel)
			break
		}
	}
	if !hasGoldMine {
		g.Log("WARNING: Gold Mine NOT found in construction items list!")
	}

	return items
}

func (g *Game) DrawConstructionMenu(screen *ebiten.Image) {
	if !g.ui.ShowConstruction {
		return
	}
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	cx, cy := w/2, h/2
	winW, winH := 500.0, 600.0

	// Semi-transparent BG
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 100}, true)

	// Window
	winX, winY := cx-winW/2, cy-winH/2
	if g.sliceTopLeft != nil {
		draw9Slice(screen, g, winX, winY, winW, winH, 24)
	} else {
		vector.DrawFilledRect(screen, float32(winX), float32(winY), float32(winW), float32(winH), color.RGBA{40, 45, 60, 255}, true)
		vector.StrokeRect(screen, float32(winX), float32(winY), float32(winW), float32(winH), 2, color.RGBA{200, 180, 50, 255}, true)
	}

	ebitenutil.DebugPrintAt(screen, "CONSTRUCTION", int(winX)+20, int(winY)+20)
	ebitenutil.DebugPrintAt(screen, "[Fermer]", int(winX+winW)-80, int(winY)+20)

	items := g.getConstructionItems()
	startY := winY + 60
	itemH := 80.0
	gap := 10.0

	// Check Global Construction Status
	isBusy := false
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			if b.Constructing {
				isBusy = true
				break
			}
		}
	}

	for i, item := range items {
		y := startY + float64(i)*(itemH+gap) - g.ui.ConstructionScrollY
		if y < startY || y > winY+winH-itemH {
			continue
		}

		ix := winX + 20
		vector.DrawFilledRect(screen, float32(ix), float32(y), float32(winW-40), float32(itemH), color.RGBA{60, 65, 80, 255}, true)
		vector.StrokeRect(screen, float32(ix), float32(y), float32(winW-40), float32(itemH), 1, color.RGBA{150, 150, 150, 255}, true)

		// Icon
		if item.Icon != nil {
			op := &ebiten.DrawImageOptions{}
			iw := item.Icon.Bounds().Dx()
			scale := 64.0 / float64(iw)
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(ix+10, y+8)
			screen.DrawImage(item.Icon, op)
		}

		// Text
		textX := int(ix) + 80
		ebitenutil.DebugPrintAt(screen, item.Name, textX, int(y)+10)
		ebitenutil.DebugPrintAt(screen, item.Desc, textX, int(y)+30)

		// Check if building exists
		var existing *domain.Building
		if g.player != nil && len(g.player.Islands) > 0 {
			for i := range g.player.Islands[0].Buildings {
				b := &g.player.Islands[0].Buildings[i]
				if b.Type == item.Type {
					existing = b
					break
				}
			}
		}

		// Determine Mode (Build vs Upgrade)
		isUpgrade := (existing != nil)
		currentLevel := 0
		if isUpgrade {
			currentLevel = existing.Level
		}

		// Dynamic Cost
		realCost := item.Cost // Default for Lvl 1
		if isUpgrade {
			realCost = g.getBuildingCost(item.Type, currentLevel) // Cost to get to L+1
		}

		// Cost Check - use fixed order to prevent flickering
		canAfford := true
		var hasResources map[domain.ResourceType]float64
		if g.player != nil && len(g.player.Islands) > 0 {
			hasResources = g.player.Islands[0].Resources
			for r, amt := range realCost {
				if hasResources[r] < amt {
					canAfford = false
					break
				}
			}
		}
		costStr := buildCostString(realCost, true, hasResources)

		// Prerequisites are checked server-side - no client-side blocking
		// The button will be enabled, and server will return prerequisites error if needed
		if false { // Keep structure but disable client-side check
			_ = item // Avoid unused variable warning
		} else {
			// For upgrade, we rely on server for advanced reqs, or basic resource/busy checks
			// Visual indication of upgrade level
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("(Niv %d -> %d)", currentLevel, currentLevel+1), int(ix)+250, int(y)+20)
		}

		// Draw cost text with opaque background to prevent ghosting
		drawTextWithBackground(screen, costStr, textX, int(y)+50, color.RGBA{60, 65, 80, 255})

		// Button
		btnW, btnH := 100.0, 30.0
		btnX := ix + winW - 40 - 20 - btnW
		btnY := y + itemH/2 - btnH/2

		btnCol := color.RGBA{50, 150, 50, 255}
		btnText := "CONSTRUIRE"

		if isUpgrade {
			btnCol = color.RGBA{0, 100, 200, 255}
			btnText = "AMÉLIORER"
		}

		if isBusy {
			btnCol = color.RGBA{100, 100, 100, 255}
			btnText = "OCCUPÉ"
		} else if !canAfford {
			btnCol = color.RGBA{150, 50, 50, 255}
			btnText = "MANQUE"
		}

		vector.DrawFilledRect(screen, float32(btnX), float32(btnY), float32(btnW), float32(btnH), btnCol, true)
		ebitenutil.DebugPrintAt(screen, btnText, int(btnX)+5, int(btnY)+8)
	}
}

func (g *Game) UpdateConstructionMenu() {
	if !g.ui.ShowConstruction {
		return
	}

	// Handle Mouse Wheel Scrolling
	_, wy := ebiten.Wheel()
	if wy != 0 {
		items := g.getConstructionItems()
		itemH := 80.0
		gap := 10.0
		totalHeight := float64(len(items)) * (itemH + gap)
		visibleHeight := 600.0 - 60.0 - 20.0 // winH - header - padding
		maxScroll := math.Max(0, totalHeight-visibleHeight)

		g.ui.ConstructionScrollY -= wy * 30 // Scroll speed
		if g.ui.ConstructionScrollY < 0 {
			g.ui.ConstructionScrollY = 0
		}
		if g.ui.ConstructionScrollY > maxScroll {
			g.ui.ConstructionScrollY = maxScroll
		}
	}

	// Close on Escape or Outside Click
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowConstruction = false
		return
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)
		w, h := float64(g.screenWidth), float64(g.screenHeight)
		cx, cy := w/2, h/2
		winW, winH := 500.0, 600.0
		winX, winY := cx-winW/2, cy-winH/2

		// Check Close (Top Right)
		if fmx > winX+winW-100 && fmx < winX+winW && fmy > winY && fmy < winY+50 {
			g.ui.ShowConstruction = false
			return
		}

		// Check Items
		items := g.getConstructionItems()
		startY := winY + 60
		itemH := 80.0
		gap := 10.0

		for i, item := range items {
			y := startY + float64(i)*(itemH+gap) - g.ui.ConstructionScrollY
			if y < startY || y > winY+winH-itemH {
				continue
			}

			ix := winX + 20
			// Button rect:
			btnW, btnH := 100.0, 30.0
			btnX := ix + winW - 40 - 20 - btnW
			btnY := y + itemH/2 - btnH/2

			if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				// Clicked Action
				if g.player != nil && len(g.player.Islands) > 0 {
					island := g.player.Islands[0]

					// Check Busy
					for _, b := range island.Buildings {
						if b.Constructing {
							g.Log("Bloqué: Construction en cours")
							g.showError = true
							g.errorMessage = "Construction déjà en cours !"
							g.errorDebounce = 30
							return
						}
					}

					// Identify if Build or Upgrade
					var existing *domain.Building
					for i := range island.Buildings {
						if island.Buildings[i].Type == item.Type {
							existing = &island.Buildings[i]
							break
						}
					}

					isUpgrade := (existing != nil)

					// Prereq check removed - server is authoritative
					// Client-side check was removed to avoid duplicate validation

					// Dynamic Cost
					currentLevel := 0
					if isUpgrade {
						currentLevel = existing.Level
					}
					realCost := item.Cost
					if isUpgrade {
						realCost = g.getBuildingCost(item.Type, currentLevel)
					}

					// Check Cost
					for r, amt := range realCost {
						if island.Resources[r] < amt {
							g.Log("Bloqué: Ressources insuffisantes (%s)", r)
							g.showError = true
							g.errorMessage = fmt.Sprintf("Ressources manquantes: %s", r)
							g.errorDebounce = 30
							return
						}
					}

					// ACTION
					if isUpgrade {
						// Upgrade
						g.Log("Ordering Upgrade: %s", item.Type)
						go func(pid string, bid string, btype string) {
							err := g.api.Upgrade(pid, bid)
							if err != nil {
								g.Log("Upgrade failed: %v", err)
								// Try to handle as prerequisites error
								if !g.handleAPIError(err, fmt.Sprintf("Amélioration %s", btype)) {
									// Not a prerequisites error - show standard error
									g.showError = true
									g.errorMessage = err.Error()
									g.errorDebounce = 60
								}
							} else {
								time.Sleep(100 * time.Millisecond)
							}
						}(g.player.ID.String(), existing.ID.String(), item.Type)

						g.ui.ShowConstruction = false
						return

					} else {
						// Build - Find free slot
						var targetX, targetY float64
						found := false
						debugSlots := []string{}

						for _, s := range g.slots {
							if s.AllowedType == item.Type {
								debugSlots = append(debugSlots, fmt.Sprintf("slot(%s) at (%.0f,%.0f)", s.AllowedType, s.X, s.Y))
								// Check if occupied
								occupied := false
								for _, b := range island.Buildings {
									if math.Abs(b.X-s.X) < 10 && math.Abs(b.Y-s.Y) < 10 {
										occupied = true
										debugSlots[len(debugSlots)-1] += " [OCCUPIED]"
										break
									}
								}
								if !occupied {
									targetX, targetY = s.X, s.Y
									found = true
									debugSlots[len(debugSlots)-1] += " [SELECTED]"
									break
								}
							}
						}

						if !found {
							g.Log("Pas d'emplacement libre pour: %s", item.Type)
							if len(debugSlots) > 0 {
								g.Log("Slots évalués: %v", debugSlots)
							} else {
								g.Log("Aucun slot trouvé pour le type: %s", item.Type)
							}
							g.showError = true
							g.errorMessage = "Pas d'emplacement libre !"
							g.errorDebounce = 30
							return
						}

						g.Log("Ordering Construction: %s", item.Type)
						go func(bType string, tx, ty float64) {
							err := g.api.Build(bType, tx, ty)
							if err != nil {
								g.Log("Build failed: %v", err)
								// Try to handle as prerequisites error
								if !g.handleAPIError(err, fmt.Sprintf("Construction %s", bType)) {
									// Not a prerequisites error - show standard error
									g.showError = true
									g.errorMessage = err.Error()
									g.errorDebounce = 60
								}
							} else {
								time.Sleep(100 * time.Millisecond)
							}
						}(item.Type, targetX, targetY)

						g.ui.ShowConstruction = false
						return
					}
				}
				return
			}
		}
	}
}
