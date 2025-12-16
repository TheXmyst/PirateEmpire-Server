package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
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

	// Ensure player has 3 fleets (after transaction commit)
	if err := ensurePlayerFleets(db, &island); err != nil {
		fmt.Printf("[FLEET] Register: Failed to create initial fleets: %v\n", err)
		// Continue anyway - fleets can be created later
	}

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

	// Ensure player has 3 fleets (migration path for old accounts)
	if err := ensurePlayerFleets(db, &island); err != nil {
		fmt.Printf("[FLEET] Login: Failed to ensure fleets: %v\n", err)
		// Continue anyway - fleets can be created later
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

	// Preload Islands, Buildings, Ships AND Fleets
	if err := db.Preload("Islands.Buildings").Preload("Islands.Ships").Preload("Islands.Fleets.Ships").First(&player, "id = ?", playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Player not found"})
	}

	// Ensure player has 3 fleets (create missing ones) - migration path for old accounts
	if len(player.Islands) > 0 {
		island := &player.Islands[0]
		if err := ensurePlayerFleets(db, island); err != nil {
			fmt.Printf("[FLEET] GetStatus: Failed to ensure fleets: %v\n", err)
			// Continue anyway - try to reload fleets
		} else {
			// Reload fleets after ensuring they exist
			if err := db.Preload("Ships").Where("island_id = ?", island.ID).Find(&island.Fleets).Error; err != nil {
				fmt.Printf("[FLEET] GetStatus: Failed to reload fleets: %v\n", err)
			}
		}
		// Log fleets being sent to client
		fleetInfo := make([]string, 0, len(island.Fleets))
		for _, f := range island.Fleets {
			fleetInfo = append(fleetInfo, fmt.Sprintf("(%s,%s)", f.ID.String(), f.Name))
		}
		fmt.Printf("[STATUS] player=%s island=%s fleets=%d ids=%v\n",
			player.ID.String(), island.ID.String(), len(island.Fleets), fleetInfo)

		// Debug log for crew counts (only when DEBUG or CAPTAIN_DEBUG is enabled)
		if os.Getenv("DEBUG") != "" || os.Getenv("CAPTAIN_DEBUG") != "" {
			for _, fleet := range island.Fleets {
				for _, ship := range fleet.Ships {
					if ship.CrewWarriors > 0 || ship.CrewArchers > 0 || ship.CrewGunners > 0 {
						fmt.Printf("[STATUS] ship_id=%s crew: warriors=%d archers=%d gunners=%d\n",
							ship.ID, ship.CrewWarriors, ship.CrewArchers, ship.CrewGunners)
					}
				}
			}
		}
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
			player.ResearchTotalDurationSeconds = 0 // Reset when research completes

			// Save and continue (so returned JSON is updated)
			db.Save(&player)
			fmt.Printf("[LAZY READ] Research Complete: %s\n", techID)
		}
	}

	// Calculate resources for each island
	now := time.Now()
	const StatusCheckpointInterval = 5 * time.Second

	for i := range player.Islands {
		island := &player.Islands[i]

		// CRITICAL FIX: Link the loaded player to the island so CalculateResources sees Techs
		// And so we don't accidentally save an empty player struct if GORM cascades.
		island.Player = player

		elapsed := now.Sub(island.LastUpdated)

		if elapsed > 0 {
			engine.CalculateResources(island, elapsed)
			// Always update LastUpdated in memory for accurate resource calculation
			// This ensures resources are calculated correctly even if we don't persist yet
			island.LastUpdated = now

			// CHECKPOINT THROTTLING: Only persist island to DB every 5 seconds max
			// This reduces DB writes from /status polling (typically 1-2s) to ~1 write per 5s per island
			// Resources remain accurate in memory; max loss on crash is ≤5s of production (acceptable)
			shouldSave := false
			if island.LastCheckpointSavedAt == nil {
				// First time or never saved: save immediately
				shouldSave = true
			} else {
				// Check if 5 seconds have passed since last checkpoint
				timeSinceLastCheckpoint := now.Sub(*island.LastCheckpointSavedAt)
				if timeSinceLastCheckpoint >= StatusCheckpointInterval {
					shouldSave = true
				}
			}

			if shouldSave {
				// Update checkpoint timestamp before saving
				island.LastCheckpointSavedAt = &now

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
				repository.RetryWrite(func() error {
					return db.Omit("Player").Save(island).Error
				}, 3)

				// Optional debug log (only if DEBUG enabled)
				if os.Getenv("DEBUG") != "" {
					fmt.Printf("[STATUS] Checkpoint saved: island=%s last_checkpoint=%v\n", island.ID, now)
				}
			}
		}

		// Process Militia recruitment completion
		if economy.ProcessMilitiaRecruitment(island, now) {
			// Recruitment completed - save island to persist crew stock
			// Force save even if checkpoint not reached (recruitment is important)
			repository.RetryWrite(func() error {
				return db.Omit("Player").Save(island).Error
			}, 3)
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
			// HOTFIX: Update Milice position
			if island.Buildings[j].Type == "Milice" {
				if island.Buildings[j].X != -887 || island.Buildings[j].Y != -253 {
					island.Buildings[j].X = -887
					island.Buildings[j].Y = -253
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

	if err := tx.Preload("Player").Preload("Player.Islands.Buildings").Preload("Buildings").First(&island, "id = ? AND player_id = ?", req.IslandID, playerID).Error; err != nil {
		tx.Rollback()
		errorMsg := "Island not found"
		fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
			playerID, req.IslandID, req.Type, errorMsg)
		return c.JSON(http.StatusNotFound, map[string]string{"error": errorMsg})
	}

	// CHECK HDV LEVEL (Global Rule)
	if err := canUpgradeBuilding(&island, req.Type, 0); err != nil {
		tx.Rollback()
		return c.JSON(http.StatusConflict, err)
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

	// CHECK ONE-PER-ISLAND FOR TAVERN
	if req.Type == "Tavern" {
		for _, b := range island.Buildings {
			if b.Type == "Tavern" && !b.Constructing {
				tx.Rollback()
				errorMsg := "Tavern already built on this island"
				fmt.Printf("Build failed: player_id=%s, island_id=%s, building_type=%s, reason=%s\n",
					playerID, req.IslandID, req.Type, errorMsg)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": errorMsg})
			}
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

	// Calculate Tech Bonuses (New System: TechModifiers)
	var mods economy.TechModifiers
	var techs []string
	if len(island.Player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
		mods = economy.ComputeTechModifiers(techs)
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

	reduction := mods.BuildTimeReduction
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

	// CHECK: Tavern cannot be upgraded
	if building.Type == "Tavern" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Tavern cannot be upgraded."})
	}

	// CHECK HDV LEVEL (Global Rule)
	if err := canUpgradeBuilding(&island, building.Type, building.Level); err != nil {
		return c.JSON(http.StatusConflict, err)
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

	// NEW: Check Prerequisites with structured errors
	missingReqs := economy.ValidateBuildingPrerequisites(&island.Player, &island, building.Type, nextLevel)
	if len(missingReqs) > 0 {
		return WriteRequirementsError(c, http.StatusConflict, missingReqs)
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

	// Calculate Tech Bonuses (New System: TechModifiers)
	var mods economy.TechModifiers
	var techs []string
	if len(island.Player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(island.Player.UnlockedTechsJSON, &techs)
		mods = economy.ComputeTechModifiers(techs)
	}

	reduction := mods.BuildTimeReduction
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
	fmt.Printf("[RESET] player=%s started\n", playerID)

	db := repository.GetDB()

	// Transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("Reset failed: player_id=%s, reason=panic: %v\n", playerID, r)
		}
	}()

	// 0. Check cooldown (24h anti-farm protection)
	var playerToCheck domain.Player
	if err := tx.Where("id = ?", playerID).First(&playerToCheck).Error; err != nil {
		tx.Rollback()
		errorMsg := "Player not found"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusNotFound, map[string]string{"error": errorMsg})
	}

	if playerToCheck.LastResetAt != nil {
		timeSinceReset := time.Since(*playerToCheck.LastResetAt)
		cooldownDuration := 24 * time.Hour
		if timeSinceReset < cooldownDuration {
			remaining := cooldownDuration - timeSinceReset
			hours := int(remaining.Hours())
			minutes := int(remaining.Minutes()) % 60
			tx.Rollback()
			errorMsg := fmt.Sprintf("Reset disponible dans %dh%dm.", hours, minutes)
			fmt.Printf("[RESET] player=%s blocked: cooldown remaining=%v\n", playerID, remaining)
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": errorMsg})
		}
	}

	// 1. Find Island (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := tx.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		errorMsg := "Island not found"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusNotFound, map[string]string{"error": errorMsg})
	}

	// 2. Count and Delete All Buildings
	var deletedBuildings int64
	if err := tx.Model(&domain.Building{}).Where("island_id = ?", island.ID).Count(&deletedBuildings).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to count buildings"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Building{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete buildings"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 3. Count and Delete All Ships (they belong to the island)
	var deletedShips int64
	if err := tx.Model(&domain.Ship{}).Where("island_id = ?", island.ID).Count(&deletedShips).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to count ships"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Ship{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete ships"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 4. Count and Delete All Fleets (they belong to the island)
	var deletedFleets int64
	if err := tx.Model(&domain.Fleet{}).Where("island_id = ?", island.ID).Count(&deletedFleets).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to count fleets"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}
	if err := tx.Where("island_id = ?", island.ID).Delete(&domain.Fleet{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete fleets"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 5. Count and Delete All Captains (they belong to the player)
	var deletedCaptains int64
	if err := tx.Model(&domain.Captain{}).Where("player_id = ?", playerID).Count(&deletedCaptains).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to count captains"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}
	if err := tx.Where("player_id = ?", playerID).Delete(&domain.Captain{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete captains"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 5.5. Count and Delete All Captain Shard Wallets (they belong to the player)
	var deletedShardWallets int64
	if err := tx.Model(&domain.CaptainShardWallet{}).Where("player_id = ?", playerID).Count(&deletedShardWallets).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to count shard wallets"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}
	if err := tx.Where("player_id = ?", playerID).Delete(&domain.CaptainShardWallet{}).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to delete shard wallets"
		fmt.Printf("Reset failed: player_id=%s, reason=%s\n", playerID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// Note: We will recreate the 3 fleets after the transaction commits

	// 6. Reset Island to initial state (same as Register)
	island.Level = 1

	island.LastUpdated = time.Now()
	island.Resources = map[domain.ResourceType]float64{
		domain.Wood:          2500.0,
		domain.Gold:          3000.0,
		domain.Stone:         2500.0,
		domain.Rum:           1000.0,
		domain.CaptainTicket: 0.0, // Explicitly reset tickets to 0
	}
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to reset island"
		fmt.Printf("Reset failed: player_id=%s, island_id=%s, reason=%s\n", playerID, island.ID, errorMsg)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	// 7. Reset Player Techs (same as initial state - empty, no research in progress)
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
	playerToReset.UnlockedTechs = []string{}       // Empty array
	playerToReset.UnlockedTechsJSON = []byte("[]") // Empty JSON array
	playerToReset.ResearchingTechID = ""
	playerToReset.ResearchFinishTime = time.Time{}
	// Reset pity counters
	playerToReset.PityLegendaryCount = 0
	playerToReset.PityRareCount = 0
	// Set LastResetAt to now (cooldown tracking)
	now := time.Now()
	playerToReset.LastResetAt = &now
	playerToReset.UpdatedAt = time.Now()

	// Save using Save() which will trigger BeforeSave hook to properly serialize UnlockedTechs
	if err := tx.Save(&playerToReset).Error; err != nil {
		tx.Rollback()
		errorMsg := "Failed to reset player techs"
		fmt.Printf("Reset failed: player_id=%s, reason=%s (db error: %v)\n", playerID, errorMsg, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errorMsg})
	}

	tx.Commit()

	// After transaction commit, recreate the 3 fleets
	if err := ensurePlayerFleets(db, &island); err != nil {
		fmt.Printf("Reset: Failed to recreate fleets: %v\n", err)
		// Continue anyway - fleets can be created later
	}

	// Log success with counts
	fmt.Printf("[RESET] player=%s ok=true deleted_buildings=%d deleted_ships=%d deleted_fleets=%d deleted_captains=%d deleted_shard_wallets=%d\n",
		playerID, deletedBuildings, deletedShips, deletedFleets, deletedCaptains, deletedShardWallets)
	fmt.Printf("[RESET] player=%s success island=%s\n", playerID, island.ID)

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

	// 5. Check Prerequisites with structured errors
	missingReqs := economy.ValidateResearchPrerequisites(player, &island, req.TechID)
	if len(missingReqs) > 0 {
		tx.Rollback()
		fmt.Printf("[PREREQ] research blocked player_id=%s tech=%s missing=%d\n", playerID, req.TechID, len(missingReqs))
		return WriteRequirementsError(c, http.StatusConflict, missingReqs)
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
	// Calculate Tech Bonuses (New System: TechModifiers)
	var unlockedList []string
	if len(player.UnlockedTechsJSON) > 0 {
		_ = json.Unmarshal(player.UnlockedTechsJSON, &unlockedList)
	}
	mods := economy.ComputeTechModifiers(unlockedList)

	// Calculate Academy Research Bonus
	academyBonus := economy.CalculateAcademyResearchBonus(maxAcad)

	// Combine tech bonus and academy bonus
	totalReduction := mods.ResearchTimeReduction + academyBonus
	if totalReduction > 0.9 {
		totalReduction = 0.9
	} // Cap total reduction at 90%

	baseTime := float64(tech.TimeSec)
	finalDuration := baseTime * (1.0 - totalReduction)

	// Store the final duration in seconds for client progress bar
	player.ResearchTotalDurationSeconds = finalDuration

	finishTime := time.Now().Add(time.Duration(finalDuration) * time.Second)
	player.ResearchingTechID = req.TechID
	player.ResearchFinishTime = finishTime

	// Debug logging
	fmt.Printf("[TECH] StartResearch: tech=%s academyLevel=%d base=%.2fs bonusTech=%.3f bonusAcademy=%.3f totalReduction=%.3f final=%.2fs\n",
		req.TechID, maxAcad, baseTime, mods.ResearchTimeReduction, academyBonus, totalReduction, finalDuration)

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

	// 2. Check Prerequisites with structured errors (before loading config)
	missingReqs := economy.ValidateShipPrerequisites(&island.Player, &island, req.ShipType)
	if len(missingReqs) > 0 {
		tx.Rollback()
		fmt.Printf("[PREREQ] ship build blocked player_id=%s ship=%s missing=%d\n", playerID, req.ShipType, len(missingReqs))
		return WriteRequirementsError(c, http.StatusConflict, missingReqs)
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

	// Prerequisites already validated above

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
	mods := economy.ComputeTechModifiers(techs)
	buildTimeSec := economy.CalculateShipBuildTime(req.ShipType, mods)
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
		fmt.Printf("[FLEET] AddShipToFleet: Invalid request: %v\n", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	fmt.Printf("[FLEET] AddShipToFleet request: fleet_id=%s, ship_id=%s\n", req.FleetID, req.ShipID)

	// Get authenticated player from context (set by RequireAuth middleware)
	// Do NOT trust req.PlayerID from client - use authenticated player only
	authenticatedPlayer := auth.GetAuthenticatedPlayer(c)
	if authenticatedPlayer == nil {
		fmt.Printf("[FLEET] AddShipToFleet: No authenticated player\n")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := authenticatedPlayer.ID
	fmt.Printf("[FLEET] AddShipToFleet: Authenticated player_id=%s\n", playerID)

	db := repository.GetDB()

	// Start transaction with SELECT FOR UPDATE for authoritative validation
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[FLEET] AddShipToFleet: Panic recovered: %v\n", r)
		}
	}()

	// 1. Load Fleet & Player Techs with FOR UPDATE (row-level lock)
	var fleet domain.Fleet
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Preload("Ships").First(&fleet, "id = ?", req.FleetID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Fleet not found: fleet_id=%s, error=%v\n", req.FleetID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable"})
	}
	fmt.Printf("[FLEET] AddShipToFleet: Fleet found: fleet_id=%s, island_id=%s, total_ships=%d\n", fleet.ID, fleet.IslandID, len(fleet.Ships))

	// Verify Ownership via Island -> Player (use authenticated playerID, not req.PlayerID)
	var island domain.Island
	if err := tx.First(&island, "id = ?", fleet.IslandID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Island not found: island_id=%s, error=%v\n", fleet.IslandID, err)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Île introuvable"})
	}
	if island.PlayerID != playerID {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Ownership mismatch: island.player_id=%s, authenticated=%s\n", island.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Cette flotte ne vous appartient pas"})
	}

	// 2. Use authenticated player for Techs (already loaded from context)
	player := authenticatedPlayer

	// Check if fleet is unlocked based on tech requirements
	if !isFleetUnlocked(fleet.Name, player.UnlockedTechs) {
		// Determine which tech is needed
		var requiredTech string
		var techName string
		if fleet.Name == "Flotte 2" {
			requiredTech = "nav_fleet_1"
			techName = "Amirauté"
		} else if fleet.Name == "Flotte 3" {
			requiredTech = "nav_fleet_2"
			techName = "Grande Armada"
		}

		if requiredTech != "" {
			fmt.Printf("[FLEET] AddShipToFleet: Fleet is locked: fleet_name=%s, required_tech=%s\n", fleet.Name, requiredTech)
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": fmt.Sprintf("Cette flotte est verrouillée. Débloquez-la via la technologie '%s' (%s).", techName, requiredTech),
			})
		}
		// Fallback for unknown fleet names
		fmt.Printf("[FLEET] AddShipToFleet: Unknown fleet name: %s\n", fleet.Name)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Flotte invalide"})
	}

	// 3. Check Capacity using active ships only (authoritative server-side validation)
	mods := economy.ComputeTechModifiers(player.UnlockedTechs)
	maxShips := economy.GetMaxShipsPerFleet(mods)
	activeShips := economy.GetActiveFleetShips(&fleet)
	activeCount := len(activeShips)

	// Count ships under construction separately for logging
	underConstructionCount := 0
	for _, ship := range activeShips {
		if ship.State == "UnderConstruction" {
			underConstructionCount++
		}
	}

	// Detailed diagnostic: log each ship's state for debugging
	fmt.Printf("[FLEET] AddShipToFleet: Fleet %s has %d total ships in DB\n", fleet.ID.String()[:8], len(fleet.Ships))
	for i, ship := range fleet.Ships {
		fmt.Printf("[FLEET] AddShipToFleet: Ship[%d] id=%s name=%s state=%s health=%.0f fleet_id=%v\n",
			i, ship.ID.String()[:8], ship.Name, ship.State, ship.Health, ship.FleetID)
	}
	fmt.Printf("[FLEET] AddShipToFleet: Capacity check: active=%d (uc=%d) total=%d max=%d\n", activeCount, underConstructionCount, len(fleet.Ships), maxShips)
	if activeCount >= maxShips {
		fmt.Printf("[FLEET] AddShipToFleet: Fleet is full (active=%d >= max=%d)\n", activeCount, maxShips)
		tx.Rollback()
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":       "FLEET_FULL",
			"reason_code": "FLEET_FULL",
			"message":     fmt.Sprintf("Flotte pleine (%d/%d)", activeCount, maxShips),
			"active":      activeCount,
			"max":         maxShips,
			"total":       len(fleet.Ships),
		})
	}

	// 4. Find Ship (use authenticated playerID, not req.PlayerID) with FOR UPDATE
	var ship domain.Ship
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&ship, "id = ? AND player_id = ?", req.ShipID, playerID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Ship not found: ship_id=%s, player_id=%s, error=%v\n", req.ShipID, playerID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}
	fmt.Printf("[FLEET] AddShipToFleet: Ship found: ship_id=%s, name=%s, type=%s, state=%s, current_fleet_id=%v\n", ship.ID, ship.Name, ship.Type, ship.State, ship.FleetID)

	// Check if ship is under construction
	if ship.State == "UnderConstruction" {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Ship is under construction\n")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Le navire est en cours de construction"})
	}

	// Check if already in a fleet
	if ship.FleetID != nil {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Ship already in fleet: fleet_id=%s\n", *ship.FleetID)
		return c.JSON(http.StatusConflict, map[string]string{"error": "Le navire est déjà assigné à une flotte"})
	}

	// 5. Update Ship
	ship.FleetID = &fleet.ID
	if err := tx.Save(&ship).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[FLEET] AddShipToFleet: Failed to save ship: error=%v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de l'assignation"})
	}

	tx.Commit()
	fmt.Printf("[FLEET] AddShipToFleet: Success! Ship %s added to fleet %s (active=%d/%d)\n", ship.ID, fleet.ID, activeCount+1, maxShips)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Navire assigné à la flotte",
		"fleet":   fleet.ID,
	})
}

