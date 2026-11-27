package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Fenêtre « Équipage » : répartir guerriers, archers et artilleurs sur un
// navire. Trois curseurs larges (glissables à la souris et au doigt), des
// boutons −10 −1 +1 +10 Max, et un seul bouton Valider qui applique tous
// les changements d'un coup.

var crewTypes = [3]domain.UnitType{domain.Warrior, domain.Archer, domain.Gunner}

var crewTypeLabels = [3]string{"Guerriers", "Archers", "Artilleurs"}
var crewTypeHints = [3]string{"abordage", "tir à distance", "canons"}
var crewTypeColors = [3]color.RGBA{
	{192, 57, 43, 255},
	{79, 143, 207, 255},
	{224, 176, 74, 255},
}

// shipMilitiaCapacity : même règle que le serveur (50 × rang du navire).
func shipMilitiaCapacity(shipType string) int {
	switch shipType {
	case "brigantine":
		return 100
	case "frigate":
		return 150
	case "galleon":
		return 200
	case "manowar":
		return 250
	}
	return 50
}

// Positions de la fenêtre (partagées par Draw et Update).
type crewLayout struct {
	x, y, w, h       float64
	closeCX, closeCY float64
	closeR           float64
	barY             float64
	rowY             [3]float64 // haut de chaque bloc
	ctrlY            float64    // décalage de la ligne de contrôles dans un bloc
	ctrlH            float64
	stepW, maxW      float64
	m10X, m1X        float64
	trackX, trackW   float64
	p1X, p10X, maxX  float64
	valueX           float64
	btnY, btnH       float64
	cancelX, okX     float64
	cancelW, okW     float64
}

const crewRowH = 104.0

func (g *Game) crewModalLayout() crewLayout {
	var l crewLayout
	l.w, l.h = 800, 620
	l.x = float64(g.screenWidth)/2 - l.w/2
	l.y = float64(g.screenHeight)/2 - l.h/2
	l.closeR = 24
	l.closeCX, l.closeCY = l.x+l.w-44, l.y+42
	l.barY = l.y + 118
	for i := range l.rowY {
		l.rowY[i] = l.y + 160 + float64(i)*(crewRowH+10)
	}
	l.ctrlY, l.ctrlH = 40, 48
	l.stepW, l.maxW = 54, 66
	gap := 8.0
	left := l.x + 24
	right := l.x + l.w - 24
	l.m10X = left + 14
	l.m1X = l.m10X + l.stepW + gap
	l.valueX = right - 14 // aligné à droite
	l.maxX = l.valueX - 60 - l.maxW
	l.p10X = l.maxX - gap - l.stepW
	l.p1X = l.p10X - gap - l.stepW
	l.trackX = l.m1X + l.stepW + 26
	l.trackW = l.p1X - 26 - l.trackX
	l.btnH = 50
	l.btnY = l.y + l.h - l.btnH - 22
	l.okW, l.cancelW = 170, 140
	l.okX = right - l.okW
	l.cancelX = l.okX - 14 - l.cancelW
	return l
}

func inRect(px, py, x, y, w, h float64) bool {
	return px >= x && px <= x+w && py >= y && py <= y+h
}

// crewModalData : le navire choisi, l'île et la flotte du navire.
func (g *Game) crewModalData() (*domain.Ship, *domain.Island, *domain.Fleet) {
	shipID, err := uuid.Parse(g.ui.SelectedShipForCrew)
	if err != nil || g.player == nil || len(g.player.Islands) == 0 {
		return nil, nil, nil
	}
	island := &g.player.Islands[0]
	var ship *domain.Ship
	for i := range island.Ships {
		if island.Ships[i].ID == shipID {
			ship = &island.Ships[i]
			break
		}
	}
	var fleet *domain.Fleet
	for i := range island.Fleets {
		f := &island.Fleets[i]
		for j := range f.Ships {
			if f.Ships[j].ID == shipID {
				fleet = f
				if ship == nil {
					ship = &f.Ships[j]
				}
			}
		}
	}
	if ship != nil && fleet == nil && ship.FleetID != nil {
		for i := range island.Fleets {
			if island.Fleets[i].ID == *ship.FleetID {
				fleet = &island.Fleets[i]
			}
		}
	}
	return ship, island, fleet
}

