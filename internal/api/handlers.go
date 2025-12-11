package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/engine"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // In real app, hash this!
	Email    string `json:"email"`
}

type RegisterResponse struct {
	PlayerID uuid.UUID `json:"player_id"`
	IslandID uuid.UUID `json:"island_id"`
	Message  string    `json:"message"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	PlayerID uuid.UUID `json:"player_id"`
	IslandID uuid.UUID `json:"island_id"` // Simplified: Assume 1 island for now
	Role     string    `json:"role"`
	IsAdmin  bool      `json:"is_admin"`
	Token    string    `json:"token,omitempty"` // Optional: auth token for Bearer authentication
}

func Register(c echo.Context) error {
	req := new(RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Input validation
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Username is required"})
	}
	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password must be at least 6 characters"})
	}

	db := repository.GetDB()

	// Check if username already exists
	var existingPlayer domain.Player
	if err := db.Where("username = ?", req.Username).First(&existingPlayer).Error; err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Username already exists"})
	}

	// Hash password before storing
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process password"})
	}

	// 1. Create Player
	role := "USER"
	// Admin logic: check environment variable or keep hardcoded for dev
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "TheXmyst" // Fallback to hardcoded for backward compatibility
	}
	if req.Username == adminUsername {
		role = "DEV"
	}

	player := domain.Player{
		ID:        uuid.New(),
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword, // Store hashed password, not plaintext
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Role:      role,
		IsAdmin:   role == "DEV" || req.Username == adminUsername,
	}

	// 2. Find or Create Sea (Matchmaking)

	// Find non-full seas, ordered by creation (fill oldest first or newest? usually fill one by one)
	// We want to fill the "current" sea.
	// Query: Seas with < 50 islands. Hard to do in pure SQL with GORM without valid subquery or join count.
	// Simpler: Get ALL seas (there wont be many). Or getting the LAST sea.

	var lastSea domain.Sea
	result := db.Order("created_at desc").Preload("Islands").First(&lastSea)

	targetSeaID := uuid.Nil
	targetIslands := []domain.Island{}

	if result.Error != nil || len(lastSea.Islands) >= 50 {
		// Create New Sea
		newSea := domain.Sea{
			ID:        uuid.New(),
			Name:      fmt.Sprintf("Sea %d", rand.Intn(10000)), // Better naming later
			CreatedAt: time.Now(),
		}
		if err := db.Create(&newSea).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate world"})
		}
		targetSeaID = newSea.ID
		targetIslands = []domain.Island{} // Empty
		fmt.Printf("Created New Sea: %s\n", newSea.Name)
	} else {
		// Use existing
		targetSeaID = lastSea.ID
		targetIslands = lastSea.Islands
		fmt.Printf("Joining Existing Sea: %s (%d/50)\n", lastSea.Name, len(lastSea.Islands))
	}

	// 3. Determine Island Position (Simple Collision Avoidance)
	// Map Size: 2000x2000? Let's say coordinates are -1000 to 1000.
	finalX, finalY := 0, 0
	placed := false

	for attempt := 0; attempt < 100; attempt++ {
		// Random Pos: -800 to 800 (keep margin)
		rx := rand.Intn(1600) - 800
		ry := rand.Intn(1600) - 800

		// Check distance
		collision := false
		for _, other := range targetIslands {
			dist := (rx-other.X)*(rx-other.X) + (ry-other.Y)*(ry-other.Y) // Squared Dist
			if dist < 40000 {                                             // Min distance 200 units (sqrt(40000)=200)
				collision = true
				break
			}
		}

		if !collision {
			finalX = rx
			finalY = ry
			placed = true
			break
		}
	}

	if !placed {
		// Fallback: Just place at 0,0 + chaos to avoid perfect overlap? Or fail?
		// For now, simple spread
		finalX = rand.Intn(100) * 10
		finalY = rand.Intn(100) * 10
	}

	// 4. Create Island
	island := domain.Island{
		ID:          uuid.New(),
		PlayerID:    player.ID,
		SeaID:       targetSeaID,
		Name:        req.Username + "'s Haven",
		Level:       1,
		X:           finalX,
		Y:           finalY,
		LastUpdated: time.Now(),
		Resources: map[domain.ResourceType]float64{
			domain.Wood:  2500.0,
			domain.Gold:  3000.0,
			domain.Stone: 2500.0,
			domain.Rum:   1000.0,
		},
	}

	// Transaction to save both
	tx := db.Begin()
	if err := tx.Create(&player).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create player: " + err.Error()})
	}

	if err := tx.Create(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create island"})
	}

	// Transaction Committed

	tx.Commit()

	return c.JSON(http.StatusCreated, RegisterResponse{
		PlayerID: player.ID,
		IslandID: island.ID,
		Message:  "Welcome Captain! Your island awaits.",
	})
}

func Login(c echo.Context) error {
	req := new(LoginRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Input validation
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)

	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Username is required"})
	}
	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}

	// STRICT: Enforce minimum password length for ALL users (including admin)
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password must be at least 6 characters"})
	}

	db := repository.GetDB()
	var player domain.Player

	// Find Player
	if err := db.Where("username = ?", req.Username).First(&player).Error; err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Determine if this is the admin user
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "TheXmyst" // Fallback for backward compatibility
	}
	isAdminUser := player.Username == adminUsername

	// STRICT ADMIN LOGIN: Admin user MUST use bcrypt, NO plaintext migration allowed
	if isAdminUser {
		// For admin: ONLY bcrypt check, NO legacy migration
		err := auth.CheckPasswordHash(req.Password, player.Password)
		if err != nil {
			// Admin login failed - log it (without leaking sensitive data)
			fmt.Printf("Admin login failed: invalid password for user %s\n", player.Username)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		}
		// Admin password is valid (bcrypt match)
		// Ensure admin status is set
		if !player.IsAdmin {
			player.IsAdmin = true
			db.Save(&player)
		}
	} else {
		// NON-ADMIN: Support bcrypt and safe legacy plaintext migration
		passwordValid := false
		err := auth.CheckPasswordHash(req.Password, player.Password)
		if err == nil {
			// Password matches bcrypt hash
			passwordValid = true
		} else {
			// Check if stored password is plaintext (legacy support for non-admin only)
			// SAFE CHECK: Only attempt migration if stored password is clearly NOT a bcrypt hash
			// Bcrypt hashes always start with "$2" (or "$2a$", "$2b$", "$2x$", "$2y$")
			isBcryptHash := strings.HasPrefix(player.Password, "$2")
			if !isBcryptHash && player.Password == req.Password {
				// Stored password is plaintext and matches - safe to migrate
				hashedPassword, hashErr := auth.HashPassword(req.Password)
				if hashErr != nil {
					// Hashing failed - do NOT log in the user
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process password"})
				}
				// Migration successful - update password and allow login
				player.Password = hashedPassword
				if err := db.Save(&player).Error; err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
				}
				passwordValid = true
			}
		}

		if !passwordValid {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		}
	}

	// Get Island
	var island domain.Island
	if err := db.Where("player_id = ?", player.ID).First(&island).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Player has no island!"})
	}

	// Generate auth token
	token, err := auth.GenerateToken(player.ID, player.Username, player.IsAdmin)
	if err != nil {
		// Log error but don't fail login - token is optional for backward compatibility
		fmt.Printf("Warning: Failed to generate auth token: %v\n", err)
	}

	return c.JSON(http.StatusOK, LoginResponse{
		PlayerID: player.ID,
		IslandID: island.ID,
		Role:     player.Role,
		IsAdmin:  player.IsAdmin,
		Token:    token, // Optional token for Bearer auth
	})
}

func GetStatus(c echo.Context) error {
	playerIDStr := c.QueryParam("player_id")
	if playerIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "player_id required"})
	}

	playerID, err := uuid.Parse(playerIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid player_id"})
	}

	db := repository.GetDB()
	var player domain.Player

	// Preload Islands, Buildings AND Ships
	if err := db.Preload("Islands.Buildings").Preload("Islands.Ships").First(&player, "id = ?", playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Player not found"})
	}

	// LAZY UPDATE: Check Research Completion on Read
	if player.ResearchingTechID != "" && !player.ResearchFinishTime.IsZero() {
		if time.Now().After(player.ResearchFinishTime) {
			techID := player.ResearchingTechID

			var unlocked []string
			if len(player.UnlockedTechsJSON) > 0 {
				_ = json.Unmarshal(player.UnlockedTechsJSON, &unlocked)
			}
			exists := false
			for _, id := range unlocked {
				if id == techID {
					exists = true
					break
				}
			}
			if !exists {
				unlocked = append(unlocked, techID)
			}
			player.UnlockedTechs = unlocked
			newJSON, _ := json.Marshal(unlocked)
			player.UnlockedTechsJSON = newJSON
			player.ResearchingTechID = ""
			player.ResearchFinishTime = time.Time{}

			// Save and continue (so returned JSON is updated)
			db.Save(&player)
			fmt.Printf("[LAZY READ] Research Complete: %s\n", techID)
		}
	}

	// Calculate resources for each island
	now := time.Now()
	for i := range player.Islands {
		island := &player.Islands[i]

		// CRITICAL FIX: Link the loaded player to the island so CalculateResources sees Techs
		// And so we don't accidentally save an empty player struct if GORM cascades.
		island.Player = player

		elapsed := now.Sub(island.LastUpdated)

		if elapsed > 0 {
			engine.CalculateResources(island, elapsed)
			island.LastUpdated = now

			// Updates ResourcesJSON via BeforeSave hook check?
			// GORM Updates/Select might skip BeforeSave hooks for strict updates.
			// Handlers.go:121 BeforeSave handles marshalling.
			// If we use Save(), it calls BeforeSave.
			// But we want to limit columns.
			// Be careful: if we limit columns to "resources", BeforeSave must populate "resources" column
			// from island.Resources map.
			// If BeforeSave runs, it updates island.ResourcesJSON.
			// Then Save() writes it.

			// Safer approach: use Save() but ensure island.Player is correct (which we did above).
			// And/Or use Omit("Player") to prevent player updates.
			db.Omit("Player").Save(island)
		}

		// HOTFIX: Update Townhall position
		for j := range island.Buildings {
			if island.Buildings[j].Type == "Hôtel de Ville" {
				if island.Buildings[j].X != -40 || island.Buildings[j].Y != -144 {
					island.Buildings[j].X = -40
					island.Buildings[j].Y = -144
					db.Save(&island.Buildings[j])
				}
			}
		}

		// CRITICAL FIX 2: Break the Cycle!
		// We assigned island.Player = player above for calculation.
		// If we leave it, JSON marshal will try: player -> islands -> island -> player -> islands ... (Loop)
		// We must clear it before returning.
		island.Player = domain.Player{}
	}

	return c.JSON(http.StatusOK, player)
}

// getAuthenticatedPlayerID extracts player ID from context (if authenticated via middleware)
// or from request (for backward compatibility)
// Returns the player ID and an error if not found
// DEPRECATED: For protected routes, use auth.GetAuthenticatedPlayer(c) directly instead.
// This function is kept for backward compatibility with non-protected endpoints.
func getAuthenticatedPlayerID(c echo.Context) (uuid.UUID, error) {
	// Try to get from auth context first (if middleware was applied)
	if player := auth.GetAuthenticatedPlayer(c); player != nil {
		return player.ID, nil
	}

	// Fallback: try to get from query param (for GET requests)
	playerIDStr := c.QueryParam("player_id")
	if playerIDStr != "" {
		playerID, err := uuid.Parse(playerIDStr)
		if err == nil {
			return playerID, nil
		}
	}

	// For POST requests, player_id should be in the request body
	// But we don't read it here to avoid consuming the body
	// The handlers should extract it from their bound request structs
	return uuid.Nil, fmt.Errorf("player_id required")
}

type BuildRequest struct {
	PlayerID uuid.UUID `json:"player_id"` // Optional: can be extracted from auth context
	IslandID uuid.UUID `json:"island_id"`
	Type     string    `json:"type"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
}

