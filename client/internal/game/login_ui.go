package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// login_ui.go contains all login-related UI and logic.
// This file was extracted from main.go during Phase 1.5 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

func (g *Game) UpdateLogin() error {
	w, h := g.screenWidth, g.screenHeight
	mx, my := cursorPosition()
	cmx, cmy := float64(mx), float64(my)

	cx, cy := float64(w)/2, float64(h)/2
	loginY := cy + 80

	// Mise à jour disponible : la fenêtre passe devant l'écran de connexion.
	if g.ui.ShowUpdateModal {
		install, later := g.updateModalButtons()
		g.ui.HoverUpdateBtn = install.has(cmx, cmy)
		if isMouseJustPressed(ebiten.MouseButtonLeft) && !g.ui.UpdateInProgress {
			switch {
			case install.has(cmx, cmy):
				g.ui.UpdateInProgress = true
				g.ui.UpdateError = ""
				g.ui.UpdateProgress = 0
				info := g.ui.UpdateInfo
				go func() {
					err := PerformUpdate(info, func(p float64) { g.ui.UpdateProgress = p })
					if err != nil {
						g.ui.UpdateError = err.Error()
						g.ui.UpdateInProgress = false
					}
				}()
			case later.has(cmx, cmy):
				g.ui.ShowUpdateModal = false
			}
		}
		return nil // Block other inputs
	}

	// Helper for checking if click is inside a rect
	checkClick := func(x, y, w, h float64) bool {
		if isMouseJustPressed(ebiten.MouseButtonLeft) {
			if cmx >= x && cmx <= x+w && cmy >= y && cmy <= y+h {
				return true
			}
		}
		return false
	}

	// Survol du bouton principal (le clic est traité juste en dessous : avant,
	// il était traité ici ET en dessous, ce qui envoyait deux connexions).
	btnX, btnY := cx-100, loginY+100
	g.ui.HoverButton = cmx >= btnX && cmx <= btnX+200 && cmy >= btnY && cmy <= btnY+50

	// Main Action Button (Connect or Register): cx-100, loginY+100 (200x50)
	if checkClick(cx-100, loginY+100, 200, 50) {
		if g.ui.IsRegisterMode {
			g.attemptRegister()
		} else {
			g.attemptLogin()
		}
	}

	// Toggle Mode Button (Switch Login/Register): cx-100, loginY+160 (200x40 - approximate hit area)
	// Drawn at regBtnY = btnY + 60
	if checkClick(cx-100, loginY+160, 200, 40) {
		g.ui.IsRegisterMode = !g.ui.IsRegisterMode
		g.ui.LoginError = "" // Clear errors when switching
	}

	// Login Inputs Focus
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		// User Field: cx-150, loginY-60, 300x50
		if cmx >= cx-150 && cmx <= cx+150 && cmy >= loginY-60 && cmy <= loginY-10 {
			g.ui.IsTypingUser = true
			g.ui.IsTypingPass = false
		} else if cmx >= cx-150 && cmx <= cx+150 && cmy >= loginY+20 && cmy <= loginY+70 {
			// Pass Field: cx-150, loginY+20, 300x50
			g.ui.IsTypingUser = false
			g.ui.IsTypingPass = true
		} else {
			// Clicked outside
			g.ui.IsTypingUser = false
			g.ui.IsTypingPass = false
		}
	}

	// Case « Se souvenir de moi » + son texte. Même position que dans
	// DrawLogin (rememberBox) : avant, la zone de clic était 40 px trop haut
	// et chevauchait « CREATE ACCOUNT ».
	if bx, by, bw, bh := rememberBox(cx, loginY); checkClick(bx, by, bw, bh) {
		g.ui.RememberMe = !g.ui.RememberMe
		if !g.ui.RememberMe {
			g.forgetSession()
		}
	}

	if isKeyJustPressed(ebiten.KeyEnter) || isKeyJustPressed(ebiten.KeyKPEnter) {
		if g.ui.IsRegisterMode {
			g.attemptRegister()
		} else {
			g.attemptLogin()
		}
	}
	if isKeyJustPressed(ebiten.KeyTab) {
		if g.ui.IsTypingUser {
			g.ui.IsTypingUser = false
			g.ui.IsTypingPass = true
		} else {
			g.ui.IsTypingUser = true
			g.ui.IsTypingPass = false
		}
	}

	g.handleTextInput()
	return nil
}

