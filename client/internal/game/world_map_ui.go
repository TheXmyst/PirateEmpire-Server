package game

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// world_map_ui.go contains all World Map rendering and interaction logic.
// This file was extracted from main.go during Phase 1.6 refactoring to reduce file size.
// No logic or behavior changes were made - only code organization.

func (g *Game) DrawWorldMap(screen *ebiten.Image) {
	if g.waterShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.Uniforms = map[string]interface{}{
			"Time":         float32(time.Now().UnixMilli()%100000) / 1000.0,
			"CameraPos":    []float32{float32(g.camX), float32(g.camY)},
			"CameraZoom":   float32(g.camZoom),
			"ScreenCenter": []float32{float32(g.screenWidth / 2), float32(g.screenHeight / 2)},
		}
		screen.DrawRectShader(g.screenWidth, g.screenHeight, g.waterShader, op)
	} else {
		screen.Fill(color.RGBA{10, 30, 80, 255})
	}

	w, h := float64(g.screenWidth), float64(g.screenHeight)
	isModalOpen := g.IsModalOpen()

	if g.player != nil && len(g.player.Islands) > 0 {
		myIsland := g.player.Islands[0]
		sx := (float64(myIsland.X)-g.camX)*g.camZoom + w/2
		sy := (float64(myIsland.Y)-g.camY)*g.camZoom + h/2

		if g.imgIsland != nil {
			op := &ebiten.DrawImageOptions{}
			scale := 0.1 * g.camZoom
			bounds := g.imgIsland.Bounds()
			iw, ih := bounds.Dx(), bounds.Dy()
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(sx-(float64(iw)*scale)/2, sy-(float64(ih)*scale)/2)
			screen.DrawImage(g.imgIsland, op)
		} else {
			ebitenutil.DrawRect(screen, sx-10, sy-10, 20, 20, color.RGBA{0, 255, 0, 255})
		}
		ebitenutil.DebugPrintAt(screen, myIsland.Name, int(sx)-20, int(sy)-40)
	}

	// Draw PVE targets
	if len(g.ui.PveTargets) > 0 {
		for _, target := range g.ui.PveTargets {
			tx := (target.RealX-g.camX)*g.camZoom + w/2
			ty := (target.RealY-g.camY)*g.camZoom + h/2

			// Draw target icon (small fleet icon)
			targetSize := 15.0 * g.camZoom
			if targetSize < 5 {
				targetSize = 5
			}
			if targetSize > 30 {
				targetSize = 30
			}

			// Color based on tier
			targetColor := color.RGBA{150, 150, 150, 255} // Tier 1: gray
			switch target.Tier {
			case 2:
				targetColor = color.RGBA{100, 150, 255, 255} // Tier 2: blue
			case 3:
				targetColor = color.RGBA{255, 200, 0, 255} // Tier 3: gold
			}

			// Highlight on hover
			if !isModalOpen && g.ui.HoverPveTarget == target.ID {
				targetColor = color.RGBA{255, 255, 255, 255} // White on hover
			}

			vector.DrawFilledRect(screen, float32(tx-targetSize/2), float32(ty-targetSize/2), float32(targetSize), float32(targetSize), targetColor, true)
			vector.StrokeRect(screen, float32(tx-targetSize/2), float32(ty-targetSize/2), float32(targetSize), float32(targetSize), 1, color.White, true)

			// Draw name and tier above
			nameY := ty - targetSize/2 - 20
			if nameY > 20 {
				ebitenutil.DebugPrintAt(screen, target.Name, int(tx)-40, int(nameY))
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Tier %d", target.Tier), int(tx)-20, int(nameY)+15)

				// DANGER RATING (Hover only)
				if !isModalOpen && g.ui.HoverPveTarget == target.ID {
					// Find active fleet for calculation
					var activeFleet *domain.Fleet

					// Retrieve active fleet logic
					if g.player != nil && len(g.player.Islands) > 0 {
						island := g.player.Islands[0]
						if island.ActiveFleetID != nil {
							for _, f := range island.Fleets {
								if f.ID == *island.ActiveFleetID {
									activeFleet = &f
									break
								}
							}
						}
					}

					// Get Rating
					rating := GetDangerRating(activeFleet, target)

					// Draw Rating Text
					ratingY := int(nameY) + 35
					textW := len(rating.Label) * 6
					textX := int(tx) - textW/2

					vector.DrawFilledRect(screen, float32(textX-5), float32(ratingY-2), float32(textW+10), 16, color.RGBA{0, 0, 0, 200}, true)
					ebitenutil.DebugPrintAt(screen, rating.Label, textX, ratingY)
					vector.DrawFilledRect(screen, float32(textX-15), float32(ratingY+2), 8, 8, rating.Color, true)
				}
			}
		}
	}

	// Draw PVP targets (RED)
	if len(g.ui.PvpTargets) > 0 {
		// Debug print count on screen (top left or near center)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("PvP Targets: %d", len(g.ui.PvpTargets)), 10, 50)

		for _, target := range g.ui.PvpTargets {
			tx := (float64(target.X)-g.camX)*g.camZoom + w/2
			ty := (float64(target.Y)-g.camY)*g.camZoom + h/2

			// Draw target icon (small fleet icon)
			targetSize := 18.0 * g.camZoom // Slightly bigger than PvE
			if targetSize < 8 {
				targetSize = 8
			}
			if targetSize > 35 {
				targetSize = 35
			}

			// Color: RED for PvP
			targetColor := color.RGBA{255, 50, 50, 255}

			// Highlight on hover
			if !isModalOpen && g.ui.HoverPvpTarget == target.ID {
				targetColor = color.RGBA{255, 100, 100, 255}
			}

			// Render Sprite
			if target.IsFleet {
				// Fleet Icon (Square or Ship Sprite)
				vector.DrawFilledRect(screen, float32(tx-targetSize/2), float32(ty-targetSize/2), float32(targetSize), float32(targetSize), targetColor, true)
				// Small arrow if moving
				if target.Speed > 0 && target.TargetX != nil && target.TargetY != nil {
					// Draw simple dir line
					dx := *target.TargetX - target.RealX
					dy := *target.TargetY - target.RealY
					dist := math.Sqrt(dx*dx + dy*dy)
					if dist > 0 {
						lineLen := 10.0
						vector.StrokeLine(screen, float32(tx), float32(ty), float32(tx+(dx/dist)*lineLen), float32(ty+(dy/dist)*lineLen), 2, color.RGBA{255, 255, 255, 200}, true)
					}
				}
			} else if g.imgIsland != nil {
				op := &ebiten.DrawImageOptions{}
				scale := 0.1 * g.camZoom
				bounds := g.imgIsland.Bounds()
				iw, ih := bounds.Dx(), bounds.Dy()
				op.GeoM.Scale(scale, scale)
				op.GeoM.Translate(tx-(float64(iw)*scale)/2, ty-(float64(ih)*scale)/2)
				screen.DrawImage(g.imgIsland, op)
			} else {
				vector.DrawFilledRect(screen, float32(tx-targetSize/2), float32(ty-targetSize/2), float32(targetSize), float32(targetSize), targetColor, true)
			}

			// Show tooltip on hover instead of white border
			// Fix: Ensure ID is not empty to avoid matching default "empty hover" state
			if !isModalOpen && g.ui.HoverPvpTarget == target.ID && target.ID != "" {
				// Guard: Don't show tooltip for own island or empty data
				if (g.player != nil && target.Name == g.player.Username) || target.Name == "" || target.Tier == 0 {
					continue
				}

				// Tooltip background
				tooltipW := 220.0
				tooltipH := 70.0 // Taller for 3 lines
				tooltipX := tx + targetSize/2 + 10
				tooltipY := ty - tooltipH/2

				vector.DrawFilledRect(screen, float32(tooltipX), float32(tooltipY), float32(tooltipW), float32(tooltipH), color.RGBA{20, 20, 30, 230}, true)
				vector.StrokeRect(screen, float32(tooltipX), float32(tooltipY), float32(tooltipW), float32(tooltipH), 2, color.RGBA{218, 165, 32, 255}, true)

				// Tooltip content
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Joueur: %s", target.Name), int(tooltipX)+10, int(tooltipY)+10)
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TH Niveau: %d", target.Tier), int(tooltipX)+10, int(tooltipY)+25)
				lootClass := g.computeLootClass(target.Tier)
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Butin: %s", lootClass), int(tooltipX)+10, int(tooltipY)+40)
				// Note: Scouting info will be added later
			}

			nameY := ty - targetSize/2 - 25
			if nameY > 20 {
				displayName := target.Name
				if displayName == "" {
					displayName = "Unknown"
				}
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (Lvl %d)", displayName, target.Tier), int(tx)-40, int(nameY))
			}
		}
	} else {
		ebitenutil.DebugPrintAt(screen, "No PvP Targets Loaded", 10, 50)
	}

	// Draw Resource Nodes
	g.ui.ResourceNodesMutex.RLock()
	for _, node := range g.ui.ResourceNodes {
		tx := (float64(node.X)-g.camX)*g.camZoom + w/2
		ty := (float64(node.Y)-g.camY)*g.camZoom + h/2

		nodeSize := 12.0 * g.camZoom
		if nodeSize < 5 {
			nodeSize = 5
		}
		if nodeSize > 25 {
			nodeSize = 25
		}

		// Color based on resource type
		var nodeColor color.RGBA
		switch node.Type {
		case domain.Wood:
			nodeColor = color.RGBA{139, 69, 19, 255} // Brown
		case domain.Stone:
			nodeColor = color.RGBA{128, 128, 128, 255} // Grey
		case domain.Gold:
			nodeColor = color.RGBA{255, 215, 0, 255} // Gold
		case domain.Rum:
			nodeColor = color.RGBA{139, 0, 0, 255} // Dark Red
		default:
			nodeColor = color.RGBA{200, 200, 200, 255}
		}

		if !isModalOpen && g.ui.HoverResourceNode == node.ID {
			// Highlight
			r, gr, b, a := nodeColor.RGBA()
			nodeColor = color.RGBA{uint8(r>>8) + 50, uint8(gr>>8) + 50, uint8(b>>8) + 50, uint8(a >> 8)}
		}

		vector.DrawFilledCircle(screen, float32(tx), float32(ty), float32(nodeSize/2), nodeColor, true)

		if !isModalOpen && g.ui.HoverResourceNode == node.ID {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%.1fx)", string(node.Type), node.Richness), int(tx)-20, int(ty)-20)
		}
	}
	g.ui.ResourceNodesMutex.RUnlock()

	// Draw Player Fleets (Moving/Returning only) - PROTECTED BY MUTEX
	g.ui.PlayerMutex.RLock()
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, f := range g.player.Islands[0].Fleets {
			// Draw fleets that are traveling or stationed at sea
			if (f.IsMoving() || f.State == domain.FleetStateSeaStationed) && len(f.Ships) > 0 {
				// Use First Ship visual position
				s := f.Ships[0]
				tx := (s.RealX-g.camX)*g.camZoom + w/2
				ty := (s.RealY-g.camY)*g.camZoom + h/2

				size := 12.0 * g.camZoom

				// Draw Square
				col := color.RGBA{0, 200, 0, 255} // Green
				vector.DrawFilledRect(screen, float32(tx-size/2), float32(ty-size/2), float32(size), float32(size), col, true)

				// Optional: Name
				if g.camZoom > 0.5 {
					ebitenutil.DebugPrintAt(screen, f.Name, int(tx), int(ty)-20)
				}
			}
		}
	}
	g.ui.PlayerMutex.RUnlock()

	// Draw Wind Indicator
	g.DrawWindIndicator(screen)

	// Draw Recall Button (Below Wind Indicator)
	g.DrawRecallButton(screen)

	g.DrawHUD(screen)

	// Draw PvP Button (always visible on world map)
	g.DrawPvpButton(screen)

	// Draw PvP UI Modals (layered on top)
	g.DrawPvpTargetList(screen)
	g.DrawPvpFleetSelection(screen)
	g.DrawPvpConfirmation(screen)

	if g.ui.ShowPveConfirmModal {
		g.DrawPveConfirmModal(screen)
	}

	// Draw Free Navigation Confirmation
	if g.ui.ShowNavFreeConfirm {
		g.DrawNavFreeConfirmModal(screen)
	}

	// Draw PvP Result (reuse PvE result UI)
	if g.ui.ShowPvpResultUI && g.ui.PvpCombatResult != nil {
		g.DrawPvpResultUI(screen)
	}

	// Chat overlay shared with island view
	g.DrawChat(screen)

	// Social overlay (shared between island and world map)
	g.DrawSocialUI(screen)

	// Draw Station Menu Overlay
	if g.ui.ShowStationMenu {
		g.DrawStationMenu(screen)
	}

	// SYSTEMIC: Draw Global Modals if active (Fix soft-lock in Sea View)
	g.DrawFleetUI(screen)
	g.DrawCaptainUI(screen)

	// Draw PvP Interception Modal
	if g.ui.ShowInterceptConfirm {
		g.DrawInterceptConfirmModal(screen)
	}

	// Draw Pursuit Warning Banner
	g.DrawPursuitBanner(screen)
}

