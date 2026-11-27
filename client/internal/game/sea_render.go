package game

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
)

// sea_render.go : la carte de la mer en « fausse 3D ».
// Tout reste de la 2D : l'eau est faite de quelques textures qui glissent,
// les îles et les navires sont des images pré-rendues depuis des modèles 3D
// (navires en 16 directions), triées de haut en bas avec une ombre portée.
// Les tailles viennent de la maquette validée par Kevin ; seaWS convertit une
// unité de la maquette en unité du monde du jeu.

const seaWS = 0.42

const (
	seaShipSpr  = 256.0
	seaShipAncX = 128.0
	seaShipAncY = 176.0
	seaShipK    = 0.95
)

// Îles : 4 angles de vue du même modèle (ox, oy = point d'ancrage dans l'image).
var seaIslandAnchor = [4][2]float64{{477.5, 372.98}, {412.5, 406.04}, {477.5, 350.94}, {412.5, 424.6}}

// Longueur des coques (unités maquette), pour placer le sillage à la poupe.
var seaShipLen = map[string]float64{"sloop": 34, "brigantine": 44, "frigate": 54, "galleon": 62, "manowar": 70}

// Reflets du soleil posés sur les crêtes de la houle (x, y dans la tuile, phase, vitesse).
var seaGlints = [][4]float64{
	{100, 236, 2.774, 1.098}, {123, 196, 0.180, 1.233}, {26, 102, 1.669, 0.936}, {64, 209, 3.655, 0.962}, {3, 231, 4.547, 1.340}, {44, 55, 2.770, 1.206}, {162, 135, 4.616, 1.153}, {39, 126, 4.192, 1.136}, {122, 45, 3.971, 1.055}, {71, 59, 1.926, 1.174},
	{118, 32, 3.461, 1.049}, {248, 54, 3.825, 0.766}, {222, 114, 4.451, 0.709}, {25, 108, 4.755, 0.931}, {61, 133, 3.109, 1.298}, {108, 54, 1.730, 1.143}, {235, 38, 4.900, 1.047}, {245, 31, 2.437, 1.392}, {47, 56, 3.517, 0.616}, {29, 35, 3.900, 0.792},
	{36, 71, 1.356, 1.081}, {250, 86, 5.265, 0.659}, {56, 234, 3.500, 1.340}, {24, 111, 6.133, 1.276}, {107, 231, 1.285, 0.851}, {42, 44, 2.732, 1.216}, {122, 21, 1.081, 0.849}, {47, 237, 4.688, 0.655}, {226, 4, 4.165, 0.885}, {39, 52, 3.422, 0.653},
	{96, 13, 0.084, 1.019}, {9, 228, 5.733, 1.372}, {122, 50, 4.006, 1.134}, {32, 83, 0.220, 0.940}, {117, 221, 4.290, 0.906}, {237, 224, 5.392, 1.204}, {17, 46, 5.647, 0.894}, {0, 210, 3.967, 0.924}, {96, 252, 1.085, 1.301}, {111, 147, 5.714, 1.270},
	{100, 101, 2.530, 0.933}, {215, 52, 3.783, 1.248}, {74, 133, 5.615, 1.236}, {114, 73, 4.518, 0.988}, {115, 219, 2.627, 1.175}, {136, 44, 5.935, 1.197}, {202, 125, 4.535, 0.995}, {101, 114, 3.102, 0.641}, {28, 46, 0.315, 1.353}, {172, 59, 1.388, 1.103},
	{39, 49, 4.311, 0.720}, {106, 221, 0.166, 0.854}, {33, 219, 5.672, 0.681}, {6, 30, 2.162, 0.940}, {116, 196, 2.364, 0.995}, {215, 127, 3.949, 0.765}, {226, 27, 2.728, 1.271}, {184, 44, 2.941, 0.766}, {110, 24, 3.160, 0.881}, {107, 35, 4.769, 1.281},
	{21, 22, 4.827, 1.008}, {87, 131, 4.447, 0.721}, {250, 84, 3.718, 1.016}, {251, 26, 2.383, 0.745}, {211, 130, 1.708, 1.009}, {246, 42, 1.385, 0.954}, {106, 225, 2.042, 1.001}, {20, 208, 3.204, 0.945}, {137, 38, 3.164, 1.018}, {41, 246, 3.179, 0.767},
}

type seaWake struct{ x, y, vx, vy, life, max float64 }
type seaCap struct{ x, y, life, max, w float64 }

type seaGfx struct {
	tried, ok                 bool
	swell, chop, depth, tileB *ebiten.Image
	isl, ring                 [4]*ebiten.Image
	ships                     map[string][16]*ebiten.Image
	bg, halo, cloud, blob     *ebiten.Image
	ringOutline, px           *ebiten.Image
	vignette                  *ebiten.Image
	vigW, vigH                int

	wakes []seaWake
	caps  []seaCap
	heads map[string]float64
	t0    time.Time
	rnd   *rand.Rand

	// Réouverture de la mer : voir seaResyncIfReopened.
	lastTick  time.Time
	snapUntil atomic.Int64
}

