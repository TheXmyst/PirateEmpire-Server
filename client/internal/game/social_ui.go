package game

import (
	"fmt"
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) ensureSocialDefaults() {
	if g.ui.SocialActiveTab == "" {
		g.ui.SocialActiveTab = "friends"
	}
	if g.ui.SocialLeaderboardTab == "" {
		g.ui.SocialLeaderboardTab = "players"
	}
	// Load leaderboards from server if empty
	// Une seule requête à la fois, et pas plus d'une tentative toutes les
	// 10 secondes si la liste reste vide (avant : une requête par image).
	if len(g.ui.SocialLeaderboardPlayers) == 0 && !g.ui.SocialSearchBusy &&
		!g.leaderboardBusy && time.Since(g.leaderboardTried) >= 10*time.Second {
		g.leaderboardBusy = true
		g.leaderboardTried = time.Now()
		go func() {
			defer func() { g.leaderboardBusy = false }()
			g.loadLeaderboards()
		}()
	}
	if g.ui.SocialGuildRank == 0 && g.ui.SocialGuildName != "" {
		g.ui.SocialGuildRank = 42
	}
	if !g.ui.SocialGuildHasTicket && g.ui.SocialGuildName == "" {
		// For test purposes the account has a ticket available until consumed
		g.ui.SocialGuildHasTicket = true
	}
}

func (g *Game) loadLeaderboards() {
	players, err := g.api.GetLeaderboardPlayers()
	if err == nil && len(players) > 0 {
		// Convert LeaderboardEntry to SocialProfile
		profiles := make([]SocialProfile, len(players))
		for i, p := range players {
			profiles[i] = SocialProfile{
				ID:         p.ID,
				Name:       p.Name,
				Reputation: p.Reputation,
				GuildName:  "", // Leaderboard doesn't include guild name for players
			}
		}
		g.ui.SocialLeaderboardPlayers = profiles
	}

	guilds, err := g.api.GetLeaderboardGuilds()
	if err == nil && len(guilds) > 0 {
		// Convert LeaderboardEntry to SocialGuildEntry
		entries := make([]SocialGuildEntry, len(guilds))
		for i, guild := range guilds {
			entries[i] = SocialGuildEntry{
				Name:        guild.Name,
				MemberCount: guild.Members,
				Reputation:  guild.Reputation,
				Rank:        guild.Rank,
			}
		}
		g.ui.SocialLeaderboardGuilds = entries
	}
}