// AssignCrewRequest is the request for POST /fleets/assign-crew
type AssignCrewRequest struct {
	ShipID   uuid.UUID `json:"ship_id"`
	Warriors int       `json:"warriors"`
	Archers  int       `json:"archers"`
	Gunners  int       `json:"gunners"`
}

// AssignCrewResponse is the response for POST /fleets/assign-crew
type AssignCrewResponse struct {
	Message    string `json:"message"`
	ShipID     string `json:"ship_id"`
	Warriors   int    `json:"warriors"`
	Archers    int    `json:"archers"`
	Gunners    int    `json:"gunners"`
	StockAfter struct {
		Warriors int `json:"warriors"`
		Archers  int `json:"archers"`
		Gunners  int `json:"gunners"`
	} `json:"stock_after"`
}

// SetActiveFleetRequest is the request for POST /fleets/set-active
type SetActiveFleetRequest struct {
	FleetID string `json:"fleet_id"`
}

// SetActiveFleet sets the active fleet for PvE
func SetActiveFleet(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	req := new(SetActiveFleetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID de flotte invalide"})
	}

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer tx.Rollback()

	// Loading island
	var island domain.Island
	if err := tx.Where("player_id = ?", player.ID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Verify fleet ownership and existence
	var fleet domain.Fleet
	if err := tx.Where("id = ? AND island_id = ?", fleetID, island.ID).First(&fleet).Error; err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Flotte introuvable ou ne vous appartient pas"})
	}

	// Update active fleet
	island.ActiveFleetID = &fleetID
	if err := tx.Save(&island).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":              true,
		"active_fleet_id": fleetID.String(),
	})
}