func (s *seaGfx) load() bool {
	if s.tried {
		return s.ok
	}
	s.tried = true
	s.heads = map[string]float64{}
	s.t0 = time.Now()
	s.rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
	if loadImage == nil {
		return false
	}
	ok := true
	tile := func(p string) *ebiten.Image {
		im := loadImage(p)
		if im == nil {
			ok = false
			return nil
		}
		// Tuiles de 256 px avec 1 px de bordure recopiée : pas de couture entre tuiles.
		return im.SubImage(image.Rect(1, 1, 257, 257)).(*ebiten.Image)
	}
	const dir = "resources/sea/v3d/"
	s.swell, s.chop, s.depth, s.tileB = tile(dir+"swellP.png"), tile(dir+"chopP.png"), tile(dir+"depthP.png"), tile(dir+"tileBP.png")
	for v := 0; v < 4; v++ {
		s.isl[v] = loadImage(fmt.Sprintf("%sisland_v%d.png", dir, v))
		s.ring[v] = loadImage(fmt.Sprintf("%sisland_v%d_ring.png", dir, v))
		if s.isl[v] == nil {
			ok = false
		}
	}
	s.ships = map[string][16]*ebiten.Image{}
	for typ := range seaShipLen {
		atlas := loadImage(dir + "ship_" + typ + ".png")
		if atlas == nil {
			ok = false
			continue
		}
		var cells [16]*ebiten.Image
		for d := 0; d < 16; d++ {
			x, y := (d%4)*256, (d/4)*256
			cells[d] = atlas.SubImage(image.Rect(x, y, x+256, y+256)).(*ebiten.Image)
		}
		s.ships[typ] = cells
	}

	s.bg = seaLinearBG()
	s.halo = seaRadial(128, 255, 236, 190, 0.22)
	s.cloud = seaRadial(128, 3, 15, 25, 0.22)
	s.blob = seaBlob(32)
	s.ringOutline = seaRingImage(96, 3)
	px := ebiten.NewImage(3, 3)
	px.Fill(color.White)
	s.px = px.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

	s.ok = ok
	return ok
}

// ---------- textures générées ----------

func seaLinearBG() *ebiten.Image {
	const n = 64
	stops := [][4]float64{{0, 15, 88, 115}, {0.55, 11, 63, 92}, {1, 7, 44, 69}}
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			u, v := float64(x)/(n-1), float64(y)/(n-1)
			p := ((1 - u) + v) / 2
			a, b := stops[0], stops[1]
			if p > stops[1][0] {
				a, b = stops[1], stops[2]
			}
			k := (p - a[0]) / (b[0] - a[0])
			img.SetNRGBA(x, y, color.NRGBA{uint8(a[1] + (b[1]-a[1])*k), uint8(a[2] + (b[2]-a[2])*k), uint8(a[3] + (b[3]-a[3])*k), 255})
		}
	}
	return ebiten.NewImageFromImage(img)
}

// seaRadial : dégradé radial d'une couleur, opaque à alpha0 au centre, transparent au bord.
func seaRadial(n int, r, g, b uint8, alpha0 float64) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float64(n-1) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)-c, float64(y)-c) / c
			a := alpha0 * math.Max(0, 1-d)
			img.SetNRGBA(x, y, color.NRGBA{r, g, b, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

// seaBlob : disque blanc aux bords doux (écume, sillages).
func seaBlob(n int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float64(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)+0.5-c, float64(y)+0.5-c) / c
			a := 1.0
			if d > 0.75 {
				a = math.Max(0, (1-d)/0.25)
			}
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

func seaRingImage(n int, width float64) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float64(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Abs(math.Hypot(float64(x)+0.5-c, float64(y)+0.5-c) - (c - width))
			a := math.Max(0, math.Min(1, width/2+0.5-d))
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

func (s *seaGfx) vignetteFor(w, h int) *ebiten.Image {
	vw, vh := w/4+1, h/4+1
	if s.vignette != nil && s.vigW == vw && s.vigH == vh {
		return s.vignette
	}
	img := image.NewNRGBA(image.Rect(0, 0, vw, vh))
	W, H := float64(w), float64(h)
	r0, r1 := math.Min(W, H)*0.35, math.Max(W, H)*0.75
	for y := 0; y < vh; y++ {
		for x := 0; x < vw; x++ {
			d := math.Hypot(float64(x)*4-W/2, float64(y)*4-H/2)
			k := math.Max(0, math.Min(1, (d-r0)/(r1-r0)))
			img.SetNRGBA(x, y, color.NRGBA{0, 10, 20, uint8(k * 0.45 * 255)})
		}
	}
	if s.vignette != nil {
		s.vignette.Deallocate()
	}
	s.vignette = ebiten.NewImageFromImage(img)
	s.vigW, s.vigH = vw, vh
	return s.vignette
}

// ---------- petits outils de dessin ----------

func seaMod(a, m float64) float64 {
	r := math.Mod(a, m)
	if r < 0 {
		r += m
	}
	return r
}

func (g *Game) seaToScreen(x, y float64) (float64, float64) {
	return (x-g.camX)*g.camZoom + float64(g.screenWidth)/2, (y-g.camY)*g.camZoom + float64(g.screenHeight)/2
}

func (s *seaGfx) rect(dst *ebiten.Image, x, y, w, h float64, r, g, b, a float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w, h)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(r*a), float32(g*a), float32(b*a), float32(a))
	dst.DrawImage(s.px, op)
}

// ellipse : disque doux étiré (rx, ry en pixels), tourné de rot.
func (s *seaGfx) ellipse(dst *ebiten.Image, cx, cy, rx, ry, rot float64, r, g, b, a float64) {
	if rx <= 0 || ry <= 0 || a <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-16, -16)
	op.GeoM.Scale(rx/16, ry/16)
	op.GeoM.Rotate(rot)
	op.GeoM.Translate(cx, cy)
	op.ColorScale.Scale(float32(r*a), float32(g*a), float32(b*a), float32(a))
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(s.blob, op)
}

