package game

// =============================================================================
//  ANIMATION DU RECRUTEMENT  (gacha_reveal.go)
// =============================================================================
//
// Après un recrutement à la Taverne, les capitaines arrivent en cartes face
// cachée, puis se retournent une par une :
//   - juste avant le retournement, la carte brille de la couleur de sa rareté ;
//   - rare : halo bleu ; légendaire : rayons dorés et étincelles ;
//   - UL : l'écran s'assombrit, éclair violet, secousse, puis une grande fiche
//     avec le nom et les talents.
// Un clic (ou un tap) révèle tout d'un coup ; un clic de plus ferme.
// Tout est calculé à partir de l'heure : aucun état à mettre à jour par frame.
// =============================================================================

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// gachaCard : un capitaine à révéler.
type gachaCard struct {
	Name      string
	Rarity    string
	Duplicate bool
	Shards    int
	Effects   []string
}

// gachaReveal : l'animation en cours.
type gachaReveal struct {
	cards     []gachaCard
	start     time.Time
	reveal    []float64 // instant (s depuis start) où la carte i commence à se retourner
	skipAt    float64   // >0 : tout a été révélé d'un coup à cet instant
	ulShown   map[int]bool
	ulOpen    int     // carte UL dont la grande fiche est affichée (-1 : aucune)
	lastInput float64 // instant du dernier clic traité
}

const (
	gachaArrive   = 0.35 // durée d'arrivée d'une carte
	gachaFlip     = 0.38 // durée du retournement
	gachaFirstGap = 0.75 // délai avant le premier retournement
)

// gachaTease : durée de la lueur avant retournement, selon la rareté.
func gachaTease(r string) float64 {
	switch r {
	case "ultra":
		return 1.1
	case "legendary":
		return 0.7
	case "rare":
		return 0.35
	}
	return 0.15
}

func newGachaReveal(cards []gachaCard) *gachaReveal {
	g := &gachaReveal{cards: cards, start: time.Now(), ulShown: map[int]bool{}, ulOpen: -1}
	t := gachaFirstGap + float64(len(cards))*0.05
	for _, c := range cards {
		t += gachaTease(c.Rarity)
		g.reveal = append(g.reveal, t)
		t += gachaFlip + 0.12
		if c.Rarity == "ultra" {
			t += 0.6 // laisser le temps à l'éclair
		}
	}
	return g
}

func (r *gachaReveal) now() float64 { return time.Since(r.start).Seconds() }

// flipProgress : 0 = face cachée, 1 = révélée.
func (r *gachaReveal) flipProgress(i int, t float64) float64 {
	if r.skipAt > 0 && t >= r.skipAt {
		return math.Min(1, (t-r.skipAt)/0.25+0.0001)
	}
	return clamp01((t - r.reveal[i]) / gachaFlip)
}

// teaseProgress : 0..1 pendant la lueur juste avant le retournement.
func (r *gachaReveal) teaseProgress(i int, t float64) float64 {
	if r.skipAt > 0 {
		return 0
	}
	start := r.reveal[i] - gachaTease(r.cards[i].Rarity)
	if t < start || t >= r.reveal[i] {
		return 0
	}
	return (t - start) / gachaTease(r.cards[i].Rarity)
}

func (r *gachaReveal) allRevealed(t float64) bool {
	if r.skipAt > 0 {
		return t >= r.skipAt+0.25
	}
	last := len(r.cards) - 1
	return last < 0 || t >= r.reveal[last]+gachaFlip
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// easeOutBack : petite surprise en fin de mouvement.
func easeOutBack(x float64) float64 {
	c1, c3 := 1.70158, 2.70158
	return 1 + c3*math.Pow(x-1, 3) + c1*math.Pow(x-1, 2)
}

func withAlpha(c color.Color, a float64) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(255 * clamp01(a))}
}

// -----------------------------------------------------------------------------
// Entrées
// -----------------------------------------------------------------------------

// updateGachaReveal : renvoie true tant que l'animation prend les clics.
func (g *Game) updateGachaReveal() bool {
	r := g.ui.TavernReveal
	if r == nil {
		return false
	}
	t := r.now()
	click := isMouseJustPressed(ebiten.MouseButtonLeft)
	esc := isKeyJustPressed(ebiten.KeyEscape)
	if !click && !esc {
		return true
	}
	// La taverne peut être mise à jour deux fois dans la même image : un seul
	// clic ne doit compter qu'une fois.
	if t-r.lastInput < 0.15 {
		return true
	}
	r.lastInput = t
	switch {
	case r.ulOpen >= 0:
		r.ulOpen = -1
		if esc {
			g.ui.TavernReveal = nil
		}
	case !r.allRevealed(t):
		r.skipAt = t
	default:
		g.ui.TavernReveal = nil
	}
	return true
}

