package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// sea_menus_ui.go : fenêtres et boutons de la vue mer, au même style bois et
// or que l'île (drawWindowFrame, drawCard, drawButton...). Les zones de clic
// sont calculées par les mêmes fonctions que le dessin pour rester alignées.

// seaWindow : fond assombri + fenêtre centrée. Renvoie le coin haut gauche.
func (g *Game) seaWindow(screen *ebiten.Image, w, h float64, title string) (float64, float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	x, y := (sw-w)/2, (sh-h)/2
	drawDimBackground(screen, 170)
	drawWindowFrame(screen, x, y, w, h, title)
	return x, y
}

// seaWrap : texte sur plusieurs lignes, renvoie le y sous le dernier.
func seaWrap(screen *ebiten.Image, s string, x, y, maxW, size float64, clr color.Color) float64 {
	f := bodyFace(size, false)
	for _, line := range wrapText(stripUnsupported(s), f, maxW) {
		drawText(screen, line, f, x, y, clr, alignLeft)
		y += size + 6
	}
	return y
}

// fleetStateLabel : état d'une flotte en français.
func fleetStateLabel(f domain.Fleet) string {
	if f.StationedNodeID != nil {
		return "En récolte"
	}
	switch f.State {
	case domain.FleetStateIdle, "":
		return "Au port"
	case domain.FleetStateMoving:
		return "En route"
	case domain.FleetStateReturning, domain.FleetStateReturningFromAttack:
		return "Retour au port"
	case domain.FleetStateStationed, domain.FleetStateSeaStationed:
		return "En station"
	case domain.FleetStateChasingPvE, domain.FleetStateChasingPvP:
		return "En poursuite"
	case domain.FleetStateTravelingToAttack:
		return "Part à l'attaque"
	}
	return string(f.State)
}

// ---------- Boussole du vent et bouton de retour ----------

func (g *Game) windRect() (cx, cy, r float64) {
	return float64(g.screenWidth) - 60, 120, 36
}

func (g *Game) DrawWindIndicator(screen *ebiten.Image) {
	cx, cy, r := g.windRect()
	vector.DrawFilledCircle(screen, float32(cx+1), float32(cy+3), float32(r), color.RGBA{0, 0, 0, 70}, vecAA)
	vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(r), themePanel, vecAA)
	vector.StrokeCircle(screen, float32(cx), float32(cy), float32(r), 2, themeGold, vecAA)
	vector.StrokeCircle(screen, float32(cx), float32(cy), float32(r-7), 1, themeGoldDim, vecAA)
	// Points cardinaux
	f := bodyFace(10, true)
	drawText(screen, "N", f, cx, cy-r+3, themeMuted, alignCenter)

	rad := g.ui.WindDirection * (math.Pi / 180.0)
	dx, dy := math.Cos(rad), math.Sin(rad)
	tipX, tipY := cx+dx*r*0.68, cy+dy*r*0.68
	tailX, tailY := cx-dx*r*0.55, cy-dy*r*0.55
	vector.StrokeLine(screen, float32(tailX), float32(tailY), float32(tipX), float32(tipY), 3, themeGold, vecAA)
	// Pointe de flèche
	px, py := -dy, dx
	var p vector.Path
	p.MoveTo(float32(tipX+dx*6), float32(tipY+dy*6))
	p.LineTo(float32(tipX-dx*5+px*6), float32(tipY-dy*5+py*6))
	p.LineTo(float32(tipX-dx*5-px*6), float32(tipY-dy*5-py*6))
	p.Close()
	drawPath(screen, &p, themeGold, 0)
	vector.DrawFilledCircle(screen, float32(cx), float32(cy), 3, themeGold, vecAA)

	label := fmt.Sprintf("Vent %s", windCompass(g.ui.WindDirection))
	fl := bodyFace(12, true)
	lw := textWidth(label, fl) + 16
	fillRoundRect(screen, cx-lw/2, cy+r+6, lw, 20, 10, themePanel)
	drawText(screen, label, fl, cx, cy+r+8, themeText, alignCenter)
}

// windCompass : direction de la flèche (0 = vers la droite, 90 = vers le bas)
// en abrégé : N, NE, E...
func windCompass(deg float64) string {
	names := []string{"E", "SE", "S", "SO", "O", "NO", "N", "NE"}
	return names[int(math.Mod(math.Mod(deg, 360)+360+22.5, 360)/45)%8]
}

// recallButtonRect : bouton « Retour au port » sous la boussole.
func (g *Game) recallButtonRect() (x, y, w, h float64) {
	w, h = 150, 36
	return float64(g.screenWidth) - w - 12, 190, w, h
}

