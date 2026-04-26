package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

const maxChatMessages = 500

type Slot struct {
	X, Y        float64
	AllowedType string
}

type LogEntry struct {
	Timestamp time.Time
	Type      string
	Message   string
}

type Game struct {
	api         *client.APIClient
	player      *domain.Player
	lastUpdate  time.Time
	lastPvePoll time.Time        // Timer for polling PvE targets
	captains    []client.Captain // List of player's captains

	// Connection State
	consecutiveFailures int

	// State
	state GameState
	ui    UIState

	waterShader *ebiten.Shader

	bgImage  *ebiten.Image
	loginBG  *ebiten.Image
	gameLogo *ebiten.Image

	iconWood      *ebiten.Image
	iconGold      *ebiten.Image
	iconStone     *ebiten.Image
	iconRum       *ebiten.Image
	iconBlueprint *ebiten.Image

	// New UI Assets
	iconHammer1     *ebiten.Image
	iconTownhall    *ebiten.Image
	iconSawmill     *ebiten.Image
	iconGoldMine    *ebiten.Image
	iconStoneQuarry *ebiten.Image
	iconWarehouse   *ebiten.Image
	iconDistillery  *ebiten.Image
	iconShipyard    *ebiten.Image
	iconAcademy     *ebiten.Image
	iconTavern      *ebiten.Image
	iconMilitia     *ebiten.Image
	iconInfirmary   *ebiten.Image

	// NPC / Overlay Assets

	// Login Button
	btnBuild *ebiten.Image

	// UI Textures (New Pirate Theme)
	btnNormal  *ebiten.Image
	btnHover   *ebiten.Image
	btnPressed *ebiten.Image

	// 9-Slice Assets
	sliceTopLeft   *ebiten.Image
	sliceTopCenter *ebiten.Image
	sliceTopRight  *ebiten.Image
	sliceMidLeft   *ebiten.Image
	sliceCenter    *ebiten.Image
	sliceMidRight  *ebiten.Image
	sliceBotLeft   *ebiten.Image
	sliceBotCenter *ebiten.Image
	sliceBotRight  *ebiten.Image

	// Animation State
	hammerFrame int
	hammerTimer int

	// Menu State
	isMenuOpen bool

	// Camera
	camX, camY             float64
	camZoom                float64
	lastMouseX, lastMouseY int

	// Construction
	slots []Slot

	// Screen Layout
	screenWidth, screenHeight int

	// Pause State
	isPaused bool

	// Developer Mode
	isDev       bool
	showDevMenu bool
	devLogs     []string // Dev tool log messages

	// Admin Status Debug

	// Dev Console State

	// Audio
	audioManager *AudioManager
	seaDMS       *SeaDMSMusic

	// Async Updates
	updateChan chan *domain.Player

	// Dev Console
	consoleLog []LogEntry
	logChan    chan LogEntry

	// Live Resource Updates
	visualResources map[domain.ResourceType]float64
	lastFrameTime   time.Time
	dt              float64

	// Chat Overlay (shared across island + world map views)
	chatMessages        []domain.ChatMessage
	chatLastFetch       time.Time
	chatPollTimer       float64
	chatScroll          int
	chatOldest          time.Time
	chatNextCursor      string
	seenChatIDs         map[uuid.UUID]struct{}
	chatHistoryLoaded   bool
	chatHistoryPlayerID uuid.UUID

	// Social
	socialStateLoaded   bool
	socialStatePlayerID uuid.UUID

	// World Map Assets
	imgIsland *ebiten.Image

	// Inspection
	selectedBuilding *domain.Building

	// Error Popup
	showError     bool
	errorMessage  string
	errorDebounce int

	// Tech UI
	techUI *TechUI
}

func (g *Game) IsCombatActive() bool {
	if g.player == nil || len(g.player.Islands) == 0 {
		return false
	}

	// 1. Check Busy Flags (Pending engagement/request)
	if g.ui.PveEngageBusy || g.ui.PvpAttackBusy {
		return true
	}

	// 2. Check Fleet States (Chasing or Traveling to attack)
	// We check ALL fleets for ALL islands (though usually only 1 island)
	for _, island := range g.player.Islands {
		for _, fleet := range island.Fleets {
			if fleet.State == domain.FleetStateChasingPvE ||
				fleet.State == domain.FleetStateChasingPvP ||
				fleet.State == domain.FleetStateTravelingToAttack {
				return true
			}
		}
	}

	// 3. Victory/Result Modal check?
	// The user said: "sans dépendre d'UI/retour île" for the return to Calm.
	// This implies once the state above is false, it should return to Calm.
	// If the result UI is open, but the fleet is already "Returning", it will return to Calm.
	// This matches the "Automatic Return" requirement.

	return false
}

