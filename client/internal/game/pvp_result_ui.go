package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawPvpResultUI renders the PvP combat result modal (reuses PvE result structure)
func (g *Game) DrawPvpResultUI(screen *ebiten.Image) {
	if !g.ui.ShowPvpResultUI || g.ui.PvpCombatResult == nil {
		return
	}

	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())

	// Modal dimensions
	modalW := 700.0
	modalH := 600.0
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2

	// Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 200}, true)

	// Modal box
	vector.DrawFilledRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), color.RGBA{40, 40, 60, 255}, true)
	vector.StrokeRect(screen, float32(modalX), float32(modalY), float32(modalW), float32(modalH), 3, color.RGBA{200, 50, 50, 255}, true)

	// Title
	result := g.ui.PvpCombatResult
	title := "⚔️ VICTOIRE !"
	titleColor := color.RGBA{50, 200, 50, 255}
	if result.Winner != "fleet_a" {
		title = "💀 DÉFAITE"
		titleColor = color.RGBA{200, 50, 50, 255}
	}

	// Title background
	vector.DrawFilledRect(screen, float32(modalX), float32(modalY), float32(modalW), 50, titleColor, true)
	ebitenutil.DebugPrintAt(screen, title, int(modalX)+280, int(modalY)+18)

	// Close button
	closeX := modalX + modalW - 40
	closeY := modalY + 10
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)
	hoverClose := fmx >= closeX && fmx <= closeX+30 && fmy >= closeY && fmy <= closeY+30

	closeColor := color.RGBA{180, 30, 30, 255}
	if hoverClose {
		closeColor = color.RGBA{220, 50, 50, 255}
	}
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), 30, 30, closeColor, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+10)

	if hoverClose && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpResultUI = false
		if g.seaDMS != nil {
			g.seaDMS.SetMode(DMSCalm, "pvp_result_closed_btn")
		}
		g.ui.PvpCombatResult = nil
		g.ui.PvpLoot = nil
		return
	}

	// Content area
	contentY := modalY + 70

	// Combat summary
	ebitenutil.DebugPrintAt(screen, "Résultat du Combat PvP", int(modalX)+30, int(contentY))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Navires détruits (vous): %d", len(result.ShipsDestroyedA)), int(modalX)+30, int(contentY+25))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Navires détruits (ennemi): %d", len(result.ShipsDestroyedB)), int(modalX)+30, int(contentY+50))

	// Loot section (only if victory)
	if result.Winner == "fleet_a" && g.ui.PvpLoot != nil {
		lootY := contentY + 100
		ebitenutil.DebugPrintAt(screen, "🏆 BUTIN RÉCUPÉRÉ:", int(modalX)+30, int(lootY))

		yOffset := lootY + 30
		for res, amount := range g.ui.PvpLoot {
			if amount > 0 {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("  • %s: %.0f", res, amount), int(modalX)+40, int(yOffset))
				yOffset += 20
			}
		}
	}

	// OK button
	btnY := modalY + modalH - 70
	btnX := modalX + (modalW-150)/2
	hoverOK := fmx >= btnX && fmx <= btnX+150 && fmy >= btnY && fmy <= btnY+40

	okColor := color.RGBA{50, 150, 50, 255}
	if hoverOK {
		okColor = color.RGBA{70, 180, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), 150, 40, okColor, true)
	ebitenutil.DebugPrintAt(screen, "OK", int(btnX)+65, int(btnY)+15)

	if hoverOK && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.ui.ShowPvpResultUI = false
		if g.seaDMS != nil {
			g.seaDMS.SetMode(DMSCalm, "pvp_result_ok")
		}
		g.ui.PvpCombatResult = nil
		g.ui.PvpLoot = nil
	}
}