func (s *seaGfx) radial(dst, img *ebiten.Image, cx, cy, rx, ry float64) {
	n := float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-n/2, -n/2)
	op.GeoM.Scale(2*rx/n, 2*ry/n)
	op.GeoM.Translate(cx, cy)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(img, op)
}

// layer : une texture d'eau répétée qui glisse (sc = taille d'une tuile en
// unités maquette / 256, ax = étirement horizontal dû au vent).
func (g *Game) seaLayer(dst, tile *ebiten.Image, sc, vx, vy, alpha, t, ax float64, lighter bool) (float64, float64, float64, float64) {
	ez := g.camZoom * seaWS
	W, H := float64(g.screenWidth), float64(g.screenHeight)
	TX, T := 256*sc*ax*ez, 256*sc*ez
	ox := seaMod(-g.camX*g.camZoom+W/2+t*vx*ez, TX) - TX
	oy := seaMod(-g.camY*g.camZoom+H/2+t*vy*ez, T) - T
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	if lighter {
		op.Blend = ebiten.BlendLighter
	}
	// Positions arrondies au pixel, sans chevauchement : pas de ligne entre deux tuiles.
	for j := 0; oy+float64(j)*T < H; j++ {
		y0, y1 := math.Round(oy+float64(j)*T), math.Round(oy+float64(j+1)*T)
		for i := 0; ox+float64(i)*TX < W; i++ {
			x0, x1 := math.Round(ox+float64(i)*TX), math.Round(ox+float64(i+1)*TX)
			op.GeoM.Reset()
			op.GeoM.Scale((x1-x0)/256, (y1-y0)/256)
			op.GeoM.Translate(x0, y0)
			op.ColorScale.Reset()
			op.ColorScale.ScaleAlpha(float32(alpha))
			dst.DrawImage(tile, op)
		}
	}
	return ox, oy, TX, T
}

// ---------- l'eau ----------

