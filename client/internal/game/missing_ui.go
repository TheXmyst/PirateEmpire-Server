package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
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

// DrawDashboard : le panneau « En cours » (constructions, recherche,
// navires, recrutement), à gauche sous la carte du capitaine.
func (g *Game) DrawDashboard(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	island := g.player.Islands[0]
	now := time.Now()

	type job struct {
		label     string
		remaining time.Duration
		progress  float64
	}
	var jobs []job
	// progressOf : avancement à partir de la durée totale (en secondes) ;
	// sans durée connue, une estimation grossière comme avant.
	progressOf := func(finish time.Time, total float64, remaining time.Duration) float64 {
		p := 0.0
		if total > 0 {
			start := finish.Add(-time.Duration(total) * time.Second)
			p = now.Sub(start).Seconds() / total
		} else if remaining.Minutes() < 1 {
			p = 0.9
		} else if remaining.Minutes() < 5 {
			p = 0.7
		} else {
			p = 0.3
		}
		return math.Max(0, math.Min(1, p))
	}

	for _, b := range island.Buildings {
		if b.Constructing && !b.FinishTime.IsZero() && b.FinishTime.After(now) {
			rem := b.FinishTime.Sub(now)
			jobs = append(jobs, job{b.Type, rem, progressOf(b.FinishTime, b.ConstructionTotalDuration, rem)})
		}
	}
	if g.player.ResearchingTechID != "" && !g.player.ResearchFinishTime.IsZero() && g.player.ResearchFinishTime.After(now) {
		rem := g.player.ResearchFinishTime.Sub(now)
		p := 0.0
		if g.player.ResearchTotalDurationSeconds > 0 {
			p = progressOf(g.player.ResearchFinishTime, g.player.ResearchTotalDurationSeconds, rem)
		}
		jobs = append(jobs, job{"Recherche : " + g.player.ResearchingTechID, rem, p})
	}
	for _, s := range island.Ships {
		if s.State == "UnderConstruction" && !s.FinishTime.IsZero() && s.FinishTime.After(now) {
			rem := s.FinishTime.Sub(now)
			jobs = append(jobs, job{"Navire : " + s.Type, rem, progressOf(s.FinishTime, s.ConstructionTotalDuration, rem)})
		}
	}
	if island.MilitiaRecruiting && island.MilitiaRecruitDoneAt != nil && island.MilitiaRecruitDoneAt.After(now) {
		rem := island.MilitiaRecruitDoneAt.Sub(now)
		total := island.MilitiaRecruitWarriors + island.MilitiaRecruitArchers + island.MilitiaRecruitGunners
		// Durée estimée : 5 minutes.
		jobs = append(jobs, job{fmt.Sprintf("Recrutement (%d)", total), rem, progressOf(*island.MilitiaRecruitDoneAt, 300, rem)})
	}
	if len(jobs) == 0 {
		return
	}

	const w, rowH = 250.0, 44.0
	x := hudMargin
	y := hudMargin + topBarH + 10
	h := 40 + float64(len(jobs))*rowH
	drawThemePanel(screen, x, y, w, h, 14)
	drawText(screen, "En cours", titleFace(20), x+12, y+6, themeGold, alignLeft)
	fLabel, fTime := bodyFace(13, false), bodyFace(13, true)
	ry := y + 36
	for _, j := range jobs {
		secs := int(j.remaining.Seconds())
		var t string
		if secs >= 3600 {
			t = fmt.Sprintf("%d:%02d:%02d", secs/3600, (secs/60)%60, secs%60)
		} else {
			t = fmt.Sprintf("%02d:%02d", secs/60, secs%60)
		}
		tw := textWidth(t, fTime)
		drawText(screen, ellipsize(j.label, fLabel, w-24-tw-8), fLabel, x+12, ry, themeText, alignLeft)
		drawText(screen, t, fTime, x+w-12, ry, themeText, alignRight)
		bx, bw := x+12, w-24
		fillRoundRect(screen, bx, ry+22, bw, 6, 3, color.NRGBA{255, 255, 255, 30})
		if j.progress > 0 {
			fillRoundRect(screen, bx, ry+22, math.Max(6, bw*j.progress), 6, 3, themeGold)
		}
		ry += rowH
	}
}

// DrawDevMenu renders the developer diagnostics menu.
func (g *Game) DrawDevMenu(screen *ebiten.Image) {
	if !g.showDevMenu {
		return
	}
	w := 400.0
	h := 300.0
	vector.DrawFilledRect(screen, 10, 100, float32(w), float32(h), color.RGBA{0, 0, 80, 200}, vecAA)
	uiPrint(screen, "--- DEV MENU ---", 20, 110)
	uiPrint(screen, "FPS: "+fmt.Sprintf("%.1f", ebiten.ActualFPS()), 20, 130)
	uiPrint(screen, "TPS: "+fmt.Sprintf("%.1f", ebiten.ActualTPS()), 20, 145)
	uiPrint(screen, "State: "+fmt.Sprintf("%d", g.state), 20, 160)
}

// DrawBuildingTooltips renders tooltips when hovering over buildings.
func (g *Game) DrawBuildingTooltips(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	mx, my := cursorPosition()
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
