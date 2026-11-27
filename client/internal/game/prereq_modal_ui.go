package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Fenêtre « Prérequis manquants ». Chaque prérequis est écrit en entier :
// les phrases longues passent à la ligne et la fenêtre grandit avec le
// texte (elle défile seulement si elle ne tient plus à l'écran).

const (
	prereqW       = 620.0
	prereqPadding = 22.0
	prereqLineH   = 22.0
	prereqRowGap  = 6.0
)

func prereqFace() text.Face { return bodyFace(17, false) }

// prereqLayout : lignes de texte de chaque prérequis et géométrie de la fenêtre.
type prereqLayoutT struct {
	x, y, w, h float64
	subLines   []string
	rows       [][]string
	listTop    float64
	listH      float64
	contentH   float64
	rowW       float64
}

func (g *Game) prereqLayout() prereqLayoutT {
	var l prereqLayoutT
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	l.w = prereqW
	l.rowW = l.w - 2*prereqPadding
	face := prereqFace()
	if g.ui.PrereqModalSubtitle != "" {
		l.subLines = wrapText(g.ui.PrereqModalSubtitle, bodyFace(14, false), l.rowW)
	}
	for _, req := range g.ui.PrereqModalReqs {
		lines := wrapText(stripUnsupported(formatRequirement(req)), face, l.rowW-50)
		if len(lines) == 0 {
			lines = []string{""}
		}
		l.rows = append(l.rows, lines)
		l.contentH += prereqRowH(len(lines)) + prereqRowGap
	}
	if len(l.rows) == 0 {
		l.contentH = 30
	}
	top := 66.0 + float64(len(l.subLines))*18
	if len(l.subLines) > 0 {
		top += 8
	}
	l.h = top + l.contentH + 78
	maxH := sh - 30
	if l.h > maxH {
		l.h = maxH
	}
	if l.h < 220 {
		l.h = 220
	}
	l.x, l.y = (sw-l.w)/2, (sh-l.h)/2
	l.listTop = l.y + top
	l.listH = l.y + l.h - 70 - l.listTop
	return l
}

func prereqRowH(lines int) float64 { return 14 + float64(lines)*prereqLineH }

func (l prereqLayoutT) maxScroll() float64 {
	return math.Max(0, l.contentH-l.listH)
}

// DrawPrereqModal draws the prerequisites modal
func (g *Game) DrawPrereqModal(screen *ebiten.Image) {
	if !g.ui.ShowPrereqModal {
		return
	}
	l := g.prereqLayout()
	x, y, w, h := l.x, l.y, l.w, l.h

	drawDimBackground(screen, 170)
	drawWindowFrame(screen, x, y, w, h, niceTitle(g.ui.PrereqModalTitle, "Prérequis manquants"))
	drawCloseButton(screen, x+w-30, y+27, 15)

	for i, ln := range l.subLines {
		drawText(screen, ln, bodyFace(14, false), x+prereqPadding, y+60+float64(i)*18, themeMuted, alignLeft)
	}

	g.ui.PrereqModalScroll = math.Max(0, math.Min(g.ui.PrereqModalScroll, l.maxScroll()))
	face := prereqFace()
	if len(l.rows) == 0 {
		drawText(screen, "Aucun prérequis indiqué.", face, x+prereqPadding, l.listTop+4, themeMuted, alignLeft)
	} else {
		clip := screen.SubImage(image.Rect(int(x), int(l.listTop), int(x+w), int(l.listTop+l.listH))).(*ebiten.Image)
		rowW := l.rowW
		if l.maxScroll() > 0 {
			rowW -= 14
		}
		cy := l.listTop - g.ui.PrereqModalScroll
		for _, lines := range l.rows {
			rh := prereqRowH(len(lines))
			if cy+rh >= l.listTop && cy <= l.listTop+l.listH {
				drawInset(clip, x+prereqPadding, cy, rowW, rh)
				fillRoundRect(clip, x+prereqPadding+14, cy+7+prereqLineH/2-3, 7, 7, 3.5, themeRed)
				for i, ln := range lines {
					drawText(clip, ln, face, x+prereqPadding+32, cy+7+float64(i)*prereqLineH, themeText, alignLeft)
				}
			}
			cy += rh + prereqRowGap
		}
		if ms := l.maxScroll(); ms > 0 {
			barX := x + w - prereqPadding - 6
			fillRoundRect(screen, barX, l.listTop, 6, l.listH, 3, color.NRGBA{255, 255, 255, 20})
			thumbH := math.Max(24, l.listH*l.listH/l.contentH)
			thumbY := l.listTop + (g.ui.PrereqModalScroll/ms)*(l.listH-thumbH)
			fillRoundRect(screen, barX, thumbY, 6, thumbH, 3, themeGoldDim)
		}
	}

	btnW, btnH := 160.0, 44.0
	drawButton(screen, x+w/2-btnW/2, y+h-58, btnW, btnH, "Compris", btnPrimary)
}

