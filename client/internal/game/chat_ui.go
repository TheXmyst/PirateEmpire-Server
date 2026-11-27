package game

import (
	"image/color"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// updateChatInput handles toggle, typing, and sending chat messages.
// Local-only for now; messages stay client-side but shared across both views.
func (g *Game) updateChatInput() {
	if g.state == StateLogin {
		return
	}

	g.updateChatSlide()
	g.ui.ChatMinimized = false

	if isMouseJustPressed(ebiten.MouseButtonLeft) {
		mx, my := cursorPosition()
		fx, fy := float64(mx), float64(my)
		px, py, pw, ph, bx, by, bw, bh := g.chatGeometry()
		bubble := hudRect{float64(bx), float64(by), float64(bw), float64(bh)}
		switch {
		case bubble.has(fx, fy) || (!g.ui.ShowChat && g.chatPreviewRect().has(fx, fy)):
			if g.ui.ShowChat {
				g.closeChat()
			} else {
				g.openChat()
			}
		case g.ui.ShowChat && g.chatCloseRect().has(fx, fy):
			g.closeChat()
		case g.ui.ShowChat && g.chatSendRect().has(fx, fy):
			g.sendChatInput()
		case g.ui.ShowChat && g.chatInputRect().has(fx, fy):
			// Tap / clic dans la zone de saisie : on écrit.
			g.ui.ChatInputFocus = true
			g.chatScroll = 0
		case g.ui.ShowChat && hudRect{float64(px), float64(py), float64(pw), float64(ph)}.has(fx, fy):
			// clic dans le chat : rien de plus
		default:
			if g.ui.ShowChat {
				g.ui.ChatInputFocus = false
			}
		}
	}

	// Mouse wheel scrolls chat history when open
	if g.ui.ShowChat && !g.ui.ChatMinimized {
		_, dy := wheelDelta()
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

	// C ouvre / ferme le chat (sauf pendant qu'on écrit dedans).
	if !g.ui.ChatInputFocus && isKeyJustPressed(ebiten.KeyC) {
		if g.ui.ShowChat {
			g.closeChat()
		} else {
			g.openChat()
		}
	}

	if !g.ui.ShowChat {
		return
	}

	// Focus chat with Enter when visible
	if isKeyJustPressed(ebiten.KeyEnter) {
		if g.ui.ChatInputFocus {
			g.sendChatInput()
		} else {
			g.ui.ChatInputFocus = true
		}
		return
	}

	// Blur chat with Escape
	if isKeyJustPressed(ebiten.KeyEscape) {
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
	if isKeyJustPressed(ebiten.KeyBackspace) {
		if len(g.ui.ChatInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(g.ui.ChatInput)
			g.ui.ChatInput = g.ui.ChatInput[:len(g.ui.ChatInput)-size]
		}
	}
}

func (g *Game) openChat() {
	g.ui.ShowChat = true
	g.ui.ChatMinimized = false
	g.ui.ChatInputFocus = false // n'entre pas en saisie (évite d'écrire le « c »)
	g.ui.ChatSlideTarget = 1
	g.chatScroll = 0
	g.chatUnread = 0
	g.chatPreviewUntil = time.Time{}
	g.ensureChatLoaded()
}

func (g *Game) closeChat() {
	g.ui.ShowChat = false
	g.ui.ChatInputFocus = false
	g.ui.ChatSlideTarget = 0
}

// sendChatInput envoie le message en cours de saisie.
func (g *Game) sendChatInput() {
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
				g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: "Système", Text: "Envoi échoué : " + friendlyError(err), Timestamp: time.Now()})
			} else {
				g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: prefix, Text: txt, Timestamp: time.Now()})
			}
		} else {
			// Pas d'API (mode déconnecté) : on l'affiche quand même localement.
			g.addIncomingChat(domain.ChatMessage{ID: uuid.New(), Author: prefix, Text: txt, Timestamp: time.Now()})
		}
	}
	g.ui.ChatInput = ""
	g.ui.ChatInputFocus = false
	g.chatScroll = 0
}

// noteNewChatMessage : un message d'un autre joueur vient d'arriver.
func (g *Game) noteNewChatMessage(m domain.ChatMessage) {
	if g.player != nil && strings.EqualFold(m.Author, g.player.Username) {
		return
	}
	if !g.ui.ShowChat {
		g.chatUnread++
		g.chatPreviewUntil = time.Now().Add(8 * time.Second)
	}
}