// pollChatFeed fetches new chat messages from server.
func (g *Game) pollChatFeed() {
	if g.api == nil || g.player == nil {
		return
	}
	prevLen := len(g.chatMessages)
	messages, err := g.api.FetchChat(g.chatLastFetch)
	if err != nil {
		return
	}
	for _, m := range messages {
		g.addIncomingChat(m)
		if m.Timestamp.After(g.chatLastFetch) {
			g.chatLastFetch = m.Timestamp
		}
	}
	if prevLen == 0 && len(g.chatMessages) > 0 {
		g.chatScroll = 0
	}
}

func (g *Game) addIncomingChat(msg domain.ChatMessage) {
	if g.seenChatIDs == nil {
		g.seenChatIDs = make(map[uuid.UUID]struct{})
	}
	if msg.ID != uuid.Nil {
		if _, ok := g.seenChatIDs[msg.ID]; ok {
			return
		}
		g.seenChatIDs[msg.ID] = struct{}{}
	}

	atBottom := g.chatScroll == 0
	g.chatMessages = append(g.chatMessages, msg)
	if len(g.chatMessages) > maxChatMessages {
		overflow := len(g.chatMessages) - maxChatMessages
		for i := 0; i < overflow; i++ {
			old := g.chatMessages[i]
			if old.ID != uuid.Nil {
				delete(g.seenChatIDs, old.ID)
			}
		}
		g.chatMessages = g.chatMessages[overflow:]
	}

	if g.chatOldest.IsZero() || msg.Timestamp.Before(g.chatOldest) {
		g.chatOldest = msg.Timestamp
	}

	if atBottom {
		g.chatScroll = 0
	}

	g.persistChatHistory()
}

// addOlderChats prepends older messages, keeping scroll position stable.
func (g *Game) addOlderChats(msgs []domain.ChatMessage) {
	if len(msgs) == 0 {
		return
	}
	if g.seenChatIDs == nil {
		g.seenChatIDs = make(map[uuid.UUID]struct{})
	}
	inserted := make([]domain.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg.ID != uuid.Nil {
			if _, ok := g.seenChatIDs[msg.ID]; ok {
				continue
			}
			g.seenChatIDs[msg.ID] = struct{}{}
		}
		inserted = append(inserted, msg)
	}
	if len(inserted) == 0 {
		return
	}
	prevLen := len(g.chatMessages)
	g.chatMessages = append(inserted, g.chatMessages...)
	added := len(g.chatMessages) - prevLen
	if added < 0 {
		added = 0
	}
	// Keep view anchored after prepending
	g.chatScroll += added

	// Update oldest timestamp
	first := g.chatMessages[0]
	if g.chatOldest.IsZero() || first.Timestamp.Before(g.chatOldest) {
		g.chatOldest = first.Timestamp
	}

	g.persistChatHistory()
}

// fetchOlderChat loads older messages before the oldest currently loaded.
func (g *Game) fetchOlderChat() {
	if g.api == nil || g.player == nil {
		return
	}

	before := g.chatOldest
	// If we have a cursor from server, prefer it
	if g.chatNextCursor != "" {
		if t, err := time.Parse(time.RFC3339, g.chatNextCursor); err == nil {
			before = t
		}
	}

	msgs, next, err := g.api.FetchChatBefore(before, 50)
	if err != nil {
		return
	}
	if len(msgs) == 0 {
		g.chatNextCursor = ""
		return
	}
	visible := g.visibleChatLines()
	prevMax := g.maxChatScroll(visible)
	g.addOlderChats(msgs)
	g.chatNextCursor = next
	// Re-clamp scroll after adding older messages
	g.clampChatScroll(visible)
	// If we were at top, maintain roughly same viewport
	newMax := g.maxChatScroll(visible)
	g.chatScroll += (newMax - prevMax)
	g.clampChatScroll(visible)
}

func (g *Game) maxChatScroll(visible int) int {
	if len(g.chatMessages) <= visible {
		return 0
	}
	return len(g.chatMessages) - visible
}

// ensureChatLoaded fetches the latest messages when opening chat or after login.
func (g *Game) ensureChatLoaded() {
	if g.api == nil || g.player == nil {
		return
	}
	if len(g.chatMessages) == 0 {
		g.chatLastFetch = time.Time{}
		g.chatOldest = time.Time{}
		g.chatNextCursor = ""
		g.pollChatFeed()
	}
}

func (g *Game) chatHistoryPath() string {
	if g.player == nil {
		return ""
	}
	return filepath.Join("client_data", fmt.Sprintf("chat_%s.json", g.player.ID.String()))
}

