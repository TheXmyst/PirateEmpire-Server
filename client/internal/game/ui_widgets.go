package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ui_widgets.go : les éléments communs des fenêtres (cadre avec titre, bouton
// fermer, boutons, onglets, cartes, barres, coûts). Ils ne changent que
// l'apparence : chaque fenêtre garde ses propres zones de clic.

type buttonKind int

const (
	btnPrimary   buttonKind = iota // doré : l'action principale
	btnSecondary                   // bois clair
	btnDanger                      // rouge : action risquée ou refusée
	btnDisabled                    // gris : impossible pour l'instant
	btnSuccess                     // vert : confirmer
)

// isHovered : la souris (ou le doigt) est sur ce rectangle.
func isHovered(x, y, w, h float64) bool {
	mx, my := cursorPosition()
	fx, fy := float64(mx), float64(my)
	return fx >= x && fx <= x+w && fy >= y && fy <= y+h
}

// drawDimBackground : assombrit le jeu derrière une fenêtre.
func drawDimBackground(screen *ebiten.Image, alpha uint8) {
	b := screen.Bounds()
	vector.DrawFilledRect(screen, 0, 0, float32(b.Dx()), float32(b.Dy()), color.RGBA{0, 0, 0, alpha}, false)
}

// drawWindowFrame : panneau de fenêtre avec son titre et un filet doré.
func drawWindowFrame(screen *ebiten.Image, x, y, w, h float64, title string) {
	// Une fenêtre arrête le défilement des listes dessinées en dessous.
	scrollBlocker(x, y, w, h)
	drawThemePanel(screen, x, y, w, h, 18)
	if title != "" {
		drawText(screen, title, titleFace(28), x+22, y+12, themeGold, alignLeft)
		vector.StrokeLine(screen, float32(x+18), float32(y+50), float32(x+w-18), float32(y+50), 1, color.NRGBA{224, 176, 74, 90}, false)
	}
}

// drawCloseButton : X rond. (cx, cy) = centre.
func drawCloseButton(screen *ebiten.Image, cx, cy, r float64) {
	drawRoundButton(screen, cx, cy, r, "close", isHovered(cx-r, cy-r, 2*r, 2*r))
}

// drawButton : bouton arrondi avec un libellé centré.
func drawButton(screen *ebiten.Image, x, y, w, h float64, label string, kind buttonKind) {
	hover := kind != btnDisabled && isHovered(x, y, w, h)
	var base, top color.Color
	fg := color.Color(themeText)
	switch kind {
	case btnPrimary:
		base, top, fg = themeGoldDark, themeGold, themeInk
		if hover {
			top = color.RGBA{243, 207, 122, 255}
		}
	case btnDanger:
		base, top = color.RGBA{110, 30, 22, 255}, color.RGBA{170, 52, 38, 255}
		if hover {
			top = color.RGBA{196, 66, 50, 255}
		}
	case btnSuccess:
		base, top = color.RGBA{38, 84, 40, 255}, color.RGBA{62, 128, 62, 255}
		if hover {
			top = color.RGBA{76, 150, 76, 255}
		}
	case btnDisabled:
		base, top, fg = color.RGBA{50, 44, 38, 255}, color.RGBA{72, 64, 56, 255}, themeMuted
	default: // btnSecondary
		base, top = color.RGBA{60, 38, 20, 255}, color.RGBA{98, 64, 34, 255}
		if hover {
			top = color.RGBA{120, 80, 44, 255}
		}
	}
	r := 8.0
	if h < 20 {
		r = h / 2
	}
	fillRoundRect(screen, x, y, w, h, r, base)
	fillRoundRect(screen, x, y, w, h-3, r, top)
	if kind == btnSecondary {
		strokeRoundRect(screen, x, y, w, h, r, 1, themeGoldDim)
	}
	size := 13.0
	if h >= 44 {
		size = 15
	}
	f := bodyFace(size, true)
	label = ellipsize(stripUnsupported(label), f, w-10)
	drawText(screen, label, f, x+w/2, y+(h-3)/2-size*0.62, fg, alignCenter)
}

// drawTab : onglet (actif = doré).
func drawTab(screen *ebiten.Image, x, y, w, h float64, label string, active bool) {
	hover := isHovered(x, y, w, h)
	bg := color.Color(color.NRGBA{255, 255, 255, 22})
	fg := color.Color(themeMuted)
	if active {
		bg, fg = color.NRGBA{224, 176, 74, 50}, themeGold
	} else if hover {
		bg, fg = color.NRGBA{255, 255, 255, 26}, themeText
	}
	fillRoundRect(screen, x, y, w, h, 8, bg)
	if active {
		strokeRoundRect(screen, x, y, w, h, 8, 1.5, themeGold)
	}
	f := bodyFace(13, true)
	drawText(screen, ellipsize(label, f, w-10), f, x+w/2, y+h/2-8, fg, alignCenter)
}

// drawCard : bloc intérieur d'une fenêtre (une ligne de liste, un détail).
func drawCard(screen *ebiten.Image, x, y, w, h float64, selected bool) {
	bg := color.Color(color.NRGBA{255, 255, 255, 22})
	if selected {
		bg = color.NRGBA{224, 176, 74, 34}
	} else if isHovered(x, y, w, h) {
		bg = color.NRGBA{255, 255, 255, 34}
	}
	fillRoundRect(screen, x, y, w, h, 10, bg)
	if selected {
		strokeRoundRect(screen, x, y, w, h, 10, 1.5, themeGold)
	} else {
		strokeRoundRect(screen, x, y, w, h, 10, 1, color.NRGBA{224, 176, 74, 75})
	}
}

// drawInset : zone creusée plus sombre (liste, panneau de détail).
func drawInset(screen *ebiten.Image, x, y, w, h float64) {
	fillRoundRect(screen, x, y, w, h, 12, color.NRGBA{0, 0, 0, 70})
	strokeRoundRect(screen, x, y, w, h, 12, 1, color.NRGBA{224, 176, 74, 40})
}

// drawProgressBar : barre de progression arrondie.
func drawProgressBar(screen *ebiten.Image, x, y, w, h, p float64, clr color.Color) {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	fillRoundRect(screen, x, y, w, h, h/2, color.NRGBA{255, 255, 255, 28})
	if p > 0 {
		fw := w * p
		if fw < h {
			fw = h
		}
		fillRoundRect(screen, x, y, fw, h, h/2, clr)
	}
}

// drawLabel : texte de fenêtre (taille, gras, couleur).
func drawLabel(screen *ebiten.Image, s string, x, y, size float64, bold bool, clr color.Color) {
	drawText(screen, stripUnsupported(s), bodyFace(size, bold), x, y, clr, alignLeft)
}

var costOrder = []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum}
var costNames = map[domain.ResourceType]string{domain.Wood: "Bois", domain.Stone: "Pierre", domain.Gold: "Or", domain.Rum: "Rhum"}

