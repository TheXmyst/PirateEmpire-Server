package game

import (
	"fmt"
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// updateChatInput handles toggle, typing, and sending chat messages.
// Local-only for now; messages stay client-side but shared across both views.
func (g *Game) updateChatInput() {
	if g.state == StateLogin {
		return
	}

	g.updateChatSlide()

	// Detect click on the side tab to slide the chat in/out
	_, _, _, _, tabX, tabY, tabW, tabH := g.chatGeometry()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		fx, fy := float32(mx), float32(my)
		if fx >= tabX && fx <= tabX+tabW && fy >= tabY && fy <= tabY+tabH {
			if g.ui.ShowChat {
				g.ui.ShowChat = false
				g.ui.ChatInputFocus = false
				g.ui.ChatSlideTarget = 0
			} else {
				g.ui.ShowChat = true
				g.ui.ChatMinimized = false
				g.ui.ChatInputFocus = false
				g.ui.ChatSlideTarget = 1
				g.chatScroll = 0
				g.ensureChatLoaded()
			}
		}
	}

	// Mouse wheel scrolls chat history when open
	if g.ui.ShowChat && !g.ui.ChatMinimized {
		_, dy := ebiten.Wheel()
		if dy != 0 {
			g.chatScroll -= int(dy)
			g.clampChatScroll(g.visibleChatLines())
			// If scrolled to the top, try to load older messages
			if dy > 0 {
				if g.chatScroll >= g.maxChatScroll(g.visibleChatLines())-1 {
					g.fetchOlderChat()
				}
			}
		}
	}

	// Toggle chat panel visibility with C
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.ui.ShowChat = !g.ui.ShowChat
		if g.ui.ShowChat {
			g.ui.ChatMinimized = false
			g.ui.ChatInputFocus = false // n'entre pas en mode saisie pour éviter d'injecter "c"
			g.ui.ChatSlideTarget = 1
			g.chatScroll = 0
			g.ensureChatLoaded()
		} else {
			g.ui.ChatInputFocus = false
			g.ui.ChatSlideTarget = 0
		}
	}

	if !g.ui.ShowChat {
		return
	}

	// Minimize/restore with M
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.ui.ChatMinimized = !g.ui.ChatMinimized
		if g.ui.ChatMinimized {
			g.ui.ChatInputFocus = false
		}
	}

	if g.ui.ChatMinimized {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.ui.ChatMinimized = false
			g.ui.ChatInputFocus = true
			g.chatScroll = 0
		}
		return
	}

	// Focus chat with Enter when visible
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if g.ui.ChatInputFocus {
			// Send message
			txt := strings.TrimSpace(g.ui.ChatInput)
			if txt != "" {
				// Auto-prefix with username: message
				prefix := "Moi"
				if g.player != nil {
					prefix = g.player.Username
				}
				lower := strings.ToLower(txt)
				expected := strings.ToLower(prefix + ":")
				if !strings.HasPrefix(lower, expected) {
					txt = prefix + ": " + txt
				}
				if g.api != nil {
					if err := g.api.SendChat(txt); err != nil {
						g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: "Système", Text: fmt.Sprintf("Envoi échoué: %v", err), Timestamp: time.Now()})
					} else {
						g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: prefix, Text: txt, Timestamp: time.Now()})
					}
				}
				// Si pas d'API (mode déconnecté), afficher quand même localement
				if g.api == nil {
					g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: prefix, Text: txt, Timestamp: time.Now()})
				}
			}
			g.ui.ChatInput = ""
			g.ui.ChatInputFocus = false
			g.chatScroll = 0
		} else {
			g.ui.ChatInputFocus = true
		}
		return
	}

	// Blur chat with Escape
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.ui.ChatInputFocus = false
		return
	}

	if !g.ui.ChatInputFocus {
		return
	}

	// Typing handling
	for _, r := range ebiten.AppendInputChars(nil) {
		g.ui.ChatInput += string(r)
	}

	// Backspace (single step)
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(g.ui.ChatInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.ChatInput)
			g.ui.ChatInput = g.ui.ChatInput[:len(g.ui.ChatInput)-size]
		}
	}
}

