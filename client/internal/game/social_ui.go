package game

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)



func (g *Game) ensureSocialDefaults() {
	if g.ui.SocialActiveTab == "" {
		g.ui.SocialActiveTab = "friends"
	}
	if g.ui.SocialLeaderboardTab == "" {
		g.ui.SocialLeaderboardTab = "players"
	}
	// Load leaderboards from server if empty
	if len(g.ui.SocialLeaderboardPlayers) == 0 && !g.ui.SocialSearchBusy {
		go g.loadLeaderboards()
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

	mx, my := ebiten.CursorPosition()
	cx, cy := float64(mx), float64(my)
	w, h := float64(g.screenWidth), float64(g.screenHeight)

	panelW, panelH := 720.0, 520.0
	panelX, panelY := (w-panelW)/2, (h-panelH)/2

	// Close on ESC
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ShowSocialUI = false
		g.ui.SocialSearchFocus = false
		g.ui.SocialGuildNameFocus = false
		return true
	}

	// Handle clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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
			for i, entry := range g.ui.SocialSearchResult {
				y := rowY + float64(i)*(rowH+6)
				if pointInRect(cx, cy, rowX, y, rowW, rowH) {
					g.addFriend(entry)
					break
				}
			}

			// Friend list remove buttons
			friendListX := rowX
			friendListY := rowY + 170
			entryW := rowW - 90.0
			for i, friend := range g.ui.SocialFriends {
				y := friendListY + float64(i)*(rowH+4)
				removeX := friendListX + entryW + 4
				if pointInRect(cx, cy, removeX, y, 80, rowH) {
					g.removeFriend(friend.ID)
					break
				}
			}
		case "guild":
			if !g.ui.SocialGuildHasTicket {
				g.ui.SocialGuildInfo = "Ticket manquant"
			}
			nameX, nameY := panelX+20, panelY+80
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
			subTabY := panelY + 70
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

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if g.ui.SocialSearchFocus && len(g.ui.SocialSearchQuery) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.SocialSearchQuery)
			g.ui.SocialSearchQuery = g.ui.SocialSearchQuery[:len(g.ui.SocialSearchQuery)-size]
		}
		if g.ui.SocialGuildNameFocus && len(g.ui.SocialGuildNameInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.SocialGuildNameInput)
			g.ui.SocialGuildNameInput = g.ui.SocialGuildNameInput[:len(g.ui.SocialGuildNameInput)-size]
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
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
		g.ui.SocialStatus = fmt.Sprintf("Erreur: %v", err)
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
		g.ui.SocialInfo = fmt.Sprintf("Erreur: %v", err)
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
		g.ui.SocialGuildError = fmt.Sprintf("Erreur: %v", err)
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
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), alpha(10, 10, 20, 180), true)

	// Panel
	draw9Slice(screen, g, panelX, panelY, panelW, panelH, 16)

	// Header and tabs
	tabW, tabH := 120.0, 32.0
	tabX := panelX + 20
	tabY := panelY + 14
	g.drawSocialTab(screen, tabX, tabY, tabW, tabH, "Amis", g.ui.SocialActiveTab == "friends")
	g.drawSocialTab(screen, tabX+tabW+10, tabY, tabW, tabH, "Guilde", g.ui.SocialActiveTab == "guild")
	g.drawSocialTab(screen, tabX+2*(tabW+10), tabY, tabW, tabH, "Leaderboard", g.ui.SocialActiveTab == "leaderboard")

	// Close button
	closeX, closeY := panelX+panelW-40, panelY+10
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), 30, 30, alpha(120, 40, 40, 230), true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeX)+10, int(closeY)+8)

	switch g.ui.SocialActiveTab {
	case "guild":
		g.drawGuildTab(screen, panelX, panelY, panelW)
	case "leaderboard":
		g.drawLeaderboardTab(screen, panelX, panelY, panelW)
	default:
		g.drawFriendsTab(screen, panelX, panelY, panelW)
	}
}

func (g *Game) drawSocialTab(screen *ebiten.Image, x, y, w, h float64, label string, active bool) {
	bg := alpha(20, 40, 60, 200)
	if active {
		bg = alpha(30, 90, 140, 230)
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), bg, true)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, alpha(200, 180, 120, 220), true)
	ebitenutil.DebugPrintAt(screen, label, int(x)+12, int(y)+8)
}

