package repository

import (
	"log"
	"path/filepath"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error
	// SQLite config: WAL mode for concurrency + busy_timeout to avoid "database is locked"
	dsn := "seadogs.db?_journal_mode=WAL&_busy_timeout=10000"
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	// Limit connections for SQLite (dev environment)
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	absPath, _ := filepath.Abs("seadogs.db")
	log.Printf("Database initialized at: %s (WAL mode)", absPath)

	// Migrate the schema
	err = DB.AutoMigrate(&domain.Player{}, &domain.Sea{}, &domain.Island{}, &domain.Building{}, &domain.Ship{}, &domain.Fleet{}, &domain.Captain{}, &domain.CaptainShardWallet{}, &domain.PvPInterceptCooldown{}, &domain.ChatMessage{}, &domain.Friend{}, &domain.Guild{}, &domain.GuildMember{}, &domain.PvEVictory{}, &domain.PvPVictory{})
	if err != nil {
		log.Fatal("failed to migrate database schema")
	}

	// Ensure unlocked_techs column exists (added after initial schema creation)
	if !DB.Migrator().HasColumn(&domain.Player{}, "unlocked_techs") {
		log.Println("Creating missing unlocked_techs column in players table...")
		if err := DB.Migrator().AddColumn(&domain.Player{}, "unlocked_techs"); err != nil {
			log.Printf("Warning: failed to add unlocked_techs column: %v", err)
		} else {
			log.Println("Successfully added unlocked_techs column")
		}
	}

	// Backfill morale_cruise: set NULL values to 50 (uninitialized -> default)
	// This ensures old rows are initialized, but still allows future 0 values
	result := DB.Model(&domain.Fleet{}).Where("morale_cruise IS NULL").Update("morale_cruise", 50)
	if result.Error != nil {
		log.Printf("Warning: failed to backfill morale_cruise: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Backfilled %d fleets with default morale_cruise=50", result.RowsAffected)
	}
}

// Helper to get DB instance
func GetDB() *gorm.DB {
	return DB
}