func Build(c echo.Context) error {
	req := new(BuildRequest)
	if err := c.Bind(req); err != nil {
		errorMsg := "Invalid request"
		fmt.Printf("Build failed: reason=%s\n", errorMsg)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
	}

	// Get authenticated player ID from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		errorMsg := "Authentication required"
		fmt.Printf("Build failed: player_id=<none>, reason=%s\n", errorMsg)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": errorMsg})
	}
	playerID := player.ID

	// Log build request at the beginning
	fmt.Printf("Build request: player_id=%s, island_id=%s, building_type=%s, x=%.2f, y=%.2f\n",
		playerID, req.IslandID, req.Type, req.X, req.Y)

	// Use Economy System for accurate Level 1 stats
	stats, err := economy.GetBuildingStats(req.Type, 1)
	if err != nil {
		errorMsg := "Invalid building type"
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
			playerID, req.IslandID, req.Type, errorMsg)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
	}

	db := repository.GetDB()
	var island domain.Island

	// Transaction to ensure atomicity
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Preload Player with Islands and Buildings so CheckPrerequisites can access them
	if err := tx.Preload("Player").Preload("Player.Islands.Buildings").Preload("Buildings").First(&island, "id = ? AND player_id = ?", req.IslandID, playerID).Error; err != nil {
		tx.Rollback()
		errorMsg := "Island not found"
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
			playerID, req.IslandID, req.Type, errorMsg)
		return c.JSON(http.StatusNotFound, map[string]string{"error": errorMsg})
	}

	// CHECK GLOBAL CONSTRUCTION LIMIT
	for _, b := range island.Buildings {
		if b.Constructing {
			tx.Rollback()
			errorMsg := "Another building is already under construction"
			fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
				playerID, req.IslandID, req.Type, errorMsg)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
		}
	}

	// CHECK PREREQUISITES
	// Log TownHall status for debugging
	thLevel := 0
	thFound := false
	thConstructing := false
	for _, b := range island.Buildings {
		if b.Type == "Hôtel de Ville" {
			thFound = true
			thLevel = b.Level
			thConstructing = b.Constructing
			fmt.Printf("TownHall found: level=%d, constructing=%v, island_id=%s\n", thLevel, thConstructing, island.ID)
			break
		}
	}
	if !thFound {
		fmt.Printf("TownHall not found for island_id=%s\n", island.ID)
	}

	if err := economy.CheckPrerequisites(&island.Player, req.Type, 1); err != nil {
		tx.Rollback()
		errorMsg := err.Error()
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
			playerID, req.IslandID, req.Type, errorMsg)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
	}

	// Calculate Tech Bonuses
	var bonuses economy.TechBonuses
	var techs []string
	if len(island.Player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
		bonuses = economy.CalculateTechBonuses(techs)
	}

	// Update resources to now before checking (to be fair)
	// In a real game, we should update engine state first.
	// For now, let's just check against stored values.

	// Check costs
	for res, amount := range stats.Cost {
		if island.Resources[res] < amount {
			tx.Rollback()
			errorMsg := fmt.Sprintf("Not enough resources: need %.0f %s, have %.0f", amount, res, island.Resources[res])
			fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
				playerID, req.IslandID, req.Type, errorMsg)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Not enough resources"})
		}
	}

	// Deduct resources
	for res, amount := range stats.Cost {
		island.Resources[res] -= amount
	}

	reduction := bonuses.BuildTimeReduce
	if reduction > 0.9 {
		reduction = 0.9
	}
	buildDuration := stats.BuildTime.Seconds() * (1.0 - reduction)

	// Create building
	building := domain.Building{
		ID:           uuid.New(),
		IslandID:     island.ID,
		Type:         req.Type,
		Level:        0,
		X:            req.X,
		Y:            req.Y,
		Constructing: true,
		FinishTime:   time.Now().Add(time.Duration(buildDuration) * time.Second),
	}

	if err := tx.Create(&building).Error; err != nil {
		tx.Rollback()
		errorMsg := "Could not create building"
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s (db error: %v)\n",
			playerID, req.IslandID, req.Type, errorMsg, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		errorMsg := "Could not update island resources"
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s (db error: %v)\n",
			playerID, req.IslandID, req.Type, errorMsg, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	tx.Commit()

	// Log success
	fmt.Printf("Build success: player_id=%s, island_id=%s, building_id=%s, building_type=%s\n",
		playerID, req.IslandID, building.ID, req.Type)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Construction started",
		"building":  building,
		"resources": island.Resources,
	})
}

