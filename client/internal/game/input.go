package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// input.go : une seule porte d'entrée pour la souris ET le tactile.
//
// Tout le jeu lit la souris via cursorPosition(), isMouseJustPressed(),
// isMousePressed() et wheelDelta() au lieu des fonctions d'Ebiten. Sur PC
// rien ne change. Sur téléphone :
//   - un tap (doigt posé puis levé sans bouger) = un clic gauche ;
//   - un doigt qui glisse sur la carte = déplacer la caméra ;
//   - un doigt qui glisse dans une fenêtre = la molette (faire défiler) ;
//   - deux doigts qui s'écartent / se rapprochent = zoom.
//
// Le clic tactile part quand le doigt se lève (comme sur un vrai bouton de
// téléphone) : ça permet de faire glisser une liste sans cliquer sur ce qu'il
// y a sous le doigt au départ.

const (
	// Distance (en pixels du jeu) au-delà de laquelle un tap devient un glisser.
	touchDragThreshold = 18.0
	// Distance de glisser qui vaut un cran de molette.
	touchWheelStep = 32.0
)

type pointerState struct {
	usingTouch  bool // la dernière action venait d'un doigt
	everTouched bool // un doigt a déjà touché l'écran depuis le lancement

	x, y             int // position du doigt (pixels du jeu)
	hideAfterRelease bool

	primary       ebiten.TouchID
	primaryActive bool
	startX        int
	startY        int
	lastX, lastY  int
	moved         bool // le doigt a assez bougé : ce n'est plus un tap
	multi         bool // un deuxième doigt s'est posé pendant ce geste
	dragScrolls   bool // le glisser fait défiler (fenêtre) au lieu de déplacer la carte

	pressed     bool // équivalent « bouton gauche enfoncé »
	justPressed bool // équivalent « clic gauche » pour cette frame

	wheelY   float64 // crans de molette générés cette frame
	wheelAcc float64

	panDX, panDY float64 // déplacement de caméra demandé (pixels écran)

	pinchDist    float64 // écart entre les deux doigts à la frame précédente
	pinchCenterX float64
	pinchCenterY float64
	zoomFactor   float64 // facteur de zoom demandé cette frame (1 = rien)

	lastMouseX, lastMouseY int
	mouseConsumed          bool // clic souris pris par un bouton rond
	clickForDraw           bool // clic pas encore vu par un Draw

	// Tap trop rapide pour être vu par le jeu (doigt posé et levé entre deux
	// frames) : le navigateur nous le signale ici (voir softkeyboard_js.go).
	frame      uint64
	missedTap  bool
	missedTapX int
	missedTapY int

	// Touches « virtuelles » envoyées par les boutons tactiles ou le clavier
	// du téléphone, valables une frame.
	virtKeys     map[ebiten.Key]bool
	nextVirtKeys map[ebiten.Key]bool
}

var pointer = pointerState{zoomFactor: 1}

var touchIDsBuf []ebiten.TouchID

// updatePointer est appelé une fois au tout début de chaque Update.
func (g *Game) updatePointer() {
	p := &pointer

	// Touches virtuelles : celles demandées depuis la dernière frame.
	p.virtKeys = p.nextVirtKeys
	p.nextVirtKeys = nil

	p.frame++
	p.justPressed = false
	p.wheelY = 0
	p.panDX, p.panDY = 0, 0
	p.zoomFactor = 1

	// La souris a bougé ou cliqué : on repasse en mode souris.
	touchIDsBuf = ebiten.AppendTouchIDs(touchIDsBuf[:0])
	ids := touchIDsBuf

	mx, my := ebiten.CursorPosition()
	if len(ids) == 0 && (mx != p.lastMouseX || my != p.lastMouseY ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)) {
		p.usingTouch = false
	}
	p.lastMouseX, p.lastMouseY = mx, my

	// Clic souris sur un bouton rond : on le prend pour nous.
	p.mouseConsumed = false
	if len(ids) == 0 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && g.consumeTouchButtonTap(mx, my) {
		p.mouseConsumed = true
	}

	if p.hideAfterRelease && len(ids) == 0 {
		// Le doigt est levé depuis une frame : on « retire » le curseur pour
		// que les effets de survol s'éteignent.
		p.x, p.y = -10000, -10000
		p.hideAfterRelease = false
		p.pressed = false
	}

	if len(ids) > 0 {
		p.usingTouch = true
		p.everTouched = true
		p.hideAfterRelease = false
	}

	// Le doigt principal est-il toujours posé ?
	stillDown := false
	if p.primaryActive {
		for _, id := range ids {
			if id == p.primary {
				stillDown = true
				break
			}
		}
	}

	if p.missedTap && !p.primaryActive {
		p.missedTap = false
		p.usingTouch = true
		p.everTouched = true
		p.x, p.y = p.missedTapX, p.missedTapY
		if !g.consumeTouchButtonTap(p.x, p.y) {
			p.justPressed = true
			p.pressed = true
		}
		p.hideAfterRelease = true
		return
	}

	switch {
	case p.primaryActive && !stillDown:
		// Doigt levé.
		p.primaryActive = false
		if !p.moved && !p.multi {
			if !g.consumeTouchButtonTap(p.x, p.y) {
				p.justPressed = true
				p.pressed = true // le code qui teste « enfoncé » voit aussi le clic
			} else {
				p.pressed = false
			}
		} else {
			p.pressed = false
		}
		p.hideAfterRelease = true
		p.pinchDist = 0

	case !p.primaryActive && len(ids) > 0:
		// Nouveau geste.
		p.primary = ids[0]
		p.primaryActive = true
		x, y := ebiten.TouchPosition(p.primary)
		p.x, p.y = x, y
		p.startX, p.startY = x, y
		p.lastX, p.lastY = x, y
		p.moved = false
		p.multi = len(ids) > 1
		p.pressed = true
		p.wheelAcc = 0
		p.pinchDist = 0
		p.dragScrolls = g.touchDragScrollsAt(x, y)

	case p.primaryActive:
		x, y := ebiten.TouchPosition(p.primary)
		p.x, p.y = x, y
		if math.Hypot(float64(x-p.startX), float64(y-p.startY)) > touchDragThreshold {
			p.moved = true
		}

		if len(ids) >= 2 {
			p.multi = true
			p.pressed = false
			ax, ay := ebiten.TouchPosition(ids[0])
			bx, by := ebiten.TouchPosition(ids[1])
			d := math.Hypot(float64(ax-bx), float64(ay-by))
			cx, cy := float64(ax+bx)/2, float64(ay+by)/2
			if p.pinchDist > 0 && d > 0 {
				p.zoomFactor = d / p.pinchDist
				p.panDX = cx - p.pinchCenterX
				p.panDY = cy - p.pinchCenterY
			}
			p.pinchDist = d
			p.pinchCenterX, p.pinchCenterY = cx, cy
		} else if !p.multi && p.moved {
			dx, dy := float64(x-p.lastX), float64(y-p.lastY)
			if p.dragScrolls {
				// Doigt qui monte = contenu qui monte = molette vers le bas.
				p.wheelAcc += dy
				for p.wheelAcc >= touchWheelStep {
					p.wheelY++
					p.wheelAcc -= touchWheelStep
				}
				for p.wheelAcc <= -touchWheelStep {
					p.wheelY--
					p.wheelAcc += touchWheelStep
				}
			} else {
				p.panDX, p.panDY = dx, dy
			}
		}
		p.lastX, p.lastY = x, y
	}
}

