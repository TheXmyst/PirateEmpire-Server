package main

import (
	"log"

	"github.com/TheXmyst/Sea-Dogs/client/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	// Initialize game
	g := game.NewGame()

	// Set window properties
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Sea Dogs - Pirate Empire")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// Run game
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