// drawSeaWater dessine la mer. Renvoie false si les images manquent (on garde
// alors l'ancien shader).
func (g *Game) drawSeaWater(screen *ebiten.Image) bool {
	s := &g.sea
	if !s.load() {
		return false
	}
	t := time.Since(s.t0).Seconds()
	W, H := float64(g.screenWidth), float64(g.screenHeight)
	ez := g.camZoom * seaWS

	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(W/64, H/64)
	screen.DrawImage(s.bg, op)

	g.seaLayer(screen, s.depth, 7, 1.5, 0.8, 0.55, t, 1, false)                      // zones plus ou moins profondes
	sx0, sy0, STX, ST := g.seaLayer(screen, s.swell, 2.2, 8, 2, 0.75, t, 2.2, false) // grande houle, étirée par le vent
	g.seaLayer(screen, s.chop, 0.8, 5, 6, 0.3, t, 1.6, false)                        // clapot fin
	g.seaLayer(screen, s.tileB, 3.2, -6, 7, 0.07, t, 1, true)                        // reflets sous la surface

	// Scintillements du soleil sur les crêtes.
	kx, k := STX/256, ST/256
	for y := sy0; y < H; y += ST {
		for x := sx0; x < W; x += STX {
			for _, gl := range seaGlints {
				a := math.Pow(math.Max(0, math.Sin(t*gl[3]*2.2+gl[2])), 14)
				if a < 0.05 {
					continue
				}
				px, py := x+gl[0]*kx, y+gl[1]*k
				if px < -4 || py < -4 || px > W+4 || py > H+4 {
					continue
				}
				sun := 0.45 + 0.55*math.Max(0, px/W-py/H+0.4)
				r := 1.2 + a*1.6*ez/0.85
				s.rect(screen, px-r*2, py-0.5, r*4, 1, 1, 0.984, 0.91, a*sun)
				s.rect(screen, px-0.5, py-r, 1, r*2, 1, 0.984, 0.91, a*sun)
			}
		}
	}

	// Moutons d'écume qui naissent et s'effacent, poussés par le vent.
	dt := g.dt
	if len(s.caps) < 26 && s.rnd.Float64() < 0.25 {
		s.caps = append(s.caps, seaCap{
			x: g.camX + (s.rnd.Float64()-0.5)*W/g.camZoom, y: g.camY + (s.rnd.Float64()-0.5)*H/g.camZoom,
			max: 2.5 + s.rnd.Float64()*2, w: 10 + s.rnd.Float64()*16,
		})
	}
	caps := s.caps[:0]
	for _, c := range s.caps {
		c.life += dt
		c.x += 9 * seaWS * dt
		if c.life > c.max {
			continue
		}
		caps = append(caps, c)
		px, py := g.seaToScreen(c.x, c.y)
		f := math.Sin(math.Pi * c.life / c.max)
		s.ellipse(screen, px, py, c.w*ez*(0.6+0.4*f), math.Max(0.8, 1.6*ez), -0.12, 0.92, 0.97, 1, 0.5*f)
		s.ellipse(screen, px-c.w*0.3*ez, py+2.5*ez, c.w*0.5*ez, math.Max(0.6, 1.1*ez), -0.12, 0.92, 0.97, 1, 0.25*f)
	}
	s.caps = caps

	// Halo du soleil (en haut à droite, comme sur l'île).
	R := math.Max(W, H) * 0.75
	s.radial(screen, s.halo, W*0.92, -H*0.1, R, R)
	return true
}

// ---------- la scène : îles, navires, étiquettes ----------

type seaItem struct {
	y    float64
	draw func()
}

type seaLabel struct {
	x, y       float64
	title, sub string
	accent     color.Color
	stars      int
	extra      func(x, top float64)
}

// seaHeading : cap lissé d'une flotte. (dx, dy) = direction du mouvement sur la
// carte ; la carte est vue de 3/4, d'où le 0.6 sur l'axe vertical.
func (g *Game) seaHeading(key string, dx, dy float64, moving bool) float64 {
	s := &g.sea
	h, known := s.heads[key]
	if moving && (dx != 0 || dy != 0) {
		target := math.Atan2(dy/0.6, dx)
		if !known {
			h = target
		} else {
			d := target - h
			for d > math.Pi {
				d -= 2 * math.Pi
			}
			for d < -math.Pi {
				d += 2 * math.Pi
			}
			h += d * math.Min(1, g.dt*3)
		}
	} else if !known {
		h = 0.35 // cap par défaut, légèrement vers le bas-droite
	}
	s.heads[key] = h
	return h
}

