package game

import (
	"embed"
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg"
	"io"
	"log"
	"os"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
)

// game_init.go contains game initialization, assets loading, and credentials handling.
// This file was extracted from main.go during Phase 1.7 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

//go:embed resources
var assetsFS embed.FS

// loadImage is a global function variable for loading images from embedded assets.
// It is initialized in loadAssets() and can be used throughout the package.
var loadImage func(path string) *ebiten.Image

// Credentials represents saved login credentials.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewGame creates and initializes a new Game instance.
func NewGame() *Game {
	config := LoadConfig()
	g := &Game{
		api:             client.NewAPIClient(config.APIURL),
		state:           StateLogin,
		lastUpdate:      time.Now(),
		updateChan:      make(chan *domain.Player),
		visualResources: make(map[domain.ResourceType]float64),
		logChan:         make(chan LogEntry, 100),
		camZoom:         1.0,
		// Chat defaults
		chatMessages:  make([]domain.ChatMessage, 0, 200),
		chatPollTimer: 0,
		chatScroll:    0,
		seenChatIDs:   make(map[uuid.UUID]struct{}),
		slots: []Slot{
			{X: -40, Y: -144, AllowedType: "Hôtel de Ville"},
			{X: 106, Y: -556, AllowedType: "Scierie"},
			{X: 470, Y: -214, AllowedType: "Mine d'Or"},
			{X: 528, Y: 114, AllowedType: "Distillerie"},
			{X: -412, Y: 208, AllowedType: "Académie"},
			{X: -1668, Y: 636, AllowedType: "Chantier Naval"},
			{X: 284, Y: 726, AllowedType: "Carrière"},
			{X: 722, Y: 540, AllowedType: "Entrepôt"},
			{X: -258, Y: -242, AllowedType: "Tavern"}, // Localized position (from localization mode)
			{X: -100, Y: -100, AllowedType: "Milice"}, // Position near Tavern
			{X: 224, Y: 331, AllowedType: "Infirmary"},
		},
	}

	// Default Focus on Username
	g.ui.IsTypingUser = true
	// Chat starts hidden/minimized
	g.ui.ShowChat = true
	g.ui.ChatMinimized = true
	g.loadCredentials()

	g.loadAssets()

	// Init Audio System
	g.audioManager = NewAudioManager()
	if err := g.audioManager.LoadMusic("title", "resources/Music/title.mp3"); err != nil {
		g.Log("Failed to load title music: %v", err)
	}
	if err := g.audioManager.LoadMusic("island", "resources/Music/island-theme.mp3"); err != nil {
		g.Log("Failed to load island music: %v", err)
	}

	// Init DMS
	g.seaDMS = NewSeaDMSMusic(g.audioManager)

	// Init Tech UI
	g.techUI = NewTechUI()
	if err := g.techUI.Load(); err != nil {
		g.Log("Failed to load Tech UI: %v", err)
	}

	// Auto-Updater Check (Async)
	go func() {
		// Wait a bit for initialization, not strictly necessary but safer
		time.Sleep(500 * time.Millisecond)
		available, url, err := CheckForUpdates(config.APIURL)
		if err == nil && available {
			g.Log("[UPDATER] New version available: %s", url)
			g.ui.ShowUpdateModal = true
			g.ui.UpdateURL = url
		} else if err != nil {
			g.Log("[UPDATER] Check failed: %v", err)
		}
	}()

	return g
}

// loadCredentials loads saved credentials from credentials.json if it exists.
func (g *Game) loadCredentials() {
	file, err := os.Open("credentials.json")
	if err != nil {
		return
	}
	defer file.Close()
	var creds Credentials
	if err := json.NewDecoder(file).Decode(&creds); err == nil {
		g.ui.Username = creds.Username
		g.ui.Password = creds.Password
		g.ui.RememberMe = true
	}
}