// niceTitle : un titre écrit en majuscules passe en casse normale.
func niceTitle(s, fallback string) string {
	if s == "" {
		return fallback
	}
	if strings.ToUpper(s) != s {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// formatRequirement formats a requirement for display
func formatRequirement(req domain.Requirement) string {
	if req.Message != "" {
		return req.Message
	}

	// Fallback formatting based on kind
	switch req.Kind {
	case "building_level":
		if req.Current == 0 {
			return fmt.Sprintf("%s niveau %d requis (non construit)", req.Name, req.Needed)
		}
		return fmt.Sprintf("%s niveau %d requis (actuel: %d)", req.Name, req.Needed, req.Current)
	case "tech":
		return fmt.Sprintf("Technologie requise: %s (non recherchée)", req.Name)
	case "resource":
		return fmt.Sprintf("%s insuffisant: besoin de %d, avez %d", req.Name, req.Needed, req.Current)
	default:
		return fmt.Sprintf("%s requis", req.Name)
	}
}

// UpdatePrereqModal handles input for the prerequisites modal
// Returns true if input was consumed
func (g *Game) UpdatePrereqModal() bool {
	if !g.ui.ShowPrereqModal {
		return false
	}

	l := g.prereqLayout()
	x, y, w, h := l.x, l.y, l.w, l.h

	// Close on ESC
	if consumeEscape() {
		g.closePrereqModal()
		return true
	}

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		fmx, fmy := float64(mx), float64(my)

		// Croix, bouton « Compris », ou clic en dehors : on ferme.
		onClose := fmx >= x+w-50 && fmx <= x+w-10 && fmy >= y+6 && fmy <= y+48
		btnW, btnH := 160.0, 44.0
		btnX, btnY := x+w/2-btnW/2, y+h-58
		onOK := fmx >= btnX && fmx <= btnX+btnW && fmy >= btnY && fmy <= btnY+btnH
		outside := fmx < x || fmx > x+w || fmy < y || fmy > y+h
		if onClose || onOK || outside {
			g.closePrereqModal()
		}
		return true
	}

	// Molette / doigt / flèches : défilement en pixels
	step := 0.0
	if _, dy := wheelDelta(); dy != 0 {
		step = -dy * 40
	}
	if isKeyJustPressed(ebiten.KeyArrowDown) {
		step = 40
	}
	if isKeyJustPressed(ebiten.KeyArrowUp) {
		step = -40
	}
	if step != 0 {
		g.ui.PrereqModalScroll = math.Max(0, math.Min(g.ui.PrereqModalScroll+step, l.maxScroll()))
	}

	return true // Always consume input when modal is open
}

// closePrereqModal closes the prerequisites modal
func (g *Game) closePrereqModal() {
	g.ui.ShowPrereqModal = false
	g.ui.PrereqModalTitle = ""
	g.ui.PrereqModalSubtitle = ""
	g.ui.PrereqModalReqs = nil
	g.ui.PrereqModalScroll = 0
}

// openPrereqModal opens the prerequisites modal with the given requirements
func (g *Game) openPrereqModal(title string, subtitle string, reqs []domain.Requirement) {
	g.ui.ShowPrereqModal = true
	g.ui.PrereqModalTitle = title
	g.ui.PrereqModalSubtitle = subtitle
	g.ui.PrereqModalReqs = reqs
	g.ui.PrereqModalScroll = 0
	fmt.Printf("[PREREQ] modal opened title=%s reqs=%d\n", title, len(reqs))
}
