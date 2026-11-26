package game

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ui_theme.go : le style de l'interface (bois sombre et or).
//   - deux polices : Pirata One pour les titres, Inter pour le reste
//     (resources/ui/fonts, licence SIL OFL) ;
//   - panneaux arrondis, boutons ronds ;
//   - petites icônes dessinées au trait (pas d'images à charger).

var (
	themeGold     = color.RGBA{224, 176, 74, 255}
	themeGoldDim  = color.NRGBA{224, 176, 74, 150}
	themeGoldDark = color.RGBA{184, 134, 42, 255}
	themePanel    = color.RGBA{28, 18, 10, 214}
	themePanelHi  = color.NRGBA{255, 255, 255, 16}
	themeText     = color.RGBA{245, 234, 214, 255}
	themeMuted    = color.RGBA{205, 191, 159, 255}
	themeGreen    = color.RGBA{159, 224, 160, 255}
	themeOrange   = color.RGBA{240, 150, 80, 255}
	themeRed      = color.RGBA{192, 57, 43, 255}
	themeInk      = color.RGBA{42, 26, 14, 255}
)

// ---------------------------------------------------------------------------
// Polices

var (
	fontsLoaded  bool
	fontTitleSrc *text.GoTextFaceSource
	fontBodySrc  *text.GoTextFaceSource
	faceCache    = map[faceKey]text.Face{}
)

type faceKey struct {
	title bool
	size  float64
	bold  bool
}

func loadFonts() {
	if fontsLoaded {
		return
	}
	fontsLoaded = true
	if b, err := readAsset("resources/ui/fonts/PirataOne-Regular.ttf"); err == nil {
		fontTitleSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(b))
	}
	if b, err := readAsset("resources/ui/fonts/Inter.ttf"); err == nil {
		fontBodySrc, _ = text.NewGoTextFaceSource(bytes.NewReader(b))
	}
	if fontTitleSrc == nil {
		fontTitleSrc = fontBodySrc
	}
}

func makeFace(k faceKey) text.Face {
	loadFonts()
	src := fontBodySrc
	if k.title {
		src = fontTitleSrc
	}
	if src == nil {
		return nil
	}
	f := &text.GoTextFace{Source: src, Size: k.size}
	if !k.title {
		if k.bold {
			f.SetVariation(text.MustParseTag("wght"), 700)
		} else {
			f.SetVariation(text.MustParseTag("wght"), 450)
		}
	}
	return f
}

func getFace(k faceKey) text.Face {
	if f, ok := faceCache[k]; ok {
		return f
	}
	f := makeFace(k)
	faceCache[k] = f
	return f
}

// titleFace : police pirate (titres, gros boutons).
func titleFace(size float64) text.Face { return getFace(faceKey{title: true, size: size}) }

// bodyFace : police lisible (chiffres, textes).
func bodyFace(size float64, bold bool) text.Face {
	return getFace(faceKey{size: size, bold: bold})
}

type textAlign int

const (
	alignLeft textAlign = iota
	alignCenter
	alignRight
)

