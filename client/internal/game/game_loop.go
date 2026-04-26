package game

import (
	"fmt"
	"math"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (g *Game) DevLog(lType, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      lType,
		Message:   msg,
	}
	select {
	case g.logChan <- entry:
	default:
	}
	fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format("15:04:05"), lType, msg)
}

func (g *Game) Log(format string, a ...interface{}) {
	g.DevLog("INFO", format, a...)
}

func (g *Game) Update() error {
	select {
	case entry := <-g.logChan:
		g.consoleLog = append(g.consoleLog, entry)
		if len(g.consoleLog) > 500 {
			g.consoleLog = g.consoleLog[1:]
		}
	default:
	}

	// Calculate Delta Time at start of frame
	now := time.Now()
	if g.lastFrameTime.IsZero() {
		g.lastFrameTime = now
	}
	g.dt = now.Sub(g.lastFrameTime).Seconds()
	if g.dt > 0.1 {
		g.dt = 0.1
	} // Cap spike
	g.lastFrameTime = now

	// Update Resources with fresh DT
	g.updateVisualResources(g.dt)

	// Poll chat feed periodically (shared island/world views)
	g.chatPollTimer += g.dt
	if g.state != StateLogin && g.chatPollTimer >= 2.0 {
		g.chatPollTimer = 0
		g.pollChatFeed()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	// Route to state-specific update logic
	if err := g.updateByState(); err != nil {
		return err
	}
	if g.state == StateLogin {
		return nil
	}

	// Reload chat history when player changes (new session)
	if g.player != nil && g.chatHistoryPlayerID != g.player.ID {
		g.resetChatForPlayer(g.player.ID)
	}
	if g.player != nil && !g.chatHistoryLoaded {
		g.loadChatHistory()
	}

	// Reload social state when player changes (new session)
	if g.player != nil && g.socialStatePlayerID != g.player.ID {
		g.resetSocialForPlayer(g.player.ID)
	}
	if g.player != nil && !g.socialStateLoaded {
		g.loadSocialState()
		g.ensureSocialDefaults()
	}

	// Ensure chat is primed with latest messages when in-game
	g.ensureChatLoaded()
	// Ensure social defaults are present (leaderboards/mock search)
	g.ensureSocialDefaults()

	// Chat input is global across island and sea views
	g.updateChatInput()

	// Handle Async Updates (apply on main thread to avoid races)
	select {
	case p := <-g.updateChan:
		if p != nil {
			g.player = p
			g.lastUpdate = time.Now()
			g.consecutiveFailures = 0
			if len(p.Islands) > 0 {
				island := p.Islands[0]
				for k, v := range island.Resources {
					g.visualResources[k] = v
				}

				// Close ship assign modal if flag is set
				if shipyardUI.CloseShipAssignModalAfterNextStatus && shipyardUI.AssignSuccessTimer <= 0 {
					shipAssigned := false
					if shipyardUI.AssignSuccessShipID != nil && len(island.Fleets) > 0 {
						for _, ship := range island.Ships {
							if ship.ID == *shipyardUI.AssignSuccessShipID {
								if ship.FleetID != nil {
									shipAssigned = true
								}
								break
							}
						}
					}

					if shipAssigned {
						shipyardUI.ShowAddShipModal = false
						shipyardUI.SelectedShipID = nil
						shipyardUI.ModalMessage = ""
						shipyardUI.AssignSuccessShipID = nil
						shipyardUI.CloseShipAssignModalAfterNextStatus = false
					}
				}
			}
		} else {
			g.consecutiveFailures++
			if g.consecutiveFailures >= 3 {
				g.Log("Connection Lost!")
				g.state = StateLogin
				g.player = nil
				g.ui.LoginError = "Connection to server lost."
			}
		}
	default:
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.isPaused = !g.isPaused
	}

	if g.isPaused {
		return nil
	}

	g.hammerTimer++
	if g.hammerTimer%10 == 0 {
		g.hammerFrame = (g.hammerFrame + 1) % 2
	}
	g.ui.CaretTimer++

	g.UpdateShipyardMenu()
	_ = g.UpdateTavernUI()
	if g.techUI != nil {
		g.techUI.Update(g)
	}
	g.UpdateConstructionMenu()
	g.UpdateFleetUI()
	g.UpdateHUDInput()
	g.UpdateSocialUI()

	// DMS Debug Keys (F9/F10)
	if g.seaDMS != nil && g.isDev {
		g.seaDMS.HandleDebugKeys(inpututil.IsKeyJustPressed, ebiten.KeyF9, ebiten.KeyF10)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Route to state-specific draw logic
	g.drawByState(screen)

	// If state was Login or WorldMap, drawByState already returned
	if g.state == StateLogin || g.state == StateWorldMap {
		return
	}

	screenW, screenH := float64(g.screenWidth), float64(g.screenHeight)

	// BG
	if g.bgImage != nil {
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterLinear
		bounds := g.bgImage.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
		op.GeoM.Translate(-g.camX, -g.camY)
		op.GeoM.Scale(g.camZoom, g.camZoom)
		op.GeoM.Translate(screenW/2, screenH/2)
		screen.DrawImage(g.bgImage, op)
	}

	// Buildings
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			var icon *ebiten.Image
			switch b.Type {
			case "Scierie":
				icon = g.iconSawmill
			case "Mine d'Or":
				icon = g.iconGoldMine
			case "Carrière":
				icon = g.iconStoneQuarry
			case "Hôtel de Ville":
				icon = g.iconTownhall
			case "Entrepôt":
				icon = g.iconWarehouse
			case "Distillerie":
				icon = g.iconDistillery
			case "Chantier Naval":
				icon = g.iconShipyard
			case "Académie":
				icon = g.iconAcademy
			case "Tavern":
				icon = g.iconTavern
			case "Milice":
				icon = g.iconMilitia
			case "Infirmary":
				icon = g.iconInfirmary
			default:
				icon = g.iconRum
			}
			if icon != nil {
				op := &ebiten.DrawImageOptions{}
				op.Filter = ebiten.FilterLinear
				bounds := icon.Bounds()
				w, h := bounds.Dx(), bounds.Dy()
				op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
				op.GeoM.Translate(b.X, b.Y)
				op.GeoM.Translate(-g.camX, -g.camY)
				op.GeoM.Scale(g.camZoom, g.camZoom)
				op.GeoM.Translate(screenW/2, screenH/2)
				if b.Constructing {
					op.ColorScale.Scale(1, 1, 1, 0.7)
				}
				screen.DrawImage(icon, op)
			}
		}
	}

	g.DrawDashboard(screen)
	g.DrawHUD(screen)

	if g.ui.LocalizationMode {
		g.DrawLocalizationOverlay(screen)
	}

	if g.showDevMenu && !g.ui.LocalizationMode {
		g.DrawDevTool(screen)
	}

	g.DrawShipyardMenu(screen)
	g.DrawTavernUI(screen)
	// Other UI draws
	g.DrawMilitiaUI(screen)
	g.DrawCaptainUI(screen)
	g.DrawInfirmaryUI(screen)
	g.DrawBuildingTooltips(screen)
	g.DrawBuildingModal(screen)
	g.DrawBuildButton(screen)
	g.DrawConstructionMenu(screen)
	g.DrawFleetUI(screen)
	g.DrawSocialUI(screen)

	if g.techUI != nil {
		g.techUI.Draw(screen, g)
	}

	// Chat overlay shared between island and sea views
	g.DrawChat(screen)

	g.DrawPveResultUI(screen)
	g.DrawPveEngageErrorModal(screen)
	g.DrawPrereqModal(screen)

	g.DrawError(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.screenWidth = outsideWidth
	g.screenHeight = outsideHeight
	return outsideWidth, outsideHeight
}

func (g *Game) calculateProduction(res domain.ResourceType) (float64, float64, float64, []string) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return 0, 0, 0, nil
	}
	island := g.player.Islands[0]
	total := island.ResourceGeneration[res]
	base := island.ResourceGenerationBase[res]
	bonus := island.ResourceGenerationBonus[res]

	var details []string
	if total > 0 {
		details = append(details, fmt.Sprintf("Production: %.0f/h", total))
		details = append(details, fmt.Sprintf("Base: %.0f/h", base))
		if bonus > 0 {
			details = append(details, fmt.Sprintf("Bonus Tech: +%.0f/h", bonus))
		}
	}
	return total, base, bonus, details
}