// drawCost : « Bois 100 · Or 50 », en rouge ce qui manque. Renvoie la
// largeur dessinée.
func drawCost(screen *ebiten.Image, costs map[domain.ResourceType]float64, have map[domain.ResourceType]float64, x, y float64) float64 {
	f := bodyFace(13, true)
	start := x
	first := true
	for _, res := range costOrder {
		amt, ok := costs[res]
		if !ok || amt == 0 {
			continue
		}
		if !first {
			drawText(screen, "·", f, x+3, y, themeMuted, alignLeft)
			x += textWidth("·", f) + 6
		}
		first = false
		name := costNames[res] + " "
		drawText(screen, name, bodyFace(13, false), x, y, themeMuted, alignLeft)
		x += textWidth(name, bodyFace(13, false))
		val := formatThousands(amt)
		clr := color.Color(themeText)
		if have != nil && have[res] < amt {
			clr = color.RGBA{235, 90, 70, 255}
		}
		drawText(screen, val, f, x, y, clr, alignLeft)
		x += textWidth(val, f)
	}
	return x - start
}

// rarityColor / rarityLabel : couleur et nom d'une rareté de capitaine.
func rarityColor(r string) color.Color {
	switch strings.ToLower(r) {
	case "rare":
		return color.RGBA{90, 170, 240, 255}
	case "legendary":
		return color.RGBA{240, 190, 60, 255}
	case "ultra":
		return color.RGBA{214, 120, 255, 255}
	case "epic":
		return color.RGBA{190, 120, 230, 255}
	}
	return color.RGBA{190, 180, 165, 255}
}

