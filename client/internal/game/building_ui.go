package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

// Helper to get cost based on type and level
func (g *Game) getBuildingCost(bType string, level int) map[domain.ResourceType]float64 {
	// Base Costs and Growth Factors
	growth := 1.39
	baseCost := map[domain.ResourceType]float64{domain.Wood: 100, domain.Gold: 100}

	switch bType {
	case "Hôtel de Ville":
		growth = 1.41
		baseCost = map[domain.ResourceType]float64{domain.Wood: 200, domain.Stone: 200, domain.Gold: 700}
	case "Scierie":
		growth = 1.39
		baseCost = map[domain.ResourceType]float64{domain.Wood: 100, domain.Gold: 50} // Fixed base from json
	case "Carrière":
		growth = 1.39
		baseCost = map[domain.ResourceType]float64{domain.Wood: 150, domain.Gold: 100}
	case "Mine d'Or":
		growth = 1.39
		baseCost = map[domain.ResourceType]float64{domain.Wood: 100, domain.Stone: 100}
	case "Distillerie":
		growth = 1.39
		baseCost = map[domain.ResourceType]float64{domain.Wood: 300, domain.Stone: 100, domain.Gold: 200}
	case "Entrepôt":
		growth = 1.40
		baseCost = map[domain.ResourceType]float64{domain.Wood: 150, domain.Stone: 150, domain.Gold: 400} // Fixed base
	case "Académie":
		growth = 1.40
		baseCost = map[domain.ResourceType]float64{domain.Wood: 150, domain.Stone: 200, domain.Gold: 500}
	case "Chantier Naval":
		growth = 1.40
		baseCost = map[domain.ResourceType]float64{domain.Wood: 150, domain.Stone: 150, domain.Gold: 400}
	}

	scale := math.Pow(growth, float64(level))

	finalCost := make(map[domain.ResourceType]float64)
	for r, val := range baseCost {
		finalCost[r] = val * scale
	}

	return finalCost
}

func (g *Game) DrawBuildingModal(screen *ebiten.Image) {
	if g.selectedBuilding == nil {
		return
	}
	sb := g.selectedBuilding
	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2

	// Window Dimensions
	w, h := 500.0, 400.0
	x, y := cx-w/2, cy-h/2

	drawDimBackground(screen, 110)
	drawWindowFrame(screen, x, y, w, h, buildingDisplayName(sb.Type))
	drawText(screen, fmt.Sprintf("Niveau %d", sb.Level), bodyFace(14, true), x+w-24, y+20, themeText, alignRight)

	status, statusClr := "Actif", color.Color(themeGreen)
	if sb.Constructing {
		status, statusClr = "En construction…", themeOrange
	}
	drawLabel(screen, "État", x+24, y+68, 13, false, themeMuted)
	drawLabel(screen, status, x+24, y+86, 15, true, statusClr)

	// La taverne ne s'améliore pas : seulement l'état.
	if sb.Type == "Tavern" {
		return
	}

	// Niveau suivant (les prérequis sont vérifiés par le serveur)
	drawInset(screen, x+20, y+124, w-40, 110)
	drawText(screen, fmt.Sprintf("Niveau %d", sb.Level+1), titleFace(22), x+36, y+134, themeGold, alignLeft)
	drawLabel(screen, "Coût", x+36, y+172, 13, false, themeMuted)
	var have map[domain.ResourceType]float64
	if g.player != nil && len(g.player.Islands) > 0 {
		have = g.player.Islands[0].Resources
	}
	drawCost(screen, g.getBuildingCost(sb.Type, sb.Level+1), have, x+36, y+192)

	// Bouton Améliorer
	btnW, btnH := 200.0, 50.0
	btnX, btnY := x+w/2-btnW/2, y+h-80
	if sb.Constructing {
		drawButton(screen, btnX, btnY, btnW, btnH, "En cours…", btnDisabled)
	} else {
		drawButton(screen, btnX, btnY, btnW, btnH, "Améliorer", btnPrimary)
	}

	// Handle Click - Upgrade
	// Logic moved to UpdateBuildingModal
}

func (g *Game) getModalRect() (float64, float64, float64, float64) {
	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	w, h := 500.0, 400.0
	x, y := cx-w/2, cy-h/2
	return x, y, w, h
}

func (g *Game) UpdateBuildingModal() bool {
	if g.selectedBuilding == nil {
		return false
	}
	sb := g.selectedBuilding
	x, y, w, h := g.getModalRect()

	// Check standard interactions
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// Tavern should not reach here (opens dedicated UI instead)
		// But if it does (constructing state), no special handling needed

		// Button Rect
		btnW, btnH := 200.0, 50.0
		btnX, btnY := x+w/2-btnW/2, y+h-80

		// Check Upgrade Button (prerequisites checked server-side)
		if !sb.Constructing {
			if fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH {
				go func(bid uuid.UUID, btype string) {
					err := g.api.Upgrade(g.player.ID.String(), bid.String())
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
						g.Log("Upgrade Order: %s", btype)
					}
					g.selectedBuilding = nil
				}(sb.ID, sb.Type)
				return true
			}
		}

		// Check Outside Click (Close)
		if fmx < x || fmx > x+w || fmy < y || fmy > y+h {
			g.selectedBuilding = nil
			return true
		}

		// Click Inside (Consume input)
		return true
	}
	return false
}

// CurvePreset matches Server Logic
type CurvePreset struct {
	Pivot       int
	EarlyBase   float64
	EarlyFactor float64
	LateBase    float64
	LateFactor  float64
}

func (g *Game) getBuildingDuration(bType string, level int) time.Duration {
	// 1. Define Curves (Must match buildings.json)
	curves := map[string]CurvePreset{
		"production": {10, 4, 2.29984, 7200, 1.17222},
		"utility":    {10, 6, 2.29984, 10800, 1.16159},
		"townhall":   {10, 12, 2.25372, 18000, 1.14266},
	}

	// 2. Map Building to Category
	category := "production" // Default
	switch bType {
	case "Hôtel de Ville":
		category = "townhall"
	case "Scierie", "Carrière", "Mine d'Or", "Distillerie":
		category = "production"
	case "Entrepôt", "Académie", "Chantier Naval", "Le Nid du Flibustier":
		category = "utility"
	}

	curve := curves[category]

	// 3. Compute Seconds
	var seconds float64
	if level <= curve.Pivot {
		// Early: time = EBase * (EFactor ^ (L-1))
		seconds = curve.EarlyBase * math.Pow(curve.EarlyFactor, float64(level-1))
	} else {
		// Late: time = LBase * (LFactor ^ (L-Pivot))
		seconds = curve.LateBase * math.Pow(curve.LateFactor, float64(level-curve.Pivot))
	}

	// 4. Apply Logistics Bonus
	speedMult := 1.0
	if g.player != nil {
		for _, t := range g.player.UnlockedTechs {
			if t == "log_build_1" {
				speedMult += 0.05
			}
			if t == "log_build_2" {
				speedMult += 0.10
			}
			if t == "log_build_3" {
				speedMult += 0.15
			}
			if t == "log_build_4" {
				speedMult += 0.20
			}
		}
	}

	finalSeconds := seconds / speedMult
	return time.Duration(finalSeconds * float64(time.Second))
}