// DrawStationMenu renders the fleet selection modal for stationing
func (g *Game) DrawStationMenu(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	overlayColor := color.RGBA{0, 0, 0, 200}
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), overlayColor, false)

	panelW, panelH := 400.0, 500.0
	panelX, panelY := (w-panelW)/2, (h-panelH)/2

	// Panel Background
	draw9Slice(screen, g, panelX, panelY, panelW, panelH, 16)

	ebitenutil.DebugPrintAt(screen, "STATION FLEET", int(panelX)+20, int(panelY)+20)

	// Close Button
	closeBtnX, closeBtnY := panelX+panelW-40, panelY+10
	vector.DrawFilledRect(screen, float32(closeBtnX), float32(closeBtnY), 30, 30, color.RGBA{200, 50, 50, 255}, true)
	ebitenutil.DebugPrintAt(screen, "X", int(closeBtnX)+10, int(closeBtnY)+8)

	// List Idle Fleets
	yOffset := panelY + 60
	if g.player != nil && len(g.player.Islands) > 0 {
		fleets := g.player.Islands[0].Fleets
		available := false
		for _, f := range fleets {
			// Fleet List Item
			itemColor := color.RGBA{60, 60, 60, 255}

			vector.DrawFilledRect(screen, float32(panelX+20), float32(yOffset), float32(panelW-40), 40, itemColor, true)

			state := f.State
			if state == "" {
				state = domain.FleetStateIdle
			} // Default
			if f.StationedNodeID != nil {
				state = domain.FleetStateStationed
			}

			label := fmt.Sprintf("%s (%d ships) [%s]", f.Name, len(f.Ships), state)
			ebitenutil.DebugPrintAt(screen, label, int(panelX)+30, int(yOffset)+12)

			// Send Button (Only if Idle)
			if state == domain.FleetStateIdle && len(f.Ships) > 0 {
				btnX := panelX + panelW - 100
				btnY := yOffset + 5
				vector.DrawFilledRect(screen, float32(btnX), float32(btnY), 70, 30, color.RGBA{0, 150, 0, 255}, true)
				ebitenutil.DebugPrintAt(screen, "SEND", int(btnX)+15, int(btnY)+8)
			} else if state == domain.FleetStateStationed {
				// RECALL Button
				btnX := panelX + panelW - 100
				btnY := yOffset + 5
				vector.DrawFilledRect(screen, float32(btnX), float32(btnY), 70, 30, color.RGBA{200, 100, 0, 255}, true) // Orange
				ebitenutil.DebugPrintAt(screen, "RECALL", int(btnX)+10, int(btnY)+8)
			}

			yOffset += 50
			available = true
		}
		if !available {
			ebitenutil.DebugPrintAt(screen, "No fleets created", int(panelX)+30, int(yOffset))
		}
	}
}