type UpgradeRequest struct {
	PlayerID   uuid.UUID `json:"player_id"`
	BuildingID uuid.UUID `json:"building_id"`
}

func Upgrade(c echo.Context) error {
	req := new(UpgradeRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := player.ID

	db := repository.GetDB()
	var building domain.Building

	// Find Building
	if err := db.First(&building, "id = ?", req.BuildingID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Building not found"})
	}

	// Find Island (Ownership Check) with Player Preload
	// Find Island with Player and Buildings (for checks)
	var island domain.Island
	if err := db.Preload("Player").Preload("Buildings").First(&island, "id = ?", building.IslandID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Island not found"})
	}
	// Use authenticated playerID, not req.PlayerID from client
	if island.PlayerID != playerID {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Not your building"})
	}

	if building.Constructing {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Already under construction"})
	}

	// CHECK GLOBAL CONSTRUCTION LIMIT (New Rule)
	for _, b := range island.Buildings {
		if b.Constructing {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Another building is already under construction"})
		}
	}

	nextLevel := building.Level + 1

	// Unmarshal Techs for Check
	if len(island.Player.UnlockedTechsJSON) > 0 {
		var techs []string
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
		island.Player.UnlockedTechs = techs
	}
	island.Player.Islands = []domain.Island{island} // Link for checker

	// NEW: Check Prerequisites
	if err := economy.CheckPrerequisites(&island.Player, building.Type, nextLevel); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	stats, err := economy.GetBuildingStats(building.Type, nextLevel)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot upgrade: " + err.Error()})
	}
	// Check Resources
	for res, amount := range stats.Cost {
		if island.Resources[res] < amount {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Not enough %s", res)})
		}
	}

	// Transaction
	tx := db.Begin()

	// Deduct Resources
	for res, amount := range stats.Cost {
		island.Resources[res] -= amount
	}

	// Calculate Tech Bonuses
	var bonuses economy.TechBonuses
	var techs []string
	if len(island.Player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
		bonuses = economy.CalculateTechBonuses(techs)
	}

	reduction := bonuses.BuildTimeReduce
	if reduction > 0.9 {
		reduction = 0.9
	}
	buildDuration := stats.BuildTime.Seconds() * (1.0 - reduction)

	// Apply Upgrade State
	building.Constructing = true
	building.FinishTime = time.Now().Add(time.Duration(buildDuration) * time.Second)

	fmt.Printf("[UPGRADE] Started for Building %s (Type: %s) to Lvl %d. Finish At: %v\n", building.ID, building.Type, nextLevel, building.FinishTime)

	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save resources"})
	}
	if err := tx.Save(&building).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save building"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Upgrade started",
		"building":  building,
		"resources": island.Resources,
	})
}

type ResetRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
}

func ResetProgress(c echo.Context) error {
	req := new(ResetRequest)
	if err := c.Bind(req); err != nil {
		errorMsg := "Invalid request"
		fmt.Printf("Reset failed: reason=%s\n", errorMsg)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		errorMsg := "Authentication required"
		fmt.Printf("Reset failed: reason=%s\n", errorMsg)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": errorMsg})
	}
	playerID := player.ID

	// Log reset request
	fmt.Printf("Reset request: player_id=%s\n", playerID)

	db := repository.GetDB()

	// Transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("Reset failed: player_id=%s, reason=panic: %v\n", playerID, r)
		}
	}()

	// 1. Find Island (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := tx.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		errorMsg := "Island not found"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusNotFound, map[string]string{"error": errorMsg})
	}

	// 2. Delete All Buildings (will recreate TownHall after)
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Building{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete buildings"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 3. Delete All Ships (they belong to the island)
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Ship{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete ships"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 4. Delete All Fleets (they belong to the island)
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Fleet{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete fleets"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 5. Reset Island to initial state (same as Register)
	island.Level = 1
	island.LastUpdated = time.Now()
	island.Resources = map[domain.ResourceType]float64{
		domain.Wood:  2500.0,
		domain.Gold:  3000.0,
		domain.Stone: 2500.0,
		domain.Rum:   1000.0,
	}
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to reset island"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 6. Reset Player Techs (same as initial state - empty, no research in progress)
	// Note: We do NOT recreate TownHall - after reset, the island is empty and the player
	// must manually start construction of TownHall as their first action (same as original design)
	// Load the player to use the proper model hooks for serialization
	var playerToReset domain.Player
	if err := tx.Where("id = ?", playerID).First(&playerToReset).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to load player for tech reset"
		fmt.Printf("Reset failed: player_id=%s, reason=%s (db error: %v)\n", playerID, errorMsg, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// Reset tech fields to initial state (empty unlocked techs, no research)
	playerToReset.UnlockedTechs = []string{} // Empty array
	playerToReset.UnlockedTechsJSON = []byte("[]") // Empty JSON array
	playerToReset.ResearchingTechID = ""
	playerToReset.ResearchFinishTime = time.Time{}
	playerToReset.UpdatedAt = time.Now()

	// Save using Save() which will trigger BeforeSave hook to properly serialize UnlockedTechs
	if err := tx.Save(&playerToReset).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to reset player techs"
		fmt.Printf("Reset failed: player_id=%s, reason=%s (db error: %v)\n", playerID, errorMsg, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	tx.Commit()

	// Log success
	fmt.Printf("Reset success: player_id=%s, island_id=%s\n", playerID, island.ID)

	// Return success - account is preserved, only progression is reset
	return c.JSON(http.StatusOK, map[string]string{"message": "progress reset"})
}

type AddResourcesRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
}

func AddResources(c echo.Context) error {
	req := new(AddResourcesRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var island domain.Island
	if err := tx.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Add 1000 to each resource
	if island.Resources == nil {
		island.Resources = make(map[domain.ResourceType]float64)
	}
	island.Resources[domain.Wood] += 1000
	island.Resources[domain.Stone] += 1000
	island.Resources[domain.Gold] += 1000
	island.Resources[domain.Rum] += 1000

	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save resources"})
	}

	tx.Commit()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Resources added",
		"resources": island.Resources,
	})
}

type StartResearchRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	TechID   string    `json:"tech_id"`
}