func rarityLabel(r string) string {
	switch strings.ToLower(r) {
	case "rare":
		return "Rare"
	case "legendary":
		return "Légendaire"
	case "ultra":
		return "UL · Ultra rare"
	case "epic":
		return "Épique"
	case "common", "":
		return "Commun"
	}
	return r
}

// drawStars : étoiles pleines / vides. alignRightEdge : x est le bord droit.
func drawStars(screen *ebiten.Image, x, cy float64, stars, max int, r float64, alignRightEdge bool) {
	gap := r*2 + 4
	if alignRightEdge {
		x -= float64(max)*gap - 4
	}
	for i := 0; i < max; i++ {
		cx := x + r + float64(i)*gap
		p := starPath(float32(cx), float32(cy), float32(r))
		if i < stars {
			drawPath(screen, p, themeGold, 0)
		} else {
			drawPath(screen, p, color.NRGBA{224, 176, 74, 120}, 1.2)
		}
	}
}

func starPath(cx, cy, r float32) *vector.Path {
	p := &vector.Path{}
	for i := 0; i < 10; i++ {
		rr := r
		if i%2 == 1 {
			rr = r * 0.45
		}
		a := float64(i)*math.Pi/5 - math.Pi/2
		px, py := cx+rr*float32(math.Cos(a)), cy+rr*float32(math.Sin(a))
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
	p.Close()
	return p
}

// wrapText coupe un texte en lignes qui tiennent dans maxW.
func wrapText(s string, face text.Face, maxW float64) []string {
	s = stripUnsupported(s)
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		cur := ""
		for _, w := range words {
			try := w
			if cur != "" {
				try = cur + " " + w
			}
			if cur != "" && textWidth(try, face) > maxW {
				lines = append(lines, cur)
				cur = w
			} else {
				cur = try
			}
		}
		lines = append(lines, cur)
	}
	return lines
}

// formatDuration : 75s -> "1:15", 3700s -> "1:01:40".
func formatDuration(secs int) string {
	if secs < 0 {
		secs = 0
	}
	if secs >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", secs/3600, (secs/60)%60, secs%60)
	}
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// ---------------------------------------------------------------------------
// Harmonisation automatique des anciennes fenêtres
//
// Les fenêtres pas encore redessinées à la main appellent uiFillRect et
// uiStrokeRect (à la place de vector.DrawFilledRect / StrokeRect). Leurs
// couleurs d'origine (bleus, verts, gris froids) sont traduites dans la
// palette bois et or, et les rectangles assez grands ont des coins arrondis.
// Les barres fines (vie, progression) gardent leurs couleurs d'origine.

func uiFillRect(screen *ebiten.Image, x, y, w, h float32, clr color.Color, _ bool) {
	if w < 14 || h < 14 {
		vector.DrawFilledRect(screen, x, y, w, h, clr, false)
		return
	}
	c := themeRemap(clr, false)
	if w >= 24 && h >= 18 {
		r := math.Min(8, float64(h)/3)
		fillRoundRect(screen, float64(x), float64(y), float64(w), float64(h), r, c)
		return
	}
	vector.DrawFilledRect(screen, x, y, w, h, c, false)
}

func uiStrokeRect(screen *ebiten.Image, x, y, w, h, width float32, clr color.Color, _ bool) {
	c := themeRemap(clr, true)
	if w >= 24 && h >= 18 {
		r := math.Min(8, float64(h)/3)
		strokeRoundRect(screen, float64(x), float64(y), float64(w), float64(h), r, float64(width), c)
		return
	}
	vector.StrokeRect(screen, x, y, w, h, width, c, false)
}