// loginErrorText : message d'erreur court, avec une majuscule.
func loginErrorText(err error) string {
	msg := err.Error()
	// « serveur injoignable (adresse) : détail technique » -> sans le détail.
	if i := strings.Index(msg, ") : "); strings.HasPrefix(msg, "serveur injoignable") && i > 0 {
		msg = msg[:i+1]
	}
	if r := []rune(msg); len(r) > 0 {
		msg = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return msg
}

// rememberBox : case « Se souvenir de moi » et son texte (zone cliquable).
func rememberBox(cx, loginY float64) (x, y, w, h float64) {
	return cx - 150, loginY + 218, 200, 24
}

// canResumeSession : « SET SAIL » peut reprendre la session mémorisée (même
// pseudo, mot de passe pas retapé).
func (g *Game) canResumeSession() bool {
	return g.ui.SavedToken != "" && g.ui.Password == "" &&
		strings.EqualFold(strings.TrimSpace(g.ui.Username), g.ui.SavedUser)
}

func (g *Game) attemptLogin() {
	if g.ui.LoginBusy {
		return
	}
	resume := g.canResumeSession()
	if g.ui.Username == "" || (g.ui.Password == "" && !resume) {
		g.ui.LoginError = "Champs vides"
		return
	}

	g.ui.LoginError = "Connexion..."
	g.ui.LoginBusy = true
	username, password, remember, token := g.ui.Username, g.ui.Password, g.ui.RememberMe, g.ui.SavedToken

	go func() {
		defer func() { g.ui.LoginBusy = false }()
		var resp *client.LoginResponse
		var err error
		if resume {
			resp, err = g.api.ResumeSession(token)
			if errors.Is(err, client.ErrSessionExpired) {
				g.forgetSession()
				g.ui.RememberMe = remember
				g.ui.IsTypingUser = false
				g.ui.IsTypingPass = true
			}
		} else {
			resp, err = g.api.Login(username, password, remember)
		}
		if err != nil {
			g.ui.LoginError = loginErrorText(err)
			g.Log("Login Failed: %v", err)
			return
		}
		if resume {
			// Le pseudo exact (majuscules) vient de la session mémorisée.
			username = g.ui.SavedUser
		}

		if g.player == nil {
			g.player = &domain.Player{}
		}

		g.player.ID = resp.PlayerID
		g.player.Username = username
		g.player.IsAdmin = resp.IsAdmin
		if resp.Role != "" {
			g.player.Role = resp.Role
		}

		// Token is already stored in g.api.Token by the Login method
		g.ui.LoginError = "Success!"
		// Trigger Polling
		g.startPolling()
		g.state = StatePlaying

		if remember && resp.Token != "" {
			g.saveSession(username, resp.Token)
		} else {
			g.forgetSession()
		}
		g.ui.Password = "" // inutile de le garder en mémoire une fois connecté
	}()
}

// saveSession garde le pseudo et le jeton de 30 jours (jamais le mot de passe).
func (g *Game) saveSession(username, token string) {
	g.ui.SavedUser, g.ui.SavedToken = username, token
	data, err := json.Marshal(Credentials{Username: username, Token: token})
	if err != nil {
		return
	}
	if err := writeUserFile("credentials.json", data); err != nil {
		g.Log("[LOGIN] Impossible de mémoriser la session: %v", err)
	}
}

// forgetSession efface la session mémorisée (case décochée, jeton expiré).
func (g *Game) forgetSession() {
	g.ui.SavedUser, g.ui.SavedToken = "", ""
	_ = removeUserFile("credentials.json")
}

func (g *Game) attemptRegister() {
	if g.ui.Username == "" || g.ui.Password == "" {
		g.ui.LoginError = "Champs vides"
		return
	}

	g.ui.LoginError = "Creation..."

	go func() {
		err := g.api.Register(g.ui.Username, g.ui.Password)
		if err != nil {
			g.ui.LoginError = loginErrorText(err)
			g.Log("Register Failed: %v", err)
			return
		}

		// Auto Login after Register
		g.attemptLogin()
	}()
}

func (g *Game) startPolling() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if g.player != nil && g.state == StatePlaying {
				p, err := g.api.GetStatus()
				if err == nil {
					g.updateChan <- p
					// Also refresh captains periodically
					captains, err := g.api.GetCaptains()
					if err == nil {
						g.captains = captains
					}
				}
			}
		}
	}()
}