// UpdateWorldMapLogic handles logic/simulation for the World Map screen (Always runs).
func (g *Game) UpdateWorldMapLogic() {
	// 1. Load Data (Always check polling)
	if len(g.ui.PveTargets) == 0 && !g.ui.PveTargetsBusy && g.player != nil && len(g.player.Islands) > 0 {
		g.loadPveTargets()
	}
	if len(g.ui.PvpTargets) == 0 && !g.ui.PvpTargetsBusy && g.player != nil && len(g.player.Islands) > 0 {
		g.loadPvpTargets()
	}
	if len(g.ui.ResourceNodes) == 0 && !g.ui.ResourceNodesBusy && g.player != nil && len(g.player.Islands) > 0 {
		g.loadResourceNodes()
	}

	if time.Since(g.lastPvePoll) > 1*time.Second {
		g.lastPvePoll = time.Now()
		g.loadPveTargets()
		g.loadWeather()
		g.loadPlayerData()
	}

	// 2. SIMULATION (Ensure visual updates continue even if modal is open)
	// Interpolate Player Fleets - PROTECTED BY MUTEX
	g.ui.PlayerMutex.Lock()
	if g.player != nil && len(g.player.Islands) > 0 {
		// Helper to simulate ship movement
		simShip := func(s *domain.Ship, tx, ty *float64, f *domain.Fleet) {
			if s.RealX == 0 && s.RealY == 0 {
				s.RealX = s.X
				s.RealY = s.Y
				return
			}

			// Movement Logic
			if tx != nil && ty != nil {
				dx := float64(*tx) - s.RealX
				dy := float64(*ty) - s.RealY
				dist := math.Sqrt(dx*dx + dy*dy)

				// Wind & Speed Logic
				angleRad := math.Atan2(float64(*ty)-s.RealY, float64(*tx)-s.RealX)
				angleDeg := angleRad * (180 / math.Pi)
				if angleDeg < 0 {
					angleDeg += 360
				}
				diff := math.Abs(angleDeg - g.ui.WindDirection)
				if diff > 180 {
					diff = 360 - diff
				}
				rad := diff * (math.Pi / 180.0)
				windFactor := 1.0 + (math.Cos(rad) * 0.3)

				totalWeightedSpeed := 0.0
				totalShips := 0
				for _, ship := range f.Ships {
					multiplier := 1.0
					switch ship.Type {
					case "sloop":
						multiplier = 1.0
					case "brigantine":
						multiplier = 0.9
					case "frigate":
						multiplier = 1.1
					case "galleon":
						multiplier = 0.7
					case "manowar":
						multiplier = 0.6
					}
					totalWeightedSpeed += multiplier
					totalShips++
				}
				baseSpeed := 5.0
				if totalShips > 0 {
					baseSpeed = 5.0 * (totalWeightedSpeed / float64(totalShips))
				}
				// Tech Bonuses
				bonus := 0.0
				for _, techID := range g.player.UnlockedTechs {
					switch techID {
					case "tech_naval_1":
						bonus += 0.03
					case "tech_naval_2":
						bonus += 0.04
					case "tech_naval_3":
						bonus += 0.05
					case "tech_naval_5":
						bonus += 0.08
					}
				}
				baseSpeed *= (1.0 + bonus)

				effectiveSpeed := baseSpeed * windFactor
				// Clamp DT to prevent "sprint" after lag spike/modal close if DT wasn't updated
				dt := g.dt
				if dt > 0.1 {
					dt = 0.1
				}

				move := effectiveSpeed * dt

				if move > dist {
					move = dist
				}

				if dist > 0 {
					ratio := move / dist
					s.RealX += dx * ratio
					s.RealY += dy * ratio
					s.X += dx * ratio
					s.Y += dy * ratio
				}
			}

			// Drift Bleed
			dt := g.dt
			if dt > 0.1 {
				dt = 0.1
			}
			rate := 0.5
			bleed := 1.0 - math.Exp(-rate*dt)
			s.RealX += (s.X - s.RealX) * bleed
			s.RealY += (s.Y - s.RealY) * bleed
		}

		for i := range g.player.Islands[0].Ships {
			simShip(&g.player.Islands[0].Ships[i], nil, nil, nil)
		}
		for i := range g.player.Islands[0].Fleets {
			f := &g.player.Islands[0].Fleets[i]
			for j := range f.Ships {
				// Convert *int to *float64 for simulation
				var tx, ty *float64
				if f.TargetX != nil {
					val := float64(*f.TargetX)
					tx = &val
				}
				if f.TargetY != nil {
					val := float64(*f.TargetY)
					ty = &val
				}
				simShip(&f.Ships[j], tx, ty, f)
			}

			// Diagnostic Log (Throttled) for Chasing state
			if f.State == domain.FleetStateChasingPvE && g.hammerFrame == 0 && len(f.Ships) > 0 {
				tx, ty := 0.0, 0.0
				if f.TargetX != nil {
					tx = float64(*f.TargetX)
				}
				if f.TargetY != nil {
					ty = float64(*f.TargetY)
				}
				g.Log("[UI_FLEET] state=%s target=(%.0f,%.0f) pos=(%.2f,%.2f)", f.State, tx, ty, f.Ships[0].RealX, f.Ships[0].RealY)
			}
		}
	}
	g.ui.PlayerMutex.Unlock()

	// Interpolate PVE Targets
	// Note: We do NOT update Hover here. Hover is Input.
	if !g.ui.PveEngageBusy {
		g.ui.PveTargetsMutex.RLock()
		for i := range g.ui.PveTargets {
			t := &g.ui.PveTargets[i]
			if t.RealX == 0 && t.RealY == 0 {
				t.RealX = float64(t.X)
				t.RealY = float64(t.Y)
			}

			dt := g.dt
			if dt > 0.1 {
				dt = 0.1
			}

			if t.TargetX != nil && t.TargetY != nil {
				dx := *t.TargetX - t.RealX
				dy := *t.TargetY - t.RealY
				dist := math.Sqrt(dx*dx + dy*dy)
				simSpeed := t.Speed
				if simSpeed < 5 {
					simSpeed = 5
				}
				moveDist := simSpeed * dt

				if dist < 2.0 {
					t.RealX = *t.TargetX
					t.RealY = *t.TargetY
				} else {
					if moveDist > dist {
						moveDist = dist
					}
					if dist > 0 {
						ratio := moveDist / dist
						t.RealX += dx * ratio
						t.RealY += dy * ratio
						t.X += dx * ratio
						// Bleed NPC drift (New: ensures NPC updates are as smooth as fleets)
						rate := 0.7
						bleed := 1.0 - math.Exp(-rate*dt)
						t.RealX += (t.X - t.RealX) * bleed
						t.RealY += (t.Y - t.RealY) * bleed
					}
				}
			}
			// The old bleed logic was here, now moved inside the `if dist > 0` block above.
		}
		// Diagnostic Log (Throttled)
		if g.hammerFrame == 0 { // ~once per 10 frames
			// g.Log("[SEA_SIM] tick dt=%.4f modal=%v targets=%d", g.dt, g.IsModalOpen(), len(g.ui.PveTargets))
		}
		g.ui.PveTargetsMutex.RUnlock()
	}

}