// touchDragScrollsAt dit si un glisser qui commence en (x, y) doit faire
// défiler une liste (fenêtre ouverte, chat) plutôt que déplacer la carte.
func (g *Game) touchDragScrollsAt(x, y int) bool {
	if g.state != StatePlaying && g.state != StateWorldMap {
		return true
	}
	if g.ShouldBlockWorldInteraction() || g.showDevMenu {
		return true
	}
	if g.ui.ShowChat && !g.ui.ChatMinimized {
		cx, cy, cw, ch, _, _, _, _ := g.chatGeometry()
		fx, fy := float32(x), float32(y)
		if fx >= cx && fx <= cx+cw && fy >= cy && fy <= cy+ch {
			return true
		}
	}
	return false
}

// cursorPosition remplace ebiten.CursorPosition : position de la souris, ou
// du doigt sur téléphone.
func cursorPosition() (int, int) {
	if pointer.usingTouch {
		return pointer.x, pointer.y
	}
	return ebiten.CursorPosition()
}

// inDraw : vrai pendant Game.Draw (voir pointer.clickForDraw).
var inDraw bool

// isMouseJustPressed remplace inpututil.IsMouseButtonJustPressed.
func isMouseJustPressed(b ebiten.MouseButton) bool {
	if inDraw && b == ebiten.MouseButtonLeft {
		return pointer.clickForDraw
	}
	if b == ebiten.MouseButtonLeft && pointer.justPressed {
		return true
	}
	if b == ebiten.MouseButtonLeft && pointer.mouseConsumed {
		return false
	}
	return inpututil.IsMouseButtonJustPressed(b)
}

// isMousePressed remplace ebiten.IsMouseButtonPressed.
func isMousePressed(b ebiten.MouseButton) bool {
	if b == ebiten.MouseButtonLeft && pointer.usingTouch && pointer.pressed {
		return true
	}
	return ebiten.IsMouseButtonPressed(b)
}

// wheelDelta remplace ebiten.Wheel : la molette, ou le glisser du doigt.
func wheelDelta() (float64, float64) {
	x, y := ebiten.Wheel()
	return x, y + pointer.wheelY
}

// isKeyJustPressed remplace inpututil.IsKeyJustPressed et ajoute les touches
// envoyées par les boutons tactiles et le clavier du téléphone.
func isKeyJustPressed(k ebiten.Key) bool {
	if pointer.virtKeys[k] {
		return true
	}
	return inpututil.IsKeyJustPressed(k)
}

// pressVirtualKey simule l'appui d'une touche à la prochaine frame.
func pressVirtualKey(k ebiten.Key) {
	if pointer.nextVirtKeys == nil {
		pointer.nextVirtKeys = map[ebiten.Key]bool{}
	}
	pointer.nextVirtKeys[k] = true
}

// touchCamera applique le glisser et le pincer à la caméra du jeu.
func (g *Game) touchCamera() {
	p := &pointer
	if p.panDX == 0 && p.panDY == 0 && p.zoomFactor == 1 {
		return
	}
	if g.camZoom <= 0 {
		g.camZoom = 1
	}
	g.camX -= p.panDX / g.camZoom
	g.camY -= p.panDY / g.camZoom

	if p.zoomFactor != 1 {
		// Zoom centré sur le milieu des deux doigts.
		sw, sh := float64(g.screenWidth)/2, float64(g.screenHeight)/2
		wx := (p.pinchCenterX-sw)/g.camZoom + g.camX
		wy := (p.pinchCenterY-sh)/g.camZoom + g.camY
		z := g.camZoom * p.zoomFactor
		if z < 0.5 {
			z = 0.5
		}
		if z > 3.0 {
			z = 3.0
		}
		g.camZoom = z
		g.camX = wx - (p.pinchCenterX-sw)/z
		g.camY = wy - (p.pinchCenterY-sh)/z
	}
}