func (g *Game) handleTextInput() {
	chars := ebiten.InputChars()
	if len(chars) > 0 {
		if g.ui.IsTypingUser {
			g.ui.Username += string(chars)
		} else if g.ui.IsTypingPass {
			g.ui.Password += string(chars)
		}
	}

	if isKeyJustPressed(ebiten.KeyBackspace) {
		// On retire une lettre entière (é, ç... prennent plusieurs octets).
		if g.ui.IsTypingUser {
			g.ui.Username = dropLastRune(g.ui.Username)
		} else if g.ui.IsTypingPass {
			g.ui.Password = dropLastRune(g.ui.Password)
		}
	}
}

func dropLastRune(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

func (g *Game) DrawLogin(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)

	if g.loginBG != nil {
		bounds := g.loginBG.Bounds()
		iw, ih := bounds.Dx(), bounds.Dy()
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterLinear
		scaleX := w / float64(iw)
		scaleY := h / float64(ih)
		scale := scaleX
		if scaleY > scale {
			scale = scaleY
		}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(w/2-float64(iw)*scale/2, h/2-float64(ih)*scale/2)
		screen.DrawImage(g.loginBG, op)
	} else {
		screen.Fill(color.Black)
	}

	cx, cy := w/2, h/2

	if g.gameLogo != nil {
		logoW := g.gameLogo.Bounds().Dx()
		targetW := 300.0
		scale := targetW / float64(logoW)
		scaledW := float64(logoW) * scale
		logoOp := &ebiten.DrawImageOptions{}
		logoOp.Filter = ebiten.FilterLinear
		logoOp.GeoM.Scale(scale, scale)
		logoOp.GeoM.Translate(cx-scaledW/2, 40)
		screen.DrawImage(g.gameLogo, logoOp)
	}

	loginY := cy + 80

	// Captain Name Input
	uiPrint(screen, "Captain Name:", int(cx)-150, int(loginY)-80)
	if g.sliceTopLeft != nil {
		draw9Slice(screen, g, cx-150, loginY-60, 300, 50, 16)
	} else {
		// Fallback dark box if assets somehow still fail
		uiFillRect(screen, float32(cx-150), float32(loginY-60), 300, 50, color.RGBA{20, 30, 40, 200}, vecAA)
	}

	registerTextField(textField{id: "login-user", x: cx - 150, y: loginY - 60, w: 300, h: 50,
		value: &g.ui.Username, focused: g.ui.IsTypingUser, label: "Nom du capitaine", next: "login-pass"})
	registerTextField(textField{id: "login-pass", x: cx - 150, y: loginY + 20, w: 300, h: 50,
		value: &g.ui.Password, focused: g.ui.IsTypingPass, label: "Mot de passe", password: true})

	utxt := g.ui.Username
	if g.ui.IsTypingUser && (g.ui.CaretTimer/30)%2 == 0 {
		utxt += "|"
	}
	uiPrint(screen, utxt, int(cx)-140, int(loginY)-45)

	// Password Input
	uiPrint(screen, "Password:", int(cx)-150, int(loginY))
	if g.sliceTopLeft != nil {
		draw9Slice(screen, g, cx-150, loginY+20, 300, 50, 16)
	} else {
		uiFillRect(screen, float32(cx-150), float32(loginY+20), 300, 50, color.RGBA{20, 30, 40, 200}, vecAA)
	}

	g.drawPasswordDots(screen, cx-140, loginY+20, 280, 50)

	// Main Action Button (Connect or Register)
	// Gold Style - SET SAIL / REGISTER
	btnX, btnY := float32(cx-100), float32(loginY+100)
	btnW, btnH := float32(200), float32(50)

	// Determine label and action based on mode
	mainBtnLabel := "SET SAIL"
	if g.ui.IsRegisterMode {
		mainBtnLabel = "SIGN UP"
	}

	// Calculate Button Color
	btnCol := color.RGBA{20, 20, 10, 255}
	if g.ui.IsRegisterMode {
		btnCol = color.RGBA{20, 10, 10, 255} // Slight reddish tint for register
	}

	// Fill
	uiFillRect(screen, btnX, btnY, btnW, btnH, btnCol, vecAA)
	// Inner Gradient-ish highlight?
	uiFillRect(screen, btnX+2, btnY+2, btnW-4, btnH/2, color.RGBA{50, 45, 10, 255}, vecAA)

	// Simple Hover Effect
	btnBorderCol := color.RGBA{218, 165, 32, 255} // Gold
	if g.ui.HoverButton {
		btnBorderCol = color.RGBA{255, 215, 0, 255} // Brighter Gold
		uiFillRect(screen, btnX, btnY, btnW, btnH, color.RGBA{40, 40, 20, 255}, vecAA)
	}

	// Double Border for "Fancy" look
	uiStrokeRect(screen, btnX, btnY, btnW, btnH, 2, btnBorderCol, vecAA)
	uiStrokeRect(screen, btnX+4, btnY+4, btnW-8, btnH-8, 1, btnBorderCol, vecAA)

	// Text
	txtX := int(btnX) + 60
	if g.ui.IsRegisterMode {
		txtX = int(btnX) + 65
	}
	uiPrint(screen, mainBtnLabel, txtX, int(btnY)+18)

	// Toggle Mode Button (Switch between Login/Register)
	regBtnY := btnY + 60

	toggleLabel := "CREATE ACCOUNT"
	if g.ui.IsRegisterMode {
		toggleLabel = "BACK TO LOGIN"
	}

	// Draw minimal text button/link style
	// uiFillRect(screen, btnX, regBtnY, btnW, 30, color.RGBA{10, 30, 40, 255}, vecAA)
	// uiStrokeRect(screen, btnX, regBtnY, btnW, 30, 1, color.RGBA{100, 150, 200, 255}, vecAA)

	// Just text with underline effect if hovered (simulated)
	// We'll just draw centered text for now
	uiPrint(screen, toggleLabel, int(btnX)+45, int(regBtnY))

	checkboxX, checkboxY, _, _ := rememberBox(cx, loginY)
	checkboxY += 2
	ebitenutil.DrawRect(screen, checkboxX, checkboxY, 20, 20, color.RGBA{200, 200, 200, 255})
	if g.ui.RememberMe {
		uiFillRect(screen, float32(checkboxX+4), float32(checkboxY+4), 12, 12, color.RGBA{0, 200, 0, 255}, vecAA)
	}
	uiPrint(screen, "Se souvenir de moi", int(checkboxX)+30, int(checkboxY)+2)

	versionLabel := "v0.9.0-alpha"
	if ClientVersion != "dev" && ClientVersion != "" {
		versionLabel += " (" + ClientVersion + ")"
	}
	uiPrint(screen, versionLabel, 10, int(h)-40)
	uiPrint(screen, "Developed by Masterkey Technologic", 10, int(h)-20)

	if g.ui.LoginError != "" {
		f := bodyFace(14, true)
		tw := textWidth(g.ui.LoginError, f)
		fillRoundRect(screen, cx-tw/2-12, loginY+188, tw+24, 26, 13, color.RGBA{20, 13, 7, 210})
		drawText(screen, g.ui.LoginError, f, cx, loginY+192, themeOrange, alignCenter)
	}

	if g.ui.ShowUpdateModal {
		g.drawUpdateModal(screen)
	}
}