// -----------------------------------------------------------------------------
// Dessin
// -----------------------------------------------------------------------------

func (g *Game) drawGachaReveal(screen *ebiten.Image) {
	r := g.ui.TavernReveal
	if r == nil {
		return
	}
	t := r.now()
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)

	// Fond sombre, plus sombre pendant l'attente d'un UL
	dark := 0.92
	for i, c := range r.cards {
		if c.Rarity == "ultra" {
			if tp := r.teaseProgress(i, t); tp > 0 {
				dark = math.Max(dark, 0.92+0.06*tp)
			}
		}
	}
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.NRGBA{8, 5, 3, uint8(255 * dark)}, false)

	// Disposition : 1 carte au centre, ou 2 rangées de 5
	n := len(r.cards)
	cols := n
	if cols > 5 {
		cols = 5
	}
	gap := 16.0
	// Écran étroit (téléphone en portrait) : moins de colonnes, cartes lisibles
	for cols > 2 && (sw-48-gap*float64(cols-1))/float64(cols) < 105 {
		cols--
	}
	rows := (n + cols - 1) / cols
	cardW := math.Min(170, (sw-48-gap*float64(cols-1))/float64(cols))
	if n == 1 {
		cardW = math.Min(230, sw*0.6)
	}
	cardH := cardW * 1.4
	if maxH := (sh - 170 - gap*float64(rows-1)) / float64(rows); cardH > maxH {
		cardH = maxH
		cardW = cardH / 1.4
	}
	totalW := float64(cols)*cardW + float64(cols-1)*gap
	totalH := float64(rows)*cardH + float64(rows-1)*gap
	x0 := sw/2 - totalW/2
	y0 := sh/2 - totalH/2 + 10

	// Secousse et éclair des UL
	shakeX, shakeY, flash := 0.0, 0.0, 0.0
	for i, c := range r.cards {
		if c.Rarity != "ultra" || r.skipAt > 0 {
			continue
		}
		dt := t - r.reveal[i]
		if dt >= 0 && dt < 0.7 {
			amp := 9 * (1 - dt/0.7)
			shakeX = math.Sin(t*70) * amp
			shakeY = math.Cos(t*53) * amp
			flash = math.Max(flash, 1-dt/0.5)
		}
	}

	title := "Recrutement"
	drawText(screen, title, titleFace(30), sw/2, y0-84, themeGold, alignCenter)

	for i, c := range r.cards {
		col, row := i%cols, i/cols
		cx := x0 + float64(col)*(cardW+gap) + cardW/2 + shakeX
		cy := y0 + float64(row)*(cardH+gap) + cardH/2 + shakeY

		// Arrivée : la carte monte depuis le bas
		at := clamp01((t - float64(i)*0.05) / gachaArrive)
		if at <= 0 {
			continue
		}
		cy += (1 - easeOutBack(at)) * 120
		alpha := at

		g.drawGachaCard(screen, r, i, c, cx, cy, cardW, cardH, t, alpha)

		// Une UL qui vient d'apparaître ouvre sa grande fiche (une fois)
		if c.Rarity == "ultra" && !r.ulShown[i] && r.flipProgress(i, t) >= 1 && r.skipAt == 0 {
			r.ulShown[i] = true
			r.ulOpen = i
		}
	}

	if flash > 0 {
		vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.NRGBA{214, 120, 255, uint8(170 * flash)}, false)
	}

	// Consigne en bas
	hint := "Touche pour tout révéler"
	if r.allRevealed(t) {
		hint = "Touche pour continuer"
	}
	pulse := 0.55 + 0.45*math.Sin(t*4)
	drawText(screen, hint, bodyFace(14, true), sw/2, y0+totalH+30, withAlpha(themeMuted, pulse), alignCenter)

	if r.ulOpen >= 0 {
		g.drawGachaULSpotlight(screen, r.cards[r.ulOpen], t)
	}
}