// seaFleet ajoute à la scène les navires d'une flotte (le premier en tête,
// les autres en formation derrière lui).
func (g *Game) seaFleet(dst *ebiten.Image, items *[]seaItem, key string, types []string, x, y, head float64, moving, hover bool, t float64) {
	s := &g.sea
	ez := g.camZoom * seaWS
	k := ez * seaShipK
	ca, sa := math.Cos(head), math.Sin(head)
	di := int(math.Round(seaMod(head, 2*math.Pi)/(2*math.Pi)*16)) % 16
	for i, typ := range types {
		cells, ok := s.ships[typ]
		if !ok {
			typ = "sloop"
			cells, ok = s.ships[typ]
			if !ok {
				continue
			}
		}
		px, py := x, y
		if i > 0 {
			back := 95 * seaWS * math.Ceil(float64(i)/2)
			lat := 62 * seaWS
			if i%2 == 0 {
				lat = -lat
			}
			px = x - back*ca - lat*sa
			py = y + (-back*sa+lat*ca)*0.6
		}
		// Sillage : petites particules d'écume à la poupe.
		if moving && s.rnd.Float64() < g.dt*22 {
			L := seaShipLen[typ] * seaWS
			s.wakes = append(s.wakes, seaWake{
				x: px - ca*L*0.5, y: py - sa*L*0.3 + 3*seaWS,
				vx: (-ca*6 + (s.rnd.Float64()-0.5)*10) * seaWS, vy: (-sa*6 + (s.rnd.Float64()-0.5)*10) * 0.6 * seaWS,
				max: 2.4 + s.rnd.Float64(),
			})
		}
		spr := cells[di]
		idx := float64(i)
		bob := math.Sin(t*1.6+idx*1.7+px*0.01/seaWS) * 1.3 * ez / 0.85
		roll := math.Sin(t*1.25+idx*2.1) * 0.025
		lead := i == 0
		// Ombre portée projetée sur l'eau (soleil en haut à droite), sous tout le reste.
		*items = append(*items, seaItem{y: py - 1e6, draw: func() {
			sx, sy := g.seaToScreen(px, py)
			var shear ebiten.GeoM
			shear.SetElement(0, 1, 0.75)
			shear.SetElement(1, 1, -0.32)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Translate(-seaShipAncX, -seaShipAncY)
			op.GeoM.Scale(k, k)
			op.GeoM.Concat(shear)
			op.GeoM.Translate(sx, sy+2)
			op.ColorScale.Scale(0, 0, 0, 0.38)
			dst.DrawImage(spr, op)
		}})
		*items = append(*items, seaItem{y: py, draw: func() {
			sx, sy := g.seaToScreen(px, py)
			fw := 1.0
			if typ == "galleon" || typ == "manowar" {
				fw = 1.4
			}
			s.ellipse(dst, sx, sy+bob*0.3, 26*k*fw, 9*k, 0, 0.92, 0.98, 1, 0.35)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Translate(-seaShipAncX, -seaShipAncY)
			op.GeoM.Scale(k, k)
			op.GeoM.Rotate(roll)
			op.GeoM.Translate(sx, sy+bob)
			dst.DrawImage(spr, op)
			if hover && lead {
				n := float64(s.ringOutline.Bounds().Dx())
				rop := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
				rop.GeoM.Translate(-n/2, -n/2)
				rop.GeoM.Scale(104*k/n, 44*k/n)
				rop.GeoM.Translate(sx, sy+3)
				rop.ColorScale.Scale(0.89*0.9, 0.71*0.9, 0.33*0.9, 0.9)
				dst.DrawImage(s.ringOutline, rop)
			}
		}})
	}
}

// seaShipHit : le pointeur (mx, my) touche-t-il un navire posé en (sx, sy) à l'écran ?
func (g *Game) seaShipHit(sx, sy, mx, my float64) bool {
	ez := g.camZoom * seaWS
	hw := math.Max(14, 48*ez)
	top := math.Max(20, 95*ez)
	bot := math.Max(10, 22*ez)
	return mx >= sx-hw && mx <= sx+hw && my >= sy-top && my <= sy+bot
}

// seaIslandHit : le pointeur touche-t-il une île posée en (sx, sy) ?
func (g *Game) seaIslandHit(sx, sy, mx, my float64) bool {
	ez := g.camZoom * seaWS
	rx, ry := math.Max(12, 160*ez), math.Max(10, 85*ez)
	dx, dy := (mx-sx)/rx, (my-(sy-15*ez))/ry
	return dx*dx+dy*dy <= 1
}

func seaIslandView(id string) int {
	h := fnv.New32a()
	h.Write([]byte(id))
	return int(h.Sum32() % 4)
}

func (g *Game) seaIsland(dst *ebiten.Image, items *[]seaItem, view int, x, y, k float64, t float64) {
	s := &g.sea
	img, ring := s.isl[view], s.ring[view]
	if img == nil {
		return
	}
	anc := seaIslandAnchor[view]
	*items = append(*items, seaItem{y: y + 45*seaWS, draw: func() {
		sx, sy := g.seaToScreen(x, y)
		zk := g.camZoom * seaWS * k
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(zk, zk)
		op.GeoM.Translate(sx-anc[0]*zk, sy-anc[1]*zk)
		dst.DrawImage(img, op)
		if ring != nil {
			op.ColorScale.ScaleAlpha(float32(0.45 + 0.3*math.Sin(t*1.4)))
			dst.DrawImage(ring, op)
		}
	}})
}

// seaPlaque : étiquette bois et or (nom + sous-titre), centrée en x, bas en y.
func (g *Game) seaPlaque(dst *ebiten.Image, l seaLabel) {
	tf, sf := bodyFace(12, true), bodyFace(11, false)
	w1 := textWidth(l.title, tf)
	w2 := 0.0
	if l.sub != "" {
		w2 = textWidth(l.sub, sf)
		if l.stars > 0 {
			w2 += 6 + 3*10
		}
	}
	w := math.Max(w1, w2) + 18
	h := 20.0
	if l.sub != "" {
		h = 34
	}
	bx, by := math.Round(l.x-w/2), math.Round(l.y-h)
	accent := l.accent
	if accent == nil {
		accent = color.NRGBA{201, 154, 69, 255}
	}
	fillRoundRect(dst, bx+2, by+3, w, h, 5, color.NRGBA{0, 0, 0, 90})
	fillRoundRect(dst, bx, by, w, h, 5, color.NRGBA{59, 39, 22, 255})
	strokeRoundRect(dst, bx, by, w, h, 5, 1.2, accent)
	drawText(dst, l.title, tf, l.x, by+3, color.NRGBA{243, 220, 160, 255}, alignCenter)
	if l.sub != "" {
		if l.stars > 0 {
			sx := l.x - w2/2
			drawText(dst, l.sub, sf, sx, by+18, color.NRGBA{199, 180, 141, 255}, alignLeft)
			drawStars(dst, sx+textWidth(l.sub, sf)+6, by+25, l.stars, 3, 4, false)
		} else {
			drawText(dst, l.sub, sf, l.x, by+18, color.NRGBA{199, 180, 141, 255}, alignCenter)
		}
	}
	if l.extra != nil {
		l.extra(l.x, by+h)
	}
}

