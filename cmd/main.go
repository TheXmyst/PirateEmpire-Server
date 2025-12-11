package main

import (
	"net/http"

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
		protected.GET("/fleets", api.GetFleets)
	}

	// Dev Routes (require authentication + admin check is done in handlers)
	devRoutes := e.Group("/dev")
	devRoutes.Use(auth.RequireAuth)
	{
		devRoutes.POST("/add-resources", api.DevAddResources)
		devRoutes.POST("/finish-building", api.DevFinishBuilding)
		devRoutes.POST("/finish-research", api.DevFinishResearch)
		devRoutes.POST("/finish-ship", api.DevFinishShip)
		devRoutes.POST("/time-skip", api.DevTimeSkip)
	}

	// Start Game Loop
	gameEngine := engine.NewEngine()
	gameEngine.Start()
	defer gameEngine.Stop()

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