func (g *Game) drawGachaCard(screen *ebiten.Image, r *gachaReveal, i int, c gachaCard, cx, cy, w, h, t, alpha float64) {
	rc := rarityColor(c.Rarity)
	flip := r.flipProgress(i, t)
	tease := r.teaseProgress(i, t)
	revealed := flip >= 0.5

	// Effets derrière la carte (une fois révélée)
	if revealed {
		since := t - r.reveal[i]
		if r.skipAt > 0 {
			since = t - r.skipAt
		}
		switch c.Rarity {
		case "legendary", "ultra":
			drawGachaRays(screen, cx, cy, w*1.05, t, rc, c.Rarity == "ultra")
			drawGachaSparks(screen, cx, cy, w, since, rc, i)
		}
		if c.Rarity != "common" {
			for k := 0; k < 3; k++ {
				pad := 4 + float64(k)*6 + 2*math.Sin(t*3+float64(k))
				strokeRoundRect(screen, cx-w/2-pad, cy-h/2-pad, w+2*pad, h+2*pad, 14+pad, 3, withAlpha(rc, 0.35*alpha/float64(k+1)))
			}
		}
	}

	// Lueur de suspense avant retournement
	if tease > 0 {
		glow := tease * (0.6 + 0.4*math.Sin(t*18))
		for k := 0; k < 4; k++ {
			pad := 3 + float64(k)*5*tease
			strokeRoundRect(screen, cx-w/2-pad, cy-h/2-pad, w+2*pad, h+2*pad, 14+pad, 3, withAlpha(rc, glow/float64(k+1)))
		}
	}

	// Retournement : la carte s'écrase en largeur puis se rouvre
	sx := math.Abs(math.Cos(flip * math.Pi))
	if sx < 0.02 {
		return
	}
	cw := w * sx
	x, y := cx-cw/2, cy-h/2
	fillRoundRect(screen, x+3, y+5, cw, h, 14, color.NRGBA{0, 0, 0, uint8(110 * alpha)})

	if !revealed {
		// Dos : bois et or
		fillRoundRect(screen, x, y, cw, h, 14, withAlpha(color.RGBA{92, 58, 30, 255}, alpha))
		strokeRoundRect(screen, x+6*sx, y+6, cw-12*sx, h-12, 10, 2, withAlpha(themeGoldDim, alpha))
		strokeRoundRect(screen, x, y, cw, h, 14, 3, withAlpha(themeGold, alpha))
		if sx > 0.5 {
			drawText(screen, "?", titleFace(w*0.42), cx, cy-w*0.28, withAlpha(themeGold, alpha), alignCenter)
		}
		return
	}

	// Face : fond sombre, bord de la rareté
	fillRoundRect(screen, x, y, cw, h, 14, withAlpha(color.RGBA{40, 26, 16, 255}, alpha))
	fillRoundRect(screen, x, y, cw, h*0.42, 14, withAlpha(rc, 0.18*alpha))
	strokeRoundRect(screen, x, y, cw, h, 14, 3, withAlpha(rc, alpha))
	if sx < 0.6 {
		return
	}
	label := rarityLabel(c.Rarity)
	if c.Rarity == "ultra" {
		label = "UL"
	}
	fs := math.Max(10, w*0.075)
	drawText(screen, strings.ToUpper(label), bodyFace(fs, true), cx, y+h*0.05, rc, alignCenter)

	// Médaillon avec l'initiale
	mr := w * 0.2
	my := y + h*0.3
	vector.DrawFilledCircle(screen, float32(cx), float32(my), float32(mr), color.RGBA{60, 38, 20, 255}, true)
	vector.StrokeCircle(screen, float32(cx), float32(my), float32(mr), 3, rc, true)
	if rs := []rune(nameInitialSource(c.Name)); len(rs) > 0 {
		drawText(screen, strings.ToUpper(string(rs[0])), titleFace(mr*1.1), cx, my-mr*0.62, rc, alignCenter)
	}

	// Nom (2 lignes max)
	nf := bodyFace(math.Max(11, w*0.085), true)
	ny := y + h*0.53
	lines := wrapText(c.Name, nf, cw-16)
	if len(lines) > 2 {
		lines = append(lines[:1], ellipsize(strings.Join(lines[1:], " "), nf, cw-16))
	}
	for _, l := range lines {
		drawText(screen, l, nf, cx, ny, themeText, alignCenter)
		ny += w * 0.11
	}

	// Badge nouveau / doublon
	badge, bc := "Nouveau !", color.Color(color.RGBA{120, 210, 120, 255})
	if c.Duplicate {
		badge, bc = fmt.Sprintf("+%d fragments", c.Shards), color.Color(themeMuted)
	}
	bf := bodyFace(math.Max(10, w*0.07), true)
	bw := textWidth(badge, bf) + 16
	bh := w * 0.13
	by := y + h - bh - h*0.06
	fillRoundRect(screen, cx-bw/2, by, bw, bh, bh/2, withAlpha(bc, 0.18))
	strokeRoundRect(screen, cx-bw/2, by, bw, bh, bh/2, 1.5, bc)
	drawText(screen, badge, bf, cx, by+bh*0.18, bc, alignCenter)
}

