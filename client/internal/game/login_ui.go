package game

import (
	"encoding/json"
	"image/color"
	"os"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// login_ui.go contains all login-related UI and logic.
// This file was extracted from main.go during Phase 1.5 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

func (g *Game) UpdateLogin() error {
	w, h := g.screenWidth, g.screenHeight
	mx, my := ebiten.CursorPosition()
	cmx, cmy := float64(mx), float64(my)

	cx, cy := float64(w)/2, float64(h)/2
	loginY := cy + 80

	// Auto-Updater Hijack
	if g.ui.ShowUpdateModal {
		cx, cy := float64(w)/2, float64(h)/2
		// Update Button: cx-100, cy+20, 200, 50
		btnX, btnY, btnW, btnH := cx-100, cy+20, 200.0, 50.0

		// Hover Check
		if cmx >= btnX && cmx <= btnX+btnW && cmy >= btnY && cmy <= btnY+btnH {
			g.ui.HoverUpdateBtn = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.ui.UpdateInProgress {
				g.ui.UpdateInProgress = true
				// Run update in goroutine to allow UI to render "Updating..."
				go func() {
					err := PerformUpdate(g.ui.UpdateURL)
					if err != nil {
						g.ui.LoginError = "Update Failed: " + err.Error()
						g.ui.ShowUpdateModal = false
						g.ui.UpdateInProgress = false
					}
				}()
			}
		} else {
			g.ui.HoverUpdateBtn = false
		}
		return nil // Block other inputs
	}

	// Helper for checking if click is inside a rect
	checkClick := func(x, y, w, h float64) bool {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if cmx >= x && cmx <= x+w && cmy >= y && cmy <= y+h {
				return true
			}
		}
		return false
	}

	// Check Hover on Connect Button
	btnX, btnY := cx-100, loginY+100
	if cmx >= btnX && cmx <= btnX+200 && cmy >= btnY && cmy <= btnY+50 {
		g.ui.HoverButton = true
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.attemptLogin()
		}
	} else {
		g.ui.HoverButton = false
	}

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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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

	// Checkbox "Remember Me": cx-150, loginY+180 (20x20)
	if checkClick(cx-150, loginY+180, 20, 20) {
		g.ui.RememberMe = !g.ui.RememberMe
	}
	// Also allow clicking the text
	if checkClick(cx-120, loginY+180, 150, 20) {
		g.ui.RememberMe = !g.ui.RememberMe
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		g.attemptLogin()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
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

func (g *Game) attemptLogin() {
	if g.ui.Username == "" || g.ui.Password == "" {
		g.ui.LoginError = "Champs vides"
		return
	}

	g.ui.LoginError = "Connexion..."

	go func() {
		resp, err := g.api.Login(g.ui.Username, g.ui.Password)
		if err != nil {
			g.ui.LoginError = "Erreur de connexion"
			g.Log("Login Failed: %v", err)
			return
		}

		if g.player == nil {
			g.player = &domain.Player{}
		}

		g.player.ID = resp.PlayerID
		g.player.Username = g.ui.Username
		g.player.IsAdmin = resp.IsAdmin
		if resp.Role != "" {
			g.player.Role = resp.Role
		}

		// Token is already stored in g.api.Token by the Login method
		g.ui.LoginError = "Success!"
		// Trigger Polling
		g.startPolling()
		g.state = StatePlaying

		if g.ui.RememberMe {
			saveCredentials(g.ui.Username, g.ui.Password)
		}
	}()
}

// saveCredentials saves username and password to a local file.
func saveCredentials(u, p string) {
	creds := Credentials{Username: u, Password: p}
	file, err := os.Create("credentials.json")
	if err != nil {
		return
	}
	defer file.Close()
	json.NewEncoder(file).Encode(creds)
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
			g.ui.LoginError = "Erreur Creation"
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

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if g.ui.IsTypingUser && len(g.ui.Username) > 0 {
			g.ui.Username = g.ui.Username[:len(g.ui.Username)-1]
		} else if g.ui.IsTypingPass && len(g.ui.Password) > 0 {
			g.ui.Password = g.ui.Password[:len(g.ui.Password)-1]
		}
	}
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
	ebitenutil.DebugPrintAt(screen, "Captain Name:", int(cx)-150, int(loginY)-80)
	if g.sliceTopLeft != nil {
		draw9Slice(screen, g, cx-150, loginY-60, 300, 50, 16)
	} else {
		// Fallback dark box if assets somehow still fail
		vector.DrawFilledRect(screen, float32(cx-150), float32(loginY-60), 300, 50, color.RGBA{20, 30, 40, 200}, true)
	}

	utxt := g.ui.Username
	if g.ui.IsTypingUser && (g.ui.CaretTimer/30)%2 == 0 {
		utxt += "|"
	}
	ebitenutil.DebugPrintAt(screen, utxt, int(cx)-140, int(loginY)-45)

	// Password Input
	ebitenutil.DebugPrintAt(screen, "Password:", int(cx)-150, int(loginY))
	if g.sliceTopLeft != nil {
		draw9Slice(screen, g, cx-150, loginY+20, 300, 50, 16)
	} else {
		vector.DrawFilledRect(screen, float32(cx-150), float32(loginY+20), 300, 50, color.RGBA{20, 30, 40, 200}, true)
	}

	ptxt := ""
	for range g.ui.Password {
		ptxt += "*"
	}
	if g.ui.IsTypingPass && (g.ui.CaretTimer/30)%2 == 0 {
		ptxt += "|"
	}
	ebitenutil.DebugPrintAt(screen, ptxt, int(cx)-140, int(loginY)+35)

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
	vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, btnCol, true)
	// Inner Gradient-ish highlight?
	vector.DrawFilledRect(screen, btnX+2, btnY+2, btnW-4, btnH/2, color.RGBA{50, 45, 10, 255}, true)

	// Simple Hover Effect
	btnBorderCol := color.RGBA{218, 165, 32, 255} // Gold
	if g.ui.HoverButton {
		btnBorderCol = color.RGBA{255, 215, 0, 255} // Brighter Gold
		vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{40, 40, 20, 255}, true)
	}

	// Double Border for "Fancy" look
	vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 2, btnBorderCol, true)
	vector.StrokeRect(screen, btnX+4, btnY+4, btnW-8, btnH-8, 1, btnBorderCol, true)

	// Text
	txtX := int(btnX) + 60
	if g.ui.IsRegisterMode {
		txtX = int(btnX) + 65
	}
	ebitenutil.DebugPrintAt(screen, mainBtnLabel, txtX, int(btnY)+18)

	// Toggle Mode Button (Switch between Login/Register)
	regBtnY := btnY + 60

	toggleLabel := "CREATE ACCOUNT"
	if g.ui.IsRegisterMode {
		toggleLabel = "BACK TO LOGIN"
	}

	// Draw minimal text button/link style
	// vector.DrawFilledRect(screen, btnX, regBtnY, btnW, 30, color.RGBA{10, 30, 40, 255}, true)
	// vector.StrokeRect(screen, btnX, regBtnY, btnW, 30, 1, color.RGBA{100, 150, 200, 255}, true)

	// Just text with underline effect if hovered (simulated)
	// We'll just draw centered text for now
	ebitenutil.DebugPrintAt(screen, toggleLabel, int(btnX)+45, int(regBtnY))

	checkboxX, checkboxY := cx-150, float64(regBtnY)+60
	ebitenutil.DrawRect(screen, checkboxX, checkboxY, 20, 20, color.RGBA{200, 200, 200, 255})
	if g.ui.RememberMe {
		vector.DrawFilledRect(screen, float32(checkboxX+4), float32(checkboxY+4), 12, 12, color.RGBA{0, 200, 0, 255}, true)
	}
	ebitenutil.DebugPrintAt(screen, "Se souvenir de moi", int(checkboxX)+30, int(checkboxY)+2)

	ebitenutil.DebugPrintAt(screen, "v0.9.0-alpha", 10, int(h)-40)
	ebitenutil.DebugPrintAt(screen, "Developed by Masterkey Technologic", 10, int(h)-20)

	if g.ui.LoginError != "" {
		ebitenutil.DebugPrintAt(screen, g.ui.LoginError, int(cx)-100, int(loginY)+230)
	}

	if g.ui.ShowUpdateModal {
		// Overlay
		vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 220}, true)

		// Modal Box
		cx, cy := w/2, h/2
		boxW, boxH := float32(400), float32(200)
		boxX, boxY := float32(cx)-boxW/2, float32(cy)-boxH/2

		vector.DrawFilledRect(screen, boxX, boxY, boxW, boxH, color.RGBA{40, 45, 50, 255}, true)
		// Fancy Border
		vector.StrokeRect(screen, boxX, boxY, boxW, boxH, 2, color.RGBA{218, 165, 32, 255}, true) // Gold Border

		// Text
		ebitenutil.DebugPrintAt(screen, "MISE A JOUR DISPONIBLE !", int(cx)-80, int(cy)-60)
		ebitenutil.DebugPrintAt(screen, "Une nouvelle version est disponible.", int(cx)-100, int(cy)-30)

		// Button
		btnW, btnH := float32(200), float32(50)
		btnX, btnY := float32(cx)-btnW/2, float32(cy)+20

		btnCol := color.RGBA{50, 150, 50, 255} // Green
		btnText := "INSTALLER"

		if g.ui.UpdateInProgress {
			btnCol = color.RGBA{100, 100, 100, 255} // Grey
			btnText = "INSTALLATION..."
		} else if g.ui.HoverUpdateBtn {
			btnCol = color.RGBA{70, 180, 70, 255} // Brighter Green
			ebiten.SetCursorShape(ebiten.CursorShapePointer)
		} else {
			ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		}

		vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, btnCol, true)
		vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 2, color.White, true)

		// Center text approx
		textX := int(btnX) + 60
		if g.ui.UpdateInProgress {
			textX = int(btnX) + 50
		}
		ebitenutil.DebugPrintAt(screen, btnText, textX, int(btnY)+18)
	}
}