var seaPveFleets = map[int][]string{1: {"sloop"}, 2: {"brigantine", "sloop"}, 3: {"galleon", "frigate", "frigate"}}

// drawSeaScene dessine îles, flottes (joueur, PvE, PvP), sillages, nuages et
// étiquettes, dans l'ordre de profondeur (ce qui est plus bas passe devant).
func (g *Game) drawSeaScene(screen *ebiten.Image, isModalOpen bool) {
	s := &g.sea
	t := time.Since(s.t0).Seconds()
	ez := g.camZoom * seaWS
	W, H := float64(g.screenWidth), float64(g.screenHeight)
	var items []seaItem
	var labels []seaLabel
	labelUp := 95 * seaWS // au-dessus des mâts

	// Ton île.
	if g.player != nil && len(g.player.Islands) > 0 {
		my := g.player.Islands[0]
		x, y := float64(my.X), float64(my.Y)
		g.seaIsland(screen, &items, 2, x, y, 0.72, t)
		lx, ly := g.seaToScreen(x, y-123*seaWS)
		labels = append(labels, seaLabel{x: lx, y: ly, title: my.Name, sub: "Ton île", accent: color.NRGBA{108, 196, 138, 255}})
	}

	// Flottes PvE.
	g.ui.PveTargetsMutex.RLock()
	for _, target := range g.ui.PveTargets {
		target := target
		dx, dy, moving := 0.0, 0.0, false
		if target.TargetX != nil && target.TargetY != nil {
			dx, dy = *target.TargetX-target.RealX, *target.TargetY-target.RealY
			moving = math.Hypot(dx, dy) > 2
		}
		head := g.seaHeading("pve:"+target.ID, dx, dy, moving)
		types := seaPveFleets[target.Tier]
		if types == nil {
			types = seaPveFleets[1]
		}
		hover := !isModalOpen && g.ui.HoverPveTarget == target.ID
		g.seaFleet(screen, &items, "pve:"+target.ID, types, target.RealX, target.RealY, head, moving, hover, t)

		lx, ly := g.seaToScreen(target.RealX, target.RealY-labelUp)
		var accent color.Color
		if target.Tier == 3 {
			accent = color.NRGBA{227, 181, 83, 255}
		}
		l := seaLabel{x: lx, y: ly - 8, title: target.Name, sub: fmt.Sprintf("Palier %d", target.Tier), stars: target.Tier, accent: accent}
		if hover {
			l.extra = func(cx, top float64) { g.seaDangerRating(screen, target, cx, top+6) }
		}
		labels = append(labels, l)
	}
	g.ui.PveTargetsMutex.RUnlock()

	// Joueurs : îles et flottes en PvP.
	var hoverPvp *domain.PveTarget
	for i := range g.ui.PvpTargets {
		target := g.ui.PvpTargets[i]
		x, y := float64(target.X), float64(target.Y)
		if target.IsFleet {
			dx, dy, moving := 0.0, 0.0, false
			if target.Speed > 0 && target.TargetX != nil && target.TargetY != nil {
				dx, dy = *target.TargetX-target.RealX, *target.TargetY-target.RealY
				moving = math.Hypot(dx, dy) > 2
			}
			hover := !isModalOpen && g.ui.HoverPvpTarget == target.ID
			head := g.seaHeading("pvp:"+target.ID, dx, dy, moving)
			g.seaFleet(screen, &items, "pvp:"+target.ID, []string{"frigate", "sloop"}, x, y, head, moving, hover, t)
			name := target.Name
			if name == "" {
				name = "Flotte inconnue"
			}
			lx, ly := g.seaToScreen(x, y-labelUp)
			labels = append(labels, seaLabel{x: lx, y: ly - 8, title: name, sub: "Flotte d'un joueur", accent: color.NRGBA{214, 84, 70, 255}})
		} else {
			if g.player != nil && target.Name == g.player.Username {
				continue // ton île est déjà dessinée
			}
			g.seaIsland(screen, &items, seaIslandView(target.ID), x, y, 0.62, t)
			name := target.Name
			if name == "" {
				name = "Inconnu"
			}
			lx, ly := g.seaToScreen(x, y-112*seaWS)
			labels = append(labels, seaLabel{x: lx, y: ly, title: name, sub: fmt.Sprintf("Joueur · niv. %d", target.Tier)})
		}
		if !isModalOpen && g.ui.HoverPvpTarget == target.ID && target.ID != "" {
			hoverPvp = &g.ui.PvpTargets[i]
		}
	}

	// Tes flottes en mer (en route ou stationnées).
	g.ui.PlayerMutex.RLock()
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, f := range g.player.Islands[0].Fleets {
			if !(f.IsMoving() || f.State == domain.FleetStateSeaStationed) || len(f.Ships) == 0 {
				continue
			}
			lead := f.Ships[0]
			dx, dy, moving := 0.0, 0.0, false
			if f.IsMoving() && f.TargetX != nil && f.TargetY != nil {
				dx, dy = float64(*f.TargetX)-lead.RealX, float64(*f.TargetY)-lead.RealY
				moving = math.Hypot(dx, dy) > 1
			}
			key := "fleet:" + f.ID.String()
			head := g.seaHeading(key, dx, dy, moving)
			var types []string
			for _, sh := range f.Ships {
				if len(types) == 3 {
					break
				}
				types = append(types, sh.Type)
			}
			g.seaFleet(screen, &items, key, types, lead.RealX, lead.RealY, head, moving, false, t)
			sub := "En station"
			if f.IsMoving() {
				sub = "En route"
			}
			if n := len(f.Ships); n > 1 {
				sub += fmt.Sprintf(" · %d navires", n)
			}
			lx, ly := g.seaToScreen(lead.RealX, lead.RealY-labelUp)
			labels = append(labels, seaLabel{x: lx, y: ly - 8, title: f.Name, sub: sub, accent: color.NRGBA{108, 196, 138, 255}})
		}
	}
	g.ui.PlayerMutex.RUnlock()

	// Sillages, sous les navires.
	dt := g.dt
	wakes := s.wakes[:0]
	for _, p := range s.wakes {
		p.life += dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		if p.life <= p.max {
			wakes = append(wakes, p)
		}
	}
	if len(wakes) > 260 {
		wakes = wakes[len(wakes)-260:]
	}
	s.wakes = wakes
	for _, p := range s.wakes {
		sx, sy := g.seaToScreen(p.x, p.y)
		if sx < -20 || sy < -20 || sx > W+20 || sy > H+20 {
			continue
		}
		k := 1 - p.life/p.max
		s.ellipse(screen, sx, sy, (2+(1-k)*6)*ez, (1.2+(1-k)*3)*ez, 0, 0.92, 0.98, 1, k*0.55)
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].y < items[j].y })
	for _, it := range items {
		it.draw()
	}

	// Ombres de nuages qui passent (elles suivent la caméra pour ne jamais manquer).
	period := 2400 * seaWS
	for _, c := range [][3]float64{{-500, -200, 260}, {300, 300, 320}, {900, -600, 240}, {-900, 600, 300}} {
		wx := g.camX + seaMod(c[0]*seaWS+t*14*seaWS-g.camX+period/2, period) - period/2
		wy := g.camY + seaMod(c[1]*seaWS-g.camY+period/2, period) - period/2
		sx, sy := g.seaToScreen(wx, wy)
		R := c[2] * ez
		s.radial(screen, s.cloud, sx, sy, R, R*0.55)
	}

	// Vignette douce.
	vg := s.vignetteFor(g.screenWidth, g.screenHeight)
	vop := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	vop.GeoM.Scale(W/float64(vg.Bounds().Dx()-1), H/float64(vg.Bounds().Dy()-1))
	screen.DrawImage(vg, vop)

	// Étiquettes par-dessus tout (elles sont trop chargées quand on dézoome beaucoup).
	for _, l := range labels {
		if l.y < -40 || l.y > H+60 || l.x < -150 || l.x > W+150 {
			continue
		}
		g.seaPlaque(screen, l)
	}

	if hoverPvp != nil {
		g.seaPvpTooltip(screen, *hoverPvp)
	}
}

