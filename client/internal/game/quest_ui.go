package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

// quest_ui.go : le journal de quêtes.
//   - l'encart « Quêtes » en haut à gauche de l'île (quête Histoire en cours,
//     pastille quand une récompense attend) ; un clic ouvre le journal ;
//   - le journal : trois onglets (Histoire, Bâtiments, Défis), une carte par
//     quête avec sa barre d'avancement et le bouton Réclamer ;
//   - la fiche « Guide » de certaines quêtes (gacha, matelots, flotte…).
// Les quêtes viennent du serveur (GET /quests), rafraîchies toutes les 15 s.

const questPollEvery = 15 * time.Second

var questTabs = []struct{ id, label string }{
	{"story", "Histoire"},
	{"building", "Bâtiments"},
	{"challenge", "Défis"},
}

type questUI struct {
	data      *client.QuestsResponse
	playerID  uuid.UUID
	busy      bool // une requête est en cours
	lastFetch time.Time

	open      bool
	tab       string
	scrollY   float64
	err       string
	tutorial  *client.Quest // fiche Guide ouverte
	toast     string
	toastTill time.Time
}

// ---------------------------------------------------------------------------
// Données

// updateQuests : charge les quêtes au besoin (appelé à chaque image).
func (g *Game) updateQuests() {
	if g.player == nil || g.state == StateLogin {
		return
	}
	q := &g.quest
	if q.playerID != g.player.ID {
		*q = questUI{playerID: g.player.ID}
	}
	if !q.busy && time.Since(q.lastFetch) >= questPollEvery {
		g.fetchQuests()
	}
}

func (g *Game) fetchQuests() {
	q := &g.quest
	q.busy = true
	q.lastFetch = time.Now()
	go func() {
		defer func() { q.busy = false }()
		data, err := g.api.GetQuests()
		if err != nil {
			g.Log("[QUESTS] chargement impossible : %v", err)
			return
		}
		q.data = data
	}()
}

func (g *Game) claimQuest(id string) {
	q := &g.quest
	if q.busy {
		return
	}
	q.busy = true
	q.err = ""
	go func() {
		defer func() { q.busy = false }()
		data, err := g.api.ClaimQuest(id)
		if err != nil {
			q.err = friendlyError(err)
			return
		}
		q.data = data
		q.lastFetch = time.Now()
		if txt := rewardText(data.Granted); txt != "" {
			q.toast = "Récompense reçue : " + txt
			if data.Message != "" {
				q.toast += ". " + data.Message
			}
			q.toastTill = time.Now().Add(4 * time.Second)
		}
		if p, err := g.api.GetStatus(); err == nil {
			g.updateChan <- p
		}
	}()
}