// shipDocked : on ne change l'équipage qu'à quai (même règle que le serveur).
func shipDocked(fleet *domain.Fleet) bool {
	return fleet == nil || fleet.State == "" || fleet.State == domain.FleetStateIdle
}

func (g *Game) crewModalValues() [3]int {
	return [3]int{g.ui.CrewModalWarriors, g.ui.CrewModalArchers, g.ui.CrewModalGunners}
}

func (g *Game) setCrewModalValues(v [3]int) {
	g.ui.CrewModalWarriors, g.ui.CrewModalArchers, g.ui.CrewModalGunners = v[0], v[1], v[2]
}

func shipCrew(ship *domain.Ship) [3]int {
	return [3]int{ship.MilitiaWarriors, ship.MilitiaArchers, ship.MilitiaGunners}
}

func islandCrew(island *domain.Island, k int) int {
	if island == nil || island.Crew == nil {
		return 0
	}
	return island.Crew[crewTypes[k]]
}

// setCrewValue change un type en restant dans les places libres et le stock.
func (g *Game) setCrewValue(k, n int, ship *domain.Ship, island *domain.Island) {
	v := g.crewModalValues()
	capacity := shipMilitiaCapacity(ship.Type)
	others := v[0] + v[1] + v[2] - v[k]
	limit := capacity - others
	if avail := shipCrew(ship)[k] + islandCrew(island, k); avail < limit {
		limit = avail
	}
	if n > limit {
		n = limit
	}
	if n < 0 {
		n = 0
	}
	v[k] = n
	g.setCrewModalValues(v)
}

func (g *Game) closeCrewModal() {
	g.ui.ShowCrewModal = false
	g.ui.SelectedShipForCrew = ""
	g.ui.CrewModalError = ""
	g.ui.DraggingCrewWarrior = false
	g.ui.DraggingCrewArcher = false
	g.ui.DraggingCrewGunner = false
}

func (g *Game) crewDragging() int {
	switch {
	case g.ui.DraggingCrewWarrior:
		return 0
	case g.ui.DraggingCrewArcher:
		return 1
	case g.ui.DraggingCrewGunner:
		return 2
	}
	return -1
}

func (g *Game) setCrewDragging(k int) {
	g.ui.DraggingCrewWarrior = k == 0
	g.ui.DraggingCrewArcher = k == 1
	g.ui.DraggingCrewGunner = k == 2
}

