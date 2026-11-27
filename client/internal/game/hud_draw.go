package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// hud_draw.go : l'interface posée sur l'île et sur la mer.
//   - en haut à gauche : la carte du capitaine ;
//   - en haut au centre : les ressources (stock et production par heure) ;
//   - en haut à droite : petits boutons ronds (voir touch_ui.go) ;
//   - en bas au centre : la barre de navigation (Construire au milieu) ;
//   - en bas à gauche : la bulle du chat (voir chat_ui.go).

type hudRect struct{ x, y, w, h float64 }

func (r hudRect) has(x, y float64) bool {
	return x >= r.x && x <= r.x+r.w && y >= r.y && y <= r.y+r.h
}

type dockButton struct {
	id, label, icon string
	r               hudRect
	big, active     bool
}

const (
	hudMargin   = 12.0
	dockBtnW    = 92.0
	dockBtnH    = 76.0
	dockBigW    = 116.0
	dockBigH    = 96.0
	dockGap     = 10.0
	dockPad     = 10.0
	resChipW    = 146.0
	resChipH    = 46.0
	resBarPad   = 6.0
	topBarH     = resChipH + 2*resBarPad
	captainCard = topBarH
)

func (g *Game) hudVisible() bool {
	return g.player != nil && len(g.player.Islands) > 0 && (g.state == StatePlaying || g.state == StateWorldMap)
}

// dockButtons : les boutons de la barre du bas, avec leur position.
func (g *Game) dockButtons() []dockButton {
	onSea := g.state == StateWorldMap
	btns := []dockButton{
		{id: "fleet", label: "Flotte", icon: "anchor", active: g.ui.ShowFleetUI},
		{id: "sea", label: "Mer", icon: "map"},
		{id: "build", label: "Construire", icon: "hammer", big: true, active: g.ui.ShowConstruction},
		{id: "captain", label: "Capitaines", icon: "hat", active: g.ui.ShowCaptainUI},
		{id: "social", label: "Social", icon: "people", active: g.ui.ShowSocialUI},
	}
	if onSea {
		btns[1].label, btns[1].icon = "Île", "island"
		// Pas de construction en mer.
		btns = append(btns[:2], btns[3:]...)
	}
	total := 2 * dockPad
	for i, b := range btns {
		if i > 0 {
			total += dockGap
		}
		if b.big {
			total += dockBigW
		} else {
			total += dockBtnW
		}
	}
	dock := g.dockRect()
	x := float64(g.screenWidth)/2 - total/2 + dockPad
	for i := range btns {
		if btns[i].big {
			btns[i].r = hudRect{x, dock.y + dock.h - dockPad - dockBigH, dockBigW, dockBigH}
			x += dockBigW + dockGap
		} else {
			btns[i].r = hudRect{x, dock.y + dockPad, dockBtnW, dockBtnH}
			x += dockBtnW + dockGap
		}
	}
	return btns
}

// dockRect : le fond de la barre du bas.
func (g *Game) dockRect() hudRect {
	n := 4
	wide := 0.0
	if g.state != StateWorldMap {
		wide = dockBigW + dockGap
	}
	w := 2*dockPad + float64(n)*dockBtnW + float64(n-1)*dockGap + wide
	h := dockBtnH + 2*dockPad
	return hudRect{float64(g.screenWidth)/2 - w/2, float64(g.screenHeight) - hudMargin - h, w, h}
}

// resourceRects : la barre des ressources et ses 4 cases.
func (g *Game) resourceRects() (bar hudRect, chips [4]hudRect) {
	w := 4*resChipW + 3*6 + 2*resBarPad
	bar = hudRect{float64(g.screenWidth)/2 - w/2, hudMargin, w, topBarH}
	for i := range chips {
		chips[i] = hudRect{bar.x + resBarPad + float64(i)*(resChipW+6), bar.y + resBarPad, resChipW, resChipH}
	}
	return
}