func (g *Game) drawFriendsTab(screen *ebiten.Image, panelX, panelY, panelW float64) {
	titleY := panelY + 56
	ebitenutil.DebugPrintAt(screen, "AMIS", int(panelX)+20, int(titleY))

	searchX, searchY := panelX+20, panelY+70
	searchW, searchH := panelW-200, 32.0
	vector.DrawFilledRect(screen, float32(searchX), float32(searchY), float32(searchW), float32(searchH), alpha(15, 15, 20, 210), true)
	vector.StrokeRect(screen, float32(searchX), float32(searchY), float32(searchW), float32(searchH), 2, alpha(180, 160, 100, 200), true)
	ebitenutil.DebugPrintAt(screen, g.ui.SocialSearchQuery, int(searchX)+8, int(searchY)+8)
	if g.ui.SocialSearchFocus {
		ebitenutil.DebugPrintAt(screen, "|", int(searchX)+8+len(g.ui.SocialSearchQuery)*7, int(searchY)+8)
	}

	btnX := searchX + searchW + 10
	btnY := searchY
	vector.DrawFilledRect(screen, float32(btnX), float32(btnY), 120, float32(searchH), alpha(40, 120, 60, 230), true)
	ebitenutil.DebugPrintAt(screen, "Recherche", int(btnX)+8, int(btnY)+8)

	statusY := searchY + searchH + 6
	if g.ui.SocialStatus != "" {
		ebitenutil.DebugPrintAt(screen, g.ui.SocialStatus, int(searchX), int(statusY))
	}
	if g.ui.SocialInfo != "" {
		ebitenutil.DebugPrintAt(screen, g.ui.SocialInfo, int(searchX)+200, int(statusY))
	}

	// Results
	rowX := searchX
	rowY := searchY + 50
	rowW := panelW - 40
	rowH := 30.0
	ebitenutil.DebugPrintAt(screen, "Résultats (cliquer pour ajouter)", int(rowX), int(rowY)-18)
	for i, entry := range g.ui.SocialSearchResult {
		y := rowY + float64(i)*(rowH+6)
		vector.DrawFilledRect(screen, float32(rowX), float32(y), float32(rowW), float32(rowH), alpha(20, 30, 40, 200), true)
		vector.StrokeRect(screen, float32(rowX), float32(y), float32(rowW), float32(rowH), 1, alpha(120, 120, 90, 180), true)
		line := fmt.Sprintf("%s  | Réputation %d  | Guilde %s", entry.Name, entry.Reputation, entry.GuildName)
		ebitenutil.DebugPrintAt(screen, line, int(rowX)+8, int(y)+8)
	}

	// Friend list
	listX := rowX
	listY := rowY + 170
	ebitenutil.DebugPrintAt(screen, "Mes amis", int(listX), int(listY)-18)
	if len(g.ui.SocialFriends) == 0 {
		ebitenutil.DebugPrintAt(screen, "(vide)", int(listX), int(listY))
		return
	}
	for i, entry := range g.ui.SocialFriends {
		y := listY + float64(i)*(rowH+4)
		entryW := rowW - 90.0
		vector.DrawFilledRect(screen, float32(listX), float32(y), float32(entryW), float32(rowH), alpha(25, 35, 50, 200), true)
		line := fmt.Sprintf("%s  | Réputation %d  | Guilde %s", entry.Name, entry.Reputation, entry.GuildName)
		ebitenutil.DebugPrintAt(screen, line, int(listX)+8, int(y)+8)

		// Remove button
		removeX := listX + entryW + 4
		removeY := y
		removeW := 80.0
		vector.DrawFilledRect(screen, float32(removeX), float32(removeY), float32(removeW), float32(rowH), alpha(180, 60, 60, 220), true)
		ebitenutil.DebugPrintAt(screen, "Retirer", int(removeX)+6, int(removeY)+8)
	}
}