// DrawCrewAssignmentModal dessine la fenêtre Équipage.
func (g *Game) DrawCrewAssignmentModal(screen *ebiten.Image) {
	if !g.ui.ShowCrewModal {
		return
	}
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	uiFillRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{0, 0, 0, 170}, vecAA)

	l := g.crewModalLayout()
	fillRoundRect(screen, l.x, l.y, l.w, l.h, 18, color.RGBA{24, 16, 9, 255}) // fond opaque : le menu flotte ne transparaît plus
	scrollBlocker(l.x, l.y, l.w, l.h)
	drawThemePanel(screen, l.x, l.y, l.w, l.h, 18)
	drawRoundButton(screen, l.closeCX, l.closeCY, l.closeR, "close", isHovered(l.closeCX-l.closeR, l.closeCY-l.closeR, 2*l.closeR, 2*l.closeR))

	ship, island, fleet := g.crewModalData()
	if ship == nil {
		drawText(screen, "Navire introuvable", bodyFace(16, true), l.x+24, l.y+30, themeText, alignLeft)
		return
	}
	docked := shipDocked(fleet)

	drawText(screen, ellipsize("Équipage — "+ship.Name, titleFace(30), l.w-120), titleFace(30), l.x+24, l.y+18, themeGold, alignLeft)
	where := "à quai sur l'île"
	if !docked {
		where = "en mer : l'équipage se change seulement à quai"
	}
	drawText(screen, shipTypeLabel(ship.Type)+" · "+where, bodyFace(14, false), l.x+24, l.y+60, themeMuted, alignLeft)

	v := g.crewModalValues()
	cur := shipCrew(ship)
	capacity := shipMilitiaCapacity(ship.Type)
	total := v[0] + v[1] + v[2]

	// Places à bord
	drawText(screen, "Places à bord", bodyFace(15, true), l.x+24, l.barY-26, themeText, alignLeft)
	drawText(screen, fmt.Sprintf("%d / %d · %d libres", total, capacity, capacity-total), bodyFace(15, true), l.x+l.w-24, l.barY-26, themeGreen, alignRight)
	barX, barW := l.x+24, l.w-48
	fillRoundRect(screen, barX, l.barY, barW, 16, 8, color.RGBA{58, 39, 20, 255})
	bx := barX
	for k := 0; k < 3; k++ {
		if v[k] <= 0 || capacity <= 0 {
			continue
		}
		w := barW * float64(v[k]) / float64(capacity)
		vector.DrawFilledRect(screen, float32(bx), float32(l.barY), float32(w), 16, crewTypeColors[k], vecAA) // pas uiFillRect : il changerait le bleu en bois
		bx += w
	}
	strokeRoundRect(screen, barX, l.barY, barW, 16, 8, 1, color.RGBA{107, 74, 34, 255})

	dragging := g.crewDragging()
	for k := 0; k < 3; k++ {
		ry := l.rowY[k]
		fillRoundRect(screen, l.x+24, ry, l.w-48, crewRowH, 12, color.NRGBA{255, 255, 255, 10})
		vector.DrawFilledCircle(screen, float32(l.x+46), float32(ry+20), 6, crewTypeColors[k], vecAA)
		drawText(screen, crewTypeLabels[k], bodyFace(17, true), l.x+60, ry+9, themeText, alignLeft)
		drawText(screen, crewTypeHints[k], bodyFace(13, false), l.x+60+textWidth(crewTypeLabels[k], bodyFace(17, true))+10, ry+13, themeMuted, alignLeft)
		stockAfter := islandCrew(island, k) - (v[k] - cur[k])
		drawText(screen, fmt.Sprintf("À bord : %d · Stock île après : %d", cur[k], stockAfter), bodyFace(13, false), l.x+l.w-38, ry+13, themeMuted, alignRight)

		cy := ry + l.ctrlY
		kind := btnSecondary
		if !docked {
			kind = btnDisabled
		}
		drawButton(screen, l.m10X, cy, l.stepW, l.ctrlH, "−10", kind)
		drawButton(screen, l.m1X, cy, l.stepW, l.ctrlH, "−1", kind)
		drawButton(screen, l.p1X, cy, l.stepW, l.ctrlH, "+1", kind)
		drawButton(screen, l.p10X, cy, l.stepW, l.ctrlH, "+10", kind)
		drawButton(screen, l.maxX, cy, l.maxW, l.ctrlH, "Max", kind)

		// Curseur : piste épaisse + grosse poignée ronde
		mid := cy + l.ctrlH/2
		fillRoundRect(screen, l.trackX, mid-7, l.trackW, 14, 7, color.RGBA{58, 39, 20, 255})
		ratio := 0.0
		if capacity > 0 {
			ratio = math.Min(1, float64(v[k])/float64(capacity))
		}
		if ratio > 0 {
			fillRoundRect(screen, l.trackX, mid-7, math.Max(14, l.trackW*ratio), 14, 7, crewTypeColors[k])
		}
		strokeRoundRect(screen, l.trackX, mid-7, l.trackW, 14, 7, 1, color.RGBA{107, 74, 34, 255})
		hx := l.trackX + l.trackW*ratio
		r := float32(18)
		if dragging == k {
			r = 21
		}
		thumb := color.Color(themeGold)
		if !docked {
			thumb = themeMuted
		}
		vector.DrawFilledCircle(screen, float32(hx)+1, float32(mid)+3, r, color.RGBA{0, 0, 0, 90}, vecAA)
		vector.DrawFilledCircle(screen, float32(hx), float32(mid), r, thumb, vecAA)
		vector.StrokeCircle(screen, float32(hx), float32(mid), r, 3, themeInk, vecAA)

		drawText(screen, fmt.Sprintf("%d", v[k]), bodyFace(22, true), l.valueX, cy+10, themeGold, alignRight)
	}

	// Résumé des changements
	var parts []string
	for k := 0; k < 3; k++ {
		if d := v[k] - cur[k]; d != 0 {
			parts = append(parts, fmt.Sprintf("%+d %s", d, strings.ToLower(crewTypeLabels[k])))
		}
	}
	msgY := l.btnY + 6
	msgW := l.cancelX - l.x - 40
	switch {
	case g.ui.CrewModalBusy:
		drawText(screen, "Envoi en cours…", bodyFace(14, false), l.x+24, msgY, themeMuted, alignLeft)
	case g.ui.CrewModalError != "":
		lines := wrapText("Erreur : "+g.ui.CrewModalError, bodyFace(14, false), msgW)
		for i, ln := range lines {
			if i > 2 {
				break
			}
			drawText(screen, ln, bodyFace(14, false), l.x+24, msgY+float64(i)*17, themeOrange, alignLeft)
		}
	case len(parts) > 0:
		lines := wrapText("Changements : "+strings.Join(parts, ", "), bodyFace(14, false), msgW)
		for i, ln := range lines {
			drawText(screen, ln, bodyFace(14, false), l.x+24, msgY+float64(i)*17, themeText, alignLeft)
		}
	default:
		drawText(screen, "Aucun changement", bodyFace(14, false), l.x+24, msgY, themeMuted, alignLeft)
	}

	drawButton(screen, l.cancelX, l.btnY, l.cancelW, l.btnH, "Fermer", btnSecondary)
	okKind := btnPrimary
	if len(parts) == 0 || g.ui.CrewModalBusy || !docked {
		okKind = btnDisabled
	}
	drawButton(screen, l.okX, l.btnY, l.okW, l.btnH, "Valider", okKind)
}