func (g *Game) captainCardRect() hudRect {
	name := ellipsize(g.player.Username, titleFace(22), 200)
	sub := g.captainCardSubtitle()
	w := math.Max(textWidth(name, titleFace(22)), textWidth(sub, bodyFace(12, false)))
	return hudRect{hudMargin, hudMargin, math.Min(captainCard+8+w+18, 300), captainCard}
}

func (g *Game) captainCardSubtitle() string {
	isl := g.player.Islands[0]
	lvl := 0
	for _, b := range isl.Buildings {
		if b.Type == "Hôtel de Ville" {
			lvl = b.Level
		}
	}
	name := isl.Name
	if name == "" {
		name = "Mon île"
	}
	if lvl > 0 {
		return fmt.Sprintf("%s · Niv. %d", name, lvl)
	}
	return name
}

// hudHit : le point (x, y) tombe-t-il sur l'interface (pour ne pas cliquer
// l'île ou la mer à travers) ?
func (g *Game) hudHit(x, y float64) bool {
	if !g.hudVisible() {
		return false
	}
	if g.dockRect().has(x, y) {
		return true
	}
	for _, b := range g.dockButtons() {
		if b.r.has(x, y) {
			return true
		}
	}
	if bar, _ := g.resourceRects(); bar.has(x, y) {
		return true
	}
	if g.captainCardRect().has(x, y) {
		return true
	}
	if g.questTrackerVisible() && g.questTrackerRect().has(x, y) {
		return true
	}
	if _, ok := g.touchButtonAt(x, y); ok {
		return true
	}
	return g.chatHit(x, y)
}

// DrawHUD dessine le haut de l'écran et la barre du bas.
func (g *Game) DrawHUD(screen *ebiten.Image) {
	if !g.hudVisible() {
		return
	}
	g.drawCaptainCard(screen)
	g.drawResourceBar(screen)
	g.drawDock(screen)
}

func (g *Game) drawCaptainCard(screen *ebiten.Image) {
	r := g.captainCardRect()
	drawThemePanel(screen, r.x, r.y, r.w, r.h, r.h/2)
	cx, cy := r.x+r.h/2, r.y+r.h/2
	rad := float32(r.h/2 - 7)
	vector.DrawFilledCircle(screen, float32(cx), float32(cy), rad, color.RGBA{110, 66, 30, 255}, vecAA)
	vector.StrokeCircle(screen, float32(cx), float32(cy), rad, 2, themeGold, vecAA)
	initial := "?"
	if n := []rune(g.player.Username); len(n) > 0 {
		initial = strings.ToUpper(string(n[0]))
	}
	drawText(screen, initial, titleFace(26), cx, cy-16, themeGold, alignCenter)
	tx := r.x + r.h + 8
	drawText(screen, ellipsize(g.player.Username, titleFace(22), 200), titleFace(22), tx, r.y+6, themeGold, alignLeft)
	drawText(screen, g.captainCardSubtitle(), bodyFace(12, false), tx, r.y+34, themeText, alignLeft)
}

var hudResources = [4]domain.ResourceType{domain.Wood, domain.Stone, domain.Rum, domain.Gold}

