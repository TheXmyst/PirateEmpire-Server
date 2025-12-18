package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api"
	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/engine"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize Database
	repository.InitDB()

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

	// Auto-Updater Routes
	// Helper variable for version - ideally load from config or Env
	const ServerClientVersion = "1.0.1"
	e.GET("/version", func(c echo.Context) error {
		scheme := "http"
		if c.Request().TLS != nil {
			scheme = "https"
		}
		host := c.Request().Host
		downloadURL := fmt.Sprintf("%s://%s/dist/client.exe", scheme, host)

		return c.JSON(http.StatusOK, map[string]string{
			"version": ServerClientVersion,
			"url":     downloadURL,
		})
	})
	// Serve static files from 'dist' directory
	e.Static("/dist", "dist")

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
		protected.POST("/add-resources", api.AddResources) // Dev Tool
		protected.POST("/build-ship", api.StartShipConstruction)
		protected.POST("/fleets/create", api.CreateFleet)
		protected.POST("/fleets/add-ship", api.AddShipToFleet)
		protected.POST("/fleets/set-active", api.SetActiveFleet)
		protected.POST("/fleets/assign-crew", api.AssignCrew)
		protected.POST("/fleets/unassign-crew", api.UnassignCrew)
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
		// PVP endpoints
		protected.GET("/pvp/targets", api.GetPvpTargets)
		protected.POST("/pvp/attack", api.AttackPvp)
		// Militia endpoints
		protected.POST("/militia/recruit", api.MilitiaRecruit)
	}

	// Dev Routes (require authentication + admin check is done in handlers)
	// Only enabled if DEV_ROUTES_ENABLED env var is set to "1", "true", or "TRUE"
	devRoutesEnabled := false
	devRoutesEnv := os.Getenv("DEV_ROUTES_ENABLED")
	if devRoutesEnv == "1" || strings.ToLower(devRoutesEnv) == "true" {
		devRoutesEnabled = true
	}
	fmt.Printf("[BOOT] dev routes enabled=%v\n", devRoutesEnabled)

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
			devRoutes.POST("/set-ship-crew", api.DevSetShipCrew)
			devRoutes.POST("/spawn-dummy", api.DevSpawnDummy)
		}
	}

	// Start Game Loop
	gameEngine := engine.NewEngine()
	gameEngine.Start()
	defer gameEngine.Stop()

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
