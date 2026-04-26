package game

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckForUpdates checks if a new version is available
// Returns: updateAvailable, downloadURL, error
func CheckForUpdates(apiURL string) (bool, string, error) {
	// 1. Fetch Version Info
	// apiURL is likely "http://host:8080/api" or similar?
	// The version route is at root "/version", so we need to be careful with URL construction.
	// Assuming apiURL in config is base URL like "http://host:8080"
	// Safe parse:

	// Quick hack: remove "/api" suffix if present? Or assuming Config.APIURL is base.
	// Let's assume Config.APIURL is "http://localhost:8080"
	versionURL := fmt.Sprintf("%s/version", apiURL)

	resp, err := http.Get(versionURL)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("status %s", resp.Status)
	}

	var res struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false, "", err
	}

	// 2. Compare Versions
	// Simple string compare for now, or major.minor.patch
	if res.Version != ClientVersion {
		// New version available!
		// Logic: If result > current -> Update
		// For now: Just "different" = update? No, ensure it's newer ideally but assume server always has latest.
		// Let's just return true if different.
		return true, res.URL, nil
	}

	return false, "", nil
}

// PerformUpdate downloads the new binary and restarts the game
func PerformUpdate(downloadURL string) error {
	fmt.Println("Downloading update from:", downloadURL)

	// 1. Download File
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	newExe := "client_new.exe"
	out, err := os.Create(newExe)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Download complete. Preparing to restart...")

	// 2. Create Batch Script for Swap & Restart
	// We need to:
	// - Wait for this process to exit
	// - Delete old client.exe
	// - Rename client_new.exe to client.exe
	// - Start client.exe
	// - Delete script

	exeName := filepath.Base(os.Args[0]) // e.g. "client.exe"
	batScript := `
@echo off
timeout /t 2 /nobreak >nul
del "` + exeName + `"
move "` + newExe + `" "` + exeName + `"
start "" "` + exeName + `"
del "%~f0"
`
	if err := os.WriteFile("update.bat", []byte(batScript), 0755); err != nil {
		return err
	}

	// 3. Execute Batch Script
	cmd := exec.Command("cmd", "/C", "start", "", "update.bat")
	if err := cmd.Start(); err != nil {
		return err
	}

	// 4. Exit Immediately
	os.Exit(0)
	return nil
}
