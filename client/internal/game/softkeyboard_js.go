//go:build js

package game

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

// softkeyboard_js.go : clavier du téléphone et plein écran dans le navigateur.
//
// Un jeu Ebiten dessine tout dans un <canvas> : le téléphone n'a aucune
// raison d'ouvrir son clavier. Quand le joueur touche un champ de texte du
// jeu, on affiche donc un vrai champ HTML en haut de l'écran (#kbd dans
// index.html) et on lui donne le focus, ce qui ouvre le clavier. Ce qu'il tape
// est recopié dans le champ du jeu.
//
// Les navigateurs n'ouvrent le clavier (et le plein écran) que pendant un
// geste du joueur : c'est pour ça qu'on écoute directement « touchend » ici
// au lieu d'attendre la frame suivante du jeu.

var kbd struct {
	initDone bool
	g        *Game

	box, label, input js.Value

	active    bool
	id        string
	lastJS    string
	frames    int
	closeNext bool // fermer après la prochaine synchro (Entrée)
	funcs     []js.Func
}

func initSoftKeyboard(g *Game) {
	kbd.initDone = true
	kbd.g = g
	doc := js.Global().Get("document")
	kbd.box = doc.Call("getElementById", "kbd")
	kbd.label = doc.Call("getElementById", "kbd-label")
	kbd.input = doc.Call("getElementById", "kbd-input")

	listen := func(target js.Value, event string, capture bool, fn func(e js.Value)) {
		f := js.FuncOf(func(this js.Value, args []js.Value) any {
			fn(args[0])
			return nil
		})
		kbd.funcs = append(kbd.funcs, f)
		opts := js.Global().Get("Object").New()
		opts.Set("capture", capture)
		opts.Set("passive", true)
		target.Call("addEventListener", event, f, opts)
	}

	// Doigt posé : on note la frame, pour repérer les taps trop rapides.
	var touchStartFrame uint64
	listen(doc, "touchstart", true, func(e js.Value) {
		touchStartFrame = pointer.frame
	})

	// Doigt levé sur le jeu : plein écran ? champ de texte ?
	listen(doc, "touchend", true, func(e js.Value) {
		t := e.Get("target")
		if !t.Truthy() || t.Get("tagName").String() != "CANVAS" {
			return
		}
		touches := e.Get("changedTouches")
		if touches.Length() == 0 {
			return
		}
		x, y := cssToGame(t, touches.Index(0))
		if pointer.frame == touchStartFrame && e.Get("touches").Length() == 0 {
			// Posé et levé avant que le jeu ait pu le voir : on le rejoue.
			pointer.missedTap = true
			pointer.missedTapX, pointer.missedTapY = int(x), int(y)
		}
		if b, ok := g.touchButtonAt(x, y); ok {
			if b.fullscreen {
				enterFullscreen()
			}
			return
		}
		if f, ok := textFieldAt(x, y); ok && kbd.input.Truthy() {
			openKeyboard(f)
		}
	})

	// Clic souris sur le bouton plein écran (le navigateur exige que la
	// demande parte d'un vrai clic).
	listen(doc, "mouseup", true, func(e js.Value) {
		t := e.Get("target")
		if pointer.usingTouch || !t.Truthy() || t.Get("tagName").String() != "CANVAS" {
			return
		}
		x, y := cssToGame(t, e)
		if b, ok := g.touchButtonAt(x, y); ok && b.fullscreen {
			enterFullscreen()
		}
	})

	// Doigt posé ailleurs que sur le champ : on ferme le clavier.
	listen(doc, "touchstart", true, func(e js.Value) {
		if !kbd.active {
			return
		}
		t := e.Get("target")
		if t.Truthy() && kbd.box.Truthy() && kbd.box.Call("contains", t).Bool() {
			return
		}
		closeKeyboard()
	})

	if !kbd.input.Truthy() {
		return
	}

	// Entrée sur le clavier du téléphone.
	f := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		if e.Get("key").String() != "Enter" {
			return nil
		}
		e.Call("preventDefault")
		if !kbd.active {
			return nil
		}
		cur, ok := textFieldByID(kbd.id)
		if ok && cur.next != "" {
			// Pseudo -> mot de passe : on enchaîne sans fermer le clavier.
			if cur.value != nil {
				*cur.value = kbd.input.Get("value").String()
			}
			if next, ok := textFieldByID(cur.next); ok {
				pressVirtualKey(ebiten.KeyTab)
				openKeyboard(next)
				return nil
			}
		}
		pressVirtualKey(ebiten.KeyEnter)
		kbd.closeNext = true
		return nil
	})
	kbd.funcs = append(kbd.funcs, f)
	kbd.input.Call("addEventListener", "keydown", f)

	// Le joueur a fermé son clavier.
	listen(kbd.input, "blur", false, func(e js.Value) {
		if kbd.active && !kbd.closeNext {
			syncValue()
			hideKeyboard()
		}
	})

	// Bouton OK de la barre : on garde le texte et on ferme.
	if ok := doc.Call("getElementById", "kbd-ok"); ok.Truthy() {
		listen(ok, "click", false, func(e js.Value) { closeKeyboard() })
	}
}

// cssToGame convertit la position d'un doigt (pixels de la page) en pixels du jeu.
func cssToGame(canvas, touch js.Value) (float64, float64) {
	r := canvas.Call("getBoundingClientRect")
	cx := touch.Get("clientX").Float() - r.Get("left").Float()
	cy := touch.Get("clientY").Float() - r.Get("top").Float()
	rw, rh := r.Get("width").Float(), r.Get("height").Float()
	if rw <= 0 || rh <= 0 || kbd.g == nil {
		return cx, cy
	}
	return cx * float64(kbd.g.screenWidth) / rw, cy * float64(kbd.g.screenHeight) / rh
}