// updateModalButtons : boutons « Installer » et « Plus tard ».
func (g *Game) updateModalButtons() (install, later hudRect) {
	cx, cy := float64(g.screenWidth)/2, float64(g.screenHeight)/2
	install = hudRect{cx - 110, cy + 30, 220, 52}
	later = hudRect{cx - 80, cy + 92, 160, 32}
	return
}

func (g *Game) drawUpdateModal(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	uiFillRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 180}, vecAA)
	cx, cy := w/2, h/2
	bw, bh := 480.0, 270.0
	drawThemePanel(screen, cx-bw/2, cy-bh/2, bw, bh, 18)
	drawText(screen, "Mise à jour disponible", titleFace(34), cx, cy-bh/2+22, themeGold, alignCenter)
	drawText(screen, "Une nouvelle version de SeaDogs est prête.", bodyFace(15, false), cx, cy-50, themeText, alignCenter)
	if g.ui.UpdateError != "" {
		drawText(screen, ellipsize(g.ui.UpdateError, bodyFace(13, false), bw-40), bodyFace(13, false), cx, cy-20, themeOrange, alignCenter)
	}

	install, later := g.updateModalButtons()
	if g.ui.UpdateInProgress {
		bar := hudRect{cx - 170, install.y + 14, 340, 14}
		fillRoundRect(screen, bar.x, bar.y, bar.w, bar.h, 7, color.NRGBA{255, 255, 255, 30})
		if p := g.ui.UpdateProgress; p > 0 {
			fillRoundRect(screen, bar.x, bar.y, math.Max(14, bar.w*math.Min(p, 1)), bar.h, 7, themeGold)
		}
		label := "Téléchargement…"
		if p := g.ui.UpdateProgress; p > 0 {
			label = fmt.Sprintf("Téléchargement… %d %%", int(math.Min(p, 1)*100))
			if p >= 1 {
				label = "Installation, le jeu va redémarrer…"
			}
		}
		drawText(screen, label, bodyFace(14, false), cx, bar.y+26, themeText, alignCenter)
		return
	}

	top := themeGold
	if g.ui.HoverUpdateBtn {
		top = color.RGBA{243, 207, 122, 255}
	}
	fillRoundRect(screen, install.x, install.y, install.w, install.h, 14, themeGoldDark)
	fillRoundRect(screen, install.x, install.y, install.w, install.h-6, 14, top)
	label := "Installer"
	if g.ui.UpdateError != "" {
		label = "Réessayer"
	}
	drawText(screen, label, titleFace(26), cx, install.y+9, themeInk, alignCenter)
	drawText(screen, "Plus tard", bodyFace(14, false), cx, later.y+7, themeMuted, alignCenter)
}

