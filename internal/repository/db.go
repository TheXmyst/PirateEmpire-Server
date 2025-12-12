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
	DB, err = gorm.Open(sqlite.Open("seadogs.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	absPath, _ := filepath.Abs("seadogs.db")
	log.Printf("Database initialized at: %s", absPath)

	// Migrate the schema
	err = DB.AutoMigrate(&domain.Player{}, &domain.Sea{}, &domain.Island{}, &domain.Building{}, &domain.Ship{}, &domain.Fleet{}, &domain.Captain{})
	if err != nil {
		log.Fatal("failed to migrate database schema")
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
