package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// UpdateShipyardMenu handles shipyard logic and modal interactions.
func (g *Game) UpdateShipyardMenu() {
	if !g.ui.ShowShipyard {
		return
	}
	// Delegate to modal update if open
	if shipyardUI.ShowAddShipModal {
		g.UpdateAddShipModalShipyard()
		return
	}
}

// DrawDashboard renders the player's construction/research/ship timers on the left side.
func (g *Game) DrawDashboard(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	island := g.player.Islands[0]
	dashX := 10.0
	dashY := 80.0 // Below HUD
	dashW := 300.0
	itemH := 60.0
	gap := 5.0
	currentY := dashY
	now := time.Now()

	// Count active timers
	activeCount := 0

	// Buildings under construction
	for _, b := range island.Buildings {
		if b.Constructing && !b.FinishTime.IsZero() {
			if b.FinishTime.After(now) {
				activeCount++
			}
		}
	}

	// Research in progress
	if g.player.ResearchingTechID != "" && !g.player.ResearchFinishTime.IsZero() {
		if g.player.ResearchFinishTime.After(now) {
			activeCount++
		}
	}

	// Ships under construction
	for _, s := range island.Ships {
		if s.State == "UnderConstruction" && !s.FinishTime.IsZero() {
			if s.FinishTime.After(now) {
				activeCount++
			}
		}
	}

	// Don't draw if nothing is in progress
	if activeCount == 0 {
		return
	}

	// Draw dashboard background
	dashH := float64(activeCount)*itemH + float64(activeCount-1)*gap + 20
	vector.DrawFilledRect(screen, float32(dashX), float32(dashY), float32(dashW), float32(dashH), color.RGBA{20, 20, 30, 220}, true)
	vector.StrokeRect(screen, float32(dashX), float32(dashY), float32(dashW), float32(dashH), 2, color.RGBA{100, 100, 150, 255}, true)

	currentY += 10

	// Draw building constructions
	for _, b := range island.Buildings {
		if b.Constructing && !b.FinishTime.IsZero() {
			remaining := b.FinishTime.Sub(now)
			if remaining > 0 {
				// Item box
				vector.DrawFilledRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), color.RGBA{40, 40, 60, 255}, true)
				vector.StrokeRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), 1, color.RGBA{80, 120, 200, 255}, true)

				// Building name
				ebitenutil.DebugPrintAt(screen, "🏗️ "+b.Type, int(dashX+20), int(currentY+10))

				// Timer
				mins := int(remaining.Minutes())
				secs := int(remaining.Seconds()) % 60
				timeStr := fmt.Sprintf("%02d:%02d", mins, secs)
				ebitenutil.DebugPrintAt(screen, timeStr, int(dashX+20), int(currentY+30))

				// Progress bar
				barW := dashW - 60
				barH := 8.0
				barX := dashX + 30
				barY := currentY + 45

				// Calculate progress using server-provided duration, or estimate if not available
				progress := 0.0
				totalDuration := b.ConstructionTotalDuration
				if totalDuration > 0 {
					// Use server-provided duration (accurate)
					startTime := b.FinishTime.Add(-time.Duration(totalDuration) * time.Second)
					elapsed := now.Sub(startTime).Seconds()
					progress = elapsed / totalDuration
				} else {
					// Fallback: show progress based on remaining time (approximate)
					// Assume construction started recently, show 10-90% range based on remaining time
					// This is just a visual indicator until server sends real duration
					if remaining.Seconds() > 0 {
						// Simple heuristic: if less than 1 minute left, show 90%, otherwise scale
						if remaining.Minutes() < 1 {
							progress = 0.9
						} else if remaining.Minutes() < 5 {
							progress = 0.7
						} else {
							progress = 0.3
						}
					}
				}
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}

				// Progress bar background
				vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{60, 60, 80, 255}, true)
				// Progress bar fill
				vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), color.RGBA{100, 200, 100, 255}, true)

				currentY += itemH + gap
			}
		}
	}

	// Draw research in progress
	if g.player.ResearchingTechID != "" && !g.player.ResearchFinishTime.IsZero() {
		remaining := g.player.ResearchFinishTime.Sub(now)
		if remaining > 0 {
			// Item box
			vector.DrawFilledRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), color.RGBA{40, 40, 60, 255}, true)
			vector.StrokeRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), 1, color.RGBA{200, 120, 80, 255}, true)

			// Research name
			ebitenutil.DebugPrintAt(screen, "🔬 "+g.player.ResearchingTechID, int(dashX+20), int(currentY+10))

			// Timer
			mins := int(remaining.Minutes())
			secs := int(remaining.Seconds()) % 60
			timeStr := fmt.Sprintf("%02d:%02d", mins, secs)
			ebitenutil.DebugPrintAt(screen, timeStr, int(dashX+20), int(currentY+30))

			// Progress bar
			barW := dashW - 60
			barH := 8.0
			barX := dashX + 30
			barY := currentY + 45
			vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{60, 60, 80, 255}, true)

			// Calculate actual progress using server-provided total duration
			progress := 0.0
			if g.player.ResearchTotalDurationSeconds > 0 {
				// Calculate start time: FinishTime - totalDuration
				startTime := g.player.ResearchFinishTime.Add(-time.Duration(g.player.ResearchTotalDurationSeconds) * time.Second)
				elapsed := now.Sub(startTime).Seconds()
				progress = elapsed / g.player.ResearchTotalDurationSeconds
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}
			}
			vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), color.RGBA{200, 150, 100, 255}, true)

			currentY += itemH + gap
		}
	}

	// Draw ships under construction
	for _, s := range island.Ships {
		if s.State == "UnderConstruction" && !s.FinishTime.IsZero() {
			remaining := s.FinishTime.Sub(now)
			if remaining > 0 {
				// Item box
				vector.DrawFilledRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), color.RGBA{40, 40, 60, 255}, true)
				vector.StrokeRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), 1, color.RGBA{120, 150, 200, 255}, true)

				// Ship name
				ebitenutil.DebugPrintAt(screen, "⛵ "+s.Type, int(dashX+20), int(currentY+10))

				// Timer
				mins := int(remaining.Minutes())
				secs := int(remaining.Seconds()) % 60
				timeStr := fmt.Sprintf("%02d:%02d", mins, secs)
				ebitenutil.DebugPrintAt(screen, timeStr, int(dashX+20), int(currentY+30))

				// Progress bar
				barW := dashW - 60
				barH := 8.0
				barX := dashX + 30
				barY := currentY + 45

				// Calculate progress using server-provided duration, or estimate if not available
				progress := 0.0
				totalDuration := s.ConstructionTotalDuration
				if totalDuration > 0 {
					// Use server-provided duration (accurate)
					startTime := s.FinishTime.Add(-time.Duration(totalDuration) * time.Second)
					elapsed := now.Sub(startTime).Seconds()
					progress = elapsed / totalDuration
				} else {
					// Fallback: show progress based on remaining time
					if remaining.Seconds() > 0 {
						if remaining.Minutes() < 1 {
							progress = 0.9
						} else if remaining.Minutes() < 5 {
							progress = 0.7
						} else {
							progress = 0.3
						}
					}
				}
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}

				vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{60, 60, 80, 255}, true)
				vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), color.RGBA{120, 180, 220, 255}, true)

				currentY += itemH + gap
			}
		}
	}

	// Draw militia recruitment in progress
	if island.MilitiaRecruiting && island.MilitiaRecruitDoneAt != nil {
		remaining := island.MilitiaRecruitDoneAt.Sub(now)
		if remaining > 0 {
			// Item box
			vector.DrawFilledRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), color.RGBA{40, 40, 60, 255}, true)
			vector.StrokeRect(screen, float32(dashX+10), float32(currentY), float32(dashW-20), float32(itemH), 1, color.RGBA{200, 100, 100, 255}, true)

			// Recruitment info
			totalUnits := island.MilitiaRecruitWarriors + island.MilitiaRecruitArchers + island.MilitiaRecruitGunners
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("👥 Recrutement (%d)", totalUnits), int(dashX+20), int(currentY+10))

			// Timer
			mins := int(remaining.Minutes())
			secs := int(remaining.Seconds()) % 60
			timeStr := fmt.Sprintf("%02d:%02d", mins, secs)
			ebitenutil.DebugPrintAt(screen, timeStr, int(dashX+20), int(currentY+30))

			// Progress bar (estimate 5 minutes for recruitment)
			barW := dashW - 60
			barH := 8.0
			barX := dashX + 30
			barY := currentY + 45

			// Estimate total duration (5 minutes = 300 seconds)
			totalDuration := 300.0
			// Calculate start time: DoneAt - totalDuration
			startTime := island.MilitiaRecruitDoneAt.Add(-time.Duration(totalDuration) * time.Second)
			elapsed := now.Sub(startTime).Seconds()
			progress := elapsed / totalDuration
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}

			vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{60, 60, 80, 255}, true)
			vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), color.RGBA{200, 120, 120, 255}, true)

			currentY += itemH + gap
		}
	}
}