func (g *Game) drawResourceBar(screen *ebiten.Image) {
	bar, chips := g.resourceRects()
	drawThemePanel(screen, bar.x, bar.y, bar.w, bar.h, 16)
	island := g.player.Islands[0]
	icons := [4]*ebiten.Image{g.iconWood, g.iconStone, g.iconRum, g.iconGold}
	mx, my := cursorPosition()
	hovered := -1
	for i, res := range hudResources {
		c := chips[i]
		hover := c.has(float64(mx), float64(my))
		if hover {
			hovered = i
		}
		bg := themePanelHi
		if hover {
			bg = color.NRGBA{255, 255, 255, 32}
		}
		fillRoundRect(screen, c.x, c.y, c.w, c.h, 10, bg)
		if icons[i] != nil {
			drawImgScaled(screen, icons[i], c.x+5, c.y+(c.h-34)/2, 34, 34)
		}
		amount := g.visualResources[res]
		limit := island.StorageLimits[res]
		tx := c.x + 46
		val := formatThousands(amount)
		valClr := color.Color(themeText)
		full := limit > 0 && amount >= limit
		if full {
			valClr = themeOrange
		}
		drawText(screen, val, bodyFace(16, true), tx, c.y+5, valClr, alignLeft)
		if limit > 0 {
			drawText(screen, "/ "+formatThousands(limit), bodyFace(11, false), tx+textWidth(val, bodyFace(16, true))+4, c.y+9, themeMuted, alignLeft)
		}
		prod := island.ResourceGeneration[res]
		sub, subClr := fmt.Sprintf("+%s/h", formatThousands(prod)), color.Color(themeGreen)
		if full {
			sub, subClr = "Plein", themeOrange
		} else if prod <= 0 {
			sub, subClr = "+0/h", themeMuted
		}
		drawText(screen, sub, bodyFace(11, false), tx, c.y+27, subClr, alignLeft)
	}

	// Détail de la production au survol (souris).
	if hovered >= 0 && !pointer.usingTouch {
		res := hudResources[hovered]
		total, _, _, details := g.calculateProduction(res)
		name := strings.ToUpper(string(res)[:1]) + string(res)[1:]
		lines := []string{
			name,
			fmt.Sprintf("Stock : %s / %s", formatThousands(g.visualResources[res]), formatThousands(island.StorageLimits[res])),
			fmt.Sprintf("Production : %s / h", formatThousands(total)),
		}
		if len(details) > 1 {
			lines = append(lines, details[1:]...)
		}
		drawTooltip(screen, mx+12, my+20, lines)
	}
}

func (g *Game) drawDock(screen *ebiten.Image) {
	d := g.dockRect()
	drawThemePanel(screen, d.x, d.y, d.w, d.h, 22)
	mx, my := cursorPosition()
	blocked := g.IsModalOpen()
	for _, b := range g.dockButtons() {
		hover := !blocked && b.r.has(float64(mx), float64(my))
		r := b.r
		if b.big {
			fillRoundRect(screen, r.x-3, r.y-3, r.w+6, r.h+6, 21, color.RGBA{0, 0, 0, 90})
			top := themeGold
			if hover || b.active {
				top = color.RGBA{243, 207, 122, 255}
			}
			fillRoundRect(screen, r.x, r.y, r.w, r.h, 18, themeGoldDark)
			fillRoundRect(screen, r.x, r.y, r.w, r.h-8, 18, top)
			strokeRoundRect(screen, r.x, r.y, r.w, r.h, 18, 2, color.RGBA{243, 207, 122, 255})
			drawIcon(screen, b.icon, r.x+r.w/2, r.y+31, 40, themeInk)
			drawText(screen, b.label, titleFace(22), r.x+r.w/2, r.y+r.h-40, themeInk, alignCenter)
			continue
		}
		bg := themePanelHi
		fg := color.Color(themeText)
		if b.active {
			bg = color.NRGBA{224, 176, 74, 60}
			fg = themeGold
		} else if hover {
			bg = color.NRGBA{255, 255, 255, 34}
		}
		fillRoundRect(screen, r.x, r.y, r.w, r.h, 14, bg)
		if b.active {
			strokeRoundRect(screen, r.x, r.y, r.w, r.h, 14, 2, themeGold)
		}
		drawIcon(screen, b.icon, r.x+r.w/2, r.y+28, 32, fg)
		drawText(screen, b.label, bodyFace(13, true), r.x+r.w/2, r.y+r.h-24, fg, alignCenter)
	}
}

// DrawBuildButton : Construire fait maintenant partie de la barre du bas.
func (g *Game) DrawBuildButton(screen *ebiten.Image) {}