// chatParts sépare « Auteur: texte » en auteur et texte.
func chatParts(m domain.ChatMessage) (string, string) {
	t := m.Text
	if p := m.Author + ":"; strings.HasPrefix(strings.ToLower(t), strings.ToLower(p)) {
		t = strings.TrimSpace(t[len(p):])
	}
	return m.Author, t
}

const (
	chatPanelW    = 340.0
	chatBubbleR   = 32.0
	chatLineH     = 20.0
	chatHeaderH   = 44.0
	chatInputH    = 38.0
	chatPreviewMx = 260.0
)

// chatGeometry : le panneau du chat (x, y, largeur, hauteur) puis la bulle
// qui l'ouvre (carré autour du rond).
func (g *Game) chatGeometry() (x, y, width, height, tabX, tabY, tabW, tabH float32) {
	bx := hudMargin
	by := float64(g.screenHeight) - hudMargin - 2*chatBubbleR
	h := chatHeaderH + float64(g.visibleChatLines())*chatLineH + 10 + chatInputH + 12
	return float32(hudMargin), float32(by - 10 - h), float32(chatPanelW), float32(h),
		float32(bx), float32(by), float32(2 * chatBubbleR), float32(2 * chatBubbleR)
}

func (g *Game) chatPanelRect() hudRect {
	x, y, w, h, _, _, _, _ := g.chatGeometry()
	return hudRect{float64(x), float64(y), float64(w), float64(h)}
}

func (g *Game) chatCloseRect() hudRect {
	p := g.chatPanelRect()
	return hudRect{p.x + p.w - 44, p.y + 6, 36, 36}
}

func (g *Game) chatInputRect() hudRect {
	p := g.chatPanelRect()
	return hudRect{p.x + 10, p.y + p.h - chatInputH - 12, p.w - 20 - chatInputH - 8, chatInputH}
}

func (g *Game) chatSendRect() hudRect {
	in := g.chatInputRect()
	return hudRect{in.x + in.w + 8, in.y, chatInputH, chatInputH}
}

// chatPreviewRect : l'aperçu du dernier message, à côté de la bulle.
func (g *Game) chatPreviewRect() hudRect {
	if g.ui.ShowChat || len(g.chatMessages) == 0 || time.Now().After(g.chatPreviewUntil) {
		return hudRect{}
	}
	_, _, _, _, bx, by, bw, bh := g.chatGeometry()
	return hudRect{float64(bx+bw) + 10, float64(by+bh/2) - 18, chatPreviewMx, 36}
}

// chatHit : le point tombe-t-il sur le chat (bulle, aperçu ou panneau) ?
func (g *Game) chatHit(x, y float64) bool {
	if g.state == StateLogin {
		return false
	}
	_, _, _, _, bx, by, bw, bh := g.chatGeometry()
	if (hudRect{float64(bx), float64(by), float64(bw), float64(bh)}).has(x, y) {
		return true
	}
	if g.chatPreviewRect().has(x, y) {
		return true
	}
	return g.ui.ShowChat && g.chatPanelRect().has(x, y)
}