func StartResearch(c echo.Context) error {
	req := new(StartResearchRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	authenticatedPlayer := auth.GetAuthenticatedPlayer(c)
	if authenticatedPlayer == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := authenticatedPlayer.ID

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Load Player & Island (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := tx.Preload("Player").Preload("Buildings").Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	player := &island.Player

	// 2. Check Busy
	if player.ResearchingTechID != "" {
		tx.Rollback()
		return c.JSON(http.StatusConflict, map[string]string{"error": "Research already in progress"})
	}

	// 3. Load Tech Config
	tech, err := economy.GetTech(req.TechID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Tech ID"})
	}

	// 4. Check Already Unlocked
	var unlocked []string
	if len(player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(player.UnlockedTechsJSON, &unlocked)
	}
	for _, id := range unlocked {
		if id == req.TechID {
			tx.Rollback()
			return c.JSON(http.StatusConflict, map[string]string{"error": "Tech already unlocked"})
		}
	}

	// 5. Check Requirements (Buildings)
	maxTH := 0
	maxAcad := 0
	for _, b := range island.Buildings {
		if !b.Constructing { // Only completed buildings count? Usually yes.
			if b.Type == "Hôtel de Ville" && b.Level > maxTH {
				maxTH = b.Level
			}
			if b.Type == "Académie" && b.Level > maxAcad {
				maxAcad = b.Level
			}
		}
	}

	if maxTH < tech.ReqTH {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requires TownHall Level %d", tech.ReqTH)})
	}
	if maxAcad < tech.ReqAcad {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requires Academy Level %d", tech.ReqAcad)})
	}

	// 6. Check Resources
	cost := tech.Cost
	if island.Resources[domain.Wood] < cost.Wood ||
		island.Resources[domain.Stone] < cost.Stone ||
		island.Resources[domain.Gold] < cost.Gold ||
		island.Resources[domain.Rum] < cost.Rum {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Insufficient resources"})
	}

	// 7. Deduct Resources
	island.Resources[domain.Wood] -= cost.Wood
	island.Resources[domain.Stone] -= cost.Stone
	island.Resources[domain.Gold] -= cost.Gold
	island.Resources[domain.Rum] -= cost.Rum

	// 8. Start Research
	// Calculate Tech Bonuses (Research Speed)
	var unlockedList []string
	if len(player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(player.UnlockedTechsJSON, &unlockedList)
	}
	bonuses := economy.CalculateTechBonuses(unlockedList)

	reduction := bonuses.ResearchTimeReduce
	if reduction > 0.9 {
		reduction = 0.9
	} // Cap reduction
	finalDuration := float64(tech.TimeSec) * (1.0 - reduction)

	finishTime := time.Now().Add(time.Duration(finalDuration) * time.Second)
	player.ResearchingTechID = req.TechID
	player.ResearchFinishTime = finishTime

	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save resources"})
	}
	if err := tx.Save(player).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save player research"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Research started",
		"tech_id":     req.TechID,
		"finish_time": finishTime,
		"resources":   island.Resources,
	})
}

type StartShipConstructionRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	IslandID uuid.UUID `json:"island_id"`
	ShipType string    `json:"ship_type"`
}

func StartShipConstruction(c echo.Context) error {
	req := new(StartShipConstructionRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := player.ID

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Load Island with Player, Buildings and Ships (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := tx.Preload("Player").Preload("Buildings").Preload("Ships").First(&island, "id = ? AND player_id = ?", req.IslandID, playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// 2. Load Ship Config & Validate Requirements
	config, err := economy.GetShipStats(req.ShipType)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ship type: " + req.ShipType})
	}

	// 3. Check Prerequisite: Shipyard Level
	hasShipyard := false
	shipyardLevel := 0
	for _, b := range island.Buildings {
		if b.Type == "Chantier Naval" && !b.Constructing {
			hasShipyard = true
			shipyardLevel = b.Level
			break
		}
	}

	if !hasShipyard {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Shipyard required to build ships"})
	}

	if shipyardLevel < config.RequiredShipyardLevel {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requires Shipyard Level %d", config.RequiredShipyardLevel)})
	}

	// 4. Check Prerequisite: Technology
	if config.RequiredTechID != "" {
		hasTech := false
		var unlocked []string
		if len(island.Player.UnlockedTechsJSON) > 0 {
			_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &unlocked)
		}
		for _, id := range unlocked {
			if id == config.RequiredTechID {
				hasTech = true
				break
			}
		}
		if !hasTech {
			tx.Rollback()
			// Improve error message if possible to show Tech Name, but ID is safe for now
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requires technology: %s", config.RequiredTechID)})
		}
	}

	// 3. Check Limit (Max 3 ships for now) - Count only Active or Constructing, not Sunk/Destroyed (State?)
	activeShips := 0
	for _, s := range island.Ships {
		if s.State != "Destroyed" {
			activeShips++
		}
	}
	if activeShips >= 20 {
		tx.Rollback()
		return c.JSON(http.StatusConflict, map[string]string{"error": "Global ship limit reached (Max 20)"})
	}

	// 4. Check Construction Queue (Max 1 at a time)
	for _, s := range island.Ships {
		if s.State == "UnderConstruction" {
			tx.Rollback()
			return c.JSON(http.StatusConflict, map[string]string{"error": "Shipyard busy"})
		}
	}

	// 5. Get Config and Cost
	stats, err := economy.GetShipStats(req.ShipType)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ship type"})
	}

	// 5.5 Validate Prerequisites (Level & Tech)
	// Check Shipyard Level
	if shipyardLevel < stats.RequiredShipyardLevel {
		tx.Rollback()
		return c.JSON(http.StatusForbidden, map[string]string{"error": fmt.Sprintf("Shipyard Level %d required", stats.RequiredShipyardLevel)})
	}

	// Check Tech
	if stats.RequiredTechID != "" {
		hasTech := false
		var unlocked []string
		if len(island.Player.UnlockedTechsJSON) > 0 {
			_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &unlocked)
		}
		for _, t := range unlocked {
			if t == stats.RequiredTechID {
				hasTech = true
				break
			}
		}
		if !hasTech {
			tx.Rollback()
			return c.JSON(http.StatusForbidden, map[string]string{"error": fmt.Sprintf("Technology %s required", stats.RequiredTechID)})
		}
	}

	// 6. Check Resources
	for res, amount := range stats.Cost {
		if island.Resources[res] < amount {
			tx.Rollback()
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Not enough %s", res)})
		}
	}

	// 7. Deduct Resources
	for res, amount := range stats.Cost {
		island.Resources[res] -= amount
	}

	// 8. Calculate Time
	var techs []string
	if len(island.Player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
	}
	bonuses := economy.CalculateTechBonuses(techs)
	buildTimeSec := economy.CalculateShipBuildTime(req.ShipType, bonuses)
	finishTime := time.Now().Add(time.Duration(buildTimeSec) * time.Second)

	// 9. Create Ship (use authenticated playerID, not req.PlayerID)
	ship := domain.Ship{
		ID:         uuid.New(),
		PlayerID:   playerID,
		IslandID:   island.ID,
		Name:       stats.Name,
		Type:       req.ShipType,
		Health:     stats.MaxHealth,
		MaxHealth:  stats.MaxHealth,
		State:      "UnderConstruction",
		FinishTime: finishTime,
		// Position? At island for now.
		X: float64(island.X),
		Y: float64(island.Y),
	}

	if err := tx.Create(&ship).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create ship"})
	}

	// Save Resources
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save resources"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Ship construction started",
		"ship":        ship,
		"resources":   island.Resources,
		"finish_time": finishTime,
	})
}