// themeRemap traduit une couleur de l'ancienne interface dans la palette du
// thème. Les valeurs color.RGBA écrites dans le code sont lues comme des
// couleurs « normales » (non prémultipliées), comme leurs auteurs les
// pensaient.
func themeRemap(clr color.Color, stroke bool) color.Color {
	var r, g, b, a float64
	switch c := clr.(type) {
	case color.RGBA:
		r, g, b, a = float64(c.R), float64(c.G), float64(c.B), float64(c.A)
	case color.NRGBA:
		return c // déjà au thème
	case color.Gray:
		r, g, b, a = float64(c.Y), float64(c.Y), float64(c.Y), 255
	default:
		return clr
	}
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	sat := mx - mn
	l := (r + g + b) / 3
	A := uint8(a)
	wood := func(l float64) color.NRGBA {
		return color.NRGBA{uint8(math.Min(255, l*0.95+10)), uint8(math.Min(255, l*0.65+6)), uint8(math.Min(255, l*0.38+3)), A}
	}
	if mx < 16 {
		return color.NRGBA{uint8(r), uint8(g), uint8(b), A} // voiles noirs
	}
	if stroke {
		if r >= g && g >= 0.55*r && r > 150 && sat > 60 {
			return color.NRGBA{224, 176, 74, A} // déjà doré
		}
		if r > 150 && g < 0.6*r && b < 0.6*r {
			return color.NRGBA{200, 80, 60, A}
		}
		return color.NRGBA{224, 176, 74, uint8(math.Min(a, 140))}
	}
	if sat < 30 {
		if l < 110 {
			return wood(l)
		}
		return color.NRGBA{uint8(l), uint8(l * 0.86), uint8(l * 0.62), A}
	}
	switch {
	case r >= g && g >= 0.55*r && r > 150: // jaune / orange : déjà dans le ton
		return color.NRGBA{uint8(math.Min(255, r)), uint8(g * 0.9), uint8(b * 0.6), A}
	case r >= g && r >= b: // rouge
		if mx < 110 {
			return color.NRGBA{uint8(l*1.2 + 12), uint8(l * 0.6), uint8(l * 0.45), A}
		}
		return color.NRGBA{165, 55, 42, A}
	case g >= r && g >= b: // vert : action principale -> ambre
		if mx < 110 {
			return wood(l)
		}
		return color.NRGBA{176, 118, 36, A}
	default: // bleu / violet -> bois
		if mx < 110 {
			return wood(l)
		}
		return color.NRGBA{112, 74, 40, A}
	}
}

// kindFromColor : choisit le style de bouton d'après la couleur que
// l'ancienne interface utilisait (vert = action, rouge = danger, gris =
// impossible, bleu = secondaire).
func kindFromColor(clr color.Color) buttonKind {
	c, ok := clr.(color.RGBA)
	if !ok {
		return btnSecondary
	}
	r, g, b := float64(c.R), float64(c.G), float64(c.B)
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	switch {
	case mx-mn < 30:
		return btnDisabled
	case r >= g && g >= 0.55*r && r > 140:
		return btnPrimary
	case r >= g && r >= b:
		return btnDanger
	case g >= r && g >= b:
		return btnPrimary
	}
	return btnSecondary
}

// uiTitle : titre de fenêtre (police pirate dorée), à la place d'un ancien
// texte en majuscules.
func uiTitle(screen *ebiten.Image, s string, x, y int) {
	drawText(screen, s, titleFace(26), float64(x), float64(y)-7, themeGold, alignLeft)
}

// uiTitleCentered draws a window title centered on cx.
func uiTitleCentered(screen *ebiten.Image, s string, cx, y int) {
	drawText(screen, s, titleFace(26), float64(cx), float64(y)-7, themeGold, alignCenter)
}

// uiSubtitle draws a section heading inside a window.
func uiSubtitle(screen *ebiten.Image, s string, x, y int) {
	drawText(screen, s, titleFace(20), float64(x), float64(y)-5, themeGold, alignLeft)
}

// drawInputField : champ de saisie (fond creusé, filet doré quand actif).
func drawInputField(screen *ebiten.Image, x, y, w, h float64, value, placeholder string, focused bool) {
	fillRoundRect(screen, x, y, w, h, 8, color.NRGBA{0, 0, 0, 120})
	border := color.Color(color.NRGBA{224, 176, 74, 70})
	if focused {
		border = themeGold
	}
	strokeRoundRect(screen, x, y, w, h, 8, 1.5, border)
	f := bodyFace(14, false)
	ty := y + (h-18)/2
	if value == "" && !focused {
		drawText(screen, placeholder, f, x+12, ty, themeMuted, alignLeft)
		return
	}
	drawText(screen, value, f, x+12, ty, themeText, alignLeft)
	if focused && (time.Now().UnixMilli()/500)%2 == 0 {
		cx := x + 13 + textWidth(value, f)
		vector.StrokeLine(screen, float32(cx), float32(ty+1), float32(cx), float32(ty+17), 1.5, themeGold, false)
	}
}

// captainMaxStars : nombre d'étoiles maximum d'un capitaine (5 pour toutes les
// raretés, comme sur le serveur).
func captainMaxStars(rarity string) int { return 5 }