// UpdateWorldMapInteraction handles input for the World Map screen.
func (g *Game) UpdateWorldMapInteraction() {
	// 3. INPUT BLOCKING
	// Clear hovers by default to prevent stuck state when modals block updates
	g.ui.HoverPveTarget = ""
	g.ui.HoverPvpTarget = ""
	g.ui.HoverResourceNode = ""

	// Check if any modal is open (including StationMenu)
	isModalOpen := g.IsModalOpen()

	// 4. STATION MENU (Special Case: Managed by WorldMap but acts as Modal)
	// If StationMenu is open, it handles input and blocks map.
	// But IsModalOpen is true if StationMenu is open.
	// We process Station Menu input IF it is the top-level blocker (or at least if open).
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	if g.ui.ShowStationMenu {
		// Log input consumption if clicking
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// g.Log("[UI_INPUT] StationMenu consuming input")
		}

		// Handle Station Menu Logic
		panelW, panelH := 400.0, 500.0
		panelX, panelY := (w-panelW)/2, (h-panelH)/2
		closeBtnX, closeBtnY := panelX+panelW-40, panelY+10

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if fmx >= closeBtnX && fmx <= closeBtnX+30 && fmy >= closeBtnY && fmy <= closeBtnY+30 {
				g.ui.ShowStationMenu = false
				return
			}
			// Handle Fleet Selection
			yOffset := panelY + 60
			if g.player != nil && len(g.player.Islands) > 0 {
				for _, f := range g.player.Islands[0].Fleets {
					// ... (Keep existing logic simplified/copied)
					state := f.State
					if state == "" {
						state = domain.FleetStateIdle
					}
					if f.StationedNodeID != nil {
						state = domain.FleetStateStationed
					}

					if state == domain.FleetStateIdle && len(f.Ships) > 0 {
						btnX := panelX + panelW - 100
						btnY := yOffset + 5
						if fmx >= btnX && fmx <= btnX+70 && fmy >= btnY && fmy <= btnY+30 {
							go func(fleetID, nodeID string) {
								err := g.api.StationFleet(fleetID, nodeID)
								if err != nil {
									g.Log("Failed to station: %v", err)
								}
							}(f.ID.String(), g.ui.StationMenuNodeID)
							g.ui.ShowStationMenu = false
							return
						}
					} else if state == domain.FleetStateStationed {
						btnX := panelX + panelW - 100
						btnY := yOffset + 5
						if fmx >= btnX && fmx <= btnX+70 && fmy >= btnY && fmy <= btnY+30 {
							go func(fleetID string) {
								err := g.api.RecallFleet(fleetID)
								if err != nil {
									g.Log("Failed to recall: %v", err)
								}
							}(f.ID.String())
							g.ui.ShowStationMenu = false
							return
						}
					}
					yOffset += 50
				}
			}
		}
		// Block further map interaction
		return
	}

	// 5. GENERIC MODAL BLOCKING
	// If any other modal is open (Fleet, PvP, etc.), block map interaction
	if isModalOpen {
		return
	}

	// 6. MAP INTERACTION (Hover / Click)

	if !g.ui.PveEngageBusy && !g.ui.ShowPveResultUI {
		// Check PvE Hover
		g.ui.PveTargetsMutex.RLock()
		for i := range g.ui.PveTargets {
			t := &g.ui.PveTargets[i]
			tx := (t.RealX-g.camX)*g.camZoom + w/2
			ty := (t.RealY-g.camY)*g.camZoom + h/2
			targetSize := 15.0 * g.camZoom
			if targetSize < 5 {
				targetSize = 5
			}
			if targetSize > 30 {
				targetSize = 30
			}

			if fmx >= tx-targetSize/2 && fmx <= tx+targetSize/2 && fmy >= ty-targetSize/2 && fmy <= ty+targetSize/2 {
				if g.ui.HoverPveTarget != t.ID {
					// Log new hover (SSOT Check)
					// g.Log("[UI_TARGET] hover name=\"%s\" id=%s", t.Name, t.ID)
					// (Commented out to avoid spam, but ready for debug if needed. User asked for "logs/diag (temp)" but "logs légers".
					// I will add it but verify if I should throttle or just use Click.)
					// User said: "Client : log [UI_TARGET] hover/click id=<uuid> (pas de logs par tick)"
					// So I will UNCOMMENT it but only log ON CHANGE.
					if g.ui.HoverPveTarget != t.ID {
						fmt.Printf("[UI_TARGET] hover_pve id=%s name=\"%s\"\n", t.ID, t.Name)
					}
				}
				g.ui.HoverPveTarget = t.ID
			}
		}
		g.ui.PveTargetsMutex.RUnlock()

		// Check Resource Node Hover
		if g.ui.HoverPveTarget == "" {
			g.ui.ResourceNodesMutex.RLock()
			for _, node := range g.ui.ResourceNodes {
				tx := (float64(node.X)-g.camX)*g.camZoom + w/2
				ty := (float64(node.Y)-g.camY)*g.camZoom + h/2
				nodeSize := 12.0 * g.camZoom
				if nodeSize < 5 {
					nodeSize = 5
				}
				if nodeSize > 25 {
					nodeSize = 25
				}

				if fmx >= tx-nodeSize/2 && fmx <= tx+nodeSize/2 && fmy >= ty-nodeSize/2 && fmy <= ty+nodeSize/2 {
					g.ui.HoverResourceNode = node.ID
				}
			}
			g.ui.ResourceNodesMutex.RUnlock()
		}

		// Check PvP Hover
		newHoverPvp := ""
		if g.ui.HoverPveTarget == "" && g.ui.HoverResourceNode == "" {
			for _, target := range g.ui.PvpTargets {
				tx := (float64(target.X)-g.camX)*g.camZoom + w/2
				ty := (float64(target.Y)-g.camY)*g.camZoom + h/2
				targetSize := 18.0 * g.camZoom
				if targetSize < 8 {
					targetSize = 8
				}
				if targetSize > 35 {
					targetSize = 35
				}

				if fmx >= tx-targetSize/2 && fmx <= tx+targetSize/2 && fmy >= ty-targetSize/2 && fmy <= ty+targetSize/2 {
					newHoverPvp = target.ID
					break
				}
			}
		}
		if g.ui.HoverPvpTarget != newHoverPvp {
			if newHoverPvp != "" {
				fmt.Printf("[UI_TARGET] hover_pvp id=%s\n", newHoverPvp)
			}
			g.ui.HoverPvpTarget = newHoverPvp
		}
	}

	// Clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// 1. Check Return Button Click FIRST (before blocking on UI hover)
		w := float64(g.screenWidth)
		btnW, btnH := 100.0, 30.0
		btnX, btnY := w-110, 200.0
		if g.isMovingToReturn(btnX, btnY, btnW, btnH) {
			activeFleetID := g.player.Islands[0].ActiveFleetID
			if activeFleetID != nil {
				fid := activeFleetID.String()
				go func() {
					err := g.api.RecallFleet(fid)
					if err != nil {
						g.Log("Recall failed: %v", err)
					} else {
						g.Log("Ordre de retrait transmis.")
					}
					g.loadPlayerData()
				}()
				return
			}
		}

		// 2. Block map clicks if hovering ANY UI button (HUD, PvP button, or Return button itself)
		if g.ui.ShowPveEngageError || g.IsHoveringAnyButton() {
			return
		}

		if g.ui.HoverResourceNode != "" {
			g.ui.ShowStationMenu = true
			g.ui.StationMenuNodeID = g.ui.HoverResourceNode
			// g.Log("Opened Station Menu")
		} else if g.ui.HoverPveTarget != "" && !g.ui.PveEngageBusy && !g.ui.ShowPveResultUI {
			g.engagePveTarget(g.ui.HoverPveTarget)
		} else if g.ui.HoverPvpTarget != "" {
			g.engagePvpTarget(g.ui.HoverPvpTarget)
		} else {
			// Check if clicking own island while being pursued (Safe Harbor)
			if g.player != nil && len(g.player.Islands) > 0 {
				island := g.player.Islands[0]
				tx := (float64(island.X)-g.camX)*g.camZoom + w/2
				ty := (float64(island.Y)-g.camY)*g.camZoom + h/2
				islandSize := 30.0 * g.camZoom

				if fmx >= tx-islandSize/2 && fmx <= tx+islandSize/2 && fmy >= ty-islandSize/2 && fmy <= ty+islandSize/2 {
					// Check if any fleet is chased
					chased := false
					var chasedFleetID string
					for _, f := range island.Fleets {
						if f.ChasedByFleetID != nil {
							chased = true
							chasedFleetID = f.ID.String()
							break
						}
					}

					if chased {
						fmt.Printf("[PVP_INTERCEPT] safe_harbor click island_id=%s\n", island.ID)
						g.Log("[PVP] Vite ! Retour au port pour échapper à la poursuite !")
						go func(fid string) {
							err := g.api.RecallFleet(fid)
							if err != nil {
								g.Log("Recall failed: %v", err)
							}
							g.loadPlayerData()
						}(chasedFleetID)
						return
					}
				}
			}

			// 3. Water Click (Free Navigation)
			if g.ui.HoverPvpTarget == "" && g.ui.HoverResourceNode == "" && g.ui.HoverPveTarget == "" {
				g.handleNavFreeClick(fmx, fmy)
			}
		}
	}
}

