package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// =============================================================================
//  DÉFILEMENT DES LISTES  (ui_scroll.go)
// =============================================================================
//
// Une seule façon de faire défiler une liste, partout :
//   - Draw appelle scrollRegion(&offset, x, y, w, h, hauteurContenu) : la zone
//     est enregistrée, le décalage est borné et une barre apparaît sur le côté
//     quand le contenu dépasse ;
//   - au début de l'Update suivant, la molette (PC) ou le doigt (téléphone,
//     pixel pour pixel) fait défiler la zone sous le pointeur. La zone dessinée
//     en dernier (la fenêtre du dessus) passe en premier.
//
// Comme l'entrée est lue dans Update et pas dans Draw, rien n'est perdu ni
// compté deux fois quand le jeu fait plus de mises à jour que d'images.
// =============================================================================

const (
	scrollWheelStep = 48.0 // pixels par cran de molette
	scrollBarW      = 5.0
)

type scrollZone struct {
	off            *float64
	x, y, w, h     float64
	contentH       float64
	blocksOnly     bool // fenêtre qui arrête le défilement des zones en dessous
	blockX, blockY float64
	blockW, blockH float64
}

var (
	scrollZonesDrawing []scrollZone // zones de l'image en cours de dessin
	scrollZonesActive  []scrollZone // zones de la dernière image dessinée
	scrollConsumed     bool         // la molette / le doigt a déjà servi cette mise à jour
)

func beginScrollZones() { scrollZonesDrawing = scrollZonesDrawing[:0] }

func endScrollZones() {
	scrollZonesActive = append(scrollZonesActive[:0], scrollZonesDrawing...)
}

// scrollMax : décalage maximum pour un contenu de hauteur contentH dans h.
func scrollMax(h, contentH float64) float64 { return math.Max(0, contentH-h) }

// scrollRegion enregistre une zone qui défile, borne son décalage et dessine
// la barre de défilement si le contenu dépasse. À appeler pendant Draw.
func scrollRegion(screen *ebiten.Image, off *float64, x, y, w, h, contentH float64) {
	if off == nil || h <= 0 {
		return
	}
	maxOff := scrollMax(h, contentH)
	if *off > maxOff {
		*off = maxOff
	}
	if *off < 0 {
		*off = 0
	}
	scrollZonesDrawing = append(scrollZonesDrawing, scrollZone{off: off, x: x, y: y, w: w, h: h, contentH: contentH})
	if screen != nil && maxOff > 0 {
		drawScrollBar(screen, x+w-scrollBarW-3, y+4, h-8, h, contentH, *off)
	}
}

// scrollBlocker : une fenêtre (x, y, w, h) qui empêche de faire défiler les
// listes dessinées en dessous d'elle. À appeler pendant Draw, avant les
// scrollRegion de cette fenêtre.
func scrollBlocker(x, y, w, h float64) {
	scrollZonesDrawing = append(scrollZonesDrawing, scrollZone{blocksOnly: true, blockX: x, blockY: y, blockW: w, blockH: h})
}

func drawScrollBar(screen *ebiten.Image, x, y, trackH, viewH, contentH, off float64) {
	fillRoundRect(screen, x, y, scrollBarW, trackH, scrollBarW/2, withAlpha(themeGoldDim, 0.35))
	barH := math.Max(28, trackH*viewH/contentH)
	maxOff := scrollMax(viewH, contentH)
	by := y
	if maxOff > 0 {
		by += (trackH - barH) * (off / maxOff)
	}
	fillRoundRect(screen, x, by, scrollBarW, barH, scrollBarW/2, themeGold)
}

// applyScrollZones : début d'Update, la molette ou le doigt fait défiler la
// zone sous le pointeur.
func applyScrollZones() {
	scrollConsumed = false
	var px, py, delta float64
	p := &pointer
	if p.usingTouch {
		if !p.primaryActive || !p.moved || p.multi || !p.dragScrolls || p.scrollDY == 0 {
			return
		}
		// Le doigt monte (dy < 0) : le contenu monte, on descend dans la liste.
		px, py, delta = float64(p.startX), float64(p.startY), -p.scrollDY
	} else {
		_, wy := ebiten.Wheel()
		if wy == 0 {
			return
		}
		mx, my := ebiten.CursorPosition()
		px, py, delta = float64(mx), float64(my), -wy*scrollWheelStep
	}
	for i := len(scrollZonesActive) - 1; i >= 0; i-- {
		z := scrollZonesActive[i]
		if z.blocksOnly {
			if px >= z.blockX && px <= z.blockX+z.blockW && py >= z.blockY && py <= z.blockY+z.blockH {
				scrollConsumed = true
				return
			}
			continue
		}
		if px < z.x || px > z.x+z.w || py < z.y || py > z.y+z.h {
			continue
		}
		maxOff := scrollMax(z.h, z.contentH)
		if maxOff <= 0 {
			// Rien à faire défiler ici, mais la zone est sous le pointeur :
			// on ne fait pas défiler ce qu'il y a derrière.
			scrollConsumed = true
			return
		}
		v := *z.off + delta
		*z.off = math.Max(0, math.Min(maxOff, v))
		scrollConsumed = true
		return
	}
}

// clipTo : une vue de l'écran limitée au rectangle (x, y, w, h). Ce qui est
// dessiné dedans et dépasse est coupé net.
func clipTo(screen *ebiten.Image, x, y, w, h float64) *ebiten.Image {
	r := image.Rect(int(x), int(y), int(x+w), int(y+h)).Intersect(screen.Bounds())
	return screen.SubImage(r).(*ebiten.Image)
}