func openKeyboard(f textField) {
	kbd.active = true
	kbd.id = f.id
	kbd.frames = 0
	kbd.closeNext = false
	v := ""
	if f.value != nil {
		v = *f.value
	}
	kbd.lastJS = v
	kbd.label.Set("textContent", f.label)
	typ := "text"
	if f.password {
		typ = "password"
	}
	kbd.input.Set("type", typ)
	mode := "text"
	if f.numeric {
		mode = "numeric"
	}
	kbd.input.Set("inputMode", mode)
	enter := "done"
	if f.next != "" {
		enter = "next"
	}
	kbd.input.Call("setAttribute", "enterkeyhint", enter)
	kbd.input.Set("value", v)
	kbd.box.Get("classList").Call("add", "open")
	kbd.input.Call("focus")
}

func hideKeyboard() {
	kbd.active = false
	kbd.closeNext = false
	if kbd.box.Truthy() {
		kbd.box.Get("classList").Call("remove", "open")
	}
}

func closeKeyboard() {
	if !kbd.active {
		return
	}
	syncValue()
	hideKeyboard()
	kbd.input.Call("blur")
}

// syncValue recopie le champ HTML vers le jeu, ou l'inverse si c'est le jeu
// qui a changé la valeur (ex. message de chat envoyé -> champ vidé).
func syncValue() {
	f, ok := textFieldByID(kbd.id)
	if !ok || f.value == nil {
		return
	}
	js := kbd.input.Get("value").String()
	if js != kbd.lastJS {
		*f.value = js
		kbd.lastJS = js
		if f.onChange != nil {
			f.onChange(js)
		}
	} else if *f.value != kbd.lastJS {
		kbd.input.Set("value", *f.value)
		kbd.lastJS = *f.value
	}
}

// syncSoftKeyboard est appelé au début de chaque Update.
func syncSoftKeyboard(g *Game) {
	if !kbd.initDone {
		initSoftKeyboard(g)
	}
	if !kbd.active {
		return
	}
	syncValue()
	kbd.frames++
	if kbd.closeNext {
		hideKeyboard()
		kbd.input.Call("blur")
		return
	}
	// Le jeu a quitté ce champ (écran changé, autre champ...) : on ferme.
	// On laisse quelques frames au jeu pour traiter le tap qui l'a ouvert.
	if kbd.frames > 20 {
		if f, ok := textFieldByID(kbd.id); !ok || !f.focused {
			closeKeyboard()
		}
	}
}

func fullscreenAvailable() bool {
	doc := js.Global().Get("document")
	return doc.Get("fullscreenEnabled").Truthy() || doc.Get("webkitFullscreenEnabled").Truthy()
}

func isFullscreenNow() bool {
	doc := js.Global().Get("document")
	return doc.Get("fullscreenElement").Truthy() || doc.Get("webkitFullscreenElement").Truthy()
}

func enterFullscreen() {
	el := js.Global().Get("document").Get("documentElement")
	var p js.Value
	if el.Get("requestFullscreen").Truthy() {
		p = el.Call("requestFullscreen")
	} else if el.Get("webkitRequestFullscreen").Truthy() {
		el.Call("webkitRequestFullscreen")
		return
	} else {
		return
	}
	// En plein écran, Android accepte de bloquer l'écran en paysage.
	if p.Truthy() && p.Get("then").Truthy() {
		var then js.Func
		then = js.FuncOf(func(this js.Value, args []js.Value) any {
			if o := js.Global().Get("screen").Get("orientation"); o.Truthy() && o.Get("lock").Truthy() {
				lp := o.Call("lock", "landscape")
				if lp.Truthy() && lp.Get("catch").Truthy() {
					lp.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any { return nil }))
				}
			}
			then.Release()
			return nil
		})
		p.Call("then", then).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any { return nil }))
	}
}

// urlFlag : vrai si l'adresse de la page contient ce paramètre
// (ex. /play/?fps&mute). Sert aux réglages de diagnostic.
func urlFlag(name string) bool {
	q := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search"))
	return q.Call("has", name).Bool()
}

var fpsFlag = -1

// showFPS : compteur de diagnostic quand l'adresse contient « ?fps ».
func showFPS() bool {
	if fpsFlag < 0 {
		fpsFlag = 0
		if urlFlag("fps") {
			fpsFlag = 1
		}
	}
	return fpsFlag == 1
}

var muteFlag = -1

// audioMuted : « &mute » dans l'adresse coupe toute la musique (test).
func audioMuted() bool {
	if muteFlag < 0 {
		muteFlag = 0
		if urlFlag("mute") {
			muteFlag = 1
		}
	}
	return muteFlag == 1
}

// browserDiag : infos de la page pour le compteur de diagnostic
// (version du jeu, taille réelle du canvas, puce graphique).
func browserDiag() string {
	w := js.Global()
	d := w.Get("seadogsDiag")
	if !d.Truthy() {
		return ""
	}
	parts := []string{}
	if v := d.Get("version"); v.Truthy() {
		parts = append(parts, "v"+v.String())
	}
	if c := w.Get("document").Call("querySelector", "canvas"); c.Truthy() {
		parts = append(parts, fmt.Sprintf("canvas %dx%d dpr %.2f/%.2f",
			c.Get("width").Int(), c.Get("height").Int(),
			w.Get("devicePixelRatio").Float(), d.Get("realDPR").Float()))
	}
	if g := d.Get("gpu"); g.Truthy() {
		parts = append(parts, g.String())
	}
	return strings.Join(parts, " | ")
}
