package game

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// sea_nav_ui.go : déplacement libre en mer.
//
// Un vrai clic (ou un tap) sur l'eau ne déclenche plus de grande fenêtre :
// il pose un petit repère doré et un bouton « Naviguer ici ». Rien ne part
// tant que le joueur n'appuie pas sur ce bouton. Le repère disparaît si on
// fait glisser la carte, si on clique ailleurs ou si on change de vue.
// Glisser au clic gauche (PC) ou au doigt déplace la carte sans rien proposer.

const (
	seaNavBtnW = 168.0
	seaNavBtnH = 44.0
)

// seaMapTap : un clic « sur la carte » vient d'avoir lieu (bouton relâché ou
// doigt levé sans avoir glissé, et l'appui avait commencé sur la carte).
func (g *Game) seaMapTap() bool {
	if !g.ui.SeaPressOnMap {
		return false
	}
	if pointer.usingTouch {
		return pointer.justPressed
	}
	return pointer.mouseTap
}

// seaMapDragging : le joueur est en train de faire glisser la carte.
func (g *Game) seaMapDragging() bool {
	if pointer.usingTouch {
		return pointer.panDX != 0 || pointer.panDY != 0 || pointer.zoomFactor != 1
	}
	return g.ui.SeaPressOnMap && pointer.mouseDragged && (pointer.mouseDX != 0 || pointer.mouseDY != 0)
}

// updateSeaMapDrag : glisser au clic gauche déplace la carte (PC). Au doigt,
// c'est déjà géré par touchCamera.
func (g *Game) updateSeaMapDrag() {
	if g.seaMapDragging() {
		if !pointer.usingTouch {
			g.camX -= pointer.mouseDX / g.camZoom
			g.camY -= pointer.mouseDY / g.camZoom
		}
		g.clearSeaNavMarker()
	}
}

func (g *Game) clearSeaNavMarker() {
	g.ui.ShowSeaNavMarker = false
}

// seaNavButtonRect : bouton « Naviguer ici » sous le repère, gardé dans l'écran.
func (g *Game) seaNavButtonRect() (x, y, w, h float64) {
	sx, sy := g.seaToScreen(float64(g.ui.NavFreeConfirmTargetX), float64(g.ui.NavFreeConfirmTargetY))
	w, h = seaNavBtnW, seaNavBtnH
	if g.ui.NavFreeConfirmType == "stop_chasing" {
		w += 70
	}
	x, y = sx-w/2, sy+22
	W, H := float64(g.screenWidth), float64(g.screenHeight)
	x = math.Max(12, math.Min(x, W-w-12))
	y = math.Max(80, math.Min(y, H-h-130)) // au-dessus de la barre du bas
	return
}

func (g *Game) seaNavButtonHit(mx, my float64) bool {
	if !g.ui.ShowSeaNavMarker {
		return false
	}
	x, y, w, h := g.seaNavButtonRect()
	return mx >= x && mx <= x+w && my >= y && my <= y+h
}

// drawSeaNavMarker : repère doré qui pulse + bouton « Naviguer ici ».
func (g *Game) drawSeaNavMarker(screen *ebiten.Image) {
	if !g.ui.ShowSeaNavMarker {
		return
	}
	sx, sy := g.seaToScreen(float64(g.ui.NavFreeConfirmTargetX), float64(g.ui.NavFreeConfirmTargetY))
	t := float64(time.Now().UnixMilli()%1200) / 1200
	pulse := float32(10 + 10*t)
	vector.StrokeCircle(screen, float32(sx), float32(sy), pulse, 2, colorAlpha(themeGold, uint8(220*(1-t))), vecAA)
	vector.DrawFilledCircle(screen, float32(sx), float32(sy+2), 7, colorAlpha(themeInk, 120), vecAA)
	vector.DrawFilledCircle(screen, float32(sx), float32(sy), 6, themeGold, vecAA)
	vector.StrokeCircle(screen, float32(sx), float32(sy), 6, 1.5, themeInk, vecAA)

	x, y, w, h := g.seaNavButtonRect()
	label := "Naviguer ici"
	if g.ui.NavFreeConfirmType == "stop_chasing" {
		label = "Lâcher la cible et y aller"
	}
	drawButton(screen, x, y, w, h, label, btnPrimary)
}

func colorAlpha(c color.RGBA, a uint8) color.NRGBA {
	return color.NRGBA{c.R, c.G, c.B, a}
}