// handleNavFreeClick handles clicks on water to initiate free navigation
func (g *Game) handleNavFreeClick(fmx, fmy float64) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	activeFleetID := g.player.Islands[0].ActiveFleetID
	if activeFleetID == nil {
		return
	}

	// Find the fleet
	var activeFleet *domain.Fleet
	for i := range g.player.Islands[0].Fleets {
		if g.player.Islands[0].Fleets[i].ID == *activeFleetID {
			activeFleet = &g.player.Islands[0].Fleets[i]
			break
		}
	}
	if activeFleet == nil {
		return
	}

	// Translate to world coordinates
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	worldX := int((fmx-w/2)/g.camZoom + g.camX)
	worldY := int((fmy-h/2)/g.camZoom + g.camY)

	// Determine Transition Type
	confirmType := "retarget"
	switch activeFleet.State {
	case domain.FleetStateIdle:
		confirmType = "docked"
	case domain.FleetStateChasingPvE:
		confirmType = "stop_chasing"
	case domain.FleetStateTravelingToAttack, domain.FleetStateReturningFromAttack:
		// Locked state
		return
	}

	g.ui.ShowNavFreeConfirm = true
	g.ui.NavFreeConfirmTargetX = worldX
	g.ui.NavFreeConfirmTargetY = worldY
	g.ui.NavFreeConfirmType = confirmType
}

// confirmNavFree executes the navigation command
func (g *Game) confirmNavFree() {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	activeFleetID := g.player.Islands[0].ActiveFleetID
	if activeFleetID == nil {
		return
	}

	fid := activeFleetID.String()
	tx, ty := g.ui.NavFreeConfirmTargetX, g.ui.NavFreeConfirmTargetY

	g.ui.ShowNavFreeConfirm = false

	go func() {
		err := g.api.NavigateFleet(fid, tx, ty)
		if err != nil {
			g.Log("Navigation failed: %v", err)
		} else {
			g.Log("Cap mis sur (%d, %d)", tx, ty)
		}
		// Refresh status to see movement
		g.loadPlayerData()
	}()
}
func (g *Game) loadPveTargets() {
	if g.ui.PveTargetsBusy {
		return
	}

	g.ui.PveTargetsBusy = true
	g.ui.PveTargetsError = ""

	go func() {
		defer func() {
			g.ui.PveTargetsBusy = false
		}()

		newTargets, err := g.api.GetPveTargets()
		if err != nil {
			g.ui.PveTargetsError = err.Error()
			// g.Log("PVE: Failed to load targets: %v", err)
			return
		}

		// Mutex Lock for Safe Update
		g.ui.PveTargetsMutex.Lock()
		defer g.ui.PveTargetsMutex.Unlock()

		currentTargets := g.ui.PveTargets

		finalTargets := make([]domain.PveTarget, len(newTargets))
		for i, nt := range newTargets {
			// Copy new data
			finalTargets[i] = nt

			// Init RealX/Y for new targets
			finalTargets[i].RealX = float64(nt.X)
			finalTargets[i].RealY = float64(nt.Y)

			// Find match in current for interpolation
			for _, ot := range currentTargets {
				if ot.ID == nt.ID {
					// Found old ghost state
					oldRealX := ot.RealX
					oldRealY := ot.RealY

					serverX := float64(nt.X)
					serverY := float64(nt.Y)

					// LATENCY COMPENSATION: Extrapolate NPC position
					latency := g.api.LastRTT / 4
					if nt.TargetX != nil && nt.TargetY != nil && latency > 0 {
						dx := *nt.TargetX - serverX
						dy := *nt.TargetY - serverY
						dist := math.Sqrt(dx*dx + dy*dy)
						if dist > 0 {
							simSpeed := nt.Speed
							if simSpeed < 5 {
								simSpeed = 5
							}
							move := simSpeed * latency.Seconds()
							if move > dist {
								move = dist
							}
							serverX += (dx / dist) * move
							serverY += (dy / dist) * move
						}
					}

					// Distance check
					dx := serverX - oldRealX
					dy := serverY - oldRealY
					dist := math.Sqrt(dx*dx + dy*dy)

					if dist > 300 {
						// Hard Snap for respawns
						finalTargets[i].RealX = serverX
						finalTargets[i].RealY = serverY
					} else {
						// Preserve visual position, let the simulation loop bleed the error
						finalTargets[i].RealX = oldRealX
						finalTargets[i].RealY = oldRealY
					}
					finalTargets[i].X = serverX
					finalTargets[i].Y = serverY
					break
				}
			}
		}

		g.ui.PveTargets = finalTargets
	}()
}