// drawGachaRays : rayons qui tournent derrière une carte légendaire ou UL.
func drawGachaRays(screen *ebiten.Image, cx, cy, size, t float64, clr color.Color, ultra bool) {
	n := 12
	speed := 0.4
	length := size * 1.1
	if ultra {
		n, speed, length = 16, 0.9, size*1.5
	}
	for k := 0; k < n; k++ {
		a := t*speed + float64(k)*2*math.Pi/float64(n)
		var p vector.Path
		p.MoveTo(float32(cx), float32(cy))
		p.LineTo(float32(cx+math.Cos(a-0.07)*length), float32(cy+math.Sin(a-0.07)*length))
		p.LineTo(float32(cx+math.Cos(a+0.07)*length), float32(cy+math.Sin(a+0.07)*length))
		p.Close()
		drawPath(screen, &p, withAlpha(clr, 0.16), 0)
	}
}

// drawGachaSparks : étincelles qui jaillissent au moment de la révélation.
func drawGachaSparks(screen *ebiten.Image, cx, cy, size, since float64, clr color.Color, seed int) {
	if since < 0 || since > 1.6 {
		return
	}
	for k := 0; k < 22; k++ {
		h := float64((k*73 + seed*31) % 100)
		a := float64(k)*2*math.Pi/22 + h*0.01
		speed := size * (0.7 + h/100)
		d := speed * since
		px := cx + math.Cos(a)*d
		py := cy + math.Sin(a)*d + 40*since*since
		life := 1 - since/1.6
		vector.DrawFilledCircle(screen, float32(px), float32(py), float32(2+2*life), withAlpha(clr, life), true)
	}
}

// drawGachaULSpotlight : grande fiche d'un capitaine UL.
func (g *Game) drawGachaULSpotlight(screen *ebiten.Image, c gachaCard, t float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	ul := rarityColor("ultra")
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.NRGBA{10, 4, 16, 200}, false)
	w := math.Min(560, sw-32)
	lf := bodyFace(15, false)
	var lines []string
	for _, e := range c.Effects {
		for k, l := range wrapText(e, lf, w-70) {
			if k == 0 {
				l = "• " + l
			} else {
				l = "  " + l
			}
			lines = append(lines, l)
		}
	}
	h := 200 + float64(len(lines))*22
	x, y := sw/2-w/2, sh/2-h/2
	drawGachaRays(screen, sw/2, sh/2, w*0.7, t, ul, true)
	fillRoundRect(screen, x, y, w, h, 18, color.RGBA{30, 16, 40, 245})
	strokeRoundRect(screen, x, y, w, h, 18, 3, ul)
	pulse := 0.6 + 0.4*math.Sin(t*5)
	strokeRoundRect(screen, x-6, y-6, w+12, h+12, 22, 2, withAlpha(ul, 0.5*pulse))

	drawText(screen, "ULTRA RARE !", titleFace(26), sw/2, y+18, ul, alignCenter)
	nf := titleFace(34)
	name := ellipsize(c.Name, nf, w-40)
	drawText(screen, name, nf, sw/2, y+58, themeText, alignCenter)
	status := "Nouveau capitaine !"
	if c.Duplicate {
		status = fmt.Sprintf("Doublon : +%d fragments", c.Shards)
	}
	drawText(screen, status, bodyFace(14, true), sw/2, y+108, themeGold, alignCenter)
	ly := y + 142
	for _, l := range lines {
		drawText(screen, l, lf, x+36, ly, themeText, alignLeft)
		ly += 22
	}
	drawText(screen, "Touche pour continuer", bodyFace(13, true), sw/2, y+h-30, withAlpha(themeMuted, pulse), alignCenter)
}

// nameInitialSource : le nom sans article (« L'Amiral Fantôme » -> « Amiral Fantôme »).
func nameInitialSource(name string) string {
	for _, art := range []string{"L'", "L’", "Le ", "La ", "Les "} {
		if strings.HasPrefix(name, art) && len(name) > len(art) {
			return name[len(art):]
		}
	}
	return name
}
