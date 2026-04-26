package game

import (
	"sync"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
)

// ui_state.go contains all UI-related state variables and types.
// This file was extracted from main.go during Phase 1 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

// GameState represents the current state of the game (login, playing, world map, etc.)
type GameState int

const (
	StateLogin GameState = iota
	StatePlaying
	StateWorldMap
)

// SocialProfile represents a lightweight player entry for search/friends/leaderboards.
type SocialProfile struct {
	ID         uuid.UUID
	Name       string
	Reputation int
	GuildName  string
}

// SocialGuildEntry represents guild leaderboard entries and the player's guild info.
type SocialGuildEntry struct {
	Name       string
	MemberCount int
	Reputation int
	Rank       int
}

// UIState contains all UI-related state variables for the game.
// This includes login form state, menu visibility flags, hover states, scroll positions, etc.
type UIState struct {
	Username        string
	Password        string
	IsTypingUser    bool
	IsTypingPass    bool
	LoginError      string
	CaretTimer      int
	RememberMe      bool
	HoverButton     bool
	IsRegisterMode  bool // Toggles between Login and Register mode
	RegisterError   string
	RegisterSuccess bool

	ShowTechUI       bool
	ShowShipyard     bool
	ShipyardDebounce int

	ShipyardTab     int
	ShipyardScrollY float64

	HoverBuildButton    bool
	BuildButtonScale    float64 // For click animation
	ShowConstruction    bool
	ConstructionScrollY float64

	ShowFleetUI      bool
	HoverFleetButton bool
	FleetButtonScale float64 // For click animation
	SelectedFleetID  string  // Selected fleet ID for details panel
	LastLogFleetID   string  // For debug logging dedup of Fleet UI
	FleetListScroll  float64 // Scroll position for fleet list

	// Cargo Transfer UI
	TransferSelectedResource string  // "wood", "stone", "gold", "rum"
	TransferAmount           float64 // Amount to transfer
	TransferMessage          string  // Feedback message
	TransferInputActive      bool    // Is the amount input focused?
	TransferInputString      string  // The string being typed

	HoverSeaButton         bool
	SeaButtonScale         float64 // For click animation
	ShowCaptainModal       bool
	SelectedShipForCaptain string  // Ship ID for which we're selecting a captain
	CaptainModalScroll     float64 // Scroll position for captain selection modal

	// Crew assignment modal
	ShowCrewModal       bool
	SelectedShipForCrew string // Ship ID for which we're assigning crew
	CrewModalWarriors   int    // Slider value for warriors
	CrewModalArchers    int    // Slider value for archers
	CrewModalGunners    int    // Slider value for gunners
	CrewModalBusy       bool   // Busy state while calling API
	CrewModalError      string // Error message from server
	// Drag tracking for sliders
	DraggingCrewWarrior bool // True when dragging warrior slider
	DraggingCrewArcher  bool // True when dragging archer slider
	DraggingCrewGunner  bool // True when dragging gunner slider

	// Dev Engagement Test
	DevEngageResult     *domain.EngagementResult
	DevEngageError      string
	DevEngageBusy       bool
	DevEngageSelectedA  int     // Index in fleets array
	DevEngageSelectedB  int     // Index in fleets array
	DevMenuFleetsLogged bool    // Flag to log fleets only once when DevMenu opens
	DevLogScrollY       float64 // Scroll position for dev menu logs (vertical)
	DevLogScrollX       float64 // Scroll position for dev menu logs (horizontal)

	// Auto-Updater
	ShowUpdateModal  bool
	UpdateURL        string
	UpdateVersion    string
	HoverUpdateBtn   bool
	UpdateInProgress bool

	// Tavern UI
	ShowTavernUI     bool   // Show dedicated Tavern/Gacha modal
	TavernLastResult string // Last summon result message
	TavernBusy       bool   // Busy state while calling API
	TavernError      string // Error message from server

	// Militia UI
	ShowMilitiaUI   bool   // Show dedicated Militia recruitment modal
	MilitiaBusy     bool   // Busy state while calling API
	MilitiaError    string // Error message from server
	MilitiaWarriors int    // Slider value for warriors
	MilitiaArchers  int    // Slider value for archers
	MilitiaGunners  int    // Slider value for gunners
	// Drag tracking for Milice sliders
	DraggingMilitiaWarrior bool // True when dragging warrior slider
	DraggingMilitiaArcher  bool // True when dragging archer slider
	DraggingMilitiaGunner  bool // True when dragging gunner slider

	// Dev Tools
	LocalizationMode bool // Localization mode: hide DevMenu, show coords overlay
	DevActionBusy    bool // Busy state while executing dev action and refreshing status
	DevSimTier       int  // Selected enemy tier for combat simulator (default 1)

	// Captain UI
	ShowCaptainUI       bool       // Show Captain roster/encyclopedia UI
	CaptainUISelectedID *uuid.UUID // Selected captain ID for details panel
	CaptainUIScroll     float64    // Scroll position for captain list
	CaptainUIError      string     // Error message from API
	CaptainUIBusy       bool       // Busy state while loading captains
	HoverCaptainButton  bool       // Hover state for Captain button
	CaptainButtonScale  float64    // Scale animation for Captain button click
	CaptainUITab        string     // Current tab: "list" or "upgrade"

	// Social UI
	ShowSocialUI       bool
	HoverSocialButton  bool
	SocialButtonScale  float64
	SocialActiveTab    string // "friends", "guild", "leaderboard"
	SocialStatus       string
	SocialError        string
	SocialInfo         string
	SocialSearchQuery  string
	SocialSearchFocus  bool
	SocialSearchBusy   bool
	SocialSearchResult []SocialProfile
	SocialFriends      []SocialProfile

	SocialGuildID        uuid.UUID
	SocialGuildHasTicket bool
	SocialGuildNameFocus bool
	SocialGuildNameInput string
	SocialGuildName      string
	SocialGuildMembers   []SocialProfile
	SocialGuildMemberCount int
	SocialGuildRank      int
	SocialGuildReputation int
	SocialGuildBusy      bool
	SocialGuildInfo      string
	SocialGuildError     string

	SocialLeaderboardTab     string // "players" or "guilds"
	SocialLeaderboardPlayers []SocialProfile
	SocialLeaderboardGuilds  []SocialGuildEntry

	// Prerequisites Modal
	ShowPrereqModal     bool                 // Show prerequisites modal
	PrereqModalTitle    string               // Modal title
	PrereqModalSubtitle string               // Optional subtitle
	PrereqModalReqs     []domain.Requirement // List of missing requirements
	PrereqModalScroll   float64              // Scroll position for requirements list

	// PVE UI
	// Infirmary UI
	ShowInfirmaryUI  bool    // Show Infirmary modal
	InfirmaryScrollY float64 // Scroll position for captain list

	// PvP UI
	PvpTargets      []domain.PveTarget // PvP Targets (reusing PveTarget struct)
	PvpTargetsBusy  bool
	PvpTargetsError string
	HoverPvpTarget  string // Hovered target ID

	// PvP Menu State
	ShowPvpUI          bool                 // Show PvP target selection modal
	PvpSelectedTarget  *domain.PveTarget    // Currently selected target for attack
	PvpSelectedFleetID string               // Fleet ID selected for attack
	ShowPvpFleetSelect bool                 // Show fleet selection dialog
	ShowPvpConfirm     bool                 // Show final confirmation dialog
	PvpAttackBusy      bool                 // Attack in progress
	PvpAttackError     string               // Error from attack
	PvpCombatResult    *domain.CombatResult // Combat result to display
	PvpLoot            map[string]float64   // Loot gained from attack
	ShowPvpResultUI    bool                 // Show PvP combat result
	PvpResultScroll    float64              // Scroll position for result UI

	// PvP Interception State
	ShowInterceptConfirm     bool
	InterceptTargetFleetID   string
	InterceptAttackerFleetID string
	InterceptBusy            bool
	InterceptError           string
	InterceptTargetName      string // Target fleet name
	InterceptAttackerName    string // Attacker fleet name

	PveTargetsMutex    sync.RWMutex         // Protects PveTargets map/slice
	PveTargets         []domain.PveTarget   // Cached PVE targets
	PveTargetsBusy     bool                 // Loading targets
	PveTargetsError    string               // Error loading targets
	PveEngageBusy      bool                 // Engaging target
	PveEngageError     string               // Error engaging
	PveEngageErrorCode string               // Error code from server (reason_code)
	ShowPveEngageError bool                 // Show engagement error modal
	PveCombatResult    *domain.CombatResult // Combat result to display
	ShowPveResultUI    bool                 // Show combat result UI
	PveResultScroll    float64              // Scroll position for result UI
	HoverPveTarget     string               // Hovered target ID

	// Chat Overlay
	ShowChat          bool    // Chat panel target visibility
	ChatMinimized     bool    // Collapsed panel to avoid blocking view
	ChatInput         string  // Current input text
	ChatInputFocus    bool    // Capture keyboard for chat
	ChatSlideProgress float64 // 0 hidden, 1 fully visible
	ChatSlideTarget   float64 // Target slide value controlled by toggle

	// PvE Chase Confirm Modal
	ShowPveConfirmModal  bool   // Show confirmations before chase
	PveConfirmTargetID   string // Target to chase
	PveConfirmTargetName string // Name of target
	PveConfirmTier       int    // Tier of target
	PveConfirmFleetID    string // Fleet to use
	PveConfirmFleetName  string // Name of fleet

	// Navigation Confirm Modals
	ShowNavFreeConfirm    bool
	NavFreeConfirmTargetX int
	NavFreeConfirmTargetY int
	NavFreeConfirmType    string // "docked", "retarget", "stop_chasing"

	// Resource Stationing UI
	ResourceNodesMutex sync.RWMutex
	ResourceNodes      []domain.ResourceNode
	ResourceNodesBusy  bool
	ResourceNodesError string
	HoverResourceNode  string
	ShowStationMenu    bool   // Show menu to pick fleet when node clicked
	StationMenuNodeID  string // Node currently selected for stationing

	// Wind System
	WindDirection  float64
	WindNextChange time.Time

	// Thread Safety
	PlayerMutex    sync.RWMutex
	PlayerDataBusy bool
}