// --- FLEET MANAGEMENT ---

type CreateFleetRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	IslandID uuid.UUID `json:"island_id"`
	Name     string    `json:"name"`
}

func CreateFleet(c echo.Context) error {
	req := new(CreateFleetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := player.ID

	db := repository.GetDB()
	var island domain.Island
	if err := db.Preload("Buildings").Preload("Fleets").First(&island, "id = ? AND player_id = ?", req.IslandID, playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// 1. Check Max Fleets
	shipyardLevel := 0
	for _, b := range island.Buildings {
		if b.Type == "Chantier Naval" && !b.Constructing {
			shipyardLevel = b.Level
			break
		}
	}

	maxFleets := economy.GetMaxFleets(shipyardLevel)
	if len(island.Fleets) >= maxFleets {
		return c.JSON(http.StatusConflict, map[string]string{"error": fmt.Sprintf("Max fleets reached (%d/%d)", len(island.Fleets), maxFleets)})
	}

	// 2. Create Fleet
	fleet := domain.Fleet{
		ID:       uuid.New(),
		IslandID: island.ID,
		Name:     req.Name,
	}
	if fleet.Name == "" {
		fleet.Name = fmt.Sprintf("Flotte %d", len(island.Fleets)+1)
	}

	if err := db.Create(&fleet).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create fleet"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Fleet created",
		"fleet":   fleet,
	})
}

type AddShipToFleetRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	FleetID  uuid.UUID `json:"fleet_id"`
	ShipID   uuid.UUID `json:"ship_id"`
}

func AddShipToFleet(c echo.Context) error {
	req := new(AddShipToFleetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	authenticatedPlayer := auth.GetAuthenticatedPlayer(c)
	if authenticatedPlayer == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := authenticatedPlayer.ID

	db := repository.GetDB()

	// 1. Load Fleet & Player Techs
	var fleet domain.Fleet
	if err := db.Preload("Ships").First(&fleet, "id = ?", req.FleetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Fleet not found"})
	}
	// Verify Ownership via Island -> Player (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := db.First(&island, "id = ?", fleet.IslandID).Error; err != nil || island.PlayerID != playerID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Not your fleet"})
	}

	// 2. Use authenticated player for Techs (already loaded from context)
	player := authenticatedPlayer

	// 3. Check Capacity
	maxShips := economy.GetMaxShipsPerFleet(player.UnlockedTechs)
	if len(fleet.Ships) >= maxShips {
		return c.JSON(http.StatusConflict, map[string]string{"error": fmt.Sprintf("Fleet is full (%d/%d)", len(fleet.Ships), maxShips)})
	}

	// 4. Find Ship (use authenticated playerID, not req.PlayerID)
	var ship domain.Ship
	if err := db.First(&ship, "id = ? AND player_id = ?", req.ShipID, playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Ship not found"})
	}
	// Check if already in a fleet (Optional: but logic implies moving or adding)
	// If ship.FleetID != nil { ... }

	// 5. Update Ship
	ship.FleetID = &fleet.ID
	if err := db.Save(&ship).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to add ship to fleet"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Ship added to fleet",
		"fleet":   fleet.ID,
	})
}

func GetFleets(c echo.Context) error {
	// Get authenticated player from context (set by RequireAuth middleware)
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
	}
	playerID := player.ID

	islandIDStr := c.QueryParam("island_id")
	islandID, err := uuid.Parse(islandIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid island_id"})
	}

	// Verify island ownership before returning fleets
	db := repository.GetDB()
	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", islandID, playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	var fleets []domain.Fleet
	if err := db.Preload("Ships").Where("island_id = ?", islandID).Find(&fleets).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch fleets"})
	}

	return c.JSON(http.StatusOK, fleets)
}

// --- Dev Handlers ---

type DevRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	Amount   float64   `json:"amount"` // For AddResources
	Hours    int       `json:"hours"`  // For TimeSkip
}