// AssignCrew assigns crew from island stock to a ship
func AssignCrew(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}
	playerID := player.ID

	// Parse request
	req := new(AssignCrewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	// Validate quantities
	if req.Warriors < 0 || req.Archers < 0 || req.Gunners < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Les quantités ne peuvent pas être négatives"})
	}
	totalToAssign := req.Warriors + req.Archers + req.Gunners
	if totalToAssign <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Vous devez assigner au moins un matelot"})
	}

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[CREW] AssignCrew: Panic recovered: %v\n", r)
		}
	}()

	// Load ship with FOR UPDATE
	var ship domain.Ship
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&ship, "id = ? AND player_id = ?", req.ShipID, playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}

	// Verify ship is active and not destroyed
	if ship.State == "Destroyed" || ship.Health <= 0 {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Le navire est détruit"})
	}

	// Check if ship is in a locked fleet
	if ship.FleetID != nil {
		var fleet domain.Fleet
		if err := tx.First(&fleet, "id = ?", *ship.FleetID).Error; err == nil {
			if economy.IsFleetLocked(&fleet) {
				tx.Rollback()
				return c.JSON(http.StatusConflict, map[string]string{"error": "La flotte est verrouillée (combat en cours)"})
			}
		}
	}

	// Load island with FOR UPDATE
	var island domain.Island
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&island, "id = ? AND player_id = ?", ship.IslandID, playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Check stock availability
	if island.CrewWarriors < req.Warriors {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Stock insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_STOCK",
			"reason_message": fmt.Sprintf("Guerriers insuffisants (nécessaire: %d, disponible: %d)", req.Warriors, island.CrewWarriors),
		})
	}
	if island.CrewArchers < req.Archers {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Stock insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_STOCK",
			"reason_message": fmt.Sprintf("Archers insuffisants (nécessaire: %d, disponible: %d)", req.Archers, island.CrewArchers),
		})
	}
	if island.CrewGunners < req.Gunners {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Stock insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_STOCK",
			"reason_message": fmt.Sprintf("Artilleurs insuffisants (nécessaire: %d, disponible: %d)", req.Gunners, island.CrewGunners),
		})
	}

	// Calculate new crew totals
	newWarriors := ship.CrewWarriors + req.Warriors
	newArchers := ship.CrewArchers + req.Archers
	newGunners := ship.CrewGunners + req.Gunners

	// Validate capacity
	maxCrew := economy.MaxCrewForShipType(ship.Type)
	totalCrew := newWarriors + newArchers + newGunners
	if totalCrew > maxCrew {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Capacité dépassée",
			"reason_code":    "CREW_OVER_CAPACITY",
			"reason_message": fmt.Sprintf("Capacité maximale pour un %s: %d (actuel: %d, assignation: %d)", ship.Type, maxCrew, economy.CrewTotal(&ship), totalCrew),
		})
	}

	// Deduct from island stock
	island.CrewWarriors -= req.Warriors
	island.CrewArchers -= req.Archers
	island.CrewGunners -= req.Gunners

	// Ensure stock doesn't go negative (defensive)
	if island.CrewWarriors < 0 {
		island.CrewWarriors = 0
	}
	if island.CrewArchers < 0 {
		island.CrewArchers = 0
	}
	if island.CrewGunners < 0 {
		island.CrewGunners = 0
	}

	// Update ship crew
	ship.CrewWarriors = newWarriors
	ship.CrewArchers = newArchers
	ship.CrewGunners = newGunners

	// Save both
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde de l'île"})
	}
	if err := tx.Save(&ship).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde du navire"})
	}

	tx.Commit()

	fmt.Printf("[CREW] AssignCrew: player=%s ship=%s warriors=%d archers=%d gunners=%d\n",
		playerID.String()[:8], ship.ID.String()[:8], req.Warriors, req.Archers, req.Gunners)

	// Build response
	response := AssignCrewResponse{
		Message:  "Équipage assigné",
		ShipID:   ship.ID.String(),
		Warriors: ship.CrewWarriors,
		Archers:  ship.CrewArchers,
		Gunners:  ship.CrewGunners,
	}
	response.StockAfter.Warriors = island.CrewWarriors
	response.StockAfter.Archers = island.CrewArchers
	response.StockAfter.Gunners = island.CrewGunners

	return c.JSON(http.StatusOK, response)
}

// UnassignCrewRequest is the request for POST /fleets/unassign-crew
type UnassignCrewRequest struct {
	ShipID   uuid.UUID `json:"ship_id"`
	Warriors int       `json:"warriors"`
	Archers  int       `json:"archers"`
	Gunners  int       `json:"gunners"`
}

// UnassignCrewResponse is the response for POST /fleets/unassign-crew
type UnassignCrewResponse struct {
	Message    string `json:"message"`
	ShipID     string `json:"ship_id"`
	Warriors   int    `json:"warriors"`
	Archers    int    `json:"archers"`
	Gunners    int    `json:"gunners"`
	StockAfter struct {
		Warriors int `json:"warriors"`
		Archers  int `json:"archers"`
		Gunners  int `json:"gunners"`
	} `json:"stock_after"`
}

// UnassignCrew removes crew from a ship and returns it to island stock
func UnassignCrew(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}
	playerID := player.ID

	// Parse request
	req := new(UnassignCrewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	// Validate quantities
	if req.Warriors < 0 || req.Archers < 0 || req.Gunners < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Les quantités ne peuvent pas être négatives"})
	}
	totalToUnassign := req.Warriors + req.Archers + req.Gunners
	if totalToUnassign <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Vous devez retirer au moins un matelot"})
	}

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[CREW] UnassignCrew: Panic recovered: %v\n", r)
		}
	}()

	// Load ship with FOR UPDATE
	var ship domain.Ship
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&ship, "id = ? AND player_id = ?", req.ShipID, playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}

	// Verify ship is active and not destroyed
	if ship.State == "Destroyed" || ship.Health <= 0 {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Le navire est détruit"})
	}

	// Check if ship is in a locked fleet
	if ship.FleetID != nil {
		var fleet domain.Fleet
		if err := tx.First(&fleet, "id = ?", *ship.FleetID).Error; err == nil {
			if economy.IsFleetLocked(&fleet) {
				tx.Rollback()
				return c.JSON(http.StatusConflict, map[string]string{"error": "La flotte est verrouillée (combat en cours)"})
			}
		}
	}

	// Check if ship has enough crew to unassign
	if ship.CrewWarriors < req.Warriors {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Équipage insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_ON_SHIP",
			"reason_message": fmt.Sprintf("Guerriers insuffisants sur le navire (nécessaire: %d, disponible: %d)", req.Warriors, ship.CrewWarriors),
		})
	}
	if ship.CrewArchers < req.Archers {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Équipage insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_ON_SHIP",
			"reason_message": fmt.Sprintf("Archers insuffisants sur le navire (nécessaire: %d, disponible: %d)", req.Archers, ship.CrewArchers),
		})
	}
	if ship.CrewGunners < req.Gunners {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          "Équipage insuffisant",
			"reason_code":    "CREW_INSUFFICIENT_ON_SHIP",
			"reason_message": fmt.Sprintf("Artilleurs insuffisants sur le navire (nécessaire: %d, disponible: %d)", req.Gunners, ship.CrewGunners),
		})
	}

	// Load island with FOR UPDATE
	var island domain.Island
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&island, "id = ? AND player_id = ?", ship.IslandID, playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Remove from ship (clamp to non-negative)
	newWarriors := ship.CrewWarriors - req.Warriors
	newArchers := ship.CrewArchers - req.Archers
	newGunners := ship.CrewGunners - req.Gunners

	if newWarriors < 0 {
		newWarriors = 0
	}
	if newArchers < 0 {
		newArchers = 0
	}
	if newGunners < 0 {
		newGunners = 0
	}

	// Return to island stock
	island.CrewWarriors += req.Warriors
	island.CrewArchers += req.Archers
	island.CrewGunners += req.Gunners

	// Update ship crew
	ship.CrewWarriors = newWarriors
	ship.CrewArchers = newArchers
	ship.CrewGunners = newGunners

	// Save both
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde de l'île"})
	}
	if err := tx.Save(&ship).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde du navire"})
	}

	tx.Commit()

	fmt.Printf("[CREW] UnassignCrew: player=%s ship=%s warriors=%d archers=%d gunners=%d\n",
		playerID.String()[:8], ship.ID.String()[:8], req.Warriors, req.Archers, req.Gunners)

	// Build response
	response := UnassignCrewResponse{
		Message:  "Équipage retiré",
		ShipID:   ship.ID.String(),
		Warriors: ship.CrewWarriors,
		Archers:  ship.CrewArchers,
		Gunners:  ship.CrewGunners,
	}
	response.StockAfter.Warriors = island.CrewWarriors
	response.StockAfter.Archers = island.CrewArchers
	response.StockAfter.Gunners = island.CrewGunners

	return c.JSON(http.StatusOK, response)
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

	// Ensure player has 3 fleets (create missing ones)
	if err := ensurePlayerFleets(db, &island); err != nil {
		fmt.Printf("[FLEET] GetFleets: Failed to ensure fleets: %v\n", err)
		// Continue anyway - return what we have
	}

	// Reload fleets after ensuring they exist
	if err := db.Preload("Ships").Where("island_id = ?", islandID).Find(&fleets).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch fleets"})
	}

	// Sort fleets by name to ensure consistent order (Flotte 1, 2, 3)
	// Create a response DTO with unlocked field
	type FleetResponse struct {
		ID           uuid.UUID     `json:"id"`
		IslandID     uuid.UUID     `json:"island_id"`
		Name         string        `json:"name"`
		Ships        []domain.Ship `json:"ships,omitempty"`
		Unlocked     bool          `json:"unlocked"`
		MoraleCruise int           `json:"morale_cruise,omitempty"` // Morale during cruise (0-100)
	}

	fleetResponses := make([]FleetResponse, 0, len(fleets))
	for _, fleet := range fleets {
		// Determine if fleet is unlocked based on fleet number and player techs
		unlocked := isFleetUnlocked(fleet.Name, player.UnlockedTechs)
		// Return 50 if MoraleCruise is nil (uninitialized), otherwise return actual value
		// This is for UI display; DB still distinguishes NULL vs 0
		moraleCruise := 50 // Default for UI
		if fleet.MoraleCruise != nil {
			moraleCruise = *fleet.MoraleCruise
		}

		fleetResponses = append(fleetResponses, FleetResponse{
			ID:           fleet.ID,
			IslandID:     fleet.IslandID,
			Name:         fleet.Name,
			Ships:        fleet.Ships,
			Unlocked:     unlocked,
			MoraleCruise: moraleCruise,
		})
	}

	return c.JSON(http.StatusOK, fleetResponses)
}

