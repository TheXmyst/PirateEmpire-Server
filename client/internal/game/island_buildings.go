package game

import (
	"bytes"
	"image"
	"image/color"
	"log"
	"math"
	"sort"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// island_buildings.go : rendu des bâtiments sur l'île.
//
// Les sprites d'origine n'ont pas tous la même luminosité ni la même
// saturation, n'ont pas d'ombre et étaient centrés sur leur milieu. Au
// chargement on les harmonise (même lumière que le décor), on calcule leur
// « pied » (le point qui touche le sol) et on génère leur ombre. Tout est fait
// une seule fois au démarrage : aucun coût pendant le jeu.

// buildingScale : taille des bâtiments par rapport à leur image (300 px).
// Les maisons peintes sur le décor font environ 1,35 fois nos sprites.
const buildingScale = 1.35

// Réglages de la retouche couleur commune.
const (
	gradeTargetLum    = 84.0 // luminosité moyenne visée (0-255)
	gradeTargetChroma = 52.0 // saturation moyenne visée (écart max-min, 0-255)
	gradeWarmR        = 1.03 // léger réchauffement (soleil du décor)
	gradeWarmB        = 0.95
	rimDepth          = 10 // largeur (px) du bord de socle adouci
)

// Ombre : le soleil du décor vient du haut à droite, les ombres partent vers
// le bas à gauche.
const (
	shadowShearX = 0.50 // décalage horizontal par pixel de hauteur
	shadowShearY = 0.30 // décalage vertical par pixel de hauteur
	shadowAlpha  = 0.50 // opacité maximale de l'ombre portée
	contactAlpha = 0.45 // opacité de l'ombre de contact sous le pied
)

// buildingSprite : un bâtiment prêt à être posé sur l'île.
type buildingSprite struct {
	img    *ebiten.Image // sprite retouché (même taille que l'original)
	shadow *ebiten.Image // ombre portée + ombre de contact
	shOX   float64       // coin haut-gauche de l'ombre, relatif au pied (px image)
	shOY   float64
	footX  float64 // pied du bâtiment dans l'image (px)
	footY  float64
	opaque image.Rectangle // zone non transparente de l'image
}

// islandSlots : emplacement (point au sol) de chaque type de bâtiment,
// calés sur les dalles et clairières peintes sur le décor.
var islandSlots = []Slot{
	{X: 0, Y: -80, AllowedType: "Hôtel de Ville"},
	{X: 100, Y: -430, AllowedType: "Scierie"},
	{X: 560, Y: -250, AllowedType: "Mine d'Or"},
	{X: 600, Y: 120, AllowedType: "Distillerie"},
	{X: -400, Y: 240, AllowedType: "Académie"},
	{X: -1630, Y: 710, AllowedType: "Chantier Naval"},
	{X: 150, Y: 840, AllowedType: "Carrière"},
	{X: 760, Y: 560, AllowedType: "Entrepôt"},
	{X: -320, Y: -200, AllowedType: "Tavern"},
	{X: -80, Y: 520, AllowedType: "Milice"},
	{X: 240, Y: 440, AllowedType: "Infirmary"},
}

// buildingGroundPos : position au sol d'un bâtiment. Chaque type n'a qu'un
// emplacement : on utilise celui-ci (les bâtiments construits avant le
// nouveau placement gardent leurs anciennes coordonnées sur le serveur, mais
// s'affichent au bon endroit).
func (g *Game) buildingGroundPos(b domain.Building) (float64, float64) {
	for _, s := range g.slots {
		if s.AllowedType == b.Type {
			return s.X, s.Y
		}
	}
	return b.X, b.Y
}

// buildingSpriteFor : sprite d'un type de bâtiment (nil si inconnu).
func (g *Game) buildingSpriteFor(t string) *buildingSprite {
	if bs, ok := g.buildingSprites[t]; ok {
		return bs
	}
	return g.buildingSprites[""]
}

// buildingScreenRect : rectangle à l'écran occupé par l'image du bâtiment
// (pour les clics et les infobulles).
func (g *Game) buildingScreenRect(b domain.Building) (x0, y0, x1, y1 float64, ok bool) {
	bs := g.buildingSpriteFor(b.Type)
	if bs == nil {
		return 0, 0, 0, 0, false
	}
	gx, gy := g.buildingGroundPos(b)
	k := buildingScale
	// On resserre un peu la zone (drapeaux et bords de socle ne comptent pas).
	r := bs.opaque
	padX := float64(r.Dx()) * 0.12
	wx0 := gx + (float64(r.Min.X)+padX-bs.footX)*k
	wx1 := gx + (float64(r.Max.X)-padX-bs.footX)*k
	wy0 := gy + (float64(r.Min.Y)+float64(r.Dy())*0.15-bs.footY)*k
	wy1 := gy + (float64(r.Max.Y)-bs.footY)*k
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	x0 = (wx0-g.camX)*g.camZoom + sw/2
	x1 = (wx1-g.camX)*g.camZoom + sw/2
	y0 = (wy0-g.camY)*g.camZoom + sh/2
	y1 = (wy1-g.camY)*g.camZoom + sh/2
	return x0, y0, x1, y1, true
}

// buildingAt : bâtiment sous le point (mx, my) de l'écran, le plus en avant
// d'abord (celui qui est dessiné par-dessus les autres).
func (g *Game) buildingAt(mx, my float64) (domain.Building, bool) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return domain.Building{}, false
	}
	for _, b := range g.sortedIslandBuildings(true) {
		x0, y0, x1, y1, ok := g.buildingScreenRect(b)
		if ok && mx >= x0 && mx <= x1 && my >= y0 && my <= y1 {
			return b, true
		}
	}
	return domain.Building{}, false
}