func (g *Game) UpdateSocialUI() bool {
	if !g.ui.ShowSocialUI {
		return false
	}

	mx, my := cursorPosition()
	cx, cy := float64(mx), float64(my)
	w, h := float64(g.screenWidth), float64(g.screenHeight)

	panelW, panelH := 720.0, 520.0
	panelX, panelY := (w-panelW)/2, (h-panelH)/2

	// Close on ESC
	if isKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowSocialUI = false
		g.ui.SocialSearchFocus = false
		g.ui.SocialGuildNameFocus = false
		return true
	}

	// Handle clicks
	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		// Close button
		if pointInRect(cx, cy, panelX+panelW-40, panelY+10, 30, 30) {
			g.ui.ShowSocialUI = false
			g.ui.SocialSearchFocus = false
			g.ui.SocialGuildNameFocus = false
			return true
		}

		// Tabs
		tabW, tabH := 120.0, 32.0
		tabX := panelX + 20
		tabY := panelY + 14
		if pointInRect(cx, cy, tabX, tabY, tabW, tabH) {
			g.ui.SocialActiveTab = "friends"
		} else if pointInRect(cx, cy, tabX+tabW+10, tabY, tabW, tabH) {
			g.ui.SocialActiveTab = "guild"
		} else if pointInRect(cx, cy, tabX+2*(tabW+10), tabY, tabW, tabH) {
			g.ui.SocialActiveTab = "leaderboard"
		}

		// Content-specific clicks
		switch g.ui.SocialActiveTab {
		case "friends":
			searchX, searchY := panelX+20, panelY+70
			searchW, searchH := panelW-200, 32.0
			if pointInRect(cx, cy, searchX, searchY, searchW, searchH) {
				g.ui.SocialSearchFocus = true
				g.ui.SocialGuildNameFocus = false
			} else {
				g.ui.SocialSearchFocus = false
			}

			// Search button
			btnX := searchX + searchW + 10
			btnY := searchY
			if pointInRect(cx, cy, btnX, btnY, 120, searchH) {
				g.runSocialSearch()
			}

			// Results
			rowX := searchX
			rowY := searchY + 50
			rowW := panelW - 40
			rowH := 30.0
			ay, ah := socialSearchArea(panelY)
			for i, entry := range g.ui.SocialSearchResult {
				y := rowY + float64(i)*(rowH+6) - socialScroll.search
				if cy >= ay && cy <= ay+ah && pointInRect(cx, cy, rowX, y, rowW, rowH) {
					g.addFriend(entry)
					break
				}
			}

			// Friend list remove buttons
			friendListX := rowX
			friendListY := rowY + 170
			entryW := rowW - 100.0
			fy, fh := socialFriendsArea(panelY)
			for i, friend := range g.ui.SocialFriends {
				y := friendListY + float64(i)*(rowH+4) - socialScroll.friends
				removeX := friendListX + entryW + 4
				if cy >= fy && cy <= fy+fh && pointInRect(cx, cy, removeX, y, 80, rowH) {
					g.removeFriend(friend.ID)
					break
				}
			}
		case "guild":
			if !g.ui.SocialGuildHasTicket {
				g.ui.SocialGuildInfo = "Ticket manquant"
			}
			nameX, nameY := panelX+20, panelY+86
			nameW, nameH := panelW-200, 32.0
			if !g.ui.SocialGuildNameFocus && pointInRect(cx, cy, nameX, nameY, nameW, nameH) {
				g.ui.SocialGuildNameFocus = true
				g.ui.SocialSearchFocus = false
			} else if g.ui.SocialGuildNameFocus && !pointInRect(cx, cy, nameX, nameY, nameW, nameH) {
				g.ui.SocialGuildNameFocus = false
			}
			btnX := nameX + nameW + 10
			btnY := nameY
			if g.ui.SocialGuildHasTicket && pointInRect(cx, cy, btnX, btnY, 120, nameH) && !g.ui.SocialGuildBusy {
				g.createGuildFromInput()
			}

			// Leave Guild button (only when in guild)
			if g.ui.SocialGuildName != "" {
				leaveX := panelX + 20
				leaveY := panelY + 170
				if pointInRect(cx, cy, leaveX, leaveY, 120, 32) {
					g.leaveGuild()
				}
			}
		case "leaderboard":
			subTabX := panelX + 20
			subTabY := panelY + 62
			if pointInRect(cx, cy, subTabX, subTabY, 140, 30) {
				g.ui.SocialLeaderboardTab = "players"
			} else if pointInRect(cx, cy, subTabX+150, subTabY, 140, 30) {
				g.ui.SocialLeaderboardTab = "guilds"
			}
		}
	}

	// Text input handling
	for _, r := range ebiten.AppendInputChars(nil) {
		if g.ui.SocialSearchFocus {
			g.ui.SocialSearchQuery += string(r)
		} else if g.ui.SocialGuildNameFocus {
			g.ui.SocialGuildNameInput += string(r)
		}
	}

	if isKeyJustPressed(ebiten.KeyBackspace) {
		if g.ui.SocialSearchFocus && len(g.ui.SocialSearchQuery) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.SocialSearchQuery)
			g.ui.SocialSearchQuery = g.ui.SocialSearchQuery[:len(g.ui.SocialSearchQuery)-size]
		}
		if g.ui.SocialGuildNameFocus && len(g.ui.SocialGuildNameInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.SocialGuildNameInput)
			g.ui.SocialGuildNameInput = g.ui.SocialGuildNameInput[:len(g.ui.SocialGuildNameInput)-size]
		}
	}

	if isKeyJustPressed(ebiten.KeyEnter) {
		switch g.ui.SocialActiveTab {
		case "friends":
			if g.ui.SocialSearchFocus {
				g.runSocialSearch()
			}
		case "guild":
			if g.ui.SocialGuildNameFocus && !g.ui.SocialGuildHasTicket {
				g.ui.SocialGuildError = "Ticket requis"
			} else if g.ui.SocialGuildNameFocus {
				g.createGuildFromInput()
			}
		}
	}

	return true
}