func (g *Game) loadChatHistory() {
	if g.chatHistoryLoaded || g.player == nil {
		return
	}
	path := g.chatHistoryPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		g.chatHistoryLoaded = true
		return
	}
	var stored []domain.ChatMessage
	if err := json.Unmarshal(data, &stored); err != nil {
		g.chatHistoryLoaded = true
		return
	}
	if len(stored) > maxChatMessages {
		stored = stored[len(stored)-maxChatMessages:]
	}
	// Replace current buffer with persisted copy
	g.chatMessages = stored
	g.seenChatIDs = make(map[uuid.UUID]struct{})
	g.chatOldest = time.Time{}
	g.chatLastFetch = time.Time{}
	for _, msg := range g.chatMessages {
		if msg.ID != uuid.Nil {
			g.seenChatIDs[msg.ID] = struct{}{}
		}
		if g.chatOldest.IsZero() || msg.Timestamp.Before(g.chatOldest) {
			g.chatOldest = msg.Timestamp
		}
		if msg.Timestamp.After(g.chatLastFetch) {
			g.chatLastFetch = msg.Timestamp
		}
	}
	g.chatHistoryLoaded = true
}

func (g *Game) persistChatHistory() {
	if g.player == nil || len(g.chatMessages) == 0 {
		return
	}
	path := g.chatHistoryPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(g.chatMessages)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func (g *Game) resetChatForPlayer(id uuid.UUID) {
	if g.chatMessages == nil {
		g.chatMessages = make([]domain.ChatMessage, 0, 200)
	} else {
		g.chatMessages = g.chatMessages[:0]
	}
	g.chatScroll = 0
	g.chatOldest = time.Time{}
	g.chatLastFetch = time.Time{}
	g.chatNextCursor = ""
	g.seenChatIDs = make(map[uuid.UUID]struct{})
	g.chatHistoryLoaded = false
	g.chatHistoryPlayerID = id
}

// --- Social Persistence ---

type socialStateFile struct {
	Friends         []SocialProfile `json:"friends"`
	GuildID         uuid.UUID       `json:"guild_id"`
	GuildName       string          `json:"guild_name"`
	GuildMembers    []SocialProfile `json:"guild_members"`
	GuildReputation int             `json:"guild_reputation"`
	GuildRank       int             `json:"guild_rank"`
	HasTicket       bool            `json:"has_ticket"`
}

func (g *Game) socialStatePath() string {
	if g.player == nil {
		return ""
	}
	return filepath.Join("client_data", fmt.Sprintf("social_%s.json", g.player.ID.String()))
}

func (g *Game) resetSocialForPlayer(id uuid.UUID) {
	g.ui.SocialFriends = nil
	g.ui.SocialGuildID = uuid.Nil
	g.ui.SocialGuildName = ""
	g.ui.SocialGuildNameInput = ""
	g.ui.SocialGuildMembers = nil
	g.ui.SocialGuildMemberCount = 0
	g.ui.SocialGuildReputation = 0
	g.ui.SocialGuildRank = 0
	g.ui.SocialGuildHasTicket = true // default test ticket available
	g.ui.SocialStatus = ""
	g.ui.SocialInfo = ""
	g.ui.SocialError = ""
	g.ui.SocialGuildError = ""
	g.ui.SocialGuildInfo = ""
	g.socialStateLoaded = false
	g.socialStatePlayerID = id
}

func (g *Game) loadSocialState() {
	if g.socialStateLoaded || g.player == nil {
		return
	}
	path := g.socialStatePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// No file yet: keep defaults (ticket true)
		g.socialStateLoaded = true
		return
	}
	var stored socialStateFile
	if err := json.Unmarshal(data, &stored); err != nil {
		g.socialStateLoaded = true
		return
	}
	g.ui.SocialFriends = stored.Friends
	g.ui.SocialGuildID = stored.GuildID
	g.ui.SocialGuildName = stored.GuildName
	g.ui.SocialGuildMembers = stored.GuildMembers
	g.ui.SocialGuildReputation = stored.GuildReputation
	g.ui.SocialGuildRank = stored.GuildRank
	g.ui.SocialGuildHasTicket = stored.HasTicket
	if !g.ui.SocialGuildHasTicket && g.ui.SocialGuildName == "" {
		// If no guild persisted but ticket consumed, grant one for testing consistency
		g.ui.SocialGuildHasTicket = true
	}
	g.socialStateLoaded = true
}

func (g *Game) persistSocialState() {
	if g.player == nil {
		return
	}
	path := g.socialStatePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	state := socialStateFile{
		Friends:      g.ui.SocialFriends,
		GuildID:      g.ui.SocialGuildID,
		GuildName:    g.ui.SocialGuildName,
		GuildMembers: g.ui.SocialGuildMembers,
		GuildReputation: g.ui.SocialGuildReputation,
		GuildRank:    g.ui.SocialGuildRank,
		HasTicket:    g.ui.SocialGuildHasTicket,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