// DrawChat : la bulle du chat en bas à gauche, et le panneau quand il est ouvert.
func (g *Game) DrawChat(screen *ebiten.Image) {
	_, _, _, _, bx, by, bw, _ := g.chatGeometry()
	cx, cy := float64(bx+bw/2), float64(by+bw/2)
	mx, my := cursorPosition()
	hover := math.Hypot(float64(mx)-cx, float64(my)-cy) <= chatBubbleR
	drawRoundButton(screen, cx, cy, chatBubbleR, "chat", hover || g.ui.ShowChat)
	if g.ui.ShowChat {
		vector.StrokeCircle(screen, float32(cx), float32(cy), chatBubbleR, 2, themeGold, vecAA)
	}
	drawBadge(screen, cx+chatBubbleR-6, cy-chatBubbleR+6, g.chatUnread)

	if !g.ui.ShowChat {
		g.drawChatPreview(screen)
		return
	}

	p := g.chatPanelRect()
	drawThemePanel(screen, p.x, p.y, p.w, p.h, 16)
	drawText(screen, "Chat", titleFace(24), p.x+14, p.y+8, themeGold, alignLeft)
	if g.chatScroll > 0 {
		drawText(screen, "Historique", bodyFace(11, false), p.x+70, p.y+18, themeMuted, alignLeft)
	}
	cr := g.chatCloseRect()
	closeHover := cr.has(float64(mx), float64(my))
	drawRoundButton(screen, cr.x+cr.w/2, cr.y+cr.h/2, 16, "close", closeHover)

	// Messages (défilement à la molette ou au doigt)
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
	msgs := g.chatMessages[start:end]
	// Les messages se collent en bas, au-dessus de la saisie.
	lineY := p.y + chatHeaderH + float64(visibleLines-len(msgs))*chatLineH
	if len(msgs) == 0 {
		drawText(screen, "Aucun message pour l'instant.", bodyFace(13, false), p.x+p.w/2, p.y+p.h/2-20, themeMuted, alignCenter)
	}
	fTime, fAuthor, fMsg := bodyFace(11, false), bodyFace(13, true), bodyFace(13, false)
	maxX := p.x + p.w - 14
	for _, msg := range msgs {
		x := p.x + 14
		drawText(screen, msg.Timestamp.Format("15:04"), fTime, x, lineY+3, themeMuted, alignLeft)
		x += 38
		author, body := chatParts(msg)
		authorClr := color.Color(themeGold)
		if author == "Système" {
			authorClr = themeOrange
		}
		author = ellipsize(author, fAuthor, 110)
		drawText(screen, author, fAuthor, x, lineY, authorClr, alignLeft)
		x += textWidth(author, fAuthor) + 6
		drawText(screen, ellipsize(body, fMsg, maxX-x), fMsg, x, lineY, themeText, alignLeft)
		lineY += chatLineH
	}

	// Zone de saisie + bouton envoyer
	in := g.chatInputRect()
	fillRoundRect(screen, in.x, in.y, in.w, in.h, 10, color.RGBA{0, 0, 0, 120})
	border := color.Color(themeGoldDim)
	if g.ui.ChatInputFocus {
		border = themeGold
	}
	strokeRoundRect(screen, in.x, in.y, in.w, in.h, 10, 1.5, border)
	registerTextField(textField{id: "chat", x: in.x, y: in.y, w: in.w, h: in.h,
		value: &g.ui.ChatInput, focused: g.ui.ChatInputFocus, label: "Message"})
	fIn := bodyFace(14, false)
	if g.ui.ChatInput == "" && !g.ui.ChatInputFocus {
		drawText(screen, "Écrire un message…", fIn, in.x+12, in.y+10, themeMuted, alignLeft)
	} else {
		shown := g.ui.ChatInput
		for textWidth(shown, fIn) > in.w-28 && len(shown) > 0 {
			_, size := utf8.DecodeRuneInString(shown)
			shown = shown[size:]
		}
		drawText(screen, shown, fIn, in.x+12, in.y+10, themeText, alignLeft)
		if g.ui.ChatInputFocus && (g.ui.CaretTimer/30)%2 == 0 {
			cx := in.x + 13 + textWidth(shown, fIn)
			vector.StrokeLine(screen, float32(cx), float32(in.y+9), float32(cx), float32(in.y+in.h-9), 1.5, themeText, vecAA)
		}
	}
	sr := g.chatSendRect()
	sendHover := sr.has(float64(mx), float64(my))
	fillRoundRect(screen, sr.x, sr.y, sr.w, sr.h, 10, themeGold)
	if sendHover {
		fillRoundRect(screen, sr.x, sr.y, sr.w, sr.h, 10, color.NRGBA{255, 255, 255, 40})
	}
	drawIcon(screen, "send", sr.x+sr.w/2, sr.y+sr.h/2, 22, themeInk)
}

func (g *Game) drawChatPreview(screen *ebiten.Image) {
	r := g.chatPreviewRect()
	if r.w == 0 {
		return
	}
	author, body := chatParts(g.chatMessages[len(g.chatMessages)-1])
	fA, fB := bodyFace(13, true), bodyFace(13, false)
	author += " :"
	line := ellipsize(body, fB, r.w-24-textWidth(author, fA)-6)
	w := 24 + textWidth(author, fA) + 6 + textWidth(line, fB)
	fillRoundRect(screen, r.x, r.y, w, r.h, 12, color.RGBA{20, 13, 7, 190})
	drawText(screen, author, fA, r.x+12, r.y+9, themeGold, alignLeft)
	drawText(screen, line, fB, r.x+12+textWidth(author, fA)+6, r.y+9, themeText, alignLeft)
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
