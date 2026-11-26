package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// touch_ui.go : ce qui rend le jeu jouable au doigt.
//   - mise à l'échelle pour les petits écrans (scaledLayout) ;
//   - petits boutons tactiles qui remplacent les touches du clavier
//     (Échap, F1, F11) ;
//   - liste des champs de texte affichés, pour ouvrir le clavier du téléphone
//     quand on touche l'un d'eux (voir softkeyboard_js.go).

const (
	designWidth  = 1280.0
	designHeight = 720.0
)

// layoutScale : taille d'un pixel du jeu en pixels de la page (1 sur PC,
// environ 0.5 sur un téléphone en paysage).
var layoutScale = 1.0

func scaledLayout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return outsideWidth, outsideHeight
	}
	s := math.Min(float64(outsideWidth)/designWidth, float64(outsideHeight)/designHeight)
	if s >= 1 {
		layoutScale = 1
		return outsideWidth, outsideHeight
	}
	layoutScale = s
	return int(math.Ceil(float64(outsideWidth) / s)), int(math.Ceil(float64(outsideHeight) / s))
}

// Update : d'abord le tactile et le clavier du téléphone, puis le jeu.
func (g *Game) Update() error {
	t0 := time.Now()
	g.updatePointer()
	syncSoftKeyboard(g)
	err := g.updateGame()
	diag.update += time.Since(t0)
	diag.updates++
	return err
}

// diag : temps passé dans le code du jeu, pour le compteur ?fps.
var diag struct {
	update, draw   time.Duration
	updates, draws int
	since          time.Time
	updMs, drawMs  float64
}

// Draw : le jeu, puis les boutons tactiles par-dessus.
func (g *Game) Draw(screen *ebiten.Image) {
	t0 := time.Now()
	beginTextFieldFrame()
	g.drawGame(screen)
	g.drawTouchButtons(screen)
	diag.draw += time.Since(t0)
	diag.draws++
	if showFPS() {
		g.drawDiag(screen)
	}
}

func (g *Game) drawDiag(screen *ebiten.Image) {
	if time.Since(diag.since) >= time.Second {
		if diag.updates > 0 {
			diag.updMs = float64(diag.update.Microseconds()) / 1000 / float64(diag.updates)
		}
		if diag.draws > 0 {
			diag.drawMs = float64(diag.draw.Microseconds()) / 1000 / float64(diag.draws)
		}
		diag.update, diag.draw, diag.updates, diag.draws = 0, 0, 0, 0
		diag.since = time.Now()
	}
	son := "son ON"
	if audioMuted() {
		son = "son OFF"
	}
	lines := []string{
		fmt.Sprintf("FPS %.0f  TPS %.0f  jeu %dx%d  %s", ebiten.ActualFPS(), ebiten.ActualTPS(), g.screenWidth, g.screenHeight, son),
		fmt.Sprintf("calcul: update %.1f ms  draw %.1f ms (par image)", diag.updMs, diag.drawMs),
	}
	if b := browserDiag(); b != "" {
		lines = append(lines, b)
	}
	maxLen := 0
	for _, l := range lines {
		if len(l) > maxLen {
			maxLen = len(l)
		}
	}
	vector.DrawFilledRect(screen, 8, 44, float32(maxLen*6+12), float32(len(lines)*16+8), color.RGBA{0, 0, 0, 200}, false)
	for i, l := range lines {
		ebitenutil.DebugPrintAt(screen, l, 14, 48+i*16)
	}
}

// ---------------------------------------------------------------------------
// Boutons tactiles

type touchButton struct {
	x, y, w, h float64
	label      string
	fullscreen bool // ce bouton passe en plein écran (géré par le navigateur)
	action     func()
}

// touchSeen : un doigt a déjà touché l'écran (on n'affiche les boutons
// tactiles qu'à partir de là, donc jamais sur PC).
func touchSeen() bool { return pointer.everTouched }