// ensurePlayerFleets ensures that a player has exactly 3 fleets (Flotte 1, 2, 3)
// This function is idempotent - it will only create missing fleets
func ensurePlayerFleets(db *gorm.DB, island *domain.Island) error {
	// Get existing fleets for this island
	var existingFleets []domain.Fleet
	if err := db.Where("island_id = ?", island.ID).Find(&existingFleets).Error; err != nil {
		return fmt.Errorf("failed to query existing fleets: %w", err)
	}

	// Create a map of existing fleet names for quick lookup
	existingNames := make(map[string]bool)
	for _, f := range existingFleets {
		existingNames[f.Name] = true
	}

	// Create missing fleets
	fleetNames := []string{"Flotte 1", "Flotte 2", "Flotte 3"}
	for _, name := range fleetNames {
		if !existingNames[name] {
			morale50 := 50
			newFleet := domain.Fleet{
				ID:           uuid.New(),
				IslandID:     island.ID,
				Name:         name,
				MoraleCruise: &morale50, // Explicitly set to 50 (non-null)
			}
			if err := db.Create(&newFleet).Error; err != nil {
				return fmt.Errorf("failed to create fleet %s: %w", name, err)
			}
			fmt.Printf("[FLEET] Created missing fleet: %s for island %s\n", name, island.ID)
		}
	}

	// Lazy update: auto-fix FlagshipShipID for existing fleets
	// Reload all fleets to check for flagship auto-fix and orphaned references
	var allFleets []domain.Fleet
	if err := db.Preload("Ships").Where("island_id = ?", island.ID).Find(&allFleets).Error; err != nil {
		return fmt.Errorf("failed to reload fleets for flagship check: %w", err)
	}
	for i := range allFleets {
		// Check if FlagshipShipID points to a non-existent ship (orphaned reference)
		if allFleets[i].FlagshipShipID != nil {
			flagshipExists := false
			for _, ship := range allFleets[i].Ships {
				if ship.ID == *allFleets[i].FlagshipShipID {
					flagshipExists = true
					// Also check if the flagship ship is destroyed (shouldn't happen with hard delete, but defensive)
					if ship.State == "Destroyed" || ship.Health <= 0 {
						flagshipExists = false
					}
					break
				}
			}
			if !flagshipExists {
				// Orphaned reference: clear it
				allFleets[i].FlagshipShipID = nil
				if err := db.Save(&allFleets[i]).Error; err != nil {
					fmt.Printf("[FLEET] Failed to clear orphaned flagship for fleet %s: %v\n", allFleets[i].ID, err)
					// Continue anyway - not critical
				} else {
					fmt.Printf("[FLEET] Cleared orphaned flagship reference for fleet %s\n", allFleets[i].ID)
				}
			}
		}

		// Auto-set flagship if nil and fleet has active ships
		if allFleets[i].FlagshipShipID == nil && len(allFleets[i].Ships) > 0 {
			// Filter out destroyed ships
			activeShips := make([]domain.Ship, 0)
			for _, ship := range allFleets[i].Ships {
				if ship.State != "Destroyed" && ship.Health > 0 {
					activeShips = append(activeShips, ship)
				}
			}
			if len(activeShips) > 0 {
				// Auto-set flagship to first active ship (sorted by ID for determinism)
				sort.Slice(activeShips, func(a, b int) bool {
					return activeShips[a].ID.String() < activeShips[b].ID.String()
				})
				allFleets[i].FlagshipShipID = &activeShips[0].ID
				if err := db.Save(&allFleets[i]).Error; err != nil {
					fmt.Printf("[FLEET] Failed to auto-fix flagship for fleet %s: %v\n", allFleets[i].ID, err)
					// Continue anyway - not critical
				} else {
					fmt.Printf("[FLEET] Auto-fixed flagship for fleet %s: ship_id=%s\n", allFleets[i].ID, activeShips[0].ID)
				}
			}
		}
	}

	return nil
}

// isFleetUnlocked determines if a fleet is unlocked based on its name and player's unlocked techs
// Fleet 1 is always unlocked
// Fleet 2 is unlocked by nav_fleet_1 (Amirauté)
// Fleet 3 is unlocked by nav_fleet_2 (Grande Armada)
func isFleetUnlocked(fleetName string, unlockedTechs []string) bool {
	switch fleetName {
	case "Flotte 1":
		return true // Always unlocked
	case "Flotte 2":
		// Unlocked by nav_fleet_1 (Amirauté)
		for _, tech := range unlockedTechs {
			if tech == "nav_fleet_1" {
				return true
			}
		}
		return false
	case "Flotte 3":
		// Unlocked by nav_fleet_2 (Grande Armada)
		for _, tech := range unlockedTechs {
			if tech == "nav_fleet_2" {
				return true
			}
		}
		return false
	default:
		// Unknown fleet name - assume unlocked for backward compatibility
		return true
	}
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

// --- CAPTAIN SYSTEM ---

type GrantCaptainRequest struct {
	TemplateID string `json:"template_id"`
	Rarity     string `json:"rarity"`
	Name       string `json:"name"`
	SkillID    string `json:"skill_id"`
}

func DevGrantCaptain(c echo.Context) error {
	req := new(GrantCaptainRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé: admin uniquement"})
	}
	playerID := player.ID

	// Validate rarity
	rarity := domain.CaptainRarity(req.Rarity)
	if rarity != domain.RarityCommon && rarity != domain.RarityRare && rarity != domain.RarityLegendary {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Rareté invalide (common, rare, legendary)"})
	}

	db := repository.GetDB()

	// Create new captain
	captain := domain.Captain{
		ID:             uuid.New(),
		PlayerID:       playerID,
		TemplateID:     req.TemplateID,
		Name:           req.Name,
		Rarity:         rarity,
		Level:          1,
		XP:             0,
		SkillID:        req.SkillID,
		AssignedShipID: nil,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := db.Create(&captain).Error; err != nil {
		fmt.Printf("[CAPTAIN] DevGrantCaptain: Failed to create captain: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la création du capitaine"})
	}

	fmt.Printf("[CAPTAIN] DevGrantCaptain: Created captain %s (%s) for player %s\n", captain.Name, captain.ID, playerID)
	return c.JSON(http.StatusOK, captain)
}

type GrantTicketsRequest struct {
	Amount int `json:"amount"`
}

func DevGrantTickets(c echo.Context) error {
	req := new(GrantTicketsRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé: admin uniquement"})
	}
	playerID := player.ID

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Amount must be positive"})
	}
	// Support bulk grants (up to 10000 for testing)
	if req.Amount > 10000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Amount too large (max 10000)"})
	}

	db := repository.GetDB()
	var island domain.Island
	if err := db.First(&island, "player_id = ?", playerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Ensure Resources map exists
	if island.Resources == nil {
		island.Resources = make(map[domain.ResourceType]float64)
	}

	// Add tickets
	currentTickets := island.Resources[domain.CaptainTicket]
	island.Resources[domain.CaptainTicket] = currentTickets + float64(req.Amount)

	if err := db.Save(&island).Error; err != nil {
		fmt.Printf("[TAVERN] DevGrantTickets: Failed to save island: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to grant tickets"})
	}

	fmt.Printf("[TAVERN] DevGrantTickets: Granted %d tickets to player %s (new total: %.0f)\n", req.Amount, playerID, island.Resources[domain.CaptainTicket])
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "Tickets granted",
		"ticket_balance": int(island.Resources[domain.CaptainTicket]),
	})
}

func GetCaptains(c echo.Context) error {
	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	var captains []domain.Captain
	if err := db.Where("player_id = ?", playerID).Find(&captains).Error; err != nil {
		fmt.Printf("[CAPTAIN] GetCaptains: Failed to load captains: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec du chargement des capitaines"})
	}

	// Create response DTO with computed passive effects and naval bonuses
	type CaptainResponse struct {
		domain.Captain
		PassiveID                  string          `json:"passive_id,omitempty"`
		PassiveValue               float64         `json:"passive_value,omitempty"`
		PassiveIntValue            int             `json:"passive_int_value,omitempty"`
		Threshold                  int             `json:"threshold,omitempty"`
		DrainPerMinute             float64         `json:"drain_per_minute,omitempty"`
		Flags                      map[string]bool `json:"flags,omitempty"`
		NavalHPBonusPct            float64         `json:"naval_hp_bonus_pct,omitempty"`
		NavalSpeedBonusPct         float64         `json:"naval_speed_bonus_pct,omitempty"`
		NavalDamageReductionPct    float64         `json:"naval_damage_reduction_pct,omitempty"`
		RumConsumptionReductionPct float64         `json:"rum_consumption_reduction_pct,omitempty"`
	}

	responses := make([]CaptainResponse, 0, len(captains))
	for _, captain := range captains {
		effect := economy.ComputeCaptainPassive(captain)
		navalBonus := economy.ComputeNavalBonuses(captain)

		// Log captain passive computation (once per request, not spammy)
		fmt.Printf("[CAPTAIN] id=%s lvl=%d stars=%d rarity=%s skill=%s effect_id=%s value=%.3f int_value=%d threshold=%d\n",
			captain.ID, captain.Level, captain.Stars, captain.Rarity, captain.SkillID,
			effect.ID, effect.Value, effect.IntValue, effect.Threshold)

		// Debug log for naval bonuses (only if CAPTAIN_DEBUG env var is set)
		if os.Getenv("CAPTAIN_DEBUG") == "true" {
			fmt.Printf("[STARS] captain=%s rarity=%s stars=%d hp=%.3f spd=%.3f dr=%.3f rum=%.3f\n",
				captain.ID, captain.Rarity, captain.Stars,
				navalBonus.NavalHPBonusPct, navalBonus.NavalSpeedBonusPct,
				navalBonus.NavalDamageReductionPct, navalBonus.RumConsumptionReductionPct)
		}

		response := CaptainResponse{
			Captain:                    captain,
			PassiveID:                  effect.ID,
			PassiveValue:               effect.Value,
			PassiveIntValue:            effect.IntValue,
			Threshold:                  effect.Threshold,
			DrainPerMinute:             effect.DrainPerMinute,
			Flags:                      effect.Flags,
			NavalHPBonusPct:            navalBonus.NavalHPBonusPct,
			NavalSpeedBonusPct:         navalBonus.NavalSpeedBonusPct,
			NavalDamageReductionPct:    navalBonus.NavalDamageReductionPct,
			RumConsumptionReductionPct: navalBonus.RumConsumptionReductionPct,
		}
		responses = append(responses, response)
	}

	fmt.Printf("[CAPTAIN] GetCaptains: Found %d captains for player %s\n", len(captains), playerID)
	return c.JSON(http.StatusOK, responses)
}

type AssignCaptainRequest struct {
	CaptainID uuid.UUID `json:"captain_id"`
	ShipID    uuid.UUID `json:"ship_id"`
}

