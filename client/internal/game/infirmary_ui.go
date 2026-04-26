package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawInfirmaryUI draws the modal for the Infirmary building.
// It lists all injured captains and their remaining recovery time.
func (g *Game) DrawInfirmaryUI(screen *ebiten.Image) {
	if !g.ui.ShowInfirmaryUI {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)

	// Semi-transparent overlay
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{0, 0, 0, 150}, false)

	// Modal Window
	winW, winH := 500.0, 600.0
	winX, winY := (w-winW)/2, (h-winH)/2

	// Draw Background (Pinkish/Red theme for Infirmary)
	vector.DrawFilledRect(screen, float32(winX), float32(winY), float32(winW), float32(winH), color.RGBA{40, 20, 25, 255}, false)
	vector.StrokeRect(screen, float32(winX), float32(winY), float32(winW), float32(winH), 2, color.RGBA{200, 100, 100, 255}, false)

	// Title
	ebitenutil.DebugPrintAt(screen, "INFIRMERIE", int(winX)+20, int(winY)+20)
	ebitenutil.DebugPrintAt(screen, "[Fermer]", int(winX+winW)-80, int(winY)+20)

	vector.StrokeLine(screen, float32(winX)+20, float32(winY)+50, float32(winX+winW)-20, float32(winY)+50, 1, color.RGBA{150, 100, 100, 255}, false)

	// List area
	listY := winY + 60
	listH := winH - 80
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
				maxStars := 1 // Common
				switch cap.Rarity {
				case "rare":
					maxStars = 3
				case "legendary":
					maxStars = 5
				}

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

	// Calculate content height for scrolling
	itemH := 80.0
	gap := 10.0
	// No items message
	if len(injuredList) == 0 {
		ebitenutil.DebugPrintAt(screen, "Aucun capitaine blessé.", int(winX)+20, int(currentY))
		return
	}

	// Draw Items
	for _, cap := range injuredList {
		if currentY+itemH < listY { // Above view
			currentY += itemH + gap
			continue
		}
		if currentY > listY+listH { // Below view
			break
		}

		// Item Box
		vector.DrawFilledRect(screen, float32(winX)+20, float32(currentY), float32(winW)-40, float32(itemH), color.RGBA{60, 30, 35, 255}, false)
		vector.StrokeRect(screen, float32(winX)+20, float32(currentY), float32(winW)-40, float32(itemH), 1, color.RGBA{150, 100, 100, 255}, false)

		// Content
		// Name & Rarity
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%s)", cap.Name, cap.Rarity), int(winX)+30, int(currentY)+10)

		// Stars
		starStr := ""
		for i := 0; i < cap.Stars; i++ {
			starStr += "★"
		}
		for i := cap.Stars; i < cap.MaxStars; i++ {
			starStr += "☆"
		}
		ebitenutil.DebugPrintAt(screen, starStr, int(winX)+30, int(currentY)+30)

		// Timer (Large & Red)
		mins := int(cap.Remaining.Minutes())
		secs := int(cap.Remaining.Seconds()) % 60
		timeStr := fmt.Sprintf("Soins: %02dm %02ds", mins, secs)
		ebitenutil.DebugPrintAt(screen, timeStr, int(winX)+300, int(currentY)+30)

		// Progress Bar (Visual fluff)
		// Assuming max injury times: Common=30m, Rare=2h, Leg=5h
		// We can't know the exact max time here easily without logic duplication,
		// so we just show an indeterminate "Healing" text or specific time.
		// Let's add a small progress bar visual representing time logic if we wanted,
		// but simple text is enough for MVP.

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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowInfirmaryUI = false
		return true
	}

	// Dimensions (Must match Draw)
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	winW, winH := 500.0, 600.0
	winX, winY := (w-winW)/2, (h-winH)/2

	// Mouse Input
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Scroll
	_, wy := ebiten.Wheel()
	if wy != 0 {
		g.ui.InfirmaryScrollY -= wy * 30
		if g.ui.InfirmaryScrollY < 0 {
			g.ui.InfirmaryScrollY = 0
		}
		// Max scroll calc could be added if needed
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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