// drawText écrit s avec le haut du texte en y. Sans police, on retombe sur
// l'ancienne écriture de debug.
func drawText(screen *ebiten.Image, s string, face text.Face, x, y float64, clr color.Color, align textAlign) {
	if face == nil {
		w := float64(len(s) * 6)
		switch align {
		case alignCenter:
			x -= w / 2
		case alignRight:
			x -= w
		}
		ebitenutil.DebugPrintAt(screen, s, int(x), int(y))
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	if gf, ok := face.(*text.GoTextFace); ok {
		op.LineSpacing = gf.Size * 1.3
	}
	switch align {
	case alignCenter:
		op.PrimaryAlign = text.AlignCenter
	case alignRight:
		op.PrimaryAlign = text.AlignEnd
	}
	text.Draw(screen, s, face, op)
}

// uiTextSize : taille de la police lisible qui remplace la police de debug
// (6x16 pixels par caractère) dans les fenêtres du jeu.
const uiTextSize = 12.5

// uiPrint remplace ebitenutil.DebugPrintAt : même position (haut-gauche du
// texte), mais avec la police lisible du jeu et une ombre légère pour rester
// lisible sur tous les fonds.
func uiPrint(screen *ebiten.Image, s string, x, y int) {
	s = stripUnsupported(s)
	if s == "" {
		return
	}
	f := bodyFace(uiTextSize, false)
	if f == nil {
		ebitenutil.DebugPrintAt(screen, s, x, y)
		return
	}
	op := &text.DrawOptions{}
	op.LineSpacing = 16 // même interligne que l'ancienne police
	op.GeoM.Translate(float64(x)+1, float64(y)+2)
	op.ColorScale.ScaleWithColor(color.NRGBA{0, 0, 0, 150})
	text.Draw(screen, s, f, op)
	op.GeoM.Translate(-1, -1)
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(themeText)
	text.Draw(screen, s, f, op)
}

// stripUnsupported retire les émojis (la police n'en a pas : ils
// s'afficheraient en carrés vides).
func stripUnsupported(s string) string {
	clean := true
	for _, r := range s {
		if isEmojiRune(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if !isEmojiRune(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func isEmojiRune(r rune) bool {
	return r >= 0x1F000 || (r >= 0x2600 && r <= 0x27BF) || (r >= 0x2B00 && r <= 0x2BFF) ||
		r == 0xFE0F || r == 0x200D || (r >= 0x2300 && r <= 0x23FF)
}

// textWidth : largeur du texte en pixels.
func textWidth(s string, face text.Face) float64 {
	if face == nil {
		return float64(len(s) * 6)
	}
	w, _ := text.Measure(s, face, 0)
	return w
}

// ellipsize coupe s pour qu'il tienne dans maxW (avec « … »).
func ellipsize(s string, face text.Face, maxW float64) string {
	if textWidth(s, face) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		c := strings.TrimRight(string(r), " ") + "…"
		if textWidth(c, face) <= maxW {
			return c
		}
	}
	return "…"
}

// formatThousands : 12345 -> "12 345".
func formatThousands(v float64) string {
	n := int64(math.Round(v))
	neg := n < 0
	if neg {
		n = -n
	}
	s := []byte{}
	digits := []byte(itoa(n))
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			s = append(s, ' ')
		}
		s = append(s, d)
	}
	if neg {
		return "-" + string(s)
	}
	return string(s)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ---------------------------------------------------------------------------
// Formes

var (
	pathVerts   []ebiten.Vertex
	pathIndices []uint16
	whitePixel  *ebiten.Image
)

func whiteSrc() *ebiten.Image {
	if whitePixel == nil {
		img := ebiten.NewImage(3, 3)
		img.Fill(color.White)
		whitePixel = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	}
	return whitePixel
}

func drawPath(screen *ebiten.Image, p *vector.Path, clr color.Color, stroke float32) {
	pathVerts, pathIndices = pathVerts[:0], pathIndices[:0]
	if stroke > 0 {
		pathVerts, pathIndices = p.AppendVerticesAndIndicesForStroke(pathVerts, pathIndices, &vector.StrokeOptions{
			Width: stroke, LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound,
		})
	} else {
		pathVerts, pathIndices = p.AppendVerticesAndIndicesForFilling(pathVerts, pathIndices)
	}
	r, g, b, a := clr.RGBA()
	for i := range pathVerts {
		pathVerts[i].SrcX, pathVerts[i].SrcY = 1, 1
		pathVerts[i].ColorR = float32(r) / 0xffff
		pathVerts[i].ColorG = float32(g) / 0xffff
		pathVerts[i].ColorB = float32(b) / 0xffff
		pathVerts[i].ColorA = float32(a) / 0xffff
	}
	op := &ebiten.DrawTrianglesOptions{AntiAlias: vecAA}
	if stroke <= 0 {
		op.FillRule = ebiten.FillRuleNonZero
	}
	screen.DrawTriangles(pathVerts, pathIndices, whiteSrc(), op)
}

func roundRectPath(x, y, w, h, r float32) *vector.Path {
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	p := &vector.Path{}
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.Arc(x+w-r, y+r, r, -math.Pi/2, 0, vector.Clockwise)
	p.LineTo(x+w, y+h-r)
	p.Arc(x+w-r, y+h-r, r, 0, math.Pi/2, vector.Clockwise)
	p.LineTo(x+r, y+h)
	p.Arc(x+r, y+h-r, r, math.Pi/2, math.Pi, vector.Clockwise)
	p.LineTo(x, y+r)
	p.Arc(x+r, y+r, r, math.Pi, math.Pi*3/2, vector.Clockwise)
	p.Close()
	return p
}

func fillRoundRect(screen *ebiten.Image, x, y, w, h, r float64, clr color.Color) {
	drawPath(screen, roundRectPath(float32(x), float32(y), float32(w), float32(h), float32(r)), clr, 0)
}

func strokeRoundRect(screen *ebiten.Image, x, y, w, h, r, width float64, clr color.Color) {
	drawPath(screen, roundRectPath(float32(x), float32(y), float32(w), float32(h), float32(r)), clr, float32(width))
}

// drawThemePanel : panneau bois sombre translucide, bord or.
func drawThemePanel(screen *ebiten.Image, x, y, w, h, r float64) {
	fillRoundRect(screen, x+2, y+4, w, h, r, color.RGBA{0, 0, 0, 70}) // ombre
	fillRoundRect(screen, x, y, w, h, r, themePanel)
	strokeRoundRect(screen, x, y, w, h, r, 2, themeGold)
}

// drawRoundButton : bouton rond (icône seule).
func drawRoundButton(screen *ebiten.Image, cx, cy, radius float64, icon string, hover bool) {
	vector.DrawFilledCircle(screen, float32(cx+1), float32(cy+3), float32(radius), color.RGBA{0, 0, 0, 70}, vecAA)
	vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(radius), themePanel, vecAA)
	border := color.Color(themeGoldDim)
	if hover {
		border = themeGold
		vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(radius), themePanelHi, vecAA)
	}
	vector.StrokeCircle(screen, float32(cx), float32(cy), float32(radius), 2, border, vecAA)
	drawIcon(screen, icon, cx, cy, radius*0.9, themeText)
}

// drawBadge : pastille rouge avec un nombre (messages non lus...).
func drawBadge(screen *ebiten.Image, cx, cy float64, n int) {
	if n <= 0 {
		return
	}
	label := itoa(int64(n))
	if n > 99 {
		label = "99+"
	}
	f := bodyFace(12, true)
	w := math.Max(24, textWidth(label, f)+12)
	fillRoundRect(screen, cx-w/2, cy-12, w, 24, 12, themeRed)
	strokeRoundRect(screen, cx-w/2, cy-12, w, 24, 12, 2, themeText)
	drawText(screen, label, f, cx, cy-8, color.White, alignCenter)
}

// ---------------------------------------------------------------------------
// Icônes au trait, dessinées dans un carré de 24 unités centré sur (cx, cy).

func drawIcon(screen *ebiten.Image, name string, cx, cy, size float64, clr color.Color) {
	s := float32(size / 24)
	ox, oy := float32(cx)-12*s, float32(cy)-12*s
	pt := func(x, y float32) (float32, float32) { return ox + x*s, oy + y*s }
	p := &vector.Path{}
	m := func(x, y float32) { p.MoveTo(pt(x, y)) }
	l := func(x, y float32) { p.LineTo(pt(x, y)) }
	circle := func(x, y, r float32) {
		px, py := pt(x+r, y)
		p.MoveTo(px, py)
		cxp, cyp := pt(x, y)
		p.Arc(cxp, cyp, r*s, 0, 2*math.Pi, vector.Clockwise)
		p.Close()
	}
	arc := func(x, y, r, a0, a1 float32) {
		cxp, cyp := pt(x, y)
		p.MoveTo(cxp+r*s*float32(math.Cos(float64(a0))), cyp+r*s*float32(math.Sin(float64(a0))))
		p.Arc(cxp, cyp, r*s, a0, a1, vector.Clockwise)
	}
	width := float32(1.9) * s
	switch name {
	case "anchor": // Flotte
		circle(12, 5, 2)
		m(12, 7)
		l(12, 20)
		m(8, 10)
		l(16, 10)
		arc(12, 12, 7, 0, math.Pi)
		m(5, 12)
		l(3.5, 14)
		m(19, 12)
		l(20.5, 14)
	case "map": // Mer
		m(3, 6)
		l(9, 4)
		l(15, 6)
		l(21, 4)
		l(21, 18)
		l(15, 20)
		l(9, 18)
		l(3, 20)
		p.Close()
		m(9, 4)
		l(9, 18)
		m(15, 6)
		l(15, 20)
	case "island": // retour à l'île
		m(3, 19)
		l(21, 19)
		arc(12, 19, 6, math.Pi, 2*math.Pi)
		m(12, 13)
		l(12, 6)
		m(12, 6)
		l(7, 8)
		m(12, 6)
		l(17, 8)
		m(12, 6)
		l(9, 3)
		m(12, 6)
		l(15, 3)
	case "hammer": // Construire
		m(13, 4)
		l(19, 10)
		l(16.5, 12.5)
		l(10.5, 6.5)
		p.Close()
		m(12, 8)
		l(4, 16)
		l(6.5, 18.5)
		l(14.5, 10.5)
	case "hat": // Capitaines (tricorne)
		m(2, 12)
		l(6, 16)
		l(12, 14.5)
		l(18, 16)
		l(22, 12)
		l(18, 8)
		l(15, 10)
		l(12, 6)
		l(9, 10)
		l(6, 8)
		p.Close()
		circle(12, 11.2, 1.3)
	case "people": // Social
		circle(9, 8, 3)
		circle(17, 9, 2.5)
		arc(9, 20, 6, math.Pi, 2*math.Pi)
		arc(17, 19, 4.5, math.Pi*1.15, 2*math.Pi)
	case "chat":
		m(4, 5)
		l(20, 5)
		l(20, 16)
		l(10, 16)
		l(5, 20)
		l(5, 16)
		l(4, 16)
		p.Close()
		m(8, 9.5)
		l(16, 9.5)
		m(8, 12.5)
		l(13, 12.5)
	case "fullscreen":
		m(4, 9)
		l(4, 4)
		l(9, 4)
		m(20, 9)
		l(20, 4)
		l(15, 4)
		m(4, 15)
		l(4, 20)
		l(9, 20)
		m(20, 15)
		l(20, 20)
		l(15, 20)
	case "code": // menu dev
		m(8, 7)
		l(3, 12)
		l(8, 17)
		m(16, 7)
		l(21, 12)
		l(16, 17)
	case "close":
		m(6, 6)
		l(18, 18)
		m(18, 6)
		l(6, 18)
		width = 2.4 * s
	case "send":
		m(4, 12)
		l(20, 4)
		l(14, 20)
		l(11, 13)
		p.Close()
	default:
		circle(12, 12, 8)
	}
	drawPath(screen, p, clr, width)
}
