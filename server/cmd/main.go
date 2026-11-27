package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api"
	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/engine"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/TheXmyst/Sea-Dogs/server/internal/webclient"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

func main() {
	// Initialize Logger
	logger.Init()

	// Clé de signature des jetons : obligatoire, sauf en développement local.
	if err := auth.InitSecret(envBool("DEV_MODE")); err != nil {
		logger.Error("[BOOT] auth secret", "error", err)
		os.Exit(1)
	}

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

	// Derrière Caddy (reverse proxy), l'adresse IP réelle du joueur arrive dans
	// l'en-tête X-Forwarded-For. Echo ne lui fait confiance que si la requête
	// vient d'une adresse locale/privée (le proxy), donc un joueur ne peut pas
	// tricher sur son IP en envoyant lui-même cet en-tête.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	// Version navigateur du jeu, sur /play (voir docs/version_navigateur.md).
	webDir := webclient.Dir()
	webclient.Register(e, webDir)
	if webclient.Installed(webDir) {
		fmt.Printf("[WEB] Version navigateur servie sur /play depuis %s\n", webDir)
	} else {
		fmt.Printf("[WEB] Pas de version navigateur dans %s (/play désactivé)\n", webDir)
	}

	// Client PC : /version (mises à jour automatiques) et /download/SeaDogs.exe.
	downloadDir := webclient.DownloadDir()
	webclient.RegisterDownloads(e, downloadDir)
	if r, err := webclient.ReadRelease(downloadDir); err == nil {
		fmt.Printf("[WEB] Client PC version %s servi sur /download/%s\n", r.Version, r.File)
	} else {
		fmt.Printf("[WEB] Pas de client PC dans %s (/version désactivé)\n", downloadDir)
	}

	e.GET("/", func(c echo.Context) error {
		// Si la version navigateur est installée, l'adresse du site mène
		// directement au jeu.
		if webclient.Installed(webDir) {
			return c.Redirect(http.StatusFound, "/play/")
		}
		return c.String(http.StatusOK, "Welcome to Sea-Dogs MMO Server!")
	})

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// /status lit directement le monde en mémoire (il gère son verrou lui-même).
	// Le jeton est vérifié sans requête SQL (RequireToken) : la route reste
	// aussi légère qu'avant, mais on ne peut lire que son propre état.
	e.GET("/status", api.GetStatus, auth.RequireToken)

	// Toutes les autres routes de jeu travaillent encore directement sur la
	// base : elles passent par world.Middleware, qui sauvegarde la mémoire
	// avant la requête et la recharge après si la requête a écrit en base.
	game := e.Group("", world.Middleware)

	// Public routes (no authentication required)
	// Limite de tentatives par adresse IP, contre les essais de mots de passe
	// en rafale : 5 essais d'affilée, puis 1 nouvel essai toutes les 6 s.
	authLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(10.0 / 60.0),
			Burst:     5,
			ExpiresIn: 10 * time.Minute,
		}),
		DenyHandler: func(c echo.Context, _ string, _ error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "Trop de tentatives, réessaie dans quelques secondes"})
		},
	})
	game.POST("/register", api.Register, authLimiter)
	game.POST("/login", api.Login, authLimiter)

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

		protected.POST("/build-ship", api.StartShipConstruction)
		protected.POST("/fleets/create", api.CreateFleet)
		protected.POST("/fleets/add-ship", api.AddShipToFleet)
		protected.GET("/fleets", api.GetFleets)
		protected.POST("/fleets/set-active", api.SetActiveFleet)
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
	devRoutesEnabled := envBool("DEV_ROUTES_ENABLED")
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
		if err := e.Start(listenAddr()); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

// envBool lit une variable d'environnement booléenne ("1", "true", "TRUE"...).
func envBool(name string) bool {
	v, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && v
}

// listenAddr lit LISTEN_ADDR (défaut ":8080", toutes les interfaces).
// En production derrière Caddy, mets LISTEN_ADDR=127.0.0.1:8080 pour que le
// serveur ne soit joignable qu'en passant par Caddy (donc en HTTPS).
func listenAddr() string {
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		return v
	}
	return ":8080"
}