// DrawDevMenu renders the developer diagnostics menu.
func (g *Game) DrawDevMenu(screen *ebiten.Image) {
	if !g.showDevMenu {
		return
	}
	w := 400.0
	h := 300.0
	vector.DrawFilledRect(screen, 10, 100, float32(w), float32(h), color.RGBA{0, 0, 80, 200}, true)
	ebitenutil.DebugPrintAt(screen, "--- DEV MENU ---", 20, 110)
	ebitenutil.DebugPrintAt(screen, "FPS: "+fmt.Sprintf("%.1f", ebiten.ActualFPS()), 20, 130)
	ebitenutil.DebugPrintAt(screen, "TPS: "+fmt.Sprintf("%.1f", ebiten.ActualTPS()), 20, 145)
	ebitenutil.DebugPrintAt(screen, "State: "+fmt.Sprintf("%d", g.state), 20, 160)
}

// DrawBuildingTooltips renders tooltips when hovering over buildings.
func (g *Game) DrawBuildingTooltips(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	mx, my := ebiten.CursorPosition()
	screenW, screenH := float64(g.screenWidth), float64(g.screenHeight)

	// Check each building for hover
	for _, b := range g.player.Islands[0].Buildings {
		bx, by := b.X, b.Y
		sx := (bx-g.camX)*g.camZoom + screenW/2
		sy := (by-g.camY)*g.camZoom + screenH/2

		// Check if mouse is over building (64x64 icon)
		hoverRadius := 32.0 * g.camZoom
		if math.Abs(float64(mx)-sx) <= hoverRadius && math.Abs(float64(my)-sy) <= hoverRadius {
			// Draw tooltip
			tooltipLines := []string{
				b.Type,
				fmt.Sprintf("Niveau %d", b.Level),
			}

			if b.Constructing {
				tooltipLines = append(tooltipLines, "En construction...")
			}

			// Draw tooltip box
			drawTooltip(screen, mx+15, my+15, tooltipLines)
			return // Only show one tooltip at a time
		}
	}
}