// recallButtonVisible : la flotte active est en mer et peut rentrer.
func (g *Game) recallButtonVisible() bool {
	if g.player == nil || len(g.player.Islands) == 0 || g.player.Islands[0].ActiveFleetID == nil {
		return false
	}
	for _, f := range g.player.Islands[0].Fleets {
		if f.ID == *g.player.Islands[0].ActiveFleetID {
			return !(f.State == domain.FleetStateIdle || f.State == domain.FleetStateReturning || f.State == domain.FleetStateReturningFromAttack)
		}
	}
	return false
}

func (g *Game) DrawRecallButton(screen *ebiten.Image) {
	if !g.recallButtonVisible() {
		return
	}
	x, y, w, h := g.recallButtonRect()
	drawButton(screen, x, y, w, h, "Retour au port", btnSecondary)
}

// ---------- Stationner une flotte sur un spot ----------

const (
	stationW, stationH = 440.0, 420.0
	stationRowH        = 58.0
	stationRowGap      = 8.0
)

// stationRowButton : bouton de la ligne i (Envoyer / Rappeler).
func (g *Game) stationRowButton(i int) (x, y, w, h float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	px, py := (sw-stationW)/2, (sh-stationH)/2
	rowY := py + 92 + float64(i)*(stationRowH+stationRowGap)
	w, h = 104, 36
	return px + stationW - 22 - 12 - w, rowY + (stationRowH-h)/2, w, h
}

func (g *Game) stationCloseButton() (cx, cy, r float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	px, py := (sw-stationW)/2, (sh-stationH)/2
	return px + stationW - 30, py + 28, 16
}

// stationFleetAction : ce que le bouton de la flotte fait ("send", "recall" ou "").
func stationFleetAction(f domain.Fleet) string {
	if f.StationedNodeID != nil || f.State == domain.FleetStateStationed {
		return "recall"
	}
	if (f.State == domain.FleetStateIdle || f.State == "") && len(f.Ships) > 0 {
		return "send"
	}
	return ""
}

func (g *Game) DrawStationMenu(screen *ebiten.Image) {
	px, py := g.seaWindow(screen, stationW, stationH, "Récolter ce spot")
	ccx, ccy, cr := g.stationCloseButton()
	drawCloseButton(screen, ccx, ccy, cr)

	drawLabel(screen, "Choisis la flotte à envoyer sur ce spot de ressources.", px+22, py+62, 13, false, themeMuted)

	if g.player == nil || len(g.player.Islands) == 0 || len(g.player.Islands[0].Fleets) == 0 {
		drawLabel(screen, "Aucune flotte pour l'instant.", px+22, py+100, 15, false, themeText)
		return
	}
	for i, f := range g.player.Islands[0].Fleets {
		bx, by, bw, bh := g.stationRowButton(i)
		rowY := by - (stationRowH-bh)/2
		drawCard(screen, px+22, rowY, stationW-44, stationRowH, false)
		drawLabel(screen, ellipsize(f.Name, bodyFace(15, true), bx-px-44), px+36, rowY+10, 15, true, themeText)
		sub := fmt.Sprintf("%d navire", len(f.Ships))
		if len(f.Ships) > 1 {
			sub += "s"
		}
		sub += " · " + fleetStateLabel(f)
		drawLabel(screen, sub, px+36, rowY+32, 12, false, themeMuted)

		switch stationFleetAction(f) {
		case "send":
			drawButton(screen, bx, by, bw, bh, "Envoyer", btnPrimary)
		case "recall":
			drawButton(screen, bx, by, bw, bh, "Rappeler", btnSecondary)
		default:
			if len(f.Ships) == 0 {
				drawButton(screen, bx, by, bw, bh, "Vide", btnDisabled)
			} else {
				drawButton(screen, bx, by, bw, bh, "Occupée", btnDisabled)
			}
		}
	}
}

// ---------- Bouton PvP ----------

func (g *Game) pvpButtonRect() (x, y, w, h float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	return sw - 220, sh - 200, 200, 50
}

func (g *Game) DrawPvpButton(screen *ebiten.Image) {
	x, y, w, h := g.pvpButtonRect()
	drawButton(screen, x, y, w, h, "Attaquer un joueur", btnDanger)
	if isHovered(x, y, w, h) && isMouseJustPressed(ebiten.MouseButtonLeft) && !g.IsModalOpen() {
		g.ui.ShowPvpUI = true
		if len(g.ui.PvpTargets) == 0 && !g.ui.PvpTargetsBusy {
			g.loadPvpTargets()
		}
	}
}

// ---------- Liste des cibles PvP ----------