// sortedIslandBuildings : bâtiments triés du fond vers l'avant (ou l'inverse).
func (g *Game) sortedIslandBuildings(frontFirst bool) []domain.Building {
	src := g.player.Islands[0].Buildings
	list := make([]domain.Building, len(src))
	copy(list, src)
	sort.SliceStable(list, func(i, j int) bool {
		_, yi := g.buildingGroundPos(list[i])
		_, yj := g.buildingGroundPos(list[j])
		if frontFirst {
			return yi > yj
		}
		return yi < yj
	})
	return list
}

// drawIslandBuildings dessine les ombres puis les bâtiments, du fond vers
// l'avant pour qu'ils se recouvrent correctement.
func (g *Game) drawIslandBuildings(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	list := g.sortedIslandBuildings(false)

	place := func(op *ebiten.DrawImageOptions, gx, gy, ox, oy float64) {
		op.GeoM.Translate(ox, oy)
		op.GeoM.Scale(buildingScale, buildingScale)
		op.GeoM.Translate(gx, gy)
		op.GeoM.Translate(-g.camX, -g.camY)
		op.GeoM.Scale(g.camZoom, g.camZoom)
		op.GeoM.Translate(sw/2, sh/2)
	}

	// 1) Toutes les ombres d'abord : une ombre ne passe jamais devant un
	//    bâtiment voisin.
	for _, b := range list {
		bs := g.buildingSpriteFor(b.Type)
		if bs == nil || bs.shadow == nil {
			continue
		}
		gx, gy := g.buildingGroundPos(b)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		place(op, gx, gy, bs.shOX, bs.shOY)
		if b.Constructing {
			op.ColorScale.ScaleAlpha(0.6)
		}
		screen.DrawImage(bs.shadow, op)
	}

	// 2) Les bâtiments.
	for _, b := range list {
		bs := g.buildingSpriteFor(b.Type)
		if bs == nil {
			continue
		}
		gx, gy := g.buildingGroundPos(b)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		place(op, gx, gy, -bs.footX, -bs.footY)
		if b.Constructing {
			op.ColorScale.ScaleAlpha(0.7)
		}
		screen.DrawImage(bs.img, op)
	}
}

// loadBuildingSprite charge, harmonise et prépare un sprite de bâtiment.
func loadBuildingSprite(path string) *buildingSprite {
	data, err := readAsset(path)
	if err != nil {
		log.Printf("Failed to open %s: %v", path, err)
		return nil
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to decode %s: %v", path, err)
		return nil
	}
	return prepareBuildingSprite(src)
}