func (g *Game) updateVisualResources(dt float64) {
	if g.player == nil || len(g.player.Islands) == 0 {
		return
	}
	island := g.player.Islands[0]
	for res, rate := range island.ResourceGeneration {
		storage := g.visualResources[res]
		limit := island.StorageLimits[res]
		if storage < limit {
			added := (rate / 3600.0) * dt
			g.visualResources[res] = math.Min(storage+added, limit)
		}
	}
}

func (g *Game) handleBuildingClick() {
	mx, my := ebiten.CursorPosition()
	screenW, screenH := float64(g.screenWidth), float64(g.screenHeight)
	if g.player != nil && len(g.player.Islands) > 0 {
		for _, b := range g.player.Islands[0].Buildings {
			bx, by := b.X, b.Y
			sx := (bx-g.camX)*g.camZoom + screenW/2
			sy := (by-g.camY)*g.camZoom + screenH/2
			if math.Abs(float64(mx)-sx) <= 65*g.camZoom && math.Abs(float64(my)-sy) <= 65*g.camZoom {
				if b.Type == "Chantier Naval" {
					g.ui.ShowShipyard = true
					return
				}
				if b.Type == "Académie" {
					g.ui.ShowTechUI = true
					if g.techUI != nil {
						g.techUI.Visible = true
					}
					return
				}
				if b.Type == "Tavern" {
					if !b.Constructing {
						g.ui.ShowTavernUI = true
						return
					}
				}
				if b.Type == "Milice" {
					if !b.Constructing {
						g.ui.ShowMilitiaUI = true
						return
					}
				}
				if b.Type == "Infirmary" {
					if !b.Constructing {
						g.ui.ShowInfirmaryUI = true
						return
					}
				}
				g.selectedBuilding = &b
				return
			}
		}
	}
}

func (g *Game) refreshStatusAfterDevAction() {
	p, err := g.api.GetStatus()
	if err == nil {
		g.updateChan <- p
	}
	g.ui.DevActionBusy = false
}
