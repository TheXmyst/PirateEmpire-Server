//go:build !js

package game

import "github.com/hajimehoshi/ebiten/v2"

// Sur PC il y a un vrai clavier : rien à faire.

func syncSoftKeyboard(g *Game) {}

func fullscreenAvailable() bool { return true }

func isFullscreenNow() bool { return ebiten.IsFullscreen() }

func showFPS() bool { return false }