// loadAssets loads all game assets (images, icons, etc.) from embedded resources.
func (g *Game) loadAssets() {
	loadImage = func(path string) *ebiten.Image {
		f, err := assetsFS.Open(path)
		if err != nil {
			log.Printf("Failed to open %s: %v", path, err)
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			log.Printf("Failed to decode %s: %v", path, err)
			return nil
		}
		return ebiten.NewImageFromImage(img)
	}

	g.bgImage = loadImage("resources/images/island.jpg")
	g.loginBG = loadImage("resources/images/login_bg.png")
	g.gameLogo = loadImage("resources/images/logo.png")

	// btn_connect removed/missing - handled in Draw

	g.btnBuild = loadImage("resources/assets/UI/icon_build_btn.png")

	g.iconWood = loadImage("resources/images/icon_wood.png")
	g.iconStone = loadImage("resources/images/icon_stone.png")
	g.iconRum = loadImage("resources/images/icon_rum.png")
	g.iconGold = loadImage("resources/images/icon_gold.png")
	g.iconBlueprint = loadImage("resources/images/icon_blueprint_btn.png")

	// Buildings (Deep paths)
	g.iconTownhall = loadImage("resources/assets/townhall/Build_townhall_LvA.png")
	g.iconSawmill = loadImage("resources/assets/sawmill/build_sawmill_LvA.png")
	g.iconGoldMine = loadImage("resources/assets/gold_mine/build_Goldmine_LvA.png")
	g.iconStoneQuarry = loadImage("resources/assets/stone_quarry/build_stonequarry_LvA.png")
	g.iconWarehouse = loadImage("resources/assets/warehouse/build_warehouse_LvA.png")
	g.iconDistillery = loadImage("resources/assets/distillery/build_distillery_LvA.png")
	g.iconShipyard = loadImage("resources/assets/shipyard/build_shipyard_LvA.png")
	g.iconAcademy = loadImage("resources/assets/academy/build_academy_LvA.png")
	g.iconTavern = loadImage("resources/assets/Tavern/build_tavern_LvA.png")
	g.iconMilitia = loadImage("resources/assets/militia/build_militia_LvA.png") // Fallback to Tavern if not found
	if g.iconMilitia == nil {
		g.iconMilitia = g.iconTavern // Use Tavern icon as fallback
	}

	// Placeholder for Infirmary (Runtime generated)
	g.iconInfirmary = ebiten.NewImage(64, 64)
	g.iconInfirmary.Fill(color.RGBA{255, 105, 180, 255}) // Hot Pink
	// Draw simple text overlay if possible, or just rely on hovering/building name
	// Basic pixel "cross" or similar could be added here
	// Horizontal bar
	// vector.DrawFilledRect(g.iconInfirmary, 16, 28, 32, 8, color.White, true)
	// Vertical bar
	// vector.DrawFilledRect(g.iconInfirmary, 28, 16, 8, 32, color.White, true)

	g.iconHammer1 = loadImage("resources/images/icon_hammer_1.png")

	// 9-slice (Corrected folder)
	g.sliceTopLeft = loadImage("resources/images/ui_9slice/top_left.png")
	g.sliceTopCenter = loadImage("resources/images/ui_9slice/top_center.png")
	g.sliceTopRight = loadImage("resources/images/ui_9slice/top_right.png")
	g.sliceMidLeft = loadImage("resources/images/ui_9slice/mid_left.png")
	g.sliceCenter = loadImage("resources/images/ui_9slice/center.png")
	g.sliceMidRight = loadImage("resources/images/ui_9slice/mid_right.png")
	g.sliceBotLeft = loadImage("resources/images/ui_9slice/bot_left.png")
	g.sliceBotCenter = loadImage("resources/images/ui_9slice/bot_center.png")
	g.sliceBotRight = loadImage("resources/images/ui_9slice/bot_right.png")

	// Load water shader for World Map
	waterShaderFile, err := assetsFS.Open("resources/shaders/water.kage")
	if err != nil {
		log.Printf("Failed to open water shader: %v", err)
	} else {
		defer waterShaderFile.Close()
		shaderSource, err := io.ReadAll(waterShaderFile)
		if err != nil {
			log.Printf("Failed to read water shader: %v", err)
		} else {
			shader, err := ebiten.NewShader(shaderSource)
			if err != nil {
				log.Printf("Failed to compile water shader: %v", err)
			} else {
				g.waterShader = shader
				log.Printf("Water shader loaded successfully")
			}
		}
	}

	// Load island image for World Map
	g.imgIsland = loadImage("resources/sea/island_asset/island_full.png")

	// Load Pirate UI Atlas (Overrides)
	g.loadUIAtlas()
}
