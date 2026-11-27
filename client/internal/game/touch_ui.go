package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
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
	applyScrollZones()
	// Un clic vu par cette mise à jour reste visible jusqu'au prochain Draw :
	// quand le jeu affiche moins d'images qu'il ne fait de mises à jour, les
	// boutons gérés dans Draw ne le ratent plus.
	// Sauf si la fenêtre des prérequis est ouverte : elle gère ses clics
	// elle-même, et ils ne doivent pas atteindre les boutons en dessous.
	if isMouseJustPressed(ebiten.MouseButtonLeft) && !g.ui.ShowPrereqModal {
		pointer.clickForDraw = true
	}
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
	inDraw = true
	beginScrollZones()
	g.drawGame(screen)
	endScrollZones()
	g.drawTouchButtons(screen)
	inDraw = false
	pointer.clickForDraw = false
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
	vector.DrawFilledRect(screen, 8, 300, float32(maxLen*6+12), float32(len(lines)*16+8), color.RGBA{0, 0, 0, 200}, false)
	for i, l := range lines {
		uiPrint(screen, l, 14, 304+i*16)
	}
}

// ---------------------------------------------------------------------------
// Boutons tactiles

type touchButton struct {
	x, y, w, h float64
	icon       string
	label      string
	fullscreen bool // ce bouton passe en plein écran (géré par le navigateur)
	action     func()
}

// touchSeen : un doigt a déjà touché l'écran.
func touchSeen() bool { return pointer.everTouched }

// touchButtons : les petits boutons ronds en haut à droite (plein écran,
// menu dev, et sur téléphone « fermer la fenêtre », qui remplace Échap).
func (g *Game) touchButtons() []touchButton {
	const size, gap = 48.0, 10.0
	x := float64(g.screenWidth) - hudMargin - size
	y := hudMargin + (topBarH-size)/2

	var btns []touchButton
	add := func(icon, label string, fs bool, action func()) {
		btns = append(btns, touchButton{x: x, y: y, w: size, h: size, icon: icon, label: label, fullscreen: fs, action: action})
		x -= size + gap
	}

	if g.state != StateLogin && touchSeen() {
		if g.showDevMenu {
			add("close", "Fermer", false, func() { g.showDevMenu = false })
		} else if g.IsModalOpen() {
			add("close", "Fermer", false, func() { pressVirtualKey(ebiten.KeyEscape) })
		}
	}
	if fullscreenAvailable() && !isFullscreenNow() {
		add("fullscreen", "Plein écran", true, func() {
			if !IsWebBuild {
				ebiten.SetFullscreen(true)
			}
		})
	}
	if g.state != StateLogin && g.player != nil && g.player.IsAdmin {
		add("code", "Menu dev", false, func() { pressVirtualKey(ebiten.KeyF1) })
	}
	return btns
}

func (b touchButton) contains(x, y float64) bool {
	return x >= b.x && x <= b.x+b.w && y >= b.y && y <= b.y+b.h
}

// consumeTouchButtonTap : un tap (ou un clic) sur un bouton rond déclenche
// son action et n'est pas transmis au reste du jeu.
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
	mx, my := cursorPosition()
	for _, b := range g.touchButtons() {
		hover := b.contains(float64(mx), float64(my))
		drawRoundButton(screen, b.x+b.w/2, b.y+b.h/2, b.w/2, b.icon, hover)
		if hover && !pointer.usingTouch {
			f := bodyFace(12, false)
			tw := textWidth(b.label, f)
			tx := math.Min(b.x+b.w/2-tw/2, float64(g.screenWidth)-tw-hudMargin-8)
			fillRoundRect(screen, tx-8, b.y+b.h+6, tw+16, 22, 8, themePanel)
			drawText(screen, b.label, f, tx, b.y+b.h+9, themeText, alignLeft)
		}
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