// prepareBuildingSprite fait le travail de loadBuildingSprite à partir d'une
// image déjà décodée.
func prepareBuildingSprite(src image.Image) *buildingSprite {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}

	opaque := alphaBounds(img, 100)
	if opaque.Empty() {
		return &buildingSprite{img: ebiten.NewImageFromImage(img), footX: float64(w) / 2, footY: float64(h), opaque: img.Bounds()}
	}

	gradeBuilding(img)
	softenBaseRim(img, opaque)

	// Le pied : milieu en largeur, un peu au-dessus du bas du socle (le
	// bas de l'image est le bord avant du socle, pas son centre).
	footX := float64(opaque.Min.X+opaque.Max.X) / 2
	footY := float64(opaque.Max.Y) - float64(opaque.Dy())*0.12

	shadow, shOX, shOY := makeBuildingShadow(img, opaque, footX, footY)

	return &buildingSprite{
		img:    ebiten.NewImageFromImage(img),
		shadow: ebiten.NewImageFromImage(shadow),
		shOX:   shOX,
		shOY:   shOY,
		footX:  footX,
		footY:  footY,
		opaque: opaque,
	}
}

// placeholderBuilding : bâtiment temporaire (sprite manquant).
func placeholderBuilding(c color.NRGBA) *buildingSprite {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return prepareBuildingSprite(img)
}

func alphaBounds(img *image.NRGBA, minA uint8) image.Rectangle {
	b := img.Bounds()
	r := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A >= minA {
				if x < r.Min.X {
					r.Min.X = x
				}
				if y < r.Min.Y {
					r.Min.Y = y
				}
				if x+1 > r.Max.X {
					r.Max.X = x + 1
				}
				if y+1 > r.Max.Y {
					r.Max.Y = y + 1
				}
			}
		}
	}
	if r.Min.X >= r.Max.X {
		return image.Rectangle{}
	}
	return r
}

func clamp255(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// gradeBuilding ramène la luminosité et la saturation moyennes du sprite
// vers des valeurs communes, puis le réchauffe légèrement.
func gradeBuilding(img *image.NRGBA) {
	var sumL, sumC, n float64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A < 200 {
				continue
			}
			r, g, bl := float64(c.R), float64(c.G), float64(c.B)
			sumL += 0.3*r + 0.59*g + 0.11*bl
			sumC += math.Max(r, math.Max(g, bl)) - math.Min(r, math.Min(g, bl))
			n++
		}
	}
	if n == 0 {
		return
	}
	gain := clampF(gradeTargetLum/(sumL/n), 0.85, 1.6)
	sat := clampF(gradeTargetChroma/(sumC/n*gain), 0.7, 1.3)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			r, g, bl := float64(c.R)*gain, float64(c.G)*gain, float64(c.B)*gain
			l := 0.3*r + 0.59*g + 0.11*bl
			r = l + (r-l)*sat
			g = l + (g-l)*sat
			bl = l + (bl-l)*sat
			img.SetNRGBA(x, y, color.NRGBA{clamp255(r * gradeWarmR), clamp255(g), clamp255(bl * gradeWarmB), c.A})
		}
	}
}

// softenBaseRim adoucit le liseré clair au bord des socles de terre : près
// du bord (dans le bas de l'image seulement), la couleur est assombrie et la
// transparence augmentée pour que le socle se fonde dans l'herbe.
func softenBaseRim(img *image.NRGBA, opaque image.Rectangle) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// depth[i] : distance (en px, plafonnée) jusqu'au pixel transparent le
	// plus proche, par érosions successives.
	depth := make([]uint8, w*h)
	solid := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			solid[y*w+x] = img.NRGBAAt(x, y).A >= 128
		}
	}
	cur := solid
	for d := 1; d <= rimDepth; d++ {
		next := make([]bool, w*h)
		for y := 1; y < h-1; y++ {
			for x := 1; x < w-1; x++ {
				i := y*w + x
				if cur[i] && cur[i-1] && cur[i+1] && cur[i-w] && cur[i+w] {
					next[i] = true
					depth[i] = uint8(d)
				}
			}
		}
		cur = next
	}

	top := float64(opaque.Max.Y) - float64(opaque.Dy())*0.4
	for y := 0; y < h; y++ {
		if float64(y) < top {
			continue
		}
		// Transition douce vers le haut de la zone traitée.
		zone := clampF((float64(y)-top)/(float64(opaque.Dy())*0.1), 0, 1)
		for x := 0; x < w; x++ {
			i := y*w + x
			c := img.NRGBAAt(x, y)
			if c.A == 0 || depth[i] >= rimDepth {
				continue
			}
			t := float64(depth[i]) / rimDepth // 0 au bord, 1 à l'intérieur
			dark := 1 - zone*(1-t)*0.25
			fade := 1 - zone*(1-t)*0.6
			img.SetNRGBA(x, y, color.NRGBA{
				clamp255(float64(c.R) * dark),
				clamp255(float64(c.G) * dark),
				clamp255(float64(c.B) * dark),
				clamp255(float64(c.A) * fade),
			})
		}
	}
}