func AssignCaptain(c echo.Context) error {
	req := new(AssignCaptainRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	fmt.Printf("[CAPTAIN] AssignCaptain request: captain_id=%s, ship_id=%s\n", req.CaptainID, req.ShipID)

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[CAPTAIN] AssignCaptain: Panic recovered: %v\n", r)
		}
	}()

	// 1. Load captain and verify ownership
	var captain domain.Captain
	if err := tx.First(&captain, "id = ?", req.CaptainID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Captain not found: captain_id=%s, error=%v\n", req.CaptainID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Capitaine introuvable"})
	}
	if captain.PlayerID != playerID {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Ownership mismatch: captain.player_id=%s, authenticated=%s\n", captain.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Ce capitaine ne vous appartient pas"})
	}

	// 2. Load ship and verify ownership
	var ship domain.Ship
	if err := tx.First(&ship, "id = ?", req.ShipID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Ship not found: ship_id=%s, error=%v\n", req.ShipID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Navire introuvable"})
	}
	if ship.PlayerID != playerID {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Ship ownership mismatch: ship.player_id=%s, authenticated=%s\n", ship.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Ce navire ne vous appartient pas"})
	}

	// 2.5. Check if ship's fleet is locked (anti-exploit: prevent swap during engagement)
	if ship.FleetID != nil {
		var fleet domain.Fleet
		if err := tx.First(&fleet, "id = ?", *ship.FleetID).Error; err == nil {
			if economy.IsFleetLocked(&fleet) {
				tx.Rollback()
				lockedUntil := "indéterminé"
				if fleet.LockedUntil != nil {
					lockedUntil = fleet.LockedUntil.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("[FLEET] blocked captain change player=%s fleet=%s reason=locked until=%s\n", playerID, fleet.ID, lockedUntil)
				return c.JSON(http.StatusConflict, map[string]string{"error": "Flotte verrouillée (engagement en cours)."})
			}
		}
	}

	// 3. If captain is already assigned to another ship, unassign from that ship
	if captain.AssignedShipID != nil && *captain.AssignedShipID != req.ShipID {
		var oldShip domain.Ship
		if err := tx.First(&oldShip, "id = ?", *captain.AssignedShipID).Error; err == nil {
			// Clear the old ship's captain if it matches
			if oldShip.CaptainID != nil && *oldShip.CaptainID == captain.ID {
				oldShip.CaptainID = nil
				if err := tx.Save(&oldShip).Error; err != nil {
					tx.Rollback()
					fmt.Printf("[CAPTAIN] AssignCaptain: Failed to unassign from old ship: %v\n", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la réassignation"})
				}
				fmt.Printf("[CAPTAIN] AssignCaptain: Unassigned captain from old ship %s\n", *captain.AssignedShipID)
			}
		}
	}

	// 4. If target ship already has a captain, unassign that captain
	if ship.CaptainID != nil {
		var oldCaptain domain.Captain
		if err := tx.First(&oldCaptain, "id = ?", *ship.CaptainID).Error; err == nil {
			oldCaptain.AssignedShipID = nil
			if err := tx.Save(&oldCaptain).Error; err != nil {
				tx.Rollback()
				fmt.Printf("[CAPTAIN] AssignCaptain: Failed to unassign old captain: %v\n", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la réassignation"})
			}
			fmt.Printf("[CAPTAIN] AssignCaptain: Unassigned old captain %s from ship\n", *ship.CaptainID)
		}
	}

	// 5. Assign captain to ship
	captain.AssignedShipID = &req.ShipID
	ship.CaptainID = &captain.ID

	if err := tx.Save(&captain).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Failed to save captain: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de l'assignation"})
	}
	if err := tx.Save(&ship).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] AssignCaptain: Failed to save ship: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de l'assignation"})
	}

	tx.Commit()

	fmt.Printf("[CAPTAIN] assign captain=%s ship=%s player=%s\n", captain.ID, ship.ID, playerID)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Capitaine assigné au navire.",
		"captain": captain,
		"ship":    ship,
	})
}

type UnassignCaptainRequest struct {
	CaptainID uuid.UUID `json:"captain_id"`
}

func UnassignCaptain(c echo.Context) error {
	req := new(UnassignCaptainRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	fmt.Printf("[CAPTAIN] UnassignCaptain request: captain_id=%s\n", req.CaptainID)

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[CAPTAIN] UnassignCaptain: Panic recovered: %v\n", r)
		}
	}()

	// 1. Load captain and verify ownership
	var captain domain.Captain
	if err := tx.First(&captain, "id = ?", req.CaptainID).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] UnassignCaptain: Captain not found: captain_id=%s, error=%v\n", req.CaptainID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Capitaine introuvable"})
	}
	if captain.PlayerID != playerID {
		tx.Rollback()
		fmt.Printf("[CAPTAIN] UnassignCaptain: Ownership mismatch: captain.player_id=%s, authenticated=%s\n", captain.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Ce capitaine ne vous appartient pas"})
	}

	// 2. If captain is assigned to a ship, clear the ship's captain reference
	if captain.AssignedShipID != nil {
		var ship domain.Ship
		if err := tx.First(&ship, "id = ?", *captain.AssignedShipID).Error; err == nil {
			// 2.5. Check if ship's fleet is locked (anti-exploit: prevent swap during engagement)
			if ship.FleetID != nil {
				var fleet domain.Fleet
				if err := tx.First(&fleet, "id = ?", *ship.FleetID).Error; err == nil {
					if economy.IsFleetLocked(&fleet) {
						tx.Rollback()
						lockedUntil := "indéterminé"
						if fleet.LockedUntil != nil {
							lockedUntil = fleet.LockedUntil.Format("2006-01-02 15:04:05")
						}
						fmt.Printf("[FLEET] blocked captain change player=%s fleet=%s reason=locked until=%s\n", playerID, fleet.ID, lockedUntil)
						return c.JSON(http.StatusConflict, map[string]string{"error": "Flotte verrouillée (engagement en cours)."})
					}
				}
			}
			// Only clear if the ship's captain matches
			if ship.CaptainID != nil && *ship.CaptainID == captain.ID {
				ship.CaptainID = nil
				if err := tx.Save(&ship).Error; err != nil {
					tx.Rollback()
					fmt.Printf("[CAPTAIN] UnassignCaptain: Failed to clear ship captain: %v\n", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec du retrait"})
				}
				fmt.Printf("[CAPTAIN] UnassignCaptain: Cleared captain from ship %s\n", *captain.AssignedShipID)
			}
		}
		// Clear captain's assignment
		captain.AssignedShipID = nil
		if err := tx.Save(&captain).Error; err != nil {
			tx.Rollback()
			fmt.Printf("[CAPTAIN] UnassignCaptain: Failed to save captain: %v\n", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec du retrait"})
		}
	}

	tx.Commit()

	fmt.Printf("[CAPTAIN] unassign captain=%s player=%s\n", captain.ID, playerID)
	return c.JSON(http.StatusOK, map[string]string{"message": "Capitaine retiré du navire."})
}

// --- TAVERN GACHA SYSTEM ---

type SummonCaptainRequest struct {
	Count int `json:"count"`
}

func SummonCaptain(c echo.Context) error {
	req := new(SummonCaptainRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate count: only 1 or 10 allowed
	if req.Count != 1 && req.Count != 10 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Count must be 1 or 10"})
	}

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[TAVERN] SummonCaptain: Panic recovered: %v\n", r)
		}
	}()

	// Load player with pity counts
	// CRITICAL: Use SELECT FOR UPDATE to prevent race conditions on pity counters
	// This locks the player row until the transaction commits, ensuring atomicity
	// when multiple x10 summons occur simultaneously
	var playerWithPity domain.Player
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&playerWithPity, "id = ?", playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Player not found"})
	}

	// Load island with buildings
	var island domain.Island
	if err := tx.Preload("Buildings").First(&island, "player_id = ?", playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Ensure Resources map exists
	if island.Resources == nil {
		island.Resources = make(map[domain.ResourceType]float64)
	}

	// Check if Tavern exists and is finished (not constructing)
	tavernFound := false
	for _, b := range island.Buildings {
		if b.Type == "Tavern" && !b.Constructing {
			tavernFound = true
			break
		}
	}
	if !tavernFound {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Tavern required"})
	}

	// Check ticket count
	currentTickets := island.Resources[domain.CaptainTicket]
	requiredTickets := float64(req.Count)
	if currentTickets < requiredTickets {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Not enough tickets"})
	}

	ticketsBefore := int(currentTickets)

	// Consume tickets
	island.Resources[domain.CaptainTicket] = currentTickets - requiredTickets

	// Prepare results array
	type SummonResult struct {
		Rarity        string          `json:"rarity"`
		TemplateID    string          `json:"template_id"`
		Name          string          `json:"name"`
		IsDuplicate   bool            `json:"is_duplicate"`
		ShardsGranted int             `json:"shards_granted,omitempty"`
		Captain       *domain.Captain `json:"captain,omitempty"`
	}

	results := make([]SummonResult, 0, req.Count)
	totalShardsGranted := 0
	duplicateCount := 0
	pityLBefore := playerWithPity.PityLegendaryCount
	legendaryForced := false

	// Perform summons
	for i := 0; i < req.Count; i++ {
		// Roll rarity with pity
		rarity, forced := economy.RollCaptainRarityWithPity(playerWithPity.PityLegendaryCount, playerWithPity.PityRareCount)
		if forced && rarity == domain.RarityLegendary {
			legendaryForced = true
		}

		// Pick template
		template, err := economy.PickCaptainTemplateByRarity(rarity)
		if err != nil {
			tx.Rollback()
			fmt.Printf("[TAVERN] SummonCaptain: Failed to pick template: %v\n", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to roll captain"})
		}

		// Check for duplicate
		var existingCaptains []domain.Captain
		if err := tx.Where("player_id = ? AND template_id = ?", playerID, template.TemplateID).Find(&existingCaptains).Error; err != nil {
			tx.Rollback()
			fmt.Printf("[TAVERN] SummonCaptain: Failed to check duplicates: %v\n", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to check duplicates"})
		}

		isDuplicate := len(existingCaptains) > 0
		shardsGranted := 0

		var captain *domain.Captain
		if isDuplicate {
			// Grant shards instead of refunding tickets
			duplicateCount++
			switch rarity {
			case domain.RarityCommon:
				shardsGranted = economy.ShardsPerCommonDup
			case domain.RarityRare:
				shardsGranted = economy.ShardsPerRareDup
			case domain.RarityLegendary:
				shardsGranted = economy.ShardsPerLegendaryDup
			}

			// Get or create shard wallet
			var wallet domain.CaptainShardWallet
			err := tx.Where("player_id = ? AND template_id = ?", playerID, template.TemplateID).First(&wallet).Error
			if err != nil {
				// Create new wallet
				wallet = domain.CaptainShardWallet{
					ID:         uuid.New(),
					PlayerID:   playerID,
					TemplateID: template.TemplateID,
					Shards:     shardsGranted,
					UpdatedAt:  time.Now(),
				}
				if err := tx.Create(&wallet).Error; err != nil {
					tx.Rollback()
					fmt.Printf("[TAVERN] SummonCaptain: Failed to create shard wallet: %v\n", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to grant shards"})
				}
			} else {
				// Update existing wallet
				wallet.Shards += shardsGranted
				wallet.UpdatedAt = time.Now()
				if err := tx.Save(&wallet).Error; err != nil {
					tx.Rollback()
					fmt.Printf("[TAVERN] SummonCaptain: Failed to update shard wallet: %v\n", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to grant shards"})
				}
			}
			totalShardsGranted += shardsGranted
		} else {
			// Create new captain
			newCaptain := domain.Captain{
				ID:             uuid.New(),
				PlayerID:       playerID,
				TemplateID:     template.TemplateID,
				Name:           template.Name,
				Rarity:         template.Rarity,
				Level:          1,
				XP:             0,
				Stars:          0, // Start at 0 stars
				SkillID:        template.SkillID,
				AssignedShipID: nil,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			if err := tx.Create(&newCaptain).Error; err != nil {
				tx.Rollback()
				fmt.Printf("[TAVERN] SummonCaptain: Failed to create captain: %v\n", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create captain"})
			}

			captain = &newCaptain
		}

		// Update pity counters
		playerWithPity.PityLegendaryCount++
		playerWithPity.PityRareCount++
		if rarity == domain.RarityLegendary {
			playerWithPity.PityLegendaryCount = 0
		}
		if rarity == domain.RarityRare || rarity == domain.RarityLegendary {
			playerWithPity.PityRareCount = 0
		}

		results = append(results, SummonResult{
			Rarity:        string(rarity),
			TemplateID:    template.TemplateID,
			Name:          template.Name,
			IsDuplicate:   isDuplicate,
			ShardsGranted: shardsGranted,
			Captain:       captain,
		})
	}

	// Save player (with updated pity)
	if err := tx.Save(&playerWithPity).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[TAVERN] SummonCaptain: Failed to save player: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save player"})
	}

	// Save island (with updated ticket count)
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[TAVERN] SummonCaptain: Failed to save island: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save island"})
	}

	tx.Commit()

	ticketsAfter := int(island.Resources[domain.CaptainTicket])
	pityLAfter := playerWithPity.PityLegendaryCount

	// Log
	fmt.Printf("[GACHA] player=%s count=%d pityL_before=%d pityL_after=%d legendary_forced=%v duplicates=%d shards_granted=%d\n",
		playerID, req.Count, pityLBefore, pityLAfter, legendaryForced, duplicateCount, totalShardsGranted)
	fmt.Printf("[TAVERN] player=%s island=%s tickets_before=%d tickets_after=%d\n",
		playerID, island.ID, ticketsBefore, ticketsAfter)

	// Build response
	response := map[string]interface{}{
		"results":              results,
		"tickets_before":       ticketsBefore,
		"tickets_after":        ticketsAfter,
		"shards_total_granted": totalShardsGranted,
		"duplicate_count":      duplicateCount,
		// Legacy field for backward compatibility
		"compensation": map[string]interface{}{
			"refunded_tickets": 0, // No longer refunding, using shards
		},
	}

	return c.JSON(http.StatusOK, response)
}

// --- CAPTAIN STARS UPGRADE ---

type UpgradeStarsRequest struct {
	CaptainID uuid.UUID `json:"captain_id"`
}

func UpgradeCaptainStars(c echo.Context) error {
	req := new(UpgradeStarsRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[STARS] UpgradeCaptainStars: Panic recovered: %v\n", r)
		}
	}()

	// Load captain and verify ownership
	var captain domain.Captain
	if err := tx.First(&captain, "id = ?", req.CaptainID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Capitaine introuvable"})
	}
	if captain.PlayerID != playerID {
		tx.Rollback()
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Ce capitaine ne vous appartient pas"})
	}

	// Check if already at max stars (Stars == MaxStars means fully upgraded)
	maxStars := economy.GetMaxStars(captain.Rarity)
	if captain.Stars >= maxStars {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Capitaine déjà au maximum (%d étoiles)", maxStars)})
	}
	if captain.Stars < 0 {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Nombre d'étoiles invalide"})
	}

	// Get upgrade cost
	cost, err := economy.GetStarUpgradeCost(captain.Rarity, captain.Stars)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Load shard wallet for this template
	var wallet domain.CaptainShardWallet
	if err := tx.Where("player_id = ? AND template_id = ?", playerID, captain.TemplateID).First(&wallet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Pas assez de fragments (besoin: %d)", cost)})
	}

	// Check if enough shards
	if wallet.Shards < cost {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Pas assez de fragments (besoin: %d, avez: %d)", cost, wallet.Shards)})
	}

	// Deduct shards and upgrade stars
	shardsBefore := wallet.Shards
	wallet.Shards -= cost
	wallet.UpdatedAt = time.Now()
	starsBefore := captain.Stars
	captain.Stars++
	captain.UpdatedAt = time.Now()

	// Save
	if err := tx.Save(&wallet).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[STARS] UpgradeCaptainStars: Failed to save wallet: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la sauvegarde"})
	}
	if err := tx.Save(&captain).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[STARS] UpgradeCaptainStars: Failed to save captain: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la sauvegarde"})
	}

	tx.Commit()

	// Log upgrade (one line per upgrade)
	fmt.Printf("[STARS] upgrade player=%s captain=%s rarity=%s stars=%d->%d cost=%d remaining=%d\n",
		playerID, captain.ID, captain.Rarity, starsBefore, captain.Stars, cost, wallet.Shards)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":       "Étoiles améliorées",
		"captain":       captain,
		"shards_before": shardsBefore,
		"shards_after":  wallet.Shards,
		"cost":          cost,
	})
}