// validateFleetForCombat performs client-side pre-validation of fleet readiness
// Returns: (isValid, reasonCode, reasonMessage)
func (g *Game) validateFleetForCombat(fleet *domain.Fleet) (bool, string, string) {
	// Check if fleet has ships
	if len(fleet.Ships) == 0 {
		return false, "FLEET_INVALID_NO_SHIPS", "Flotte vide"
	}

	// Check if at least one ship is active (not destroyed, HP > 0)
	hasActiveShip := false
	for _, ship := range fleet.Ships {
		if ship.State != "Destroyed" && ship.Health > 0 {
			hasActiveShip = true
			break
		}
	}
	if !hasActiveShip {
		return false, "FLEET_INVALID_ALL_DESTROYED", "Tous les navires de la flotte sont détruits"
	}

	// Check if flagship exists and is active
	// Find flagship (first ship or explicit flagship)
	var flagship *domain.Ship
	for i := range fleet.Ships {
		if fleet.Ships[i].State != "Destroyed" && fleet.Ships[i].Health > 0 {
			flagship = &fleet.Ships[i]
			break
		}
	}
	if flagship == nil {
		return false, "FLEET_INVALID_NO_FLAGSHIP", "Aucun navire amiral disponible"
	}

	// Note: Captain injury check is done server-side
	// Client-side pre-validation doesn't have access to InjuredUntil field
	// The server will validate this and return appropriate error

	return true, "", ""
}

// engagePveTarget opens the confirmation modal for PvE chase
func (g *Game) engagePveTarget(targetID string) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	island := g.player.Islands[0]
	if len(island.Fleets) == 0 {
		g.Log("[PVE] No fleets available")
		g.ui.PveEngageError = "Aucune flotte disponible"
		g.ui.PveEngageErrorCode = "FLEET_INVALID_NO_SHIPS"
		g.ui.ShowPveEngageError = true
		return
	}

	// Find target to get tier for logging
	var targetName string
	var targetTier int
	for _, t := range g.ui.PveTargets {
		if t.ID == targetID {
			targetName = t.Name
			targetTier = t.Tier
			break
		}
	}

	// Use active fleet
	var fleet *domain.Fleet
	if island.ActiveFleetID != nil {
		for i := range island.Fleets {
			if island.Fleets[i].ID == *island.ActiveFleetID {
				fleet = &island.Fleets[i]
				break
			}
		}
	}

	if fleet == nil {
		g.Log("[PVE] No active fleet selected")
		g.ui.PveEngageError = "Veuillez activer une flotte dans le menu Gestion des Flottes"
		g.ui.PveEngageErrorCode = "NO_ACTIVE_FLEET"
		g.ui.ShowPveEngageError = true
		return
	}

	// Pre-validate fleet before engaging
	isValid, reasonCode, reasonMsg := g.validateFleetForCombat(fleet)
	if !isValid {
		g.Log("[PVE] Pre-validation failed: %s (%s)", reasonMsg, reasonCode)
		g.ui.PveEngageError = reasonMsg
		g.ui.PveEngageErrorCode = reasonCode
		g.ui.ShowPveEngageError = true
		return
	}

	// Open Confirmation Modal
	g.ui.ShowPveConfirmModal = true
	g.ui.PveConfirmTargetID = targetID
	g.ui.PveConfirmTargetName = targetName
	g.ui.PveConfirmTier = targetTier
	g.ui.PveConfirmFleetID = fleet.ID.String()
	g.ui.PveConfirmFleetName = fleet.Name

	g.Log("[UI_PVE] click target_name=%s target_id=%s", targetName, targetID)
}

// DrawPveConfirmModal renders the PvE chase confirmation dialog
func (g *Game) DrawPveConfirmModal(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	overlayColor := color.RGBA{0, 0, 0, 200}
	vector.DrawFilledRect(screen, 0, 0, float32(w), float32(h), overlayColor, false)

	panelW, panelH := 450.0, 250.0
	panelX, panelY := (w-panelW)/2, (h-panelH)/2

	// Draw Panel
	draw9Slice(screen, g, panelX, panelY, panelW, panelH, 16)

	// Title
	ebitenutil.DebugPrintAt(screen, "CONFIRMATION D'ATTAQUE", int(panelX)+20, int(panelY)+20)

	// Content
	msg1 := fmt.Sprintf("Lancer l'attaque contre %s (Niveau %d)", g.ui.PveConfirmTargetName, g.ui.PveConfirmTier)
	msg2 := fmt.Sprintf("avec la flotte %s ?", g.ui.PveConfirmFleetName)

	ebitenutil.DebugPrintAt(screen, msg1, int(panelX)+30, int(panelY)+70)
	ebitenutil.DebugPrintAt(screen, msg2, int(panelX)+30, int(panelY)+90)

	// Infos
	ebitenutil.DebugPrintAt(screen, "La flotte poursuivra sa cible jusqu'à l'engagement.", int(panelX)+30, int(panelY)+120)

	// Inputs
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)

	// Buttons
	btnW, btnH := 120.0, 40.0

	// Cancel (Red)
	cancelX, cancelY := panelX+50, panelY+panelH-60
	cancelCol := color.RGBA{180, 50, 50, 255}
	if fmx >= cancelX && fmx <= cancelX+btnW && fmy >= cancelY && fmy <= cancelY+btnH {
		cancelCol = color.RGBA{220, 70, 70, 255}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.ui.ShowPveConfirmModal = false
			if g.seaDMS != nil {
				g.seaDMS.SetMode(DMSCalm, "pve_cancel")
			}
			return
		}
	}
	vector.DrawFilledRect(screen, float32(cancelX), float32(cancelY), float32(btnW), float32(btnH), cancelCol, true)
	ebitenutil.DebugPrintAt(screen, "ANNULER", int(cancelX)+25, int(cancelY)+12)

	// Confirm (Green)
	confirmX, confirmY := panelX+panelW-50-btnW, panelY+panelH-60
	confirmCol := color.RGBA{50, 150, 50, 255}
	if fmx >= confirmX && fmx <= confirmX+btnW && fmy >= confirmY && fmy <= confirmY+btnH {
		confirmCol = color.RGBA{70, 200, 70, 255}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Execute Chase
			g.confirmPveChase()
			g.ui.ShowPveConfirmModal = false
			return
		}
	}
	vector.DrawFilledRect(screen, float32(confirmX), float32(confirmY), float32(btnW), float32(btnH), confirmCol, true)
	ebitenutil.DebugPrintAt(screen, "CONFIRMER", int(confirmX)+20, int(confirmY)+12)
}