func (g *Game) drawGuildTab(screen *ebiten.Image, panelX, panelY, panelW float64) {
	titleY := panelY + 56
	ebitenutil.DebugPrintAt(screen, "GUILDE", int(panelX)+20, int(titleY))

	if g.ui.SocialGuildName == "" {
		ebitenutil.DebugPrintAt(screen, "Pas de guilde actuelle", int(panelX)+20, int(titleY)+20)
		nameX, nameY := panelX+20, panelY+80
		nameW, nameH := panelW-200, 32.0
		vector.DrawFilledRect(screen, float32(nameX), float32(nameY), float32(nameW), float32(nameH), alpha(15, 15, 20, 210), true)
		vector.StrokeRect(screen, float32(nameX), float32(nameY), float32(nameW), float32(nameH), 2, alpha(180, 160, 100, 200), true)
		placeholder := "Nom de guilde"
		text := g.ui.SocialGuildNameInput
		if text == "" {
			text = placeholder
		}
		ebitenutil.DebugPrintAt(screen, text, int(nameX)+8, int(nameY)+8)
		if g.ui.SocialGuildNameFocus {
			ebitenutil.DebugPrintAt(screen, "|", int(nameX)+8+len(text)*7, int(nameY)+8)
		}

		btnX := nameX + nameW + 10
		btnY := nameY
		bg := alpha(60, 120, 80, 230)
		if !g.ui.SocialGuildHasTicket {
			bg = alpha(80, 80, 80, 200)
		}
		vector.DrawFilledRect(screen, float32(btnX), float32(btnY), 120, float32(nameH), bg, true)
		label := "Créer (-ticket)"
		if !g.ui.SocialGuildHasTicket {
			label = "Ticket requis"
		}
		ebitenutil.DebugPrintAt(screen, label, int(btnX)+6, int(btnY)+8)

		infoY := nameY + nameH + 10
		if g.ui.SocialGuildError != "" {
			ebitenutil.DebugPrintAt(screen, g.ui.SocialGuildError, int(nameX), int(infoY))
		} else if g.ui.SocialGuildInfo != "" {
			ebitenutil.DebugPrintAt(screen, g.ui.SocialGuildInfo, int(nameX), int(infoY))
		} else if g.ui.SocialGuildHasTicket {
			ebitenutil.DebugPrintAt(screen, "Ticket disponible pour les tests", int(nameX), int(infoY))
		}
		return
	}

	// Guild summary
	infoY := panelY + 90
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Nom: %s", g.ui.SocialGuildName), int(panelX)+20, int(infoY))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Membres: %d / 15", len(g.ui.SocialGuildMembers)), int(panelX)+20, int(infoY)+20)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Réputation: %d", g.ui.SocialGuildReputation), int(panelX)+20, int(infoY)+40)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Rang: #%d", g.ui.SocialGuildRank), int(panelX)+20, int(infoY)+60)

	// Leave Guild button
	leaveX := panelX + 20
	leaveY := infoY + 80
	vector.DrawFilledRect(screen, float32(leaveX), float32(leaveY), 120, 32, alpha(180, 100, 40, 220), true)
	ebitenutil.DebugPrintAt(screen, "Quitter", int(leaveX)+10, int(leaveY)+8)

	listY := leaveY + 40
	ebitenutil.DebugPrintAt(screen, "Membres", int(panelX)+20, int(listY)-18)
	rowH := 28.0
	rowW := panelW - 40
	for i, m := range g.ui.SocialGuildMembers {
		y := listY + float64(i)*(rowH+4)
		vector.DrawFilledRect(screen, float32(panelX+20), float32(y), float32(rowW), float32(rowH), alpha(25, 35, 50, 200), true)
			line := fmt.Sprintf("%s | Réputation %d", m.Name, m.Reputation)
		ebitenutil.DebugPrintAt(screen, line, int(panelX)+28, int(y)+8)
	}
}

func (g *Game) drawLeaderboardTab(screen *ebiten.Image, panelX, panelY, panelW float64) {
	titleY := panelY + 56
	ebitenutil.DebugPrintAt(screen, "LEADERBOARD", int(panelX)+20, int(titleY))

	subTabX := panelX + 20
	subTabY := panelY + 70
	g.drawSocialTab(screen, subTabX, subTabY, 140, 30, "Joueurs", g.ui.SocialLeaderboardTab == "players")
	g.drawSocialTab(screen, subTabX+150, subTabY, 140, 30, "Guildes", g.ui.SocialLeaderboardTab == "guilds")

	listY := subTabY + 50
	rowH := 30.0
	rowW := panelW - 40

	switch g.ui.SocialLeaderboardTab {
	case "guilds":
		ebitenutil.DebugPrintAt(screen, "Classement Guildes", int(panelX)+20, int(listY)-18)
		for i, gentry := range g.ui.SocialLeaderboardGuilds {
			y := listY + float64(i)*(rowH+4)
			vector.DrawFilledRect(screen, float32(panelX+20), float32(y), float32(rowW), float32(rowH), alpha(25, 35, 50, 200), true)
			line := fmt.Sprintf("#%d | %s | Membres %d | Réputation %d", gentry.Rank, gentry.Name, gentry.MemberCount, gentry.Reputation)
			ebitenutil.DebugPrintAt(screen, line, int(panelX)+28, int(y)+8)
		}
	default:
		ebitenutil.DebugPrintAt(screen, "Classement Joueurs", int(panelX)+20, int(listY)-18)
		for i, p := range g.ui.SocialLeaderboardPlayers {
			y := listY + float64(i)*(rowH+4)
			vector.DrawFilledRect(screen, float32(panelX+20), float32(y), float32(rowW), float32(rowH), alpha(25, 35, 50, 200), true)
			line := fmt.Sprintf("#%d | %s | Réputation %d | Guilde %s", i+1, p.Name, p.Reputation, p.GuildName)
			ebitenutil.DebugPrintAt(screen, line, int(panelX)+28, int(y)+8)
		}
	}
}

func (g *Game) removeFriend(friendID uuid.UUID) {
	g.ui.SocialSearchBusy = true
	go func() {
		defer func() { g.ui.SocialSearchBusy = false }()
		err := g.api.RemoveFriend(friendID)
		if err != nil {
			g.ui.SocialStatus = fmt.Sprintf("Erreur: %v", err)
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
			g.ui.SocialStatus = fmt.Sprintf("Erreur: %v", err)
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