// --- SHARDS EXCHANGE ---

// Exchange rates: shards per ticket (rebalanced to be usable while still punitive)
const (
	ShardsPerTicketCommon = 120 // Common: 120 shards => 1 ticket
	ShardsPerTicketRare   = 180 // Rare: 180 shards => 1 ticket
	// Legendary exchange is NOT allowed (disabled for economy balance)
)

type ExchangeShardsRequest struct {
	Rarity string `json:"rarity"` // "common", "rare", "legendary"
	Count  int    `json:"count"`  // Number of tickets to craft
}

func ExchangeShards(c echo.Context) error {
	req := new(ExchangeShardsRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate rarity (legendary exchange is disabled)
	var rarity domain.CaptainRarity
	var shardsPerTicket int
	switch req.Rarity {
	case "common":
		rarity = domain.RarityCommon
		shardsPerTicket = ShardsPerTicketCommon
	case "rare":
		rarity = domain.RarityRare
		shardsPerTicket = ShardsPerTicketRare
	case "legendary":
		// Legendary exchange is NOT allowed (anti-exploit measure)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Les fragments légendaires ne peuvent pas être échangés contre des tickets."})
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Rareté invalide (common/rare uniquement)"})
	}

	if req.Count <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Count must be positive"})
	}

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentification requise"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[EXCHANGE] ExchangeShards: Panic recovered: %v\n", r)
		}
	}()

	// Load player for daily cap check
	var playerForCap domain.Player
	if err := tx.Where("id = ?", playerID).First(&playerForCap).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Joueur introuvable"})
	}

	// Check/reset daily cap
	today := time.Now().Format("2006-01-02")
	if playerForCap.DailyShardExchangeDay != today {
		// New day: reset counter
		playerForCap.DailyShardExchangeCount = 0
		playerForCap.DailyShardExchangeDay = today
		if err := tx.Save(&playerForCap).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la mise à jour du compteur"})
		}
	}

	// Check daily cap (20 exchanges per day)
	const DailyExchangeCap = 20
	if playerForCap.DailyShardExchangeCount >= DailyExchangeCap {
		tx.Rollback()
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "Limite quotidienne atteinte (20 échanges/jour)."})
	}

	// Load all shard wallets for this rarity
	// We need to get all templates of this rarity
	templates := economy.GetTemplatesByRarity(rarity)
	if len(templates) == 0 {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Aucun template trouvé pour cette rareté"})
	}

	// Get all wallets for these templates
	var wallets []domain.CaptainShardWallet
	templateIDs := make([]string, len(templates))
	for i, t := range templates {
		templateIDs[i] = t.TemplateID
	}
	if err := tx.Where("player_id = ? AND template_id IN ?", playerID, templateIDs).Find(&wallets).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[EXCHANGE] ExchangeShards: Failed to load wallets: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec du chargement des fragments"})
	}

	// Calculate total shards available
	totalShards := 0
	for _, w := range wallets {
		totalShards += w.Shards
	}

	// Calculate required shards
	requiredShards := req.Count * shardsPerTicket
	if totalShards < requiredShards {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Pas assez de fragments (besoin: %d, avez: %d)", requiredShards, totalShards)})
	}

	// Sort wallets by shards (descending) and template_id (for stable sort)
	sort.Slice(wallets, func(i, j int) bool {
		if wallets[i].Shards != wallets[j].Shards {
			return wallets[i].Shards > wallets[j].Shards
		}
		return wallets[i].TemplateID < wallets[j].TemplateID
	})

	// Consume shards deterministically
	shardsToConsume := requiredShards
	details := make([]map[string]interface{}, 0)
	for i := range wallets {
		if shardsToConsume <= 0 {
			break
		}
		consumeFromThis := shardsToConsume
		if consumeFromThis > wallets[i].Shards {
			consumeFromThis = wallets[i].Shards
		}
		shardsBefore := wallets[i].Shards
		wallets[i].Shards -= consumeFromThis
		wallets[i].UpdatedAt = time.Now()
		shardsToConsume -= consumeFromThis

		details = append(details, map[string]interface{}{
			"template_id":   wallets[i].TemplateID,
			"shards_before": shardsBefore,
			"shards_after":  wallets[i].Shards,
			"consumed":      consumeFromThis,
		})

		if wallets[i].Shards == 0 {
			// Delete wallet if empty
			if err := tx.Delete(&wallets[i]).Error; err != nil {
				tx.Rollback()
				fmt.Printf("[EXCHANGE] ExchangeShards: Failed to delete empty wallet: %v\n", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la sauvegarde"})
			}
		} else {
			if err := tx.Save(&wallets[i]).Error; err != nil {
				tx.Rollback()
				fmt.Printf("[EXCHANGE] ExchangeShards: Failed to save wallet: %v\n", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la sauvegarde"})
			}
		}
	}

	// Load island and add tickets
	var island domain.Island
	if err := tx.First(&island, "player_id = ?", playerID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}
	if island.Resources == nil {
		island.Resources = make(map[domain.ResourceType]float64)
	}
	ticketsBefore := int(island.Resources[domain.CaptainTicket])
	island.Resources[domain.CaptainTicket] += float64(req.Count)
	ticketsAfter := int(island.Resources[domain.CaptainTicket])

	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[EXCHANGE] ExchangeShards: Failed to save island: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la sauvegarde"})
	}

	// Increment daily exchange count
	playerForCap.DailyShardExchangeCount++
	if err := tx.Save(&playerForCap).Error; err != nil {
		tx.Rollback()
		fmt.Printf("[EXCHANGE] ExchangeShards: Failed to update daily count: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Échec de la mise à jour du compteur"})
	}

	tx.Commit()

	// Log action-level (one line per exchange)
	fmt.Printf("[SHARDS] exchange player=%s rarity=%s shards_spent=%d tickets_gained=%d daily=%d/20\n",
		playerID, req.Rarity, requiredShards, req.Count, playerForCap.DailyShardExchangeCount)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "Fragments échangés",
		"tickets_before": ticketsBefore,
		"tickets_after":  ticketsAfter,
		"shards_spent":   requiredShards,
		"details":        details,
	})
}

// --- ENGAGEMENT MORALE SYSTEM ---

type SimulateEngagementRequest struct {
	FleetAID       string `json:"fleet_a_id"`            // Accept as string, parse to UUID
	FleetBID       string `json:"fleet_b_id"`            // Accept as string, parse to UUID
	SimulateCombat bool   `json:"simulate_combat"`       // If true, simulate full combat (default: false)
	CombatSeed     *int64 `json:"combat_seed,omitempty"` // Optional deterministic RNG seed for combat
}