// DrawNavFreeConfirmModal renders the Free Navigation confirmation dialog
func (g *Game) DrawNavFreeConfirmModal(screen *ebiten.Image) {
	w, h := float64(g.screenWidth), float64(g.screenHeight)
	panelW, panelH := 450.0, 200.0
	px, py := (w-panelW)/2, (h-panelH)/2

	// Semi-transparent overlay
	overlay := ebiten.NewImage(int(w), int(h))
	overlay.Fill(color.RGBA{0, 0, 0, 180})
	screen.DrawImage(overlay, nil)

	// Panel Background
	ebitenutil.DrawRect(screen, px, py, panelW, panelH, color.RGBA{20, 30, 45, 255})
	ebitenutil.DrawRect(screen, px+2, py+2, panelW-4, panelH-4, color.RGBA{40, 50, 70, 255})

	// Title based on type
	title := "Navigation Libre"
	msg := "Voulez-vous mettre le cap ?"
	switch g.ui.NavFreeConfirmType {
	case "docked":
		title = "Départ en Mer"
		msg = "Souhaitez-vous passer en navigation libre ?"
	case "retarget":
		title = "Changement de Cap"
		msg = "Souhaitez-vous changer de destination ?"
	case "stop_chasing":
		title = "Abandon de Poursuite"
		msg = "Souhaitez-vous abandonner la cible pour ce nouveau cap ?"
	}

	ebitenutil.DebugPrintAt(screen, title, int(px+20), int(py+20))
	ebitenutil.DebugPrintAt(screen, msg, int(px+20), int(py+60))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Coordonnées : (%d, %d)", g.ui.NavFreeConfirmTargetX, g.ui.NavFreeConfirmTargetY), int(px+20), int(py+90))

	// Buttons
	btnW, btnH := 120.0, 40.0

	// CONFIRM
	confX, confY := px+panelW/2-btnW-20, py+panelH-60
	confColor := color.RGBA{50, 150, 50, 255}
	ebitenutil.DrawRect(screen, confX, confY, btnW, btnH, confColor)
	ebitenutil.DebugPrintAt(screen, "CONFIRMER", int(confX+25), int(confY+12))

	// CANCEL
	canX, canY := px+panelW/2+20, py+panelH-60
	canColor := color.RGBA{150, 50, 50, 255}
	ebitenutil.DrawRect(screen, canX, canY, btnW, btnH, canColor)
	ebitenutil.DebugPrintAt(screen, "ANNULER", int(canX+35), int(canY+12))

	// Click Handling
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)

		if fmx >= confX && fmx <= confX+btnW && fmy >= confY && fmy <= confY+btnH {
			g.confirmNavFree()
		} else if fmx >= canX && fmx <= canX+btnW && fmy >= canY && fmy <= canY+btnH {
			g.ui.ShowNavFreeConfirm = false
		}
	}
}
func (g *Game) confirmPveChase() {
	g.Log("[UI_PVE] send /pve/attack target_id=%s fleet_id=%s", g.ui.PveConfirmTargetID, g.ui.PveConfirmFleetID)

	go func() {
		err := g.api.SendPveAttack(g.ui.PveConfirmFleetID, g.ui.PveConfirmTargetID)
		if err != nil {
			g.Log("[PVE] Chase start failed: %v", err)

			// Detect desync error
			if err.Error() == "Invalid target PVE ID" || err.Error() == "Cible PvE introuvable ou expirée" {
				g.ui.PveEngageError = "Cible PvE invalide (désync). Rafraîchis la carte."
				g.ui.ShowPveEngageError = true

				// Force refresh
				g.ui.PveTargets = nil
				g.loadPveTargets()
			} else {
				g.ui.PveEngageError = err.Error()
				g.ui.ShowPveEngageError = true
			}
			return
		}

		g.Log("[PVE] Chase started successfully")

		// Refresh status
		g.refreshStatusAfterDevAction()

		// TODO: Add Toast feedback if Toast system existed?
		// For now we rely on the fleet turning Green/Moving in the map.
	}()
}

// engagePvpTarget engages a PvP target
func (g *Game) engagePvpTarget(targetID string) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}

	island := g.player.Islands[0]

	// Find the target in the list to check if it's a fleet
	var target *domain.PveTarget
	for i := range g.ui.PvpTargets {
		if g.ui.PvpTargets[i].ID == targetID {
			target = &g.ui.PvpTargets[i]
			break
		}
	}

	if target == nil {
		fmt.Printf("[PVP] engage failed: target %s not found in local list\n", targetID)
		return
	}

	// Use active fleet
	var fleet *domain.Fleet
	if island.ActiveFleetID != nil {
		for i := range island.Fleets {
			if island.Fleets[i].ID == *island.ActiveFleetID {
				fleet = &island.Fleets[i]
				break
			}
		}
	}

	if fleet == nil {
		g.Log("[PVP] No active fleet selected")
		g.ui.PveEngageError = "Veuillez activer une flotte dans le menu Gestion des Flottes"
		g.ui.PveEngageErrorCode = "NO_ACTIVE_FLEET"
		g.ui.ShowPveEngageError = true
		return
	}

	// Case 1: Interception (Fleet vs Fleet at Sea)
	if target.IsFleet {
		fmt.Printf("[UI_TARGET] click_fleet id=%s name=\"%s\"\n", target.ID, target.Name)
		g.ui.InterceptTargetFleetID = target.ID
		g.ui.InterceptTargetName = target.Name
		g.ui.InterceptAttackerFleetID = fleet.ID.String()
		g.ui.InterceptAttackerName = fleet.Name
		g.ui.ShowInterceptConfirm = true
		g.ui.InterceptError = ""
		return
	}

	// Case 2: Island Attack (Standard PvP Travel)
	fmt.Printf("[UI_TARGET] click_island id=%s name=\"%s\"\n", target.ID, target.Name)

	// Pre-validate fleet before engaging
	isValid, reasonCode, reasonMsg := g.validateFleetForCombat(fleet)
	if !isValid {
		g.Log("[PVP] Pre-validation failed: %s (%s)", reasonMsg, reasonCode)
		g.ui.PveEngageError = reasonMsg
		g.ui.PveEngageErrorCode = reasonCode
		g.ui.ShowPveEngageError = true
		return
	}

	g.ui.PveEngageBusy = true
	g.ui.PveEngageError = ""
	g.ui.PveEngageErrorCode = ""
	g.ui.ShowPveEngageError = false
	g.ui.PveCombatResult = nil
	g.ui.ShowPveResultUI = false

	fleetID := fleet.ID

	g.Log("[PVP] engage start island=%s fleet=%s", targetID, fleetID.String()[:8])

	go func() {
		defer func() {
			g.ui.PveEngageBusy = false
		}()

		// Send fleet to attack (travel system)
		travelTime, distance, err := g.api.SendPvpAttack(fleetID.String(), targetID)
		if err != nil {
			g.ui.PveEngageError = err.Error()
			g.ui.ShowPveEngageError = true
			g.Log("[PVP] engage failed err=%v", err)
			return
		}

		g.Log("[PVP] Flotte envoyée ! Distance: %.0f, Temps: %.1f min", distance, travelTime)
		g.loadPlayerData()
		g.loadPvpTargets()
	}()
}

// loadWeather fetches wind info
func (g *Game) loadWeather() {
	go func() {
		dir, next, err := g.api.GetWeather()
		if err == nil {
			g.ui.WindDirection = dir
			g.ui.WindNextChange = next
		}
	}()
}

// DrawWindIndicator draws the compass arrow
func (g *Game) DrawWindIndicator(screen *ebiten.Image) {
	centerX := float64(g.screenWidth) - 60
	centerY := 120.0
	radius := 40.0

	// Draw Circle Background
	vector.DrawFilledCircle(screen, float32(centerX), float32(centerY), float32(radius), color.RGBA{0, 0, 0, 150}, true)
	vector.StrokeCircle(screen, float32(centerX), float32(centerY), float32(radius), 2, color.White, true)

	// Calculate Arrow Tip
	// Wind Direction 0 = East? North?
	// Usually 0=North, 90=East.
	// Math: Cos=X, Sin=Y logic uses 0=East (Right), 90=South (Down) in screen coords?
	// Let's assume standard Math: 0=Right, 90=Down (screen Y+).
	// If server generates 0-360 random:
	rad := g.ui.WindDirection * (math.Pi / 180.0)

	tipX := centerX + math.Cos(rad)*radius*0.8
	tipY := centerY + math.Sin(rad)*radius*0.8

	// Draw Line
	vector.StrokeLine(screen, float32(centerX), float32(centerY), float32(tipX), float32(tipY), 3, color.RGBA{100, 200, 255, 255}, true)

	// Label
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("WIND %.0f", g.ui.WindDirection), int(centerX)-20, int(centerY)+45)
}