func (g *Game) DrawPvpTargetList(screen *ebiten.Image) {
	if !g.ui.ShowPvpUI {
		return
	}
	// La sélection de flotte passe par-dessus : on ne dessine pas deux voiles sombres.
	if g.ui.ShowPvpFleetSelect {
		return
	}
	mw, mh := 600.0, 500.0
	mx, my := g.seaWindow(screen, mw, mh, "Attaquer un joueur")
	ccx, ccy := mx+mw-30, my+28
	drawCloseButton(screen, ccx, ccy, 16)
	click := isMouseJustPressed(ebiten.MouseButtonLeft)
	if click && isHovered(ccx-16, ccy-16, 32, 32) {
		g.ui.ShowPvpUI = false
		return
	}

	if g.ui.PvpAttackError != "" {
		seaWrap(screen, g.ui.PvpAttackError, mx+22, my+mh-46, mw-44, 13, themeOrange)
	}

	switch {
	case g.ui.PvpTargetsBusy:
		drawText(screen, "Chargement...", bodyFace(15, false), mx+mw/2, my+mh/2, themeMuted, alignCenter)
		return
	case g.ui.PvpTargetsError != "":
		seaWrap(screen, g.ui.PvpTargetsError, mx+22, my+70, mw-44, 15, themeOrange)
		return
	case len(g.ui.PvpTargets) == 0:
		drawText(screen, "Aucune cible à portée pour l'instant.", bodyFace(15, false), mx+mw/2, my+mh/2, themeMuted, alignCenter)
		return
	}

	rowH, gap := 70.0, 10.0
	for i, t := range g.ui.PvpTargets {
		ry := my + 70 + float64(i)*(rowH+gap)
		if ry+rowH > my+mh-56 {
			break
		}
		drawCard(screen, mx+22, ry, mw-44, rowH, false)
		drawLabel(screen, t.Name, mx+38, ry+14, 16, true, themeText)
		drawLabel(screen, fmt.Sprintf("Niveau %d", t.Tier), mx+38, ry+40, 13, false, themeMuted)
		bw, bh := 120.0, 38.0
		bx, by := mx+mw-22-14-bw, ry+(rowH-bh)/2
		drawButton(screen, bx, by, bw, bh, "Attaquer", btnDanger)
		if click && isHovered(bx, by, bw, bh) {
			g.ui.PvpSelectedTarget = &g.ui.PvpTargets[i]
			g.ui.ShowPvpFleetSelect = true
			g.ui.PvpAttackError = ""
		}
	}
}

// ---------- Choix de la flotte pour le PvP ----------

func (g *Game) DrawPvpFleetSelection(screen *ebiten.Image) {
	if !g.ui.ShowPvpFleetSelect || g.ui.PvpSelectedTarget == nil || g.ui.ShowPvpConfirm {
		return
	}
	mw, mh := 500.0, 420.0
	mx, my := g.seaWindow(screen, mw, mh, "Choisir une flotte")
	ccx, ccy := mx+mw-30, my+28
	drawCloseButton(screen, ccx, ccy, 16)
	click := isMouseJustPressed(ebiten.MouseButtonLeft)
	if click && isHovered(ccx-16, ccy-16, 32, 32) {
		g.ui.ShowPvpFleetSelect = false
		g.ui.PvpSelectedFleetID = ""
		return
	}
	drawLabel(screen, "Cible : "+g.ui.PvpSelectedTarget.Name, mx+22, my+62, 14, true, themeGold)

	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	rowH, gap := 58.0, 8.0
	for i, f := range g.player.Islands[0].Fleets {
		ry := my + 92 + float64(i)*(rowH+gap)
		if ry+rowH > my+mh-70 {
			break
		}
		away := f.State != domain.FleetStateIdle && f.State != ""
		empty := len(f.Ships) == 0
		drawCard(screen, mx+22, ry, mw-44, rowH, false)
		drawLabel(screen, f.Name, mx+38, ry+10, 15, true, themeText)
		status := "Disponible"
		clr := color.Color(themeGreen)
		switch {
		case empty:
			status, clr = "Aucun navire", themeMuted
		case away:
			status, clr = fleetStateLabel(f), themeOrange
		}
		drawLabel(screen, fmt.Sprintf("%d navires · %s", len(f.Ships), status), mx+38, ry+32, 12, false, clr)
		bw, bh := 110.0, 36.0
		bx, by := mx+mw-22-12-bw, ry+(rowH-bh)/2
		if away || empty {
			drawButton(screen, bx, by, bw, bh, "Choisir", btnDisabled)
			continue
		}
		drawButton(screen, bx, by, bw, bh, "Choisir", btnPrimary)
		if click && isHovered(bx, by, bw, bh) {
			g.ui.PvpSelectedFleetID = f.ID.String()
			g.ui.ShowPvpConfirm = true
		}
	}

	bw, bh := 130.0, 40.0
	bx, by := mx+(mw-bw)/2, my+mh-56
	drawButton(screen, bx, by, bw, bh, "Retour", btnSecondary)
	if click && isHovered(bx, by, bw, bh) {
		g.ui.ShowPvpFleetSelect = false
		g.ui.PvpSelectedFleetID = ""
	}
}

