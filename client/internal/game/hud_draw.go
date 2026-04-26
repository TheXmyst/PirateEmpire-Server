package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// hud_draw.go contains all HUD rendering/drawing code.
// This file was extracted from main.go during Phase 1.2 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// DrawHUD renders the top HUD bar with resources, player info, and the Fleet button.
func (g *Game) DrawHUD(screen *ebiten.Image) {
	if g.player != nil && len(g.player.Islands) > 0 {
		screenW := float64(g.screenWidth)
		hudH := 50.0
		hudY := 5.0

		// Drop Shadow
		shadowH := 20
		startAlpha := 90.0
		shadowY := int(hudY+hudH) - 5
		for i := 0; i < shadowH; i++ {
			alpha := uint8(startAlpha * (1.0 - float64(i)/float64(shadowH)))
			vector.DrawFilledRect(screen, 0, float32(shadowY+i), float32(screenW), 1, color.RGBA{0, 0, 0, alpha}, true)
		}

		draw9Slice(screen, g, 0, hudY, screenW, hudH, 16)
		limits := g.player.Islands[0].StorageLimits
		resY := hudY + 10.0

		drawResource(screen, g.iconWood, g.visualResources[domain.Wood], limits[domain.Wood], 20, resY)
		drawResource(screen, g.iconStone, g.visualResources[domain.Stone], limits[domain.Stone], 190, resY)
		drawResource(screen, g.iconRum, g.visualResources[domain.Rum], limits[domain.Rum], 360, resY)
		drawResource(screen, g.iconGold, g.visualResources[domain.Gold], limits[domain.Gold], 530, resY)

		infoW := 350.0
		infoX := screenW - infoW
		sepX := float32(infoX) - 20
		vector.StrokeLine(screen, sepX, float32(hudY)+10, sepX, float32(hudY+hudH)-10, 2, color.RGBA{139, 69, 19, 150}, true)
		vector.StrokeLine(screen, sepX+1, float32(hudY)+10, sepX+1, float32(hudY+hudH)-10, 1, color.RGBA{255, 215, 0, 100}, true)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Captain: %s | Island: %s", g.player.Username, g.player.Islands[0].Name), int(infoX), int(hudY)+18)

		// Resource Tooltips
		mx, my := ebiten.CursorPosition()
		cmx, cmy := float64(mx), float64(my)
		if cmy >= hudY && cmy <= hudY+hudH {
			var tooltipRes domain.ResourceType
			if cmx >= 20 && cmx < 170 {
				tooltipRes = domain.Wood
			} else if cmx >= 190 && cmx < 340 {
				tooltipRes = domain.Stone
			} else if cmx >= 360 && cmx < 510 {
				tooltipRes = domain.Rum
			} else if cmx >= 530 && cmx < 680 {
				tooltipRes = domain.Gold
			}

			if tooltipRes != "" {
				total, _, _, details := g.calculateProduction(tooltipRes)
				var lines []string
				lines = append(lines, fmt.Sprintf("%s Generation", strings.ToUpper(string(tooltipRes)[:1])+string(tooltipRes)[1:]))
				lines = append(lines, fmt.Sprintf("Stock: %.0f / %.0f", g.visualResources[tooltipRes], limits[tooltipRes]))
				lines = append(lines, "----------------")
				lines = append(lines, fmt.Sprintf("Total: %.2f / h", total))
				if len(details) > 0 {
					lines = append(lines, "----------------")
					lines = append(lines, details...)
				}
				drawTooltip(screen, mx+10, my+20, lines)
			}
		}

		// Sea and Fleet Buttons - positioned to the left of the build button (bottom right)
		// Build button is at: x = screenW - 100 - 20, y = h - 100 - 20
		screenH := float64(g.screenHeight)
		btnSize := 100.0
		btnGap := 10.0
		margin := 20.0
		buildBtnX := screenW - btnSize - margin
		buildBtnY := screenH - btnSize - margin

		// Fleet button is to the left of the build button
		fleetBtnX := buildBtnX - btnSize - btnGap
		fleetBtnY := buildBtnY

		// Sea button is to the left of the fleet button
		seaBtnX := fleetBtnX - btnSize - btnGap
		seaBtnY := buildBtnY

		// Captain button is to the left of the sea button
		captainBtnX := seaBtnX - btnSize - btnGap
		captainBtnY := buildBtnY

		// Social button is to the left of the captain button
		socialBtnX := captainBtnX - btnSize - btnGap
		socialBtnY := buildBtnY

		// Helper for drawing textured buttons
		drawBtn := func(x, y, size, scale float64, label string, hover, pressed bool, colorFallback color.Color) {
			scaledSize := size * scale
			offsetX := (scaledSize - size) / 2
			offsetY := (scaledSize - size) / 2

			// Select Texture
			var img *ebiten.Image
			if pressed {
				img = g.btnPressed
			} else if hover {
				img = g.btnHover
			} else {
				img = g.btnNormal
			}

			if img != nil {
				drawImgScaled(screen, img, x-offsetX, y-offsetY, scaledSize, scaledSize)
				// Small stroke for visibility on dark maps
				vector.StrokeRect(screen, float32(x-offsetX), float32(y-offsetY), float32(scaledSize), float32(scaledSize), 1, color.RGBA{255, 215, 0, 100}, true)
			} else {
				// Fallback
				vector.DrawFilledRect(screen, float32(x-offsetX), float32(y-offsetY), float32(scaledSize), float32(scaledSize), colorFallback, true)
				vector.StrokeRect(screen, float32(x-offsetX), float32(y-offsetY), float32(scaledSize), float32(scaledSize), 2, color.RGBA{255, 255, 255, 200}, true)
			}

			// Draw Label
			ebitenutil.DebugPrintAt(screen, label, int(x+20), int(y+40))
		}

		// Draw Social Button (teal fallback)
		socialCol := color.RGBA{0, 160, 160, 255}
		if g.ui.ShowSocialUI {
			socialCol = color.RGBA{0, 190, 190, 255}
		}
		drawBtn(socialBtnX, socialBtnY, btnSize, g.ui.SocialButtonScale, "SOC", g.ui.HoverSocialButton, false, socialCol)

		// Draw Captain Button (purple/violet fallback)
		captainCol := color.RGBA{150, 100, 200, 255}
		drawBtn(captainBtnX, captainBtnY, btnSize, g.ui.CaptainButtonScale, "CAP", g.ui.HoverCaptainButton, false, captainCol)

		// Draw Sea Button (blue fallback)
		seaCol := color.RGBA{0, 100, 200, 255}
		seaLabel := "MER"
		if g.state == StateWorldMap {
			seaLabel = "ILE"
		}
		drawBtn(seaBtnX, seaBtnY, btnSize, g.ui.SeaButtonScale, seaLabel, g.ui.HoverSeaButton, false, seaCol)

		// Draw Fleet Button (Green Fallback)
		fleetCol := color.RGBA{0, 150, 0, 255}
		if g.ui.ShowFleetUI {
			fleetCol = color.RGBA{0, 200, 0, 255}
		}
		drawBtn(fleetBtnX, fleetBtnY, btnSize, g.ui.FleetButtonScale, "FLOTTE", g.ui.HoverFleetButton, false, fleetCol)
	}
}