func (g *Game) runSocialSearch() {
	q := strings.TrimSpace(g.ui.SocialSearchQuery)
	if q == "" {
		g.ui.SocialSearchResult = []SocialProfile{}
		g.ui.SocialStatus = "Entrez un nom pour rechercher"
		return
	}

	// Call API
	results, err := g.api.SearchFriends(q)
	if err != nil {
		g.ui.SocialStatus = "Erreur : " + friendlyError(err)
		return
	}

	// Convert APIClient.PlayerSummary to SocialProfile
	searchResults := make([]SocialProfile, 0, len(results))
	for _, p := range results {
		searchResults = append(searchResults, SocialProfile{
			ID:   p.ID,
			Name: p.Username,
		})
	}

	if len(searchResults) == 0 {
		g.ui.SocialStatus = "Aucun résultat"
	} else {
		g.ui.SocialStatus = fmt.Sprintf("%d résultat(s)", len(searchResults))
	}
	g.ui.SocialSearchResult = searchResults
}

func (g *Game) addFriend(p SocialProfile) {
	// Check if already friended locally
	for _, f := range g.ui.SocialFriends {
		if f.ID == p.ID {
			g.ui.SocialInfo = fmt.Sprintf("%s est déjà dans la liste", p.Name)
			return
		}
	}

	// Call API
	err := g.api.AddFriend(p.ID)
	if err != nil {
		g.ui.SocialInfo = "Erreur : " + friendlyError(err)
		return
	}

	g.ui.SocialFriends = append(g.ui.SocialFriends, p)
	g.ui.SocialInfo = fmt.Sprintf("%s ajouté(e) aux amis", p.Name)
	g.persistSocialState()
}

func (g *Game) createGuildFromInput() {
	name := strings.TrimSpace(g.ui.SocialGuildNameInput)
	if name == "" {
		g.ui.SocialGuildError = "Nom de guilde requis"
		return
	}

	// Call API
	guildInfo, err := g.api.CreateGuild(name)
	if err != nil {
		g.ui.SocialGuildError = "Erreur : " + friendlyError(err)
		return
	}

	g.ui.SocialGuildError = ""
	g.ui.SocialGuildInfo = "Guilde créée!"
	g.ui.SocialGuildNameInput = ""
	g.ui.SocialGuildName = guildInfo.Name
	g.ui.SocialGuildID = guildInfo.ID

	// Convert members
	memberProfiles := make([]SocialProfile, 0, len(guildInfo.Members))
	for _, m := range guildInfo.Members {
		memberProfiles = append(memberProfiles, SocialProfile{
			ID:   m.ID,
			Name: m.Username,
		})
	}

	g.ui.SocialGuildMembers = memberProfiles
	g.ui.SocialGuildHasTicket = false
	g.ui.SocialGuildMemberCount = guildInfo.MemberCount
	g.persistSocialState()
}