// DrawRecallButton draws a "Retour à l'île" button below the wind indicator
func (g *Game) DrawRecallButton(screen *ebiten.Image) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	activeFleetID := g.player.Islands[0].ActiveFleetID
	if activeFleetID == nil {
		return
	}

	// Find active fleet to check if it's already home/idle
	var activeFleet *domain.Fleet
	for i := range g.player.Islands[0].Fleets {
		if g.player.Islands[0].Fleets[i].ID == *activeFleetID {
			activeFleet = &g.player.Islands[0].Fleets[i]
			break
		}
	}

	if activeFleet == nil || activeFleet.State == domain.FleetStateIdle || activeFleet.State == domain.FleetStateReturning || activeFleet.State == domain.FleetStateReturningFromAttack {
		return
	}

	w := float64(g.screenWidth)
	btnW, btnH := 100.0, 30.0
	btnX, btnY := w-110, 200.0 // Below Wind Indicator (centerY=120 + radius=40 + margin)

	// Draw Button
	col := color.RGBA{40, 60, 90, 200}
	if g.isMovingToReturn(btnX, btnY, btnW, btnH) {
		col = color.RGBA{60, 90, 130, 255}
	}
	ebitenutil.DrawRect(screen, btnX, btnY, btnW, btnH, col)
	ebitenutil.DebugPrintAt(screen, "RETOUR ILE", int(btnX+15), int(btnY+8))
}

func (g *Game) isMovingToReturn(x, y, w, h float64) bool {
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float64(mx), float64(my)
	return fmx >= x && fmx <= x+w && fmy >= y && fmy <= y+h
}

// loadResourceNodes loads nodes from server
func (g *Game) loadResourceNodes() {
	if g.ui.ResourceNodesBusy {
		return
	}

	g.ui.ResourceNodesBusy = true
	g.ui.ResourceNodesError = ""

	go func() {
		defer func() {
			g.ui.ResourceNodesBusy = false
		}()

		nodes, err := g.api.GetResourceNodes()
		if err != nil {
			g.ui.ResourceNodesError = err.Error()
			g.Log("Failed to load resource nodes: %v", err)
			return
		}

		g.ui.ResourceNodesMutex.Lock()
		g.ui.ResourceNodes = nodes
		g.ui.ResourceNodesMutex.Unlock()
	}()
}

// loadPlayerData fetches latest player/fleet state
func (g *Game) loadPlayerData() {
	if g.ui.PlayerDataBusy {
		return
	}
	g.ui.PlayerDataBusy = true

	go func() {
		defer func() { g.ui.PlayerDataBusy = false }()

		p, err := g.api.GetStatus()
		if err == nil && p != nil {
			// INTERPOLATION MERGE: Keep visual state while updating server state
			g.ui.PlayerMutex.RLock() // Use RLock to build oldMap
			if g.player != nil && len(g.player.Islands) > 0 && len(p.Islands) > 0 {
				oldIs := g.player.Islands[0]
				newIs := p.Islands[0]

				// Update visual resources (Dashboard parity)
				for k, v := range newIs.Resources {
					g.visualResources[k] = v
				}

				// Collect old visual state into a map
				oldMap := make(map[uuid.UUID]domain.Ship)
				for _, os := range oldIs.Ships {
					oldMap[os.ID] = os
				}
				for _, of := range oldIs.Fleets {
					for _, os := range of.Ships {
						oldMap[os.ID] = os
					}
				}
				g.ui.PlayerMutex.RUnlock()

				// Collect and update all new ships (in both lists)
				updateShip := func(ns *domain.Ship, tx, ty *float64, fleet *domain.Fleet) {
					if os, ok := oldMap[ns.ID]; ok {
						// Track if server position actually moved (signals a fresh sync)
						ns.ServerChanged = (math.Abs(ns.X-os.X) > 0.05 || math.Abs(ns.Y-os.Y) > 0.05)
						ns.LastServerX = os.X
						ns.LastServerY = os.Y

						// LATENCY COMPENSATION: Extrapolate server position forward by RTT/2
						latency := g.api.LastRTT / 4 // More conservative extrapolation
						if tx != nil && ty != nil && latency > 0 {
							dx := float64(*tx) - ns.X
							dy := float64(*ty) - ns.Y
							dist := math.Sqrt(dx*dx + dy*dy)
							if dist > 0 {
								// Parity logic for wind
								angleRad := math.Atan2(dy, dx)
								angleDeg := angleRad * (180 / math.Pi)
								if angleDeg < 0 {
									angleDeg += 360
								}

								diff := math.Abs(angleDeg - g.ui.WindDirection)
								if diff > 180 {
									diff = 360 - diff
								}
								windRad := diff * (math.Pi / 180.0)
								windFactor := 1.0 + (math.Cos(windRad) * 0.3)

								// Calculate Fleet Speed based on ship composition (Parity)
								totalWeightedSpeed := 0.0
								totalShips := 0
								for _, ship := range fleet.Ships {
									multiplier := 1.0
									switch ship.Type {
									case "sloop":
										multiplier = 1.0
									case "brigantine":
										multiplier = 0.9
									case "frigate":
										multiplier = 1.1
									case "galleon":
										multiplier = 0.7
									case "manowar":
										multiplier = 0.6
									}
									totalWeightedSpeed += multiplier
									totalShips++
								}

								baseSpeed := 5.0
								if totalShips > 0 {
									avgMultiplier := totalWeightedSpeed / float64(totalShips)
									baseSpeed = 5.0 * avgMultiplier
								}

								// Apply Tech Bonuses
								bonus := 0.0
								for _, techID := range p.UnlockedTechs {
									switch techID {
									case "tech_naval_1":
										bonus += 0.03
									case "tech_naval_2":
										bonus += 0.04
									case "tech_naval_3":
										bonus += 0.05
									case "tech_naval_5":
										bonus += 0.08
									}
								}
								speed := baseSpeed * (1.0 + bonus) * windFactor
								move := speed * latency.Seconds()
								if move > dist {
									move = dist
								}
								ratio := move / dist
								ns.X += dx * ratio
								ns.Y += dy * ratio
							}
						}

						if os.RealX != 0 || os.RealY != 0 {
							ns.RealX = os.RealX
							ns.RealY = os.RealY
						} else {
							ns.RealX = ns.X
							ns.RealY = ns.Y
						}
					} else {
						ns.RealX = ns.X
						ns.RealY = ns.Y
					}
				}

				for j := range newIs.Ships {
					updateShip(&newIs.Ships[j], nil, nil, nil)
				}
				for j := range newIs.Fleets {
					f := &newIs.Fleets[j]
					for k := range f.Ships {
						// Convert *int to *float64 for interpolation
						var tx, ty *float64
						if f.TargetX != nil {
							val := float64(*f.TargetX)
							tx = &val
						}
						if f.TargetY != nil {
							val := float64(*f.TargetY)
							ty = &val
						}
						updateShip(&f.Ships[k], tx, ty, f)
					}
				}

				// Log parity with legacy polling loop
				totalShips := 0
				for _, f := range newIs.Fleets {
					totalShips += len(f.Ships)
				}
				g.Log("[STATUS] refreshed fleets=%d ships=%d", len(newIs.Fleets), totalShips)
			} else {
				g.ui.PlayerMutex.RUnlock()
			}

			// Update player state (Atomic pointer swap) PROTECTED BY MUTEX
			g.ui.PlayerMutex.Lock()
			g.player = p
			g.ui.PlayerMutex.Unlock()
		}
	}()
}

// computeLootClass returns a string representation of the loot tier based on Town Hall level
func (g *Game) computeLootClass(tier int) string {
	if tier < 4 {
		return "Pauvre"
	}
	if tier < 7 {
		return "Moyen"
	}
	return "Riche"
}
