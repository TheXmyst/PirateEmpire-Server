package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawInterceptConfirmModal renders the confirmation dialog for PvP interception
func (g *Game) DrawInterceptConfirmModal(screen *ebiten.Image) {
	if !g.ui.ShowInterceptConfirm {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	modalW, modalH := 400.0, 250.0
	modalX, modalY := (w-modalW)/2, (h-modalH)/2

	// Shadow / Overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 180}, true)

	// Panel
	draw9Slice(screen, g, modalX, modalY, modalW, modalH, 16)

	// Title
	title := "INTERCEPTION"
	ebitenutil.DebugPrintAt(screen, title, int(modalX+modalW/2-40), int(modalY)+20)

	// Text
	text := fmt.Sprintf("Voulez-vous intercepter la flotte\n\"%s\" avec votre flotte \"%s\" ?",
		g.ui.InterceptTargetName, g.ui.InterceptAttackerName)
	ebitenutil.DebugPrintAt(screen, text, int(modalX+20), int(modalY)+60)

	note := "Note: La poursuite dure max 2 mins.\nCooldown de 10 mins si échec."
	ebitenutil.DebugPrintAt(screen, note, int(modalX+20), int(modalY)+110)

	if g.ui.InterceptError != "" {
		vector.DrawFilledRect(screen, float32(modalX+20), float32(modalY+150), float32(modalW-40), 40, color.RGBA{150, 0, 0, 200}, true)
		ebitenutil.DebugPrintAt(screen, g.ui.InterceptError, int(modalX+30), int(modalY)+155)
	}

	// Buttons
	btnW, btnH := 120.0, 40.0

	// CONFIRM
	confirmX, confirmY := modalX+40, modalY+modalH-60
	confirmColor := color.RGBA{0, 150, 0, 255}
	if g.ui.InterceptBusy {
		confirmColor = color.RGBA{50, 50, 50, 255}
	}
	vector.DrawFilledRect(screen, float32(confirmX), float32(confirmY), float32(btnW), float32(btnH), confirmColor, true)
	ebitenutil.DebugPrintAt(screen, "POURSUIVRE", int(confirmX+20), int(confirmY+12))

	// CANCEL
	cancelX, cancelY := modalX+modalW-160, modalY+modalH-60
	vector.DrawFilledRect(screen, float32(cancelX), float32(cancelY), float32(btnW), float32(btnH), color.RGBA{150, 0, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "ANNULER", int(cancelX+30), int(cancelY+12))

	// Interaction
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)

		if fmx >= cancelX && fmx <= cancelX+btnW && fmy >= cancelY && fmy <= cancelY+btnH {
			g.ui.ShowInterceptConfirm = false
			return
		}

		if !g.ui.InterceptBusy && fmx >= confirmX && fmx <= confirmX+btnW && fmy >= confirmY && fmy <= confirmY+btnH {
			g.confirmIntercept()
		}
	}
}

func (g *Game) confirmIntercept() {
	g.ui.InterceptBusy = true
	g.ui.InterceptError = ""

	go func() {
		defer func() { g.ui.InterceptBusy = false }()

		err := g.api.StartIntercept(g.ui.InterceptAttackerFleetID, g.ui.InterceptTargetFleetID)
		if err != nil {
			g.ui.InterceptError = err.Error()
			return
		}

		g.Log("[PVP] Interception lancée !")
		g.ui.ShowInterceptConfirm = false
		g.loadPlayerData()
		g.loadPvpTargets()
	}()
}

// DrawPursuitBanner renders a warning if the player is being chased
func (g *Game) DrawPursuitBanner(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	isChased := false
	for _, f := range g.player.Islands[0].Fleets {
		if f.ChasedByFleetID != nil {
			isChased = true
			break
		}
	}

	if !isChased {
		return
	}

	w := float64(g.screenWidth)
	bannerH := 40.0
	bannerY := 60.0 // Just below Top HUD

	vector.DrawFilledRect(screen, 0, float32(bannerY), float32(w), float32(bannerH), color.RGBA{200, 0, 0, 200}, true)
	vector.StrokeLine(screen, 0, float32(bannerY), float32(w), float32(bannerY), 2, color.RGBA{255, 255, 0, 255}, true)
	vector.StrokeLine(screen, 0, float32(bannerY+bannerH), float32(w), float32(bannerY+bannerH), 2, color.RGBA{255, 255, 0, 255}, true)

	text := "!!! ALERTE : VOUS ÊTES POURSUIVI !!! REJOIGNEZ VOTRE PORT !"
	ebitenutil.DebugPrintAt(screen, text, int(w/2-180), int(bannerY+12))
}