// ---------- Confirmation de l'attaque PvP ----------

func (g *Game) DrawPvpConfirmation(screen *ebiten.Image) {
	if !g.ui.ShowPvpConfirm || g.ui.PvpSelectedTarget == nil || g.ui.PvpSelectedFleetID == "" {
		return
	}
	mw, mh := 460.0, 320.0
	mx, my := g.seaWindow(screen, mw, mh, "Confirmer l'attaque")
	strokeRoundRect(screen, mx, my, mw, mh, 18, 2, themeRed)

	fleetName := "Flotte"
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, f := range g.player.Islands[0].Fleets {
			if f.ID.String() == g.ui.PvpSelectedFleetID {
				fleetName = f.Name
				break
			}
		}
	}
	drawLabel(screen, "Cible : "+g.ui.PvpSelectedTarget.Name, mx+24, my+66, 15, true, themeText)
	drawLabel(screen, "Flotte : "+fleetName, mx+24, my+90, 15, true, themeText)
	yy := my + 128
	yy = seaWrap(screen, "• Les navires détruits sont perdus pour de bon.", mx+24, yy, mw-48, 13, themeOrange)
	seaWrap(screen, "• Ta flotte sera bloquée 5 à 10 minutes après le combat.", mx+24, yy+2, mw-48, 13, themeOrange)
	if g.ui.PvpAttackError != "" {
		seaWrap(screen, g.ui.PvpAttackError, mx+24, my+mh-96, mw-48, 13, themeRed)
	}

	click := isMouseJustPressed(ebiten.MouseButtonLeft)
	bw, bh := 140.0, 42.0
	cx, cy := mx+40, my+mh-60
	ax := mx + mw - 40 - bw
	drawButton(screen, cx, cy, bw, bh, "Annuler", btnSecondary)
	if g.ui.PvpAttackBusy {
		drawButton(screen, ax, cy, bw, bh, "Envoi...", btnDisabled)
	} else {
		drawButton(screen, ax, cy, bw, bh, "Attaquer", btnDanger)
	}
	if click && isHovered(cx, cy, bw, bh) {
		g.ui.ShowPvpConfirm = false
	}
	if click && isHovered(ax, cy, bw, bh) && !g.ui.PvpAttackBusy {
		g.Log("[PVP UI] Confirmation button clicked!")
		g.executePvpAttack()
	}
}

// ---------- Interception PvP ----------

func (g *Game) DrawInterceptConfirmModal(screen *ebiten.Image) {
	if !g.ui.ShowInterceptConfirm {
		return
	}
	mw, mh := 440.0, 290.0
	mx, my := g.seaWindow(screen, mw, mh, "Interception")
	yy := seaWrap(screen, fmt.Sprintf("Intercepter la flotte « %s » avec ta flotte « %s » ?", g.ui.InterceptTargetName, g.ui.InterceptAttackerName), mx+24, my+66, mw-48, 15, themeText)
	yy = seaWrap(screen, "La poursuite dure 2 minutes au plus. Si elle échoue, tu devras attendre 10 minutes avant de réessayer.", mx+24, yy+8, mw-48, 13, themeMuted)
	if g.ui.InterceptError != "" {
		seaWrap(screen, g.ui.InterceptError, mx+24, yy+6, mw-48, 13, themeRed)
	}

	bw, bh := 140.0, 42.0
	cx, cy := g.interceptButtons()
	drawButton(screen, cx, cy, bw, bh, "Annuler", btnSecondary)
	if g.ui.InterceptBusy {
		drawButton(screen, mx+mw-40-bw, cy, bw, bh, "Envoi...", btnDisabled)
	} else {
		drawButton(screen, mx+mw-40-bw, cy, bw, bh, "Poursuivre", btnDanger)
	}

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		if isHovered(cx, cy, bw, bh) {
			g.ui.ShowInterceptConfirm = false
			return
		}
		if !g.ui.InterceptBusy && isHovered(mx+mw-40-bw, cy, bw, bh) {
			g.confirmIntercept()
		}
	}
}

