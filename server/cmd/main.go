package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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

	// Clean up ghost captains (assigned to non-existent ships)
	// This happens if a ship was physically deleted but the captain's foreign key wasn't nulled (no CASCADE/SET NULL)
	var ghostCount int64
	db.Model(&domain.Captain{}).
		Where("assigned_ship_id IS NOT NULL AND assigned_ship_id NOT IN (SELECT id FROM ships)").
		Count(&ghostCount)

	if ghostCount > 0 {
		fmt.Printf("[CLEANUP] Found %d ghost captains (assigned to deleted ships). Unassigning...\n", ghostCount)
		err := db.Exec("UPDATE captains SET assigned_ship_id = NULL WHERE assigned_ship_id IS NOT NULL AND assigned_ship_id NOT IN (SELECT id FROM ships)").Error
		if err != nil {
			fmt.Printf("[CLEANUP] Failed to unassign ghost captains: %v\n", err)
		} else {
			fmt.Printf("[CLEANUP] Success. %d captains freed.\n", ghostCount)
		}
	}

	// Initialize Economy
	if err := economy.LoadConfig("configs/buildings.json"); err != nil {
		panic(err)
	}

	// Charger le monde en mémoire (une seule grosse lecture, au démarrage).
	world, err := engine.LoadWorld(db)
	if err != nil {
		panic(err)
	}
	api.SetWorld(world)

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

	// /status lit directement le monde en mémoire (il gère son verrou lui-même).
	e.GET("/status", api.GetStatus) // Status endpoint accepts player_id as query param for backward compatibility

	// Toutes les autres routes de jeu travaillent encore directement sur la
	// base : elles passent par world.Middleware, qui sauvegarde la mémoire
	// avant la requête et la recharge après si la requête a écrit en base.
	game := e.Group("", world.Middleware)

	// Public routes (no authentication required)
	game.POST("/register", api.Register)
	game.POST("/login", api.Login)

	// Protected routes (require authentication)
	protected := game.Group("")
	protected.Use(auth.RequireAuth)
	{
		// Chat
		protected.POST("/chat/send", api.SendChat)
		protected.GET("/chat/feed", api.GetChat)

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
		protected.POST("/pve/engage", api.EngagePve)     // Deprecated?
		protected.POST("/pve/attack", api.SendPveAttack) // Phase K: PvE Chase Start
		// Ship Militia endpoints
		protected.POST("/ship/militia/assign", api.AssignShipMilitia)
		protected.POST("/ship/militia/unassign", api.UnassignShipMilitia)
		protected.POST("/ship/militia/recruit", api.RecruitMilitia)
		// Weather endpoint
		protected.GET("/weather", api.GetWeather)
		// PvP endpoints
		protected.GET("/pvp/targets", api.GetPvpTargets)
		protected.POST("/pvp/attack", api.AttackPvp)
		protected.POST("/pvp/send-attack", api.SendPvpAttack) // Fix: Register travel route
		protected.POST("/pvp/intercept/start", api.StartIntercept)
		protected.POST("/pvp/intercept/abort", api.AbortIntercept)
		// Stationing endpoints
		protected.POST("/fleets/station", api.StationFleet)
		protected.POST("/fleets/recall", api.RecallFleet)
		protected.POST("/fleets/navigate", api.NavigateFleet)
		protected.GET("/fleets/resource-nodes", api.GetResourceNodes)
		// Cargo Transfer endpoints
		protected.POST("/fleets/cargo/transfer-to-fleet", api.TransferToFleet)
		protected.POST("/fleets/cargo/transfer-to-island", api.TransferToIsland)

		// Social endpoints
		protected.POST("/social/friends/search", api.SearchFriends)
		protected.POST("/social/friends/add", api.AddFriend)
		protected.POST("/social/friends/remove", api.RemoveFriend)
		protected.GET("/social/friends", api.ListFriends)
		protected.POST("/guild/create", api.CreateGuild)
		protected.GET("/guild/me", api.GetGuild)
		protected.POST("/guild/leave", api.LeaveGuild)
		protected.GET("/leaderboard/players", api.GetLeaderboardPlayers)
		protected.GET("/leaderboard/guilds", api.GetLeaderboardGuilds)
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
		devRoutes := game.Group("/dev")
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

	// Start Game Loop (tick en mémoire + sauvegarde périodique)
	gameEngine := engine.NewEngine(world, saveIntervalFromEnv())
	gameEngine.Start()

	// Start server (dans une goroutine, pour pouvoir attendre Ctrl+C ici)
	serverErr := make(chan error, 1)
	go func() {
		if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Arrêt propre : sur Ctrl+C (SIGINT) ou SIGTERM (docker stop, systemd...)
	//  1. on arrête d'accepter des requêtes et on laisse finir celles en cours
	//  2. on arrête le tick
	//  3. on sauvegarde une dernière fois tout ce qui est en mémoire
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-quit:
		logger.Info("[BOOT] shutdown requested", "signal", sig.String())
	case err := <-serverErr:
		logger.Error("[BOOT] http server stopped", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("[BOOT] http shutdown", "error", err)
	}
	gameEngine.Stop() // inclut la sauvegarde finale
	logger.Info("[BOOT] bye, world saved")
}

// saveIntervalFromEnv lit WORLD_SAVE_INTERVAL_SECONDS (défaut : 5 s).
func saveIntervalFromEnv() time.Duration {
	if v := os.Getenv("WORLD_SAVE_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return engine.DefaultSaveInterval
}