// DevSimulateEngagement simulates an engagement between two fleets (admin only, for testing)
// TODO: Real combat system will lock fleets during engagement (LockFleetForEngagement)
// This dev tool does NOT lock fleets (doesn't change real state)
func DevSimulateEngagement(c echo.Context) error {
	// Parse request (Echo will read body, but we need raw for debug)
	// Read body first for logging, then restore it for Echo
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Impossible de lire le corps de la requête"})
	}
	// Restore body for Echo
	c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	fmt.Printf("[ENGAGE] Raw request body: %s\n", string(bodyBytes))

	// Parse request
	req := new(SimulateEngagementRequest)
	if err := json.Unmarshal(bodyBytes, req); err != nil {
		fmt.Printf("[ENGAGE] Parse error: %v\n", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	fmt.Printf("[ENGAGE] Parsed fleet_a_id='%s' fleet_b_id='%s'\n", req.FleetAID, req.FleetBID)

	// Validate and parse UUIDs
	if req.FleetAID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fleet_a_id manquant"})
	}
	if req.FleetBID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fleet_b_id manquant"})
	}

	fleetAID, err := uuid.Parse(req.FleetAID)
	if err != nil {
		fmt.Printf("[ENGAGE] Invalid fleet_a_id UUID: '%s' error: %v\n", req.FleetAID, err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("fleet_a_id invalide (UUID attendu): '%s'", req.FleetAID)})
	}

	fleetBID, err := uuid.Parse(req.FleetBID)
	if err != nil {
		fmt.Printf("[ENGAGE] Invalid fleet_b_id UUID: '%s' error: %v\n", req.FleetBID, err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("fleet_b_id invalide (UUID attendu): '%s'", req.FleetBID)})
	}

	fmt.Printf("[ENGAGE] Parsed UUIDs: fleetAID=%s fleetBID=%s\n", fleetAID, fleetBID)

	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé: admin uniquement"})
	}
	playerID := player.ID

	db := repository.GetDB()

	fmt.Printf("[ENGAGE] Requested fleet IDs: fleetAID=%s fleetBID=%s playerID=%s\n", fleetAID, fleetBID, playerID)

	// Load Fleet A by ID only (no island_id constraint, no pre-filtering)
	var fleetA domain.Fleet
	if err := db.Preload("Ships", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id ASC")
	}).First(&fleetA, "id = ?", fleetAID).Error; err != nil {
		// Secondary check: does the fleet exist at all in DB?
		var count int64
		if countErr := db.Model(&domain.Fleet{}).Where("id = ?", fleetAID).Count(&count).Error; countErr != nil {
			fmt.Printf("[ENGAGE] DB error checking Fleet A existence: id=%s error=%v\n", fleetAID, countErr)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur base de données lors de la vérification de la flotte A: %v", countErr)})
		}
		if count > 0 {
			fmt.Printf("[ENGAGE] Fleet A exists in DB but query failed: id=%s error=%v (possible DB constraint issue)\n", fleetAID, err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur base de données lors du chargement de la flotte A (existe en DB): %v", err)})
		} else {
			fmt.Printf("[ENGAGE] Fleet A does not exist in DB: id=%s error=%v\n", fleetAID, err)
			return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Flotte A introuvable (id=%s)", fleetAID)})
		}
	}
	fmt.Printf("[ENGAGE] FleetA loaded successfully: id=%s island_id=%s name=%s ships=%d\n",
		fleetA.ID, fleetA.IslandID, fleetA.Name, len(fleetA.Ships))

	// Load Fleet B by ID only (no island_id constraint, no pre-filtering)
	var fleetB domain.Fleet
	if err := db.Preload("Ships", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id ASC")
	}).First(&fleetB, "id = ?", fleetBID).Error; err != nil {
		// Secondary check: does the fleet exist at all in DB?
		var count int64
		if countErr := db.Model(&domain.Fleet{}).Where("id = ?", fleetBID).Count(&count).Error; countErr != nil {
			fmt.Printf("[ENGAGE] DB error checking Fleet B existence: id=%s error=%v\n", fleetBID, countErr)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur base de données lors de la vérification de la flotte B: %v", countErr)})
		}
		if count > 0 {
			fmt.Printf("[ENGAGE] Fleet B exists in DB but query failed: id=%s error=%v (possible DB constraint issue)\n", fleetBID, err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur base de données lors du chargement de la flotte B (existe en DB): %v", err)})
		} else {
			fmt.Printf("[ENGAGE] Fleet B does not exist in DB: id=%s error=%v\n", fleetBID, err)
			return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Flotte B introuvable (id=%s)", fleetBID)})
		}
	}
	fmt.Printf("[ENGAGE] FleetB loaded successfully: id=%s island_id=%s name=%s ships=%d\n",
		fleetB.ID, fleetB.IslandID, fleetB.Name, len(fleetB.Ships))

	// Load islands for ownership verification (using IslandID from loaded fleets, NOT pre-selected island)
	var islandA domain.Island
	if err := db.First(&islandA, "id = ?", fleetA.IslandID).Error; err != nil {
		fmt.Printf("[ENGAGE] IslandA not found: fleetA.IslandID=%s error=%v\n", fleetA.IslandID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Île A introuvable (id=%s)", fleetA.IslandID)})
	}
	fmt.Printf("[ENGAGE] IslandA loaded from fleetA.IslandID: id=%s player_id=%s\n", islandA.ID, islandA.PlayerID)

	var islandB domain.Island
	if err := db.First(&islandB, "id = ?", fleetB.IslandID).Error; err != nil {
		fmt.Printf("[ENGAGE] IslandB not found: fleetB.IslandID=%s error=%v\n", fleetB.IslandID, err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Île B introuvable (id=%s)", fleetB.IslandID)})
	}
	fmt.Printf("[ENGAGE] IslandB loaded from fleetB.IslandID: id=%s player_id=%s\n", islandB.ID, islandB.PlayerID)

	// Verify ownership: both islands must belong to authenticated player
	if islandA.PlayerID != playerID {
		fmt.Printf("[ENGAGE] Ownership mismatch: FleetA island.player_id=%s authenticated=%s\n", islandA.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Flotte A ne vous appartient pas"})
	}
	if islandB.PlayerID != playerID {
		fmt.Printf("[ENGAGE] Ownership mismatch: FleetB island.player_id=%s authenticated=%s\n", islandB.PlayerID, playerID)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Flotte B ne vous appartient pas"})
	}

	// Select flagship ships deterministically (using explicit FlagshipShipID or fallback)
	flagshipA, explicitA, reasonA := economy.SelectFlagshipShip(&fleetA)
	flagshipB, explicitB, reasonB := economy.SelectFlagshipShip(&fleetB)

	// Get flagship captains
	var captA *domain.Captain
	var captB *domain.Captain

	if flagshipA != nil && flagshipA.CaptainID != nil {
		var captain domain.Captain
		if err := db.First(&captain, "id = ?", *flagshipA.CaptainID).Error; err == nil {
			captA = &captain
			fmt.Printf("[ENGAGE] FleetA flagship: ship_id=%s captain_id=%s captain_name=%s skill=%s\n",
				flagshipA.ID, captA.ID, captA.Name, captA.SkillID)
		}
	}
	if captA == nil && flagshipA != nil {
		fmt.Printf("[ENGAGE] FleetA: flagship ship_id=%s but no captain assigned\n", flagshipA.ID)
	} else if flagshipA == nil {
		fmt.Printf("[ENGAGE] FleetA: no flagship (no ships in fleet)\n")
	}

	if flagshipB != nil && flagshipB.CaptainID != nil {
		var captain domain.Captain
		if err := db.First(&captain, "id = ?", *flagshipB.CaptainID).Error; err == nil {
			captB = &captain
			fmt.Printf("[ENGAGE] FleetB flagship: ship_id=%s captain_id=%s captain_name=%s skill=%s\n",
				flagshipB.ID, captB.ID, captB.Name, captB.SkillID)
		}
	}
	if captB == nil && flagshipB != nil {
		fmt.Printf("[ENGAGE] FleetB: flagship ship_id=%s but no captain assigned\n", flagshipB.ID)
	} else if flagshipB == nil {
		fmt.Printf("[ENGAGE] FleetB: no flagship (no ships in fleet)\n")
	}

	// Add flagship info to applied notes (before computing engagement)
	// This will be added to the result by ComputeEngagementMorale, but we log it here too
	if flagshipA != nil {
		fmt.Printf("[ENGAGE] FleetA flagship: ship_id=%s type=%s captain_id=%s\n", flagshipA.ID, flagshipA.Type, *flagshipA.CaptainID)
	} else {
		fmt.Printf("[ENGAGE] FleetA: no flagship (no ships with captain)\n")
	}
	if flagshipB != nil {
		fmt.Printf("[ENGAGE] FleetB flagship: ship_id=%s type=%s captain_id=%s\n", flagshipB.ID, flagshipB.Type, *flagshipB.CaptainID)
	} else {
		fmt.Printf("[ENGAGE] FleetB: no flagship (no ships with captain)\n")
	}

	// Compute engagement morale
	result := economy.ComputeEngagementMorale(fleetA, fleetB, captA, captB)

	// Compute combat stats for flagships (with captain star bonuses)
	var combatStatsA *economy.ShipCombatStats
	var combatStatsB *economy.ShipCombatStats

	if flagshipA != nil {
		statsA, err := economy.ComputeShipCombatStatsWithCaptain(flagshipA, captA)
		if err == nil {
			combatStatsA = &statsA
			result.Applied = append(result.Applied, fmt.Sprintf("FleetA flagship combat stats: HP=%.1f/%.1f Speed=%.2f/%.2f DR=%.2f%%/%.2f%% Rum=%.1f/%.1f",
				statsA.BaseHP, statsA.EffectiveHP, statsA.BaseSpeed, statsA.EffectiveSpeed,
				statsA.BaseDamageReduction*100, statsA.EffectiveDamageReduction*100,
				statsA.BaseRumConsumption, statsA.EffectiveRumConsumption))
			// Add detailed applied notes from combat stats
			for _, note := range statsA.Applied {
				result.Applied = append(result.Applied, fmt.Sprintf("FleetA: %s", note))
			}
		} else {
			fmt.Printf("[ENGAGE] Failed to compute combat stats for FleetA flagship: %v\n", err)
		}
	}

	if flagshipB != nil {
		statsB, err := economy.ComputeShipCombatStatsWithCaptain(flagshipB, captB)
		if err == nil {
			combatStatsB = &statsB
			result.Applied = append(result.Applied, fmt.Sprintf("FleetB flagship combat stats: HP=%.1f/%.1f Speed=%.2f/%.2f DR=%.2f%%/%.2f%% Rum=%.1f/%.1f",
				statsB.BaseHP, statsB.EffectiveHP, statsB.BaseSpeed, statsB.EffectiveSpeed,
				statsB.BaseDamageReduction*100, statsB.EffectiveDamageReduction*100,
				statsB.BaseRumConsumption, statsB.EffectiveRumConsumption))
			// Add detailed applied notes from combat stats
			for _, note := range statsB.Applied {
				result.Applied = append(result.Applied, fmt.Sprintf("FleetB: %s", note))
			}
		} else {
			fmt.Printf("[ENGAGE] Failed to compute combat stats for FleetB flagship: %v\n", err)
		}
	}

	// Add flagship info to applied notes (with selection reason and crew info)
	if flagshipA != nil {
		dominantCrewA := economy.GetDominantCrewType(flagshipA)
		result.Applied = append([]string{fmt.Sprintf("FleetA flagship selection: explicit=%v ship_id=%s type=%s reason=%s crew_dominant=%s (warriors=%d archers=%d gunners=%d)",
			explicitA, flagshipA.ID, flagshipA.Type, reasonA, dominantCrewA, flagshipA.CrewWarriors, flagshipA.CrewArchers, flagshipA.CrewGunners)}, result.Applied...)
	} else {
		result.Applied = append([]string{"FleetA: no flagship"}, result.Applied...)
	}
	if flagshipB != nil {
		dominantCrewB := economy.GetDominantCrewType(flagshipB)
		result.Applied = append([]string{fmt.Sprintf("FleetB flagship selection: explicit=%v ship_id=%s type=%s reason=%s crew_dominant=%s (warriors=%d archers=%d gunners=%d)",
			explicitB, flagshipB.ID, flagshipB.Type, reasonB, dominantCrewB, flagshipB.CrewWarriors, flagshipB.CrewArchers, flagshipB.CrewGunners)}, result.Applied...)
	} else {
		result.Applied = append([]string{"FleetB: no flagship"}, result.Applied...)
	}

	// Log engagement (once per call)
	fmt.Printf("[ENGAGE] fleetA=%s moraleA=%d fleetB=%s moraleB=%d dM=%d bonus=%.0f%% atkA=%.2f defA=%.2f atkB=%.2f defB=%.2f\n",
		fleetA.ID, result.EngagementMoraleA, fleetB.ID, result.EngagementMoraleB,
		result.Delta, result.BonusPercent*100,
		result.AtkMultA, result.DefMultA, result.AtkMultB, result.DefMultB)

	if captA != nil {
		fmt.Printf("[ENGAGE] captainA=%s skill=%s stars=%d\n", captA.ID, captA.SkillID, captA.Stars)
	}
	if captB != nil {
		fmt.Printf("[ENGAGE] captainB=%s skill=%s stars=%d\n", captB.ID, captB.SkillID, captB.Stars)
	}

	// Build response with combat stats
	response := map[string]interface{}{
		"fleet_a_id":          result.FleetAID,
		"fleet_b_id":          result.FleetBID,
		"engagement_morale_a": result.EngagementMoraleA,
		"engagement_morale_b": result.EngagementMoraleB,
		"delta":               result.Delta,
		"bonus_percent":       result.BonusPercent,
		"atk_mult_a":          result.AtkMultA,
		"def_mult_a":          result.DefMultA,
		"atk_mult_b":          result.AtkMultB,
		"def_mult_b":          result.DefMultB,
		"applied":             result.Applied,
	}

	// Add combat stats to response if available
	if combatStatsA != nil {
		response["fleet_a_combat_stats"] = combatStatsA
	}
	if combatStatsB != nil {
		response["fleet_b_combat_stats"] = combatStatsB
	}

	// If simulate_combat is true, execute full combat simulation
	if req.SimulateCombat {
		// Generate deterministic seed if not provided
		combatSeed := time.Now().UnixNano()
		if req.CombatSeed != nil {
			combatSeed = *req.CombatSeed
		}

		// Execute naval combat
		combatResult, err := economy.ExecuteNavalCombat(
			&fleetA, &fleetB,
			captA, captB,
			result,
			combatSeed,
		)
		if err != nil {
			fmt.Printf("[ENGAGE] Combat simulation error: %v\n", err)
			response["combat_error"] = err.Error()
		} else {
			// Add combat result to response
			response["combat"] = map[string]interface{}{
				"winner":            combatResult.Winner,
				"rounds":            combatResult.Rounds,
				"ships_destroyed_a": combatResult.ShipsDestroyedA,
				"ships_destroyed_b": combatResult.ShipsDestroyedB,
				"captain_injured_a": combatResult.CaptainInjuredA,
				"captain_injured_b": combatResult.CaptainInjuredB,
				"combat_applied":    combatResult.Applied,
				"combat_seed":       combatSeed,
			}
		}
	}

	return c.JSON(http.StatusOK, response)
}

// sortShipsByID sorts ships by ID ASC for deterministic flagship selection
func sortShipsByID(ships *[]domain.Ship) {
	sort.Slice(*ships, func(i, j int) bool {
		return (*ships)[i].ID.String() < (*ships)[j].ID.String()
	})
}

// DevSetShipCrewRequest is the request for the /dev/set-ship-crew endpoint
type DevSetShipCrewRequest struct {
	ShipID   string `json:"ship_id"`  // UUID as string
	Warriors int    `json:"warriors"` // Must be >= 0
	Archers  int    `json:"archers"`  // Must be >= 0
	Gunners  int    `json:"gunners"`  // Must be >= 0
}

// DevSetShipCrewResponse is the response for the /dev/set-ship-crew endpoint
type DevSetShipCrewResponse struct {
	ShipID   string `json:"ship_id"`
	Warriors int    `json:"warriors"`
	Archers  int    `json:"archers"`
	Gunners  int    `json:"gunners"`
	Message  string `json:"message"`
}

// DevSetShipCrew sets the crew composition for a ship (dev only, for testing)
func DevSetShipCrew(c echo.Context) error {
	// Get authenticated player from context
	player := auth.GetAuthenticatedPlayer(c)
	if err := checkDevAdmin(player); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé: admin uniquement"})
	}
	playerID := player.ID

	// Parse request
	req := new(DevSetShipCrewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	// Validate ship_id
	if req.ShipID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ship_id manquant"})
	}
	shipID, err := uuid.Parse(req.ShipID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("ship_id invalide (UUID attendu): '%s'", req.ShipID)})
	}

	// Validate crew counts (must be >= 0)
	if req.Warriors < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "warriors doit être >= 0"})
	}
	if req.Archers < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "archers doit être >= 0"})
	}
	if req.Gunners < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "gunners doit être >= 0"})
	}

	db := repository.GetDB()

	// Load ship
	var ship domain.Ship
	if err := db.First(&ship, "id = ?", shipID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Navire introuvable (id=%s)", shipID)})
	}

	// Verify ownership: ship must belong to a player's island
	var island domain.Island
	if err := db.First(&island, "id = ? AND player_id = ?", ship.IslandID, playerID).Error; err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Ce navire ne vous appartient pas"})
	}

	// Update crew counts (temporarily for validation)
	ship.CrewWarriors = req.Warriors
	ship.CrewArchers = req.Archers
	ship.CrewGunners = req.Gunners

	// Validate crew bounds (using ship's actual type)
	isValid, reasonCode, reasonMsg := economy.ValidateShipCrewBounds(&ship)
	if !isValid {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":       reasonMsg,
			"reason_code": reasonCode,
			"message":     reasonMsg,
		})
	}

	// Save ship
	if err := db.Save(&ship).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur lors de la sauvegarde: %v", err)})
	}

	fmt.Printf("[DEV] SetShipCrew: ship_id=%s player_id=%s warriors=%d archers=%d gunners=%d\n",
		shipID, playerID, req.Warriors, req.Archers, req.Gunners)

	return c.JSON(http.StatusOK, DevSetShipCrewResponse{
		ShipID:   shipID.String(),
		Warriors: req.Warriors,
		Archers:  req.Archers,
		Gunners:  req.Gunners,
		Message:  "Équipage mis à jour",
	})
}