func (g *Game) DrawSocialUI(screen *ebiten.Image) {
	if !g.ui.ShowSocialUI {
		return
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	panelW, panelH := 720.0, 520.0
	panelX, panelY := (w-panelW)/2, (h-panelH)/2

	// Overlay
	uiFillRect(screen, 0, 0, float32(w), float32(h), alpha(10, 10, 20, 180), vecAA)

	// Panel
	scrollBlocker(panelX, panelY, panelW, panelH)
	draw9Slice(screen, g, panelX, panelY, panelW, panelH, 16)

	// Header and tabs
	tabW, tabH := 120.0, 32.0
	tabX := panelX + 20
	tabY := panelY + 14
	g.drawSocialTab(screen, tabX, tabY, tabW, tabH, "Amis", g.ui.SocialActiveTab == "friends")
	g.drawSocialTab(screen, tabX+tabW+10, tabY, tabW, tabH, "Guilde", g.ui.SocialActiveTab == "guild")
	g.drawSocialTab(screen, tabX+2*(tabW+10), tabY, tabW, tabH, "Classement", g.ui.SocialActiveTab == "leaderboard")

	// Close button
	closeX, closeY := panelX+panelW-40, panelY+10
	drawCloseButton(screen, float64(closeX)+float64(30)/2, float64(closeY)+float64(30)/2, 15)

	switch g.ui.SocialActiveTab {
	case "guild":
		g.drawGuildTab(screen, panelX, panelY, panelW)
	case "leaderboard":
		g.drawLeaderboardTab(screen, panelX, panelY, panelW)
	default:
		g.drawFriendsTab(screen, panelX, panelY, panelW)
	}
}

// Défilement des listes de la fenêtre Social (ui_scroll.go).
var socialScroll struct{ search, friends, guild, board float64 }

const socialPanelH = 520.0

// socialSearchArea / socialFriendsArea : zones des listes de l'onglet Amis.
func socialSearchArea(panelY float64) (y, h float64) { return panelY + 120, 132 }
func socialFriendsArea(panelY float64) (y, h float64) {
	return panelY + 290, panelY + socialPanelH - 14 - (panelY + 290)
}

func (g *Game) drawSocialTab(screen *ebiten.Image, x, y, w, h float64, label string, active bool) {
	drawTab(screen, x, y, w, h, label, active)
}

func (g *Game) drawFriendsTab(screen *ebiten.Image, panelX, panelY, panelW float64) {

	searchX, searchY := panelX+20, panelY+70
	searchW, searchH := panelW-200, 32.0
	registerTextField(textField{id: "social-search", x: searchX, y: searchY, w: searchW, h: searchH,
		value: &g.ui.SocialSearchQuery, focused: g.ui.SocialSearchFocus, label: "Chercher un joueur"})
	drawInputField(screen, searchX, searchY, searchW, searchH, g.ui.SocialSearchQuery, "Pseudo d'un joueur", g.ui.SocialSearchFocus)

	btnX := searchX + searchW + 10
	btnY := searchY
	drawButton(screen, btnX, btnY, 120, searchH, "Rechercher", btnPrimary)

	statusY := searchY + searchH + 6
	if g.ui.SocialStatus != "" {
		uiPrint(screen, g.ui.SocialStatus, int(searchX), int(statusY))
	}
	if g.ui.SocialInfo != "" {
		uiPrint(screen, g.ui.SocialInfo, int(searchX)+200, int(statusY))
	}

	// Results
	rowX := searchX
	rowY := searchY + 50
	rowW := panelW - 40
	rowH := 30.0
	if len(g.ui.SocialSearchResult) > 0 {
		drawText(screen, "Clique sur un joueur pour l'ajouter en ami", bodyFace(12, true), rowX, rowY-20, themeMuted, alignLeft)
	}
	{
		ay, ah := socialSearchArea(panelY)
		scrollRegion(screen, &socialScroll.search, rowX, ay, rowW, ah, float64(len(g.ui.SocialSearchResult))*(rowH+6))
		view := clipTo(screen, rowX, ay, rowW, ah)
		for i, entry := range g.ui.SocialSearchResult {
			y := rowY + float64(i)*(rowH+6) - socialScroll.search
			if y+rowH < ay || y > ay+ah {
				continue
			}
			line := fmt.Sprintf("%s  ·  Réputation %d  ·  Guilde %s", entry.Name, entry.Reputation, entry.GuildName)
			drawCard(view, rowX, y, rowW-12, rowH, false)
			drawLabel(view, line, rowX+10, y+8, 13, false, themeText)
		}
	}

	// Friend list
	listX := rowX
	listY := rowY + 170
	uiSubtitle(screen, "Mes amis", int(listX), int(listY)-30)
	if len(g.ui.SocialFriends) == 0 {
		drawText(screen, "Pas encore d'amis. Cherche un pseudo ci-dessus !", bodyFace(14, false), listX, listY+4, themeMuted, alignLeft)
		return
	}
	fy, fh := socialFriendsArea(panelY)
	scrollRegion(screen, &socialScroll.friends, listX, fy, rowW, fh, float64(len(g.ui.SocialFriends))*(rowH+4))
	full := screen
	for i, entry := range g.ui.SocialFriends {
		y := listY + float64(i)*(rowH+4) - socialScroll.friends
		if y+rowH < fy || y > fy+fh {
			continue
		}
		screen := clipTo(full, listX, fy, rowW, fh)
		entryW := rowW - 100.0
		line := fmt.Sprintf("%s  ·  Réputation %d  ·  Guilde %s", entry.Name, entry.Reputation, entry.GuildName)
		drawCard(screen, listX, y, entryW, rowH, false)
		drawLabel(screen, line, listX+10, y+8, 13, false, themeText)

		// Remove button
		removeX := listX + entryW + 4
		removeY := y
		removeW := 80.0
		drawButton(screen, removeX, removeY, removeW, rowH, "Retirer", btnDanger)
	}
}

func (g *Game) drawGuildTab(screen *ebiten.Image, panelX, panelY, panelW float64) {
	if g.ui.SocialGuildName == "" {
		drawText(screen, "Tu n'as pas encore de guilde. Fondes-en une :", bodyFace(14, false), panelX+20, panelY+62, themeText, alignLeft)
		nameX, nameY := panelX+20, panelY+86
		nameW, nameH := panelW-200, 32.0
		registerTextField(textField{id: "guild-name", x: nameX, y: nameY, w: nameW, h: nameH,
			value: &g.ui.SocialGuildNameInput, focused: g.ui.SocialGuildNameFocus, label: "Nom de guilde"})
		drawInputField(screen, nameX, nameY, nameW, nameH, g.ui.SocialGuildNameInput, "Nom de la guilde", g.ui.SocialGuildNameFocus)

		btnX := nameX + nameW + 10
		btnY := nameY
		kind, label := btnPrimary, "Fonder"
		if !g.ui.SocialGuildHasTicket {
			kind, label = btnDisabled, "Ticket requis"
		}
		drawButton(screen, btnX, btnY, 120, nameH, label, kind)

		infoY := nameY + nameH + 10
		if g.ui.SocialGuildError != "" {
			uiPrint(screen, g.ui.SocialGuildError, int(nameX), int(infoY))
		} else if g.ui.SocialGuildInfo != "" {
			uiPrint(screen, g.ui.SocialGuildInfo, int(nameX), int(infoY))
		} else if g.ui.SocialGuildHasTicket {
			drawText(screen, "Tu as un ticket de guilde : fonder une guilde le consomme.", bodyFace(13, false), nameX, infoY, themeMuted, alignLeft)
		}
		return
	}

	// Guild summary
	infoY := panelY + 90
	drawText(screen, g.ui.SocialGuildName, titleFace(28), panelX+20, infoY-30, themeGold, alignLeft)
	drawText(screen, fmt.Sprintf("Membres %d / 15 · Réputation %d · Rang #%d", len(g.ui.SocialGuildMembers), g.ui.SocialGuildReputation, g.ui.SocialGuildRank), bodyFace(14, false), panelX+20, infoY+10, themeText, alignLeft)

	// Leave Guild button
	leaveX := panelX + 20
	leaveY := infoY + 80
	drawButton(screen, leaveX, leaveY, 120, 32, "Quitter", btnDanger)

	listY := leaveY + 40
	uiSubtitle(screen, "Membres", int(panelX)+20, int(listY)-22)
	rowH := 28.0
	rowW := panelW - 40
	listH := panelY + socialPanelH - 14 - listY
	scrollRegion(screen, &socialScroll.guild, panelX+20, listY, rowW, listH, float64(len(g.ui.SocialGuildMembers))*(rowH+4))
	full := screen
	rowW -= 12
	for i, m := range g.ui.SocialGuildMembers {
		y := listY + float64(i)*(rowH+4) - socialScroll.guild
		if y+rowH < listY || y > listY+listH {
			continue
		}
		screen := clipTo(full, panelX+20, listY, rowW+12, listH)
		drawCard(screen, panelX+20, y, rowW, rowH, false)
		drawText(screen, m.Name, bodyFace(14, true), panelX+32, y+5, themeText, alignLeft)
		drawText(screen, fmt.Sprintf("Réputation %d", m.Reputation), bodyFace(13, false), panelX+20+rowW-12, y+6, themeMuted, alignRight)
	}
}

func (g *Game) drawLeaderboardTab(screen *ebiten.Image, panelX, panelY, panelW float64) {
	subTabX := panelX + 20
	subTabY := panelY + 62
	g.drawSocialTab(screen, subTabX, subTabY, 140, 30, "Joueurs", g.ui.SocialLeaderboardTab == "players")
	g.drawSocialTab(screen, subTabX+150, subTabY, 140, 30, "Guildes", g.ui.SocialLeaderboardTab == "guilds")

	listY := subTabY + 50
	rowH := 30.0
	rowW := panelW - 40
	listH := panelY + socialPanelH - 14 - listY
	n := len(g.ui.SocialLeaderboardPlayers)
	if g.ui.SocialLeaderboardTab == "guilds" {
		n = len(g.ui.SocialLeaderboardGuilds)
	}
	scrollRegion(screen, &socialScroll.board, panelX+20, listY, rowW, listH, float64(n)*(rowH+4))
	full := screen
	screen = clipTo(full, panelX+20, listY, rowW, listH)
	listY -= socialScroll.board
	rowW -= 12

	switch g.ui.SocialLeaderboardTab {
	case "guilds":
		for i, gentry := range g.ui.SocialLeaderboardGuilds {
			y := listY + float64(i)*(rowH+4)
			drawLeaderRow(screen, panelX+20, y, rowW, rowH, gentry.Rank, gentry.Name, fmt.Sprintf("%d membres · Réputation %d", gentry.MemberCount, gentry.Reputation))
		}
	default:
		for i, p := range g.ui.SocialLeaderboardPlayers {
			y := listY + float64(i)*(rowH+4)
			detail := fmt.Sprintf("Réputation %d", p.Reputation)
			if p.GuildName != "" {
				detail = p.GuildName + " · " + detail
			}
			drawLeaderRow(screen, panelX+20, y, rowW, rowH, i+1, p.Name, detail)
		}
	}
}

func (g *Game) removeFriend(friendID uuid.UUID) {
	g.ui.SocialSearchBusy = true
	go func() {
		defer func() { g.ui.SocialSearchBusy = false }()
		err := g.api.RemoveFriend(friendID)
		if err != nil {
			g.ui.SocialStatus = "Erreur : " + friendlyError(err)
			return
		}

		// Remove from local list
		filtered := []SocialProfile{}
		for _, f := range g.ui.SocialFriends {
			if f.ID != friendID {
				filtered = append(filtered, f)
			}
		}
		g.ui.SocialFriends = filtered
		g.ui.SocialStatus = "Ami supprimé"
		g.persistSocialState()
	}()
}

func (g *Game) leaveGuild() {
	g.ui.SocialGuildBusy = true
	go func() {
		defer func() { g.ui.SocialGuildBusy = false }()
		err := g.api.LeaveGuild()
		if err != nil {
			g.ui.SocialStatus = "Erreur : " + friendlyError(err)
			return
		}

		// Reset guild state
		g.ui.SocialGuildName = ""
		g.ui.SocialGuildMembers = []SocialProfile{}
		g.ui.SocialGuildID = uuid.UUID{}
		g.ui.SocialGuildMemberCount = 0
		g.ui.SocialGuildReputation = 0
		g.ui.SocialGuildRank = 0
		g.ui.SocialGuildHasTicket = true
		g.ui.SocialGuildInfo = "Guilde quittée"
		g.persistSocialState()
	}()
}

func pointInRect(px, py, x, y, w, h float64) bool {
	return px >= x && px <= x+w && py >= y && py <= y+h
}

// drawLeaderRow : une ligne de classement (rang, nom, détail à droite).
func drawLeaderRow(screen *ebiten.Image, x, y, w, h float64, rank int, name, detail string) {
	drawCard(screen, x, y, w, h, false)
	clr := color.Color(themeMuted)
	if rank <= 3 {
		clr = themeGold
	}
	drawText(screen, fmt.Sprintf("#%d", rank), bodyFace(14, true), x+12, y+6, clr, alignLeft)
	drawText(screen, name, bodyFace(14, true), x+56, y+6, themeText, alignLeft)
	drawText(screen, detail, bodyFace(13, false), x+w-12, y+7, themeMuted, alignRight)
}