// drawResource draws a single resource icon and amount text in the HUD.
func drawResource(screen *ebiten.Image, icon *ebiten.Image, amount, maxAmount float64, x, y float64) {
	if icon != nil {
		op := &ebiten.DrawImageOptions{}
		bounds := icon.Bounds()
		w := bounds.Dx()
		s := 32.0 / float64(w)
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(x, y)
		screen.DrawImage(icon, op)
	}
	text := fmt.Sprintf("%.0f / %.0f", amount, maxAmount)
	if amount >= maxAmount && maxAmount > 0 {
		text += " (FULL)"
	}
	drawTextWithBackground(screen, text, int(x)+35, int(y)+10, color.RGBA{30, 40, 50, 255})
}

// DrawBuildButton renders the large construction button in the bottom right corner.
func (g *Game) DrawBuildButton(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	sw := float64(g.screenWidth)
	sh := float64(g.screenHeight)

	btnSize := 100.0
	margin := 20.0
	x := sw - btnSize - margin
	y := sh - btnSize - margin

	// Select Texture based on hover state
	var img *ebiten.Image
	if g.ui.HoverBuildButton {
		img = g.btnHover
	} else {
		img = g.btnNormal
	}

	scale := g.ui.BuildButtonScale
	if scale == 0 {
		scale = 1.0
	}

	scaledSize := btnSize * scale
	offsetX := (scaledSize - btnSize) / 2
	offsetY := (scaledSize - btnSize) / 2

	if img != nil {
		drawImgScaled(screen, img, x-offsetX, y-offsetY, scaledSize, scaledSize)
	} else {
		// Fallback if texture missing
		vector.DrawFilledRect(screen, float32(x-offsetX), float32(y-offsetY), float32(scaledSize), float32(scaledSize), color.RGBA{139, 69, 19, 255}, true)
	}

	// Draw Build Icon (Hammer) on top
	if g.btnBuild != nil {
		op := &ebiten.DrawImageOptions{}
		bounds := g.btnBuild.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		s := (btnSize * 0.6) / float64(w)
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(x+(btnSize-(float64(w)*s))/2, y+(btnSize-(float64(h)*s))/2)
		screen.DrawImage(g.btnBuild, op)
	}

	ebitenutil.DebugPrintAt(screen, "BUILD", int(x)+30, int(y)+75)
}