// Mot de passe masqué : des points dessinés un par un, tous identiques.
// (Avant, une ligne d'astérisques : la police en décalait certains en hauteur
// selon leurs voisins, et au-delà de ~40 caractères ils sortaient du cadre.)
const (
	passDotGap    = 10.0 // distance entre deux points
	passDotRadius = 3.2
)

// drawPasswordDots dessine le contenu du champ mot de passe (x, w : zone de
// texte à l'intérieur du cadre ; y, h : le cadre).
func (g *Game) drawPasswordDots(screen *ebiten.Image, x, y, w, h float64) {
	midY := y + h/2
	caretOn := g.ui.IsTypingPass && (g.ui.CaretTimer/30)%2 == 0
	n := utf8.RuneCountInString(g.ui.Password)

	if n == 0 && g.canResumeSession() {
		// Session mémorisée : pas besoin de retaper le mot de passe.
		f := bodyFace(uiTextSize, false)
		drawText(screen, "Mot de passe mémorisé", f, x, midY-8, color.RGBA{200, 185, 150, 170}, alignLeft)
		if caretOn {
			uiFillRect(screen, float32(x+textWidth("Mot de passe mémorisé", f)+4), float32(midY-8), 1.5, 16, themeText, vecAA)
		}
		return
	}

	// Mot de passe long : on montre les derniers points, comme un vrai champ
	// qui défile.
	maxDots := int((w - 12) / passDotGap)
	shown := n
	if shown > maxDots {
		shown = maxDots
	}
	for i := 0; i < shown; i++ {
		dx := x + passDotRadius + float64(i)*passDotGap
		vector.DrawFilledCircle(screen, float32(dx), float32(midY), passDotRadius, themeText, vecAA)
	}
	if caretOn {
		caretX := x + float64(shown)*passDotGap
		if shown == 0 {
			caretX = x
		}
		uiFillRect(screen, float32(caretX), float32(midY-8), 1.5, 16, themeText, vecAA)
	}
}