// UpdateCrewAssignmentModal gère la souris / le doigt pour la fenêtre
// Équipage. Renvoie vrai quand elle est ouverte (elle prend toutes les entrées).
func (g *Game) UpdateCrewAssignmentModal() bool {
	if !g.ui.ShowCrewModal {
		return false
	}
	if consumeEscape() {
		g.closeCrewModal()
		return true
	}

	l := g.crewModalLayout()
	mx, my := cursorPosition()
	fx, fy := float64(mx), float64(my)
	down := isPointerJustDown()
	clicked := isMouseJustPressed(ebiten.MouseButtonLeft)

	// Fermer : la croix, ou un clic / tap en dehors de la fenêtre.
	if clicked && g.crewDragging() < 0 {
		if math.Hypot(fx-l.closeCX, fy-l.closeCY) <= l.closeR+6 || !inRect(fx, fy, l.x, l.y, l.w, l.h) {
			g.closeCrewModal()
			return true
		}
	}

	ship, island, fleet := g.crewModalData()
	if ship == nil {
		return true
	}
	docked := shipDocked(fleet)
	capacity := shipMilitiaCapacity(ship.Type)

	if !isMousePressed(ebiten.MouseButtonLeft) {
		g.setCrewDragging(-1)
	}

	if docked && !g.ui.CrewModalBusy {
		// Début du glisser : le doigt / le bouton se pose sur la piste.
		if down {
			for k := 0; k < 3; k++ {
				cy := l.rowY[k] + l.ctrlY
				if inRect(fx, fy, l.trackX-20, cy-6, l.trackW+40, l.ctrlH+12) {
					g.setCrewDragging(k)
					break
				}
			}
		}
		if k := g.crewDragging(); k >= 0 && isMousePressed(ebiten.MouseButtonLeft) {
			ratio := (fx - l.trackX) / l.trackW
			ratio = math.Max(0, math.Min(1, ratio))
			g.setCrewValue(k, int(math.Round(ratio*float64(capacity))), ship, island)
			return true
		}

		if clicked {
			v := g.crewModalValues()
			for k := 0; k < 3; k++ {
				cy := l.rowY[k] + l.ctrlY
				if fy < cy || fy > cy+l.ctrlH {
					continue
				}
				switch {
				case inRect(fx, fy, l.m10X, cy, l.stepW, l.ctrlH):
					g.setCrewValue(k, v[k]-10, ship, island)
				case inRect(fx, fy, l.m1X, cy, l.stepW, l.ctrlH):
					g.setCrewValue(k, v[k]-1, ship, island)
				case inRect(fx, fy, l.p1X, cy, l.stepW, l.ctrlH):
					g.setCrewValue(k, v[k]+1, ship, island)
				case inRect(fx, fy, l.p10X, cy, l.stepW, l.ctrlH):
					g.setCrewValue(k, v[k]+10, ship, island)
				case inRect(fx, fy, l.maxX, cy, l.maxW, l.ctrlH):
					g.setCrewValue(k, math.MaxInt32, ship, island)
				}
			}
		}
	}

	if clicked && inRect(fx, fy, l.cancelX, l.btnY, l.cancelW, l.btnH) {
		g.closeCrewModal()
		return true
	}
	if clicked && docked && !g.ui.CrewModalBusy && inRect(fx, fy, l.okX, l.btnY, l.okW, l.btnH) {
		v := g.crewModalValues()
		cur := shipCrew(ship)
		var add, remove [3]int
		changed := false
		for k := 0; k < 3; k++ {
			d := v[k] - cur[k]
			if d > 0 {
				add[k] = d
			} else if d < 0 {
				remove[k] = -d
			}
			changed = changed || d != 0
		}
		if changed {
			g.applyCrewChanges(ship.ID, add, remove)
		}
	}
	return true
}