// MilitiaRecruitRequest is the request for POST /militia/recruit
type MilitiaRecruitRequest struct {
	Warriors int `json:"warriors"`
	Archers  int `json:"archers"`
	Gunners  int `json:"gunners"`
}

// MilitiaRecruitResponse is the response for POST /militia/recruit
type MilitiaRecruitResponse struct {
	DoneAt  time.Time `json:"done_at"`
	Pending struct {
		Warriors int `json:"warriors"`
		Archers  int `json:"archers"`
		Gunners  int `json:"gunners"`
	} `json:"pending"`
	GoldAfter float64 `json:"gold_after,omitempty"`
	RumAfter  float64 `json:"rum_after,omitempty"`
	Message   string  `json:"message"`
}

// MilitiaRecruit handles militia recruitment
func MilitiaRecruit(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}
	playerID := player.ID

	// Parse request
	req := new(MilitiaRecruitRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[MILITIA] Recruit: Panic recovered: %v\n", r)
		}
	}()

	// Load island with buildings
	var island domain.Island
	if err := tx.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Load resources (via CalculateResources or direct load)
	// For now, we'll recalculate resources to ensure accuracy
	now := time.Now()
	elapsed := now.Sub(island.LastUpdated)
	if elapsed > 0 {
		engine.CalculateResources(&island, elapsed)
		island.LastUpdated = now
	}

	// Validate request
	isValid, reasonCode, reasonMsg := economy.ValidateRecruitRequest(&island, req.Warriors, req.Archers, req.Gunners)
	if !isValid {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":          reasonMsg,
			"reason_code":    reasonCode,
			"reason_message": reasonMsg,
		})
	}

	// Get Militia building level
	militia := economy.GetMilitiaBuilding(&island)
	militiaLevel := 0
	if militia != nil {
		militiaLevel = militia.Level
	}

	// Calculate duration
	duration := economy.CalculateRecruitDuration(req.Warriors, req.Archers, req.Gunners, militiaLevel)
	doneAt := now.Add(duration)

	// Calculate and deduct costs
	gold, rum := economy.CalculateRecruitCost(req.Warriors, req.Archers, req.Gunners)
	island.Resources[domain.Gold] -= float64(gold)
	island.Resources[domain.Rum] -= float64(rum)

	// Set recruitment state
	island.MilitiaRecruiting = true
	island.MilitiaRecruitDoneAt = &doneAt
	island.MilitiaRecruitWarriors = req.Warriors
	island.MilitiaRecruitArchers = req.Archers
	island.MilitiaRecruitGunners = req.Gunners

	// Save island
	if err := tx.Save(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	tx.Commit()

	fmt.Printf("[MILITIA] Recruit: player=%s warriors=%d archers=%d gunners=%d done_at=%v\n",
		playerID.String()[:8], req.Warriors, req.Archers, req.Gunners, doneAt)

	// Build response
	response := MilitiaRecruitResponse{
		DoneAt:    doneAt,
		GoldAfter: island.Resources[domain.Gold],
		RumAfter:  island.Resources[domain.Rum],
		Message:   "Recrutement lancé",
	}
	response.Pending.Warriors = req.Warriors
	response.Pending.Archers = req.Archers
	response.Pending.Gunners = req.Gunners

	return c.JSON(http.StatusOK, response)
}

// canUpgradeBuilding checks if a building can be upgraded based on the HDV Global Rule
// Rule: No building can exceed the Town Hall level
// Returns nil if OK, or an error struct (HdvLimitError) if blocked.
func canUpgradeBuilding(island *domain.Island, buildingType string, currentLevel int) interface{} {
	// EXCEPTION: Town Hall itself checks nothing (it limits others)
	if buildingType == "Hôtel de Ville" {
		return nil
	}

	// Find Town Hall Level
	hdvLevel := 0
	for _, b := range island.Buildings {
		if b.Type == "Hôtel de Ville" {
			hdvLevel = b.Level
			break
		}
	}

	// Rule: Building Level (Next) <= HDV Level
	// If currentLevel is X, next is X+1.
	// We want X+1 <= HDV.
	// So if X+1 > HDV, block.
	nextLevel := currentLevel + 1

	if nextLevel > hdvLevel {
		return map[string]interface{}{
			"code":           "HDV_LEVEL_TOO_LOW",
			"required_level": nextLevel,
			"current_level":  hdvLevel,
			"message":        fmt.Sprintf("Hôtel de Ville niveau %d requis.", nextLevel),
		}
	}

	return nil
}