// seaDangerRating : pastille de danger sous l'étiquette d'une flotte PvE survolée.
func (g *Game) seaDangerRating(screen *ebiten.Image, target domain.PveTarget, cx, y float64) {
	var activeFleet *domain.Fleet
	if g.player != nil && len(g.player.Islands) > 0 {
		island := g.player.Islands[0]
		if island.ActiveFleetID != nil {
			for _, f := range island.Fleets {
				if f.ID == *island.ActiveFleetID {
					f := f
					activeFleet = &f
					break
				}
			}
		}
	}
	rating := GetDangerRating(activeFleet, target)
	textW := float64(len(rating.Label) * 6)
	textX := cx - textW/2
	uiFillRect(screen, float32(textX-5), float32(y-2), float32(textW+10), 16, color.RGBA{0, 0, 0, 200}, vecAA)
	uiPrint(screen, rating.Label, int(textX), int(y))
	uiFillRect(screen, float32(textX-15), float32(y+2), 8, 8, rating.Color, vecAA)
}

// seaPvpTooltip : infos d'un joueur survolé (même contenu qu'avant).
func (g *Game) seaPvpTooltip(screen *ebiten.Image, target domain.PveTarget) {
	if (g.player != nil && target.Name == g.player.Username) || target.Name == "" || target.Tier == 0 {
		return
	}
	tx, ty := g.seaToScreen(float64(target.X), float64(target.Y))
	tooltipW, tooltipH := 220.0, 70.0
	tooltipX := tx + 60*g.camZoom*seaWS + 10
	tooltipY := ty - tooltipH/2
	uiFillRect(screen, float32(tooltipX), float32(tooltipY), float32(tooltipW), float32(tooltipH), color.RGBA{20, 20, 30, 230}, vecAA)
	uiStrokeRect(screen, float32(tooltipX), float32(tooltipY), float32(tooltipW), float32(tooltipH), 2, color.RGBA{218, 165, 32, 255}, vecAA)
	uiPrint(screen, fmt.Sprintf("Joueur: %s", target.Name), int(tooltipX)+10, int(tooltipY)+10)
	uiPrint(screen, fmt.Sprintf("TH Niveau: %d", target.Tier), int(tooltipX)+10, int(tooltipY)+25)
	uiPrint(screen, fmt.Sprintf("Butin: %s", g.computeLootClass(target.Tier)), int(tooltipX)+10, int(tooltipY)+40)
}