// questsOf : les quêtes d'un onglet, à réclamer d'abord, verrouillées à la fin.
func (q *questUI) questsOf(tab string) []client.Quest {
	if q.data == nil {
		return nil
	}
	var out []client.Quest
	for _, x := range q.data.Quests {
		if x.Family == tab {
			out = append(out, x)
		}
	}
	rank := func(x client.Quest) int {
		switch {
		case x.Done:
			return 0
		case x.Locked:
			return 2
		}
		return 1
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

func (q *questUI) claimableIn(tab string) int {
	n := 0
	for _, x := range q.questsOf(tab) {
		if x.Done {
			n++
		}
	}
	return n
}

func (q *questUI) claimable() int {
	if q.data == nil {
		return 0
	}
	return q.data.Claimable
}

// currentStory : la quête Histoire en cours (nil si tout est fini).
func (q *questUI) currentStory() *client.Quest {
	for _, x := range q.questsOf("story") {
		x := x
		return &x
	}
	return nil
}

func rewardText(r client.QuestReward) string {
	var parts []string
	add := func(v float64, name string) {
		if v > 0 {
			parts = append(parts, "+"+formatThousands(v)+" "+name)
		}
	}
	add(r.Wood, "bois")
	add(r.Stone, "pierre")
	add(r.Gold, "or")
	add(r.Rum, "rhum")
	if r.Tickets > 0 {
		s := fmt.Sprintf("+%d ticket", r.Tickets)
		if r.Tickets > 1 {
			s += "s"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func progressLabel(x client.Quest) string {
	return formatThousands(math.Min(x.Progress, x.Target)) + " / " + formatThousands(x.Target)
}

// ---------------------------------------------------------------------------
// Encart sur l'île

func (g *Game) questTrackerVisible() bool {
	return g.state == StatePlaying && g.hudVisible() && g.quest.data != nil
}

func (g *Game) questTrackerRect() hudRect {
	return hudRect{hudMargin, hudMargin + topBarH + 10, 270, 84}
}

// questTrackerBottom : où peut commencer le panneau suivant (« En cours »).
func (g *Game) questTrackerBottom() float64 {
	if !g.questTrackerVisible() {
		return hudMargin + topBarH + 10
	}
	r := g.questTrackerRect()
	return r.y + r.h + 10
}

func (g *Game) drawQuestTracker(screen *ebiten.Image) {
	if !g.questTrackerVisible() {
		return
	}
	q := &g.quest
	r := g.questTrackerRect()
	drawThemePanel(screen, r.x, r.y, r.w, r.h, 14)
	if !g.IsModalOpen() && r.has(cursorPositionF()) {
		fillRoundRect(screen, r.x, r.y, r.w, r.h, 14, themePanelHi)
	}
	drawIcon(screen, "scroll", r.x+22, r.y+20, 26, themeGold)
	drawText(screen, "Quêtes", titleFace(20), r.x+40, r.y+6, themeGold, alignLeft)
	if n := q.claimable(); n > 0 {
		drawBadge(screen, r.x+r.w-4, r.y+4, n)
	}

	f := bodyFace(12, true)
	cur := q.currentStory()
	switch {
	case cur != nil:
		drawText(screen, ellipsize(cur.Title, f, r.w-24), f, r.x+12, r.y+36, themeText, alignLeft)
		bx, by, bw := r.x+12, r.y+60, r.w-24-70
		p := 0.0
		if cur.Target > 0 {
			p = math.Min(1, cur.Progress/cur.Target)
		}
		fillRoundRect(screen, bx, by, bw, 8, 4, color.NRGBA{0, 0, 0, 110})
		drawProgressBar(screen, bx, by, bw, 8, p, themeGold)
		lbl := progressLabel(*cur)
		clr := color.Color(themeMuted)
		if cur.Done {
			lbl, clr = "Terminée !", themeGreen
		}
		drawText(screen, lbl, bodyFace(11, true), r.x+r.w-12, by-5, clr, alignRight)
	case q.claimable() > 0:
		drawText(screen, "Des récompenses t'attendent !", f, r.x+12, r.y+40, themeGreen, alignLeft)
	default:
		drawText(screen, "Histoire terminée, bravo capitaine !", f, r.x+12, r.y+40, themeMuted, alignLeft)
	}
}

// ---------------------------------------------------------------------------
// Journal : géométrie partagée par l'affichage et les clics

type questCard struct {
	quest           client.Quest
	r               hudRect
	claim, guide    hudRect
	hasClaim, hasGd bool
}

type questLayout struct {
	win, close, list, claimAll hudRect
	tabs                       []hudRect
	cards                      []questCard
	contentH                   float64
}

const questCardH, questCardGap = 96.0, 10.0

func (g *Game) questLayout() questLayout {
	w := math.Min(780, float64(g.screenWidth)-40)
	h := math.Min(580, float64(g.screenHeight)-40)
	L := questLayout{win: hudRect{float64(g.screenWidth)/2 - w/2, float64(g.screenHeight)/2 - h/2, w, h}}
	x, y := L.win.x, L.win.y
	L.close = hudRect{x + w - 46, y + 11, 32, 32}
	tx := x + 20
	for range questTabs {
		L.tabs = append(L.tabs, hudRect{tx, y + 62, 150, 34})
		tx += 160
	}
	L.claimAll = hudRect{x + w - 20 - 170, y + 62, 170, 34}
	L.list = hudRect{x + 20, y + 108, w - 40, h - 108 - 20}

	cy := L.list.y + 10 - g.quest.scrollY
	for _, qq := range g.quest.questsOf(g.quest.tab) {
		c := questCard{quest: qq, r: hudRect{L.list.x + 10, cy, L.list.w - 20, questCardH}}
		bx := c.r.x + c.r.w - 14 - 120
		if qq.Done {
			c.claim, c.hasClaim = hudRect{bx, cy + questCardH - 14 - 36, 120, 36}, true
		}
		if qq.Tutorial != "" {
			c.guide, c.hasGd = hudRect{bx, cy + 12, 120, 28}, true
		}
		L.cards = append(L.cards, c)
		cy += questCardH + questCardGap
	}
	L.contentH = cy + g.quest.scrollY - L.list.y
	return L
}

// ---------------------------------------------------------------------------
// Journal : affichage

func (g *Game) drawQuestUI(screen *ebiten.Image) {
	q := &g.quest
	defer g.drawQuestToast(screen) // par-dessus le journal
	if !q.open {
		return
	}
	L := g.questLayout()
	drawDimBackground(screen, 150)
	drawWindowFrame(screen, L.win.x, L.win.y, L.win.w, L.win.h, "Journal de quêtes")
	drawCloseButton(screen, L.close.x+16, L.close.y+16, 16)

	// Avancement total
	if q.data != nil {
		done, total := 0, 0
		for _, s := range q.data.Summary {
			done += s.Claimed
			total += s.Total
		}
		drawText(screen, fmt.Sprintf("%d / %d quêtes accomplies", done, total), bodyFace(13, false),
			L.close.x-14, L.win.y+20, themeMuted, alignRight)
	}

	for i, t := range questTabs {
		r := L.tabs[i]
		label := t.label
		if q.data != nil {
			if s, ok := q.data.Summary[t.id]; ok {
				label = fmt.Sprintf("%s  %d/%d", t.label, s.Claimed, s.Total)
			}
		}
		drawTab(screen, r.x, r.y, r.w, r.h, label, q.tab == t.id)
		if n := q.claimableIn(t.id); n > 0 {
			drawBadge(screen, r.x+r.w-4, r.y+2, n)
		}
	}
	if n := q.claimable(); n > 0 {
		kind := btnPrimary
		if q.busy {
			kind = btnDisabled
		}
		drawButton(screen, L.claimAll.x, L.claimAll.y, L.claimAll.w, L.claimAll.h, fmt.Sprintf("Tout réclamer (%d)", n), kind)
	}

	drawInset(screen, L.list.x, L.list.y, L.list.w, L.list.h)
	clip := screen.SubImage(image.Rect(int(L.list.x), int(L.list.y+2), int(L.list.x+L.list.w), int(L.list.y+L.list.h-2))).(*ebiten.Image)
	if q.data == nil {
		drawText(screen, "Chargement des quêtes…", bodyFace(15, false), L.list.x+L.list.w/2, L.list.y+40, themeMuted, alignCenter)
	} else if len(L.cards) == 0 {
		msg := "Rien pour l'instant : continue de développer ton île !"
		if q.tab == "story" {
			msg = "Tu as terminé toute l'Histoire. Bravo, capitaine !"
		}
		drawText(screen, msg, bodyFace(15, false), L.list.x+L.list.w/2, L.list.y+40, themeMuted, alignCenter)
	}
	for _, c := range L.cards {
		if c.r.y+c.r.h < L.list.y || c.r.y > L.list.y+L.list.h {
			continue
		}
		g.drawQuestCard(clip, c, q.busy)
	}

	// Barre de défilement
	if L.contentH > L.list.h {
		track := L.list.h - 20
		barH := math.Max(40, track*L.list.h/L.contentH)
		maxScroll := L.contentH - L.list.h
		by := L.list.y + 10 + (track-barH)*(q.scrollY/maxScroll)
		fillRoundRect(screen, L.list.x+L.list.w-8, by, 4, barH, 2, themeGoldDim)
	}

	if q.err != "" {
		drawText(screen, ellipsize(q.err, bodyFace(13, true), L.list.w), bodyFace(13, true), L.list.x+L.list.w/2, L.win.y+L.win.h-18, themeOrange, alignCenter)
	}

	if q.tutorial != nil {
		g.drawQuestTutorial(screen, q.tutorial)
	}
}

func (g *Game) drawQuestCard(screen *ebiten.Image, c questCard, busy bool) {
	x := c.quest
	r := c.r
	drawCard(screen, r.x, r.y, r.w, r.h, x.Done)
	textW := r.w - 28 - 140
	title := bodyFace(15, true)
	tclr := color.Color(themeText)
	if x.Done {
		tclr = themeGold
	} else if x.Locked {
		tclr = themeMuted
	}
	drawText(screen, ellipsize(x.Title, title, textW), title, r.x+14, r.y+10, tclr, alignLeft)

	desc := x.Description
	dclr := color.Color(themeMuted)
	if x.Locked && x.Hint != "" {
		desc, dclr = "Verrouillé : "+x.Hint, themeOrange
	}
	body := bodyFace(12, false)
	lines := wrapText(desc, body, textW)
	for i, l := range lines {
		if i == 2 {
			break
		}
		if i == 1 && len(lines) > 2 {
			l = ellipsize(l+"…", body, textW)
		}
		drawText(screen, l, body, r.x+14, r.y+32+float64(i)*16, dclr, alignLeft)
	}

	// Avancement + récompense
	by := r.y + r.h - 22
	if !x.Done {
		p := 0.0
		if x.Target > 0 {
			p = math.Min(1, x.Progress/x.Target)
		}
		fillRoundRect(screen, r.x+14, by+3, 140, 8, 4, color.NRGBA{0, 0, 0, 110})
		drawProgressBar(screen, r.x+14, by+3, 140, 8, p, themeGold)
		drawText(screen, progressLabel(x), bodyFace(11, true), r.x+162, by-2, themeMuted, alignLeft)
	} else {
		drawText(screen, "Objectif atteint !", bodyFace(12, true), r.x+14, by-3, themeGreen, alignLeft)
	}
	g.drawQuestReward(screen, x.Reward, r.x+14+250, by-4, r.x+r.w-150)

	if c.hasGd {
		drawButton(screen, c.guide.x, c.guide.y, c.guide.w, c.guide.h, "Guide", btnSecondary)
	}
	if c.hasClaim {
		kind := btnPrimary
		if busy {
			kind = btnDisabled
		}
		drawButton(screen, c.claim.x, c.claim.y, c.claim.w, c.claim.h, "Réclamer", kind)
	} else if x.Locked {
		drawText(screen, "Verrouillée", bodyFace(12, true), r.x+r.w-14-60, r.y+r.h-36, themeMuted, alignCenter)
	}
}

// drawQuestReward : icônes et montants de la récompense, sur une ligne.
func (g *Game) drawQuestReward(screen *ebiten.Image, rw client.QuestReward, x, y, maxX float64) {
	f := bodyFace(12, true)
	items := []struct {
		v    float64
		icon *ebiten.Image
	}{{rw.Wood, g.iconWood}, {rw.Stone, g.iconStone}, {rw.Gold, g.iconGold}, {rw.Rum, g.iconRum}}
	for _, it := range items {
		if it.v <= 0 {
			continue
		}
		s := formatThousands(it.v)
		w := 20 + textWidth(s, f) + 10
		if x+w > maxX {
			break
		}
		drawImgScaled(screen, it.icon, x, y, 18, 18)
		drawText(screen, s, f, x+20, y+1, themeText, alignLeft)
		x += w
	}
	if rw.Tickets > 0 && x < maxX {
		s := fmt.Sprintf("+%d ticket", rw.Tickets)
		if rw.Tickets > 1 {
			s += "s"
		}
		drawText(screen, s, f, x, y+1, themeGold, alignLeft)
	}
}

func (g *Game) questTutorialRect() (win, ok hudRect) {
	w := math.Min(600, float64(g.screenWidth)-60)
	h := math.Min(400, float64(g.screenHeight)-60)
	win = hudRect{float64(g.screenWidth)/2 - w/2, float64(g.screenHeight)/2 - h/2, w, h}
	ok = hudRect{win.x + w/2 - 90, win.y + h - 62, 180, 44}
	return
}

func (g *Game) drawQuestTutorial(screen *ebiten.Image, x *client.Quest) {
	win, ok := g.questTutorialRect()
	drawDimBackground(screen, 120)
	drawWindowFrame(screen, win.x, win.y, win.w, win.h, "Guide : "+x.Title)
	f := bodyFace(14, false)
	ty := win.y + 70
	for _, l := range wrapText(x.Tutorial, f, win.w-56) {
		if ty > ok.y-28 {
			break
		}
		drawText(screen, l, f, win.x+28, ty, themeText, alignLeft)
		ty += 21
	}
	drawButton(screen, ok.x, ok.y, ok.w, ok.h, "Compris !", btnPrimary)
}

func (g *Game) drawQuestToast(screen *ebiten.Image) {
	q := &g.quest
	if q.toast == "" || time.Now().After(q.toastTill) || g.state == StateLogin {
		return
	}
	f := bodyFace(14, true)
	msg := ellipsize(q.toast, f, float64(g.screenWidth)-80)
	w := textWidth(msg, f) + 40
	x := float64(g.screenWidth)/2 - w/2
	y := hudMargin + topBarH + 12
	drawThemePanel(screen, x, y, w, 40, 20)
	drawText(screen, msg, f, x+20, y+11, themeGreen, alignLeft)
}

// ---------------------------------------------------------------------------
// Clics

func cursorPositionF() (float64, float64) {
	mx, my := cursorPosition()
	return float64(mx), float64(my)
}

func (g *Game) openQuestUI() {
	g.closeMainPanels()
	q := &g.quest
	q.open = true
	q.err = ""
	q.scrollY = 0
	if q.tab == "" {
		q.tab = "story"
	}
	// Onglet par défaut : celui où une récompense attend.
	if q.claimableIn(q.tab) == 0 {
		for _, t := range questTabs {
			if q.claimableIn(t.id) > 0 {
				q.tab = t.id
				break
			}
		}
	}
	if !q.busy {
		g.fetchQuests()
	}
}

// UpdateQuestUI : clics de l'encart et du journal. Renvoie true si l'entrée
// a été utilisée.
func (g *Game) UpdateQuestUI() bool {
	q := &g.quest
	if !q.open {
		// Clic sur l'encart de l'île.
		if g.questTrackerVisible() && !g.IsModalOpen() && isMouseJustPressed(ebiten.MouseButtonLeft) {
			if g.questTrackerRect().has(cursorPositionF()) {
				g.openQuestUI()
				return true
			}
		}
		return false
	}

	// Fiche Guide par-dessus le journal.
	if q.tutorial != nil {
		if isKeyJustPressed(ebiten.KeyEscape) {
			q.tutorial = nil
			return true
		}
		if isMouseJustPressed(ebiten.MouseButtonLeft) {
			win, ok := g.questTutorialRect()
			mx, my := cursorPositionF()
			if ok.has(mx, my) || !win.has(mx, my) {
				q.tutorial = nil
			}
		}
		return true
	}

	if isKeyJustPressed(ebiten.KeyEscape) {
		q.open = false
		return true
	}

	L := g.questLayout()
	if _, wy := wheelDelta(); wy != 0 {
		maxScroll := math.Max(0, L.contentH-L.list.h)
		q.scrollY = math.Max(0, math.Min(maxScroll, q.scrollY-wy*40))
	}
	if !isMouseJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	mx, my := cursorPositionF()
	switch {
	case L.close.has(mx, my) || !L.win.has(mx, my):
		q.open = false
		return true
	case q.claimable() > 0 && L.claimAll.has(mx, my):
		g.claimQuest("")
		return true
	}
	for i, t := range questTabs {
		if L.tabs[i].has(mx, my) {
			q.tab, q.scrollY = t.id, 0
			return true
		}
	}
	if !L.list.has(mx, my) {
		return true
	}
	for _, c := range L.cards {
		if c.hasClaim && c.claim.has(mx, my) {
			g.claimQuest(c.quest.ID)
			return true
		}
		if c.hasGd && c.guide.has(mx, my) {
			qq := c.quest
			q.tutorial = &qq
			return true
		}
	}
	return true
}