func (g *Game) interceptButtons() (cancelX, y float64) {
	sw, sh := float64(g.screenWidth), float64(g.screenHeight)
	mx, my := (sw-440)/2, (sh-290)/2
	return mx + 40, my + 290 - 60
}

// DrawPursuitBanner : bandeau d'alerte quand une de tes flottes est poursuivie.
func (g *Game) DrawPursuitBanner(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	chased := false
	for _, f := range g.player.Islands[0].Fleets {
		if f.ChasedByFleetID != nil {
			chased = true
			break
		}
	}
	if !chased {
		return
	}
	sw := float64(g.screenWidth)
	text := "Tu es poursuivi ! Rentre au port en touchant ton île."
	f := bodyFace(15, true)
	bw := math.Min(textWidth(text, f)+56, sw-24)
	bx, by, bh := (sw-bw)/2, 84.0, 40.0
	fillRoundRect(screen, bx+2, by+4, bw, bh, 12, color.RGBA{0, 0, 0, 70})
	fillRoundRect(screen, bx, by, bw, bh, 12, color.RGBA{120, 28, 20, 235})
	strokeRoundRect(screen, bx, by, bw, bh, 12, 2, themeGold)
	vector.DrawFilledCircle(screen, float32(bx+24), float32(by+bh/2), 10, themeGold, vecAA)
	drawText(screen, "!", bodyFace(15, true), bx+24, by+bh/2-10, themeInk, alignCenter)
	drawText(screen, ellipsize(text, f, bw-56), f, bx+40, by+bh/2-10, themeText, alignLeft)
}

// ---------- Résultat d'un combat PvP ----------

func (g *Game) DrawPvpResultUI(screen *ebiten.Image) {
	if !g.ui.ShowPvpResultUI || g.ui.PvpCombatResult == nil {
		return
	}
	result := g.ui.PvpCombatResult
	win := result.Winner == "fleet_a"
	title := "Défaite"
	if win {
		title = "Victoire !"
	}
	mw, mh := 520.0, 420.0
	mx, my := g.seaWindow(screen, mw, mh, "")
	accent := color.Color(themeRed)
	if win {
		accent = themeGreen
	}
	strokeRoundRect(screen, mx, my, mw, mh, 18, 2, accent)
	drawText(screen, title, titleFace(34), mx+mw/2, my+18, accent, alignCenter)
	drawText(screen, "Combat contre un joueur", bodyFace(13, false), mx+mw/2, my+64, themeMuted, alignCenter)

	ccx, ccy := mx+mw-30, my+28
	drawCloseButton(screen, ccx, ccy, 16)

	cardY := my + 96
	cw := (mw - 44 - 12) / 2
	drawCard(screen, mx+22, cardY, cw, 70, false)
	drawCard(screen, mx+22+cw+12, cardY, cw, 70, false)
	drawLabel(screen, "Navires perdus", mx+36, cardY+12, 12, false, themeMuted)
	drawLabel(screen, fmt.Sprintf("%d", len(result.ShipsDestroyedA)), mx+36, cardY+32, 22, true, themeText)
	drawLabel(screen, "Navires ennemis coulés", mx+36+cw+12, cardY+12, 12, false, themeMuted)
	drawLabel(screen, fmt.Sprintf("%d", len(result.ShipsDestroyedB)), mx+36+cw+12, cardY+32, 22, true, themeText)

	if win && g.ui.PvpLoot != nil {
		ly := cardY + 92
		drawText(screen, "Butin", titleFace(22), mx+22, ly, themeGold, alignLeft)
		ly += 34
		for _, res := range []domain.ResourceType{domain.Wood, domain.Stone, domain.Gold, domain.Rum} {
			if amt := g.ui.PvpLoot[string(res)]; amt > 0 {
				drawLabel(screen, fmt.Sprintf("%s : +%s", costNames[res], formatThousands(amt)), mx+36, ly, 15, true, themeGreen)
				ly += 24
			}
		}
	}

	bw, bh := 150.0, 42.0
	bx, by := mx+(mw-bw)/2, my+mh-62
	drawButton(screen, bx, by, bw, bh, "Ok", btnPrimary)

	if isMouseJustPressed(ebiten.MouseButtonLeft) && (isHovered(bx, by, bw, bh) || isHovered(ccx-16, ccy-16, 32, 32)) {
		g.ui.ShowPvpResultUI = false
		if g.seaDMS != nil {
			g.seaDMS.SetMode(DMSCalm, "pvp_result_closed")
		}
		g.ui.PvpCombatResult = nil
		g.ui.PvpLoot = nil
	}
}
