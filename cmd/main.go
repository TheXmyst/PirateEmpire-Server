package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api"
	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/engine"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize Logger
	logger.Init()

	// Initialize Database
	repository.InitDB()

	// Clean up stale destroyed ships (one-time logic to align with new permanent destruction rule)
	db := repository.GetDB()
	var result struct{ Count int64 }
	db.Model(&domain.Ship{}).Where("state = ?", "Destroyed").Count(&result.Count)
	if result.Count > 0 {
		fmt.Printf("[CLEANUP] Found %d destroyed ships in DB. Purging...\n", result.Count)
		db.Where("state = ?", "Destroyed").Delete(&domain.Ship{})
		fmt.Printf("[CLEANUP] Success. Fleet capacity freed for all players.\n")
	}

	// Initialize Economy
	if err := economy.LoadConfig("configs/buildings.json"); err != nil {
		panic(err)
	}

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Welcome to Sea-Dogs MMO Server!")
	})

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public routes (no authentication required)
	e.POST("/register", api.Register)
	e.POST("/login", api.Login)
	e.GET("/status", api.GetStatus) // Status endpoint accepts player_id as query param for backward compatibility

	// Protected routes (require authentication)
	protected := e.Group("")
	protected.Use(auth.RequireAuth)
	{
		protected.POST("/build", api.Build)
		protected.POST("/upgrade", api.Upgrade)
		protected.POST("/research/start", api.StartResearch)
		protected.POST("/reset", api.ResetProgress)

		protected.POST("/dev/set-ship-militia", api.DevSetShipMilitia)
		protected.POST("/dev/set-ship-crew", api.DevSetShipMilitia) // Deprecated alias
		protected.POST("/build-ship", api.StartShipConstruction)
		protected.POST("/fleets/create", api.CreateFleet)
		protected.POST("/fleets/add-ship", api.AddShipToFleet)
		protected.GET("/fleets", api.GetFleets)
		// Captain endpoints
		protected.GET("/captains", api.GetCaptains)
		protected.POST("/captains/assign", api.AssignCaptain)
		protected.POST("/captains/unassign", api.UnassignCaptain)
		protected.POST("/captains/upgrade-stars", api.UpgradeCaptainStars)
		// Tavern endpoints
		protected.POST("/tavern/summon-captain", api.SummonCaptain)
		protected.POST("/tavern/exchange-shards", api.ExchangeShards)
		// PVE endpoints
		protected.GET("/pve/targets", api.GetPveTargets)
		protected.POST("/pve/engage", api.EngagePve)
		// Ship Militia endpoints
		protected.POST("/ship/militia/assign", api.AssignShipMilitia)
		protected.POST("/ship/militia/unassign", api.UnassignShipMilitia)
		protected.POST("/ship/militia/recruit", api.RecruitMilitia)
		// Weather endpoint
		protected.GET("/weather", api.GetWeather)
		// PvP endpoints
		protected.GET("/pvp/targets", api.GetPvpTargets)
		protected.POST("/pvp/attack", api.AttackPvp)
		// Stationing endpoints
		protected.POST("/fleets/station", api.StationFleet)
		protected.POST("/fleets/recall", api.RecallFleet)
		protected.GET("/fleets/resource-nodes", api.GetResourceNodes)
		// Cargo Transfer endpoints
		protected.POST("/fleets/cargo/transfer-to-fleet", api.TransferToFleet)
		protected.POST("/fleets/cargo/transfer-to-island", api.TransferToIsland)
	}

	// Dev Routes (require authentication + admin check is done in handlers)
	// Only enabled if DEV_ROUTES_ENABLED env var is set to "1", "true", or "TRUE"
	devRoutesEnabled := false
	devRoutesEnv := os.Getenv("DEV_ROUTES_ENABLED")
	if devRoutesEnv == "1" || strings.ToLower(devRoutesEnv) == "true" {
		devRoutesEnabled = true
	}
	logger.Info("[BOOT] dev routes enabled", "enabled", devRoutesEnabled)

	if devRoutesEnabled {
		devRoutes := e.Group("/dev")
		devRoutes.Use(auth.RequireAuth)
		{
			devRoutes.POST("/add-resources", api.DevAddResources)
			devRoutes.POST("/finish-building", api.DevFinishBuilding)
			devRoutes.POST("/finish-research", api.DevFinishResearch)
			devRoutes.POST("/finish-ship", api.DevFinishShip)
			devRoutes.POST("/time-skip", api.DevTimeSkip)
			devRoutes.POST("/grant-captain", api.DevGrantCaptain)
			devRoutes.POST("/grant-tickets", api.DevGrantTickets)
			devRoutes.POST("/simulate-engagement", api.DevSimulateEngagement)
			devRoutes.POST("/set-ship-militia", api.DevSetShipMilitia)
			devRoutes.POST("/set-ship-crew", api.DevSetShipMilitia)
		}
	}

	// Start Game Loop
	gameEngine := engine.NewEngine()
	gameEngine.Start()
	defer gameEngine.Stop()

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