// checkDevAdmin verifies that the authenticated player from context is an admin
// Do NOT use this with client-provided player_id - always use authenticated player from context
func checkDevAdmin(player *domain.Player) error {
	if player == nil {
		return fmt.Errorf("authentication required")
	}
	if !player.IsAdmin {
		return fmt.Errorf("forbidden: admin only")
	}
	return nil
}

func DevAddResources(c echo.Context) error {
	req := new(DevRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	playerID := player.ID

	db := repository.GetDB()
	var island domain.Island
	if err := db.First(&island, "player_id = ?", playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Add Resources (1000 default if amount 0, else amount)
	amt := 1000.0
	if req.Amount > 0 {
		amt = req.Amount
	}

	for k := range island.Resources {
		island.Resources[k] += amt
	}
	// Also ensure defaults if map missing keys? (Island hooks handle this).
	// But hooks only run on Load. If map is empty here?
	// The DB fetch ran AfterFind, so Resources should be populated.
	// But if they are 0, they might not be in the map if map init was weird?
	// AfterFind ensures make().
	// Just explicitly set main ones just in case.
	island.Resources[domain.Wood] += amt
	island.Resources[domain.Stone] += amt
	island.Resources[domain.Rum] += amt
	island.Resources[domain.Gold] += amt

	if err := db.Save(&island).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Resources Added"})
}

func DevFinishBuilding(c echo.Context) error {
	req := new(DevRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	playerID := player.ID

	db := repository.GetDB()
	// Find Constructing Building (use authenticated playerID, not req.PlayerID)
	var b domain.Building
	if err := db.Where("island_id IN (SELECT id FROM islands WHERE player_id = ?) AND constructing = ?", playerID, true).First(&b).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"message": "No construction found"})
	}

	// Finish immediately - same logic as game loop
	// Game loop does: if constructing && now > finishTime { constructing=false; Level++ }
	// So we set constructing=false and increment level to match the normal completion flow
	b.Constructing = false
	if b.Level == 0 {
		b.Level = 1 // First construction: 0 -> 1
	} else {
		b.Level++ // Upgrade: current level -> next level
	}
	b.FinishTime = time.Time{} // Clear finish time

	if err := db.Save(&b).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Finished building %s (Level %d)", b.Type, b.Level)})
}

func DevFinishResearch(c echo.Context) error {
	req := new(DevRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}

	if player.ResearchingTechID == "" {
		return c.JSON(http.StatusOK, map[string]string{"message": "No research active"})
	}

	player.ResearchFinishTime = time.Now().Add(-1 * time.Hour)
	repository.GetDB().Save(player) // Ignoring err for brevity

	return c.JSON(http.StatusOK, map[string]string{"message": "Research Finished"})
}

func DevFinishShip(c echo.Context) error {
	req := new(DevRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	playerID := player.ID

	db := repository.GetDB()
	var s domain.Ship
	if err := db.Where("player_id = ? AND state = ?", playerID, "UnderConstruction").First(&s).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"message": "No ship under construction"})
	}

	s.FinishTime = time.Now().Add(-1 * time.Hour)
	db.Save(&s)

	return c.JSON(http.StatusOK, map[string]string{"message": "Ship Construction Finished"})
}

func DevTimeSkip(c echo.Context) error {
	req := new(DevRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}
	hours := 1
	if req.Hours > 0 {
		hours = req.Hours
	}

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	playerID := player.ID

	db := repository.GetDB()

	duration := time.Duration(hours) * time.Hour

	// 1. Move Island LastUpdated BACK (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := db.First(&island, "player_id = ?", playerID).Error; err == nil {
		island.LastUpdated = island.LastUpdated.Add(-duration)
		db.Save(&island)
	}

	// 2. Move active timers BACK (so they finish sooner relative to Now)
	// Actually: If FinishTime is 15:00. Now is 14:00.
	// Using "Skip 1h", we conceptually jump to 15:00.
	// This means everything scheduled for future should be "brought closer".
	// FinishTime -= duration.

	db.Model(&domain.Building{}).Where("island_id = ? AND constructing = ?", island.ID, true).Update("finish_time", gorm.Expr("finish_time - ?", duration)) // GORM calc? or load/save.
	// Postgres interval? Sqlite?
	// Safer to run loops in code for compatibility.

	// Buildings
	var buildings []domain.Building
	db.Where("island_id = ? AND constructing = ?", island.ID, true).Find(&buildings)
	for _, b := range buildings {
		b.FinishTime = b.FinishTime.Add(-duration)
		db.Save(&b)
	}

	// Research (use authenticated player, not req.PlayerID)
	if player.ResearchingTechID != "" {
		player.ResearchFinishTime = player.ResearchFinishTime.Add(-duration)
		db.Save(player)
	}

	// Ships (use authenticated playerID, not req.PlayerID)
	var ships []domain.Ship
	db.Where("player_id = ? AND state = ?", playerID, "UnderConstruction").Find(&ships)
	for _, s := range ships {
		s.FinishTime = s.FinishTime.Add(-duration)
		db.Save(&s)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Skipped %d hours", hours)})
}
