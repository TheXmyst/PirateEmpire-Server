package economy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

func TestEconomyCalculator(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_buildings.json")

	jsonContent := `{
	  "buildings": {
	    "TestHall": {
	      "base_cost": { "wood": 100, "stone": 50 },
	      "cost_growth": 1.10,
	      "base_build_time": 60,
	      "time_growth": 1.1,
	      "base_production": 0,
	      "production_growth": 0
	    },
	    "TestMine": {
	      "base_cost": { "wood": 100 },
	      "cost_growth": 1.1,
	      "base_build_time": 30,
	      "time_growth": 1.1,
	      "base_production": 100,
	      "production_growth": 1.5
	    }
	  }
	}`

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// 1. Test LoadConfig
	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// 2. Test GetBuildingStats Level 1 (Should match base)
	stats1, err := GetBuildingStats("TestHall", 1)
	if err != nil {
		t.Fatalf("GetBuildingStats(1) failed: %v", err)
	}
	if stats1.Cost[domain.Wood] != 100 {
		t.Errorf("Level 1 Cost expected 100, got %f", stats1.Cost[domain.Wood])
	}
	if stats1.BuildTime != 60*time.Second {
		t.Errorf("Level 1 Time expected 60s, got %v", stats1.BuildTime)
	}

	// 3. Test GetBuildingStats Level 2 (Growth 1.1)
	// Cost = 100 * 1.1^1 = 110
	stats2, err := GetBuildingStats("TestHall", 2)
	if err != nil {
		t.Fatalf("GetBuildingStats(2) failed: %v", err)
	}
	if stats2.Cost[domain.Wood] != 110 {
		t.Errorf("Level 2 Cost expected 110, got %f", stats2.Cost[domain.Wood])
	}

	// 4. Test Production Logic (TestMine)
	// Level 1: 100
	mine1, _ := GetBuildingStats("TestMine", 1)
	if mine1.Production != 100 {
		t.Errorf("Mine Level 1 Production expected 100, got %f", mine1.Production)
	}

	// Level 2: 100 * 1.5^1 = 150
	mine2, _ := GetBuildingStats("TestMine", 2)
	if mine2.Production != 150 {
		t.Errorf("Mine Level 2 Production expected 150, got %f", mine2.Production)
	}

	// 5. Test Invalid Level
	_, err = GetBuildingStats("TestHall", 0)
	if err == nil {
		t.Error("Expected error for level 0")
	}
	_, err = GetBuildingStats("TestHall", 31)
	if err == nil {
		t.Error("Expected error for level 31")
	}

	// 6. Test Unknown Building
	_, err = GetBuildingStats("GhostHouse", 1)
	if err == nil {
		t.Error("Expected error for unknown building")
	}
}