// makeBuildingShadow fabrique l'ombre portée (silhouette projetée vers le
// bas-gauche) et l'ombre de contact (ovale sombre sous le pied). Retourne
// l'image et la position de son coin haut-gauche relative au pied.
func makeBuildingShadow(img *image.NRGBA, opaque image.Rectangle, footX, footY float64) (*image.NRGBA, float64, float64) {
	H := footY - float64(opaque.Min.Y) // hauteur max au-dessus du pied
	pad := 16.0
	minX := float64(opaque.Min.X) - H*shadowShearX - pad
	maxX := float64(opaque.Max.X) + pad
	minY := footY - float64(opaque.Dy())*0.2 - pad
	maxY := float64(opaque.Max.Y) + H*shadowShearY + pad
	sw, sh := int(maxX-minX)+1, int(maxY-minY)+1
	acc := make([]float64, sw*sh)

	// Silhouette projetée : chaque pixel opaque au-dessus du pied est
	// « couché » au sol, d'autant plus loin qu'il est haut.
	for y := opaque.Min.Y; y < opaque.Max.Y; y++ {
		hgt := footY - float64(y)
		if hgt < 0 {
			hgt = 0
		}
		for x := opaque.Min.X; x < opaque.Max.X; x++ {
			if img.NRGBAAt(x, y).A < 128 {
				continue
			}
			px := float64(x) - hgt*shadowShearX - minX
			py := footY + hgt*shadowShearY - minY
			ix, iy := int(px), int(py)
			if ix >= 0 && iy >= 0 && ix < sw && iy < sh {
				acc[iy*sw+ix] = 1
			}
		}
	}
	// Bouche les trous laissés par la projection (étirée horizontalement).
	for pass := 0; pass < 2; pass++ {
		acc = boxBlur(acc, sw, sh, 2)
		for i := range acc {
			if acc[i] > 0.15 {
				acc[i] = 1
			}
		}
	}
	acc = boxBlur(acc, sw, sh, 5)
	acc = boxBlur(acc, sw, sh, 5)
	for i := range acc {
		acc[i] *= shadowAlpha
	}

	// Ombre de contact : ovale flou centré sous le pied.
	rx := float64(opaque.Dx()) * 0.42
	ry := float64(opaque.Dy()) * 0.12
	cx, cy := footX-minX, footY-minY
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
			d := math.Sqrt(dx*dx + dy*dy)
			if d < 1 {
				v := contactAlpha * (1 - d*d)
				i := y*sw + x
				acc[i] = 1 - (1-acc[i])*(1-v)
			}
		}
	}

	out := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	for i, a := range acc {
		out.Pix[i*4+0] = 18
		out.Pix[i*4+1] = 22
		out.Pix[i*4+2] = 8
		out.Pix[i*4+3] = clamp255(a * 255)
	}
	return out, minX - footX, minY - footY
}

// boxBlur : flou rapide (horizontal puis vertical) de rayon r.
func boxBlur(src []float64, w, h, r int) []float64 {
	tmp := make([]float64, w*h)
	dst := make([]float64, w*h)
	n := float64(2*r + 1)
	for y := 0; y < h; y++ {
		sum := 0.0
		for x := -r; x <= r; x++ {
			if x >= 0 && x < w {
				sum += src[y*w+x]
			}
		}
		for x := 0; x < w; x++ {
			tmp[y*w+x] = sum / n
			if x-r >= 0 {
				sum -= src[y*w+x-r]
			}
			if x+r+1 < w {
				sum += src[y*w+x+r+1]
			}
		}
	}
	for x := 0; x < w; x++ {
		sum := 0.0
		for y := -r; y <= r; y++ {
			if y >= 0 && y < h {
				sum += tmp[y*w+x]
			}
		}
		for y := 0; y < h; y++ {
			dst[y*w+x] = sum / n
			if y-r >= 0 {
				sum -= tmp[(y-r)*w+x]
			}
			if y+r+1 < h {
				sum += tmp[(y+r+1)*w+x]
			}
		}
	}
	return dst
}