// DrawChat renders the chat overlay; minimized view is a slim bar.
func (g *Game) DrawChat(screen *ebiten.Image) {
	x, y, width, height, tabX, tabY, tabW, tabH := g.chatGeometry()
	showPanel := g.ui.ShowChat || g.ui.ChatSlideProgress > 0.02

	// Side toggle tab
	vector.DrawFilledRect(screen, tabX, tabY, tabW, tabH, alpha(30, 30, 50, 220), true)
	vector.StrokeRect(screen, tabX, tabY, tabW, tabH, 2, alpha(200, 180, 120, 220), true)
	tabLabel := ">"
	if g.ui.ShowChat {
		tabLabel = "<"
	}
	ebitenutil.DebugPrintAt(screen, "CHAT", int(tabX)+4, int(tabY)+int(tabH)/2-12)
	ebitenutil.DebugPrintAt(screen, tabLabel, int(tabX)+int(tabW)/2-2, int(tabY)+int(tabH)/2+4)

	if !showPanel {
		return
	}

	// Background
	vector.DrawFilledRect(screen, x, y, width, height, alpha(30, 30, 40, 200), true)
	vector.StrokeRect(screen, x, y, width, height, 2, alpha(200, 180, 120, 200), true)

	header := "CHAT (C ou bouton latéral, M minimiser, Entrée saisir, Molette défiler)"
	ebitenutil.DebugPrintAt(screen, header, int(x)+8, int(y)+6)
	if g.chatScroll > 0 {
		ebitenutil.DebugPrintAt(screen, "↑ Historique", int(x)+int(width)-100, int(y)+6)
	}

	if g.ui.ChatMinimized {
		return
	}

	// Messages (scrollable)
	visibleLines := g.visibleChatLines()
	start := len(g.chatMessages) - visibleLines - g.chatScroll
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(g.chatMessages) {
		end = len(g.chatMessages)
	}
	if start > end {
		start = end
	}
	for i, msg := range g.chatMessages[start:end] {
		lineY := int(y) + 28 + i*18
		ts := msg.Timestamp.Format("15:04")
		display := msg.Text
		// Fallback to author prefix if text does not already start with it
		prefix := msg.Author + ":"
		if !strings.HasPrefix(strings.ToLower(display), strings.ToLower(prefix)) {
			display = msg.Author + ": " + msg.Text
		}
		ebitenutil.DebugPrintAt(screen, ts+" | "+display, int(x)+8, lineY)
	}

	// Input box
	boxY := y + height - 28
	vector.DrawFilledRect(screen, x+8, boxY, width-16, 20, alpha(10, 10, 10, 180), true)
	prompt := g.ui.ChatInput
	if !g.ui.ChatInputFocus && prompt == "" {
		prompt = "Appuyer sur Entrée pour écrire"
	}
	ebitenutil.DebugPrintAt(screen, prompt, int(x)+12, int(boxY)+4)
}

func (g *Game) chatGeometry() (x, y, width, height, tabX, tabY, tabW, tabH float32) {
	margin := float32(20)
	width = float32(360)
	tabW = float32(36)
	visibleLines := g.visibleChatLines()
	height = float32(60 + visibleLines*18)
	if g.ui.ChatMinimized {
		height = 32
	}
	y = float32(g.screenHeight) - height - margin
	visibleX := margin
	hiddenX := margin - width + tabW
	x = hiddenX + (visibleX-hiddenX)*float32(g.ui.ChatSlideProgress)
	tabH = float32(64)
	tabY = y + height/2 - tabH/2
	tabX = x + width - tabW
	return
}

func (g *Game) updateChatSlide() {
	speed := 0.2
	delta := g.ui.ChatSlideTarget - g.ui.ChatSlideProgress
	if delta > 0.001 {
		g.ui.ChatSlideProgress += delta * speed
		if g.ui.ChatSlideProgress > 1 {
			g.ui.ChatSlideProgress = 1
		}
		return
	}
	if delta < -0.001 {
		g.ui.ChatSlideProgress += delta * speed
		if g.ui.ChatSlideProgress < 0 {
			g.ui.ChatSlideProgress = 0
		}
		return
	}
	g.ui.ChatSlideProgress = g.ui.ChatSlideTarget
}

// alpha creates a color.RGBA shortcut.
func alpha(r, gC, b, a uint8) color.RGBA {
	return color.RGBA{r, gC, b, a}
}

func (g *Game) clampChatScroll(visible int) {
	maxScroll := 0
	if len(g.chatMessages) > visible {
		maxScroll = len(g.chatMessages) - visible
	}
	if g.chatScroll < 0 {
		g.chatScroll = 0
	}
	if g.chatScroll > maxScroll {
		g.chatScroll = maxScroll
	}
}

func (g *Game) visibleChatLines() int {
	return 10
}
