package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 1. Define Subcommands
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	grantCmd := flag.NewFlagSet("grant", flag.ExitOnError)
	moveCmd := flag.NewFlagSet("move", flag.ExitOnError)

	// 2. Define Flags
	listDbFlag := listCmd.String("db", "", "Path to database file")

	grantDbFlag := grantCmd.String("db", "", "Path to database file")
	grantUser := grantCmd.String("user", "", "Username to grant admin access")

	moveDbFlag := moveCmd.String("db", "", "Path to database file")
	moveUser := moveCmd.String("user", "", "Username to move")
	moveTarget := moveCmd.String("target", "", "Target username (to join their sea)")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'list', 'grant', or 'move' subcommands")
		os.Exit(1)
	}

	cmd := os.Args[1]
	dbPath := ""

	// 3. Parse specific subcommand headers
	switch cmd {
	case "list":
		listCmd.Parse(os.Args[2:])
		dbPath = *listDbFlag
	case "grant":
		grantCmd.Parse(os.Args[2:])
		dbPath = *grantDbFlag
	case "move":
		moveCmd.Parse(os.Args[2:])
		dbPath = *moveDbFlag
	case "fix-captains":
		// No specific flags for now, uses default db detection
	default:
		fmt.Println("Expected 'list', 'grant', 'move', or 'fix-captains' subcommands")
		os.Exit(1)
	}

	// 4. DB Auto-detection
	if dbPath == "" {
		paths := []string{"../../seadogs.db", "../seadogs.db", "seadogs.db"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				dbPath = p
				break
			}
		}
	}

	fmt.Printf("Using Database: %s\n", dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database at %s: %v", dbPath, err)
	}

	// 5. Build Logic
	switch cmd {
	case "list":
		var players []domain.Player
		db.Preload("Islands").Preload("Islands.Buildings").Find(&players)
		fmt.Printf("Displaying %d players:\n", len(players))
		fmt.Println("---------------------------------------------------")
		for _, p := range players {
			// Debug filter
			if p.Username != "TheXmyst" && p.Username != "PsyKoHazarD" {
				continue
			}

			fmt.Printf("PLAYER: %s (ID: %s)\n", p.Username, p.ID)
			fmt.Printf(" - Admin: %v\n", p.IsAdmin)
			if len(p.Islands) > 0 {
				fmt.Printf(" - SeaID: %s\n", p.Islands[0].SeaID)
				fmt.Printf(" - Coords: %d, %d\n", p.Islands[0].X, p.Islands[0].Y)

				thLvl := 0
				for _, b := range p.Islands[0].Buildings {
					if b.Type == "Hôtel de Ville" {
						thLvl = b.Level
						break
					}
				}
				fmt.Printf("CMP|%s|Sea:%s|X:%d|Y:%d|TH:%d\n", p.Username, p.Islands[0].SeaID, p.Islands[0].X, p.Islands[0].Y, thLvl)
			} else {
				fmt.Printf("CMP|%s|NoIsland\n", p.Username)
			}
		}

	case "grant":
		if *grantUser == "" {
			fmt.Println("Please provide -user argument")
			os.Exit(1)
		}
		var player domain.Player
		if err := db.Where("username = ?", *grantUser).First(&player).Error; err != nil {
			log.Fatalf("Player not found: %s", *grantUser)
		}
		player.IsAdmin = true
		db.Save(&player)
		fmt.Printf("Successfully granted ADMIN access to %s\n", player.Username)

	case "move":
		if *moveUser == "" || *moveTarget == "" {
			fmt.Println("Usage: move -user <username> -target <target_username>")
			os.Exit(1)
		}

		var player domain.Player
		if err := db.Preload("Islands").Where("username = ?", *moveUser).First(&player).Error; err != nil {
			log.Fatalf("Player not found: %s", *moveUser)
		}

		var targetPlayer domain.Player
		if err := db.Preload("Islands").Where("username = ?", *moveTarget).First(&targetPlayer).Error; err != nil {
			log.Fatalf("Target Player not found: %s", *moveTarget)
		}

		if len(player.Islands) == 0 || len(targetPlayer.Islands) == 0 {
			log.Fatalf("One of the players has no island")
		}

		// Update SeaID
		targetSea := targetPlayer.Islands[0].SeaID
		player.Islands[0].SeaID = targetSea

		// Move near target (offset 150)
		player.Islands[0].X = targetPlayer.Islands[0].X + 150
		player.Islands[0].Y = targetPlayer.Islands[0].Y + 150

		if err := db.Save(&player.Islands[0]).Error; err != nil {
			log.Fatalf("Failed to save move: %v", err)
		}
		fmt.Printf("Successfully moved %s to Sea %s (near %s)\n", player.Username, targetSea, targetPlayer.Username)

	case "fix-captains":
		fmt.Println("Scanning for stuck captains (assigned to non-existent ships)...")
		var captains []domain.Captain
		// Find captains that ARE assigned
		if err := db.Where("assigned_ship_id IS NOT NULL").Find(&captains).Error; err != nil {
			log.Fatalf("Failed to load captains: %v", err)
		}

		fixedCount := 0
		for idx := range captains {
			c := &captains[idx]

			// Check if ship exists
			var count int64
			db.Model(&domain.Ship{}).Where("id = ?", c.AssignedShipID).Count(&count)

			if count == 0 {
				fmt.Printf("Fixing captain %s (Name: %s) -> Unassigning from missing ship %s\n", c.ID, c.Name, c.AssignedShipID)
				c.AssignedShipID = nil
				db.Save(c)
				fixedCount++
			}
		}
		fmt.Printf("Done. Fixed %d captains.\n", fixedCount)
	}
}