func (g *Game) touchButtons() []touchButton {
	if !touchSeen() {
		return nil
	}
	const bw, bh, gap = 110.0, 60.0, 12.0
	x := float64(g.screenWidth) - bw - 14
	y := float64(g.screenHeight) * 0.30

	var btns []touchButton
	add := func(label string, fs bool, action func()) {
		btns = append(btns, touchButton{x: x, y: y, w: bw, h: bh, label: label, fullscreen: fs, action: action})
		y += bh + gap
	}

	if g.state != StateLogin {
		if g.showDevMenu {
			add("FERMER", false, func() { g.showDevMenu = false })
		} else if g.IsModalOpen() {
			add("FERMER", false, func() { pressVirtualKey(ebiten.KeyEscape) })
		}
	}
	if fullscreenAvailable() && !isFullscreenNow() {
		add("PLEIN ECRAN", true, func() {
			if !IsWebBuild {
				ebiten.SetFullscreen(true)
			}
		})
	}
	if g.state != StateLogin && g.player != nil && g.player.IsAdmin {
		add("DEV", false, func() { pressVirtualKey(ebiten.KeyF1) })
	}
	return btns
}

func (b touchButton) contains(x, y float64) bool {
	return x >= b.x && x <= b.x+b.w && y >= b.y && y <= b.y+b.h
}

// consumeTouchButtonTap : un tap sur un bouton tactile déclenche son action
// et n'est pas transmis au reste du jeu.
func (g *Game) consumeTouchButtonTap(x, y int) bool {
	for _, b := range g.touchButtons() {
		if b.contains(float64(x), float64(y)) {
			b.action()
			return true
		}
	}
	return false
}

// touchButtonAt sert au navigateur (plein écran) : quel bouton est sous le doigt ?
func (g *Game) touchButtonAt(x, y float64) (touchButton, bool) {
	for _, b := range g.touchButtons() {
		if b.contains(x, y) {
			return b, true
		}
	}
	return touchButton{}, false
}

func (g *Game) drawTouchButtons(screen *ebiten.Image) {
	for _, b := range g.touchButtons() {
		x, y, w, h := float32(b.x), float32(b.y), float32(b.w), float32(b.h)
		vector.DrawFilledRect(screen, x, y, w, h, color.RGBA{60, 38, 18, 215}, vecAA)
		vector.StrokeRect(screen, x, y, w, h, 3, color.RGBA{218, 165, 32, 255}, vecAA)
		tx := int(b.x+b.w/2) - len(b.label)*3
		ebitenutil.DebugPrintAt(screen, b.label, tx, int(b.y+b.h/2)-8)
	}
}

// ---------------------------------------------------------------------------
// Champs de texte

type textField struct {
	id       string
	x, y     float64
	w, h     float64
	value    *string
	focused  bool
	label    string
	password bool
	numeric  bool
	next     string         // champ suivant quand on appuie sur Entrée (ex. pseudo -> mot de passe)
	onChange func(v string) // appelé quand le clavier du téléphone modifie la valeur
}

var (
	textFields     []textField // champs dessinés pendant la frame en cours
	textFieldsDone []textField // champs de la dernière frame complète
)

func beginTextFieldFrame() {
	textFieldsDone = append(textFieldsDone[:0], textFields...)
	textFields = textFields[:0]
}

// registerTextField : à appeler en dessinant un champ de texte.
func registerTextField(f textField) {
	textFields = append(textFields, f)
}

func textFieldAt(x, y float64) (textField, bool) {
	for i := len(textFieldsDone) - 1; i >= 0; i-- {
		f := textFieldsDone[i]
		if x >= f.x && x <= f.x+f.w && y >= f.y && y <= f.y+f.h {
			return f, true
		}
	}
	return textField{}, false
}

func textFieldByID(id string) (textField, bool) {
	for _, f := range textFieldsDone {
		if f.id == id {
			return f, true
		}
	}
	return textField{}, false
}

// vecAA : anticrénelage des formes (rectangles, cadres, cercles).
// Dans le navigateur, Ebiten le fait en redessinant chaque forme dans une
// image 2x plus grande puis en la réduisant : avec ~300 formes par écran,
// ça écroulait les téléphones (7-9 images/s). Sur PC on le garde.
var vecAA = !IsWebBuild
