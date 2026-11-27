package game

import (
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawInfirmaryUI draws the modal for the Infirmary building.
// It lists all injured captains and their remaining recovery time.
func (g *Game) DrawInfirmaryUI(screen *ebiten.Image) {
	if !g.ui.ShowInfirmaryUI {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)

	drawDimBackground(screen, 150)

	// Modal Window
	winW, winH := 520.0, 520.0
	winX, winY := (w-winW)/2, (h-winH)/2

	drawWindowFrame(screen, winX, winY, winW, winH, "Infirmerie")
	drawCloseButton(screen, winX+winW-30, winY+27, 15)
	drawText(screen, "Les capitaines blessés au combat se reposent ici.", bodyFace(13, false), winX+22, winY+60, themeMuted, alignLeft)

	// List area
	listY := winY + 90
	listH := winH - 110
	currentY := listY - g.ui.InfirmaryScrollY

	// Filter injured captains
	type InjuredCap struct {
		Name      string
		Rarity    string
		Remaining time.Duration
		Stars     int
		MaxStars  int
	}

	injuredList := []InjuredCap{}
	now := time.Now()

	for _, cap := range g.captains {
		if cap.InjuredUntil != nil {
			// InjuredUntil is already a *time.Time
			injuredTime := *cap.InjuredUntil
			if injuredTime.After(now) {
				maxStars := captainMaxStars(cap.Rarity)

				injuredList = append(injuredList, InjuredCap{
					Name:      cap.Name,
					Rarity:    cap.Rarity,
					Remaining: injuredTime.Sub(now),
					Stars:     cap.Stars,
					MaxStars:  maxStars,
				})
			}
		}
	}

	itemH := 66.0
	gap := 10.0
	if len(injuredList) == 0 {
		drawInset(screen, winX+20, listY, winW-40, 90)
		drawText(screen, "Aucun capitaine blessé.", bodyFace(15, true), winX+winW/2, listY+26, themeText, alignCenter)
		drawText(screen, "Tout l'équipage est d'attaque !", bodyFace(13, false), winX+winW/2, listY+50, themeMuted, alignCenter)
		return
	}

	view := screen.SubImage(image.Rect(int(winX), int(listY), int(winX+winW), int(listY+listH))).(*ebiten.Image)
	for _, cap := range injuredList {
		if currentY+itemH < listY {
			currentY += itemH + gap
			continue
		}
		if currentY > listY+listH {
			break
		}
		ix, iw := winX+20, winW-40
		drawCard(view, ix, currentY, iw, itemH, false)
		fillRoundRect(view, ix+14, currentY+itemH/2-5, 10, 10, 5, rarityColor(cap.Rarity))
		drawText(view, cap.Name, titleFace(20), ix+32, currentY+8, themeGold, alignLeft)
		drawText(view, rarityLabel(cap.Rarity), bodyFace(12, false), ix+32, currentY+38, themeMuted, alignLeft)
		drawStars(view, ix+32+textWidth(rarityLabel(cap.Rarity), bodyFace(12, false))+10, currentY+46, cap.Stars, cap.MaxStars, 5, false)
		drawText(view, "Soigné dans", bodyFace(12, false), ix+iw-16, currentY+12, themeMuted, alignRight)
		drawText(view, formatDuration(int(cap.Remaining.Seconds())), bodyFace(17, true), ix+iw-16, currentY+30, themeOrange, alignRight)
		currentY += itemH + gap
	}
}

// UpdateInfirmaryUI handles input for the Infirmary modal.
// Returns true if the UI consumed the input (blocking other interactions).
func (g *Game) UpdateInfirmaryUI() bool {
	if !g.ui.ShowInfirmaryUI {
		return false
	}

	// Close on Escape
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowInfirmaryUI = false
		return true
	}

	// Dimensions (Must match Draw)
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	winW, winH := 520.0, 520.0
	winX, winY := (w-winW)/2, (h-winH)/2

	// Mouse Input
	mx, my := cursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll
	_, wy := wheelDelta()
	if wy != 0 {
		g.ui.InfirmaryScrollY -= wy * 30
		if g.ui.InfirmaryScrollY < 0 {
			g.ui.InfirmaryScrollY = 0
		}
		// Max scroll calc could be added if needed
	}

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		// Close button (Top Right)
		if fmx >= winX+winW-100 && fmx <= winX+winW && fmy >= winY && fmy <= winY+50 {
			g.ui.ShowInfirmaryUI = false
			return true
		}

		// Click Outside -> Close
		if fmx < winX || fmx > winX+winW || fmy < winY || fmy > winY+winH {
			g.ui.ShowInfirmaryUI = false
			return true
		}

		return true // Consumed click inside
	}

	return true // Blocking hover/interaction
}