// applyCrewChanges retire d'abord (ça libère des places), puis ajoute.
// En cas de succès la fenêtre se ferme ; sinon l'erreur reste affichée.
func (g *Game) applyCrewChanges(shipID uuid.UUID, add, remove [3]int) {
	g.ui.CrewModalBusy = true
	g.ui.CrewModalError = ""
	go func() {
		defer func() { g.ui.CrewModalBusy = false }()
		fail := func(err error) {
			msg := err.Error()
			for k, t := range crewTypes {
				msg = strings.Replace(msg, string(t)+": ", crewTypeLabels[k]+" : ", 1)
			}
			if strings.Contains(msg, "session expirée") {
				msg = "Session expirée, veuillez vous reconnecter"
			}
			g.ui.CrewModalError = msg
			g.Log("Crew change error: %v", err)
			g.refreshStatusAfterDevAction()
		}
		if remove[0]+remove[1]+remove[2] > 0 {
			if err := g.api.UnassignCrew(shipID, remove[0], remove[1], remove[2]); err != nil {
				fail(err)
				return
			}
		}
		if add[0]+add[1]+add[2] > 0 {
			if err := g.api.AssignCrew(shipID, add[0], add[1], add[2]); err != nil {
				fail(err)
				return
			}
		}
		g.refreshStatusAfterDevAction()
		if g.ui.SelectedShipForCrew == shipID.String() {
			g.closeCrewModal()
		}
		g.Log("Crew changed: ship=%s +%v -%v", shipID.String()[:8], add, remove)
	}()
}
