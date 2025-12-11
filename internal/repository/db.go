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
	err = DB.AutoMigrate(&domain.Player{}, &domain.Sea{}, &domain.Island{}, &domain.Building{}, &domain.Ship{})
	if err != nil {
		log.Fatal("failed to migrate database schema")
	}
}

// Helper to get DB instance
func GetDB() *gorm.DB {
	return DB
}