// ---------- réouverture de la mer ----------

// Quand la mer est fermée, la simulation des flottes s'arrête : à la
// réouverture, les positions affichées sont celles d'il y a longtemps et les
// navires « foncent » rattraper leur vraie position. On les place donc
// directement là où ils doivent être, puis les premières réponses du serveur
// sont prises telles quelles (sans lissage) pendant quelques secondes.
const seaSnapWindow = 3 * time.Second

func (g *Game) seaResyncIfReopened() {
	s := &g.sea
	now := time.Now()
	gap := now.Sub(s.lastTick)
	first := s.lastTick.IsZero()
	s.lastTick = now
	if !first && gap < 600*time.Millisecond {
		return
	}
	s.snapUntil.Store(now.Add(seaSnapWindow).UnixNano())
	g.lastPvePoll = time.Time{} // redemande tout au serveur tout de suite
	if first {
		return
	}
	secs := gap.Seconds()

	// Flottes PvE : avance le long de leur route le temps de l'absence.
	g.ui.PveTargetsMutex.Lock()
	for i := range g.ui.PveTargets {
		t := &g.ui.PveTargets[i]
		if t.TargetX != nil && t.TargetY != nil {
			dx, dy := *t.TargetX-t.X, *t.TargetY-t.Y
			if d := math.Hypot(dx, dy); d > 0 {
				sp := math.Max(t.Speed, 5)
				m := math.Min(d, sp*secs)
				t.X += dx / d * m
				t.Y += dy / d * m
			}
		}
		t.RealX, t.RealY = t.X, t.Y
	}
	g.ui.PveTargetsMutex.Unlock()

	// Tes flottes : même chose (vitesse de base ; le serveur corrige ensuite).
	g.ui.PlayerMutex.Lock()
	if g.player != nil && len(g.player.Islands) > 0 {
		for i := range g.player.Islands[0].Fleets {
			f := &g.player.Islands[0].Fleets[i]
			for j := range f.Ships {
				sh := &f.Ships[j]
				if f.IsMoving() && f.TargetX != nil && f.TargetY != nil {
					dx, dy := float64(*f.TargetX)-sh.X, float64(*f.TargetY)-sh.Y
					if d := math.Hypot(dx, dy); d > 0 {
						m := math.Min(d, 5*secs)
						sh.X += dx / d * m
						sh.Y += dy / d * m
					}
				}
				sh.RealX, sh.RealY = sh.X, sh.Y
			}
		}
	}
	g.ui.PlayerMutex.Unlock()
	// Les caps repartent de zéro : pas de navire qui pivote sur lui-même.
	s.heads = map[string]float64{}
	s.wakes = s.wakes[:0]
}

// seaSnapping : vrai juste après l'ouverture de la mer ; les positions reçues
// du serveur remplacent alors directement les positions affichées.
func (g *Game) seaSnapping() bool {
	return time.Now().UnixNano() < g.sea.snapUntil.Load()
}
