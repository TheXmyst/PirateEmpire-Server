package game

import (
	"image"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

// UIAtlas represents the loaded UI texture atlas
// For the button pack, we don't strictly need a global atlas variable if we slice immediately,
// but we'll keep the variable convention.
var uiAtlas *ebiten.Image

// loadUIAtlas loads the pirate theme button pack and slices it.
func (g *Game) loadUIAtlas() {
	// Try loading the button pack file
	// Note: We try both "assets/..." and "client/assets/..." to be safe with CWD
	paths := []string{
		"assets/ui/button_wood_pack.png",
		"client/assets/ui/button_wood_pack.png",
		"../assets/ui/button_wood_pack.png",
	}
	var f *os.File
	var err error

	for _, p := range paths {
		f, err = os.Open(p)
		if err == nil {
			g.Log("UI Button Pack loaded from: %s", p)
			break
		}
	}

	// Fallback if file not found
	if f == nil {
		g.Log("UI Button Pack NOT FOUND. Checked paths: %v. Using Fallback Placeholders.", paths)

		// Create Placeholder textures (Hot Pink to be obvious)
		g.btnNormal = ebiten.NewImage(32, 32)
		g.btnNormal.Fill(color.RGBA{255, 105, 180, 255}) // Pink
		g.btnHover = ebiten.NewImage(32, 32)
		g.btnHover.Fill(color.RGBA{255, 160, 200, 255}) // Light Pink
		g.btnPressed = ebiten.NewImage(32, 32)
		g.btnPressed.Fill(color.RGBA{200, 50, 150, 255}) // Dark Pink
		return
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		g.Log("Failed to decode UI Button Pack: %v. Using Fallback (Cyan).", err)
		// Fallback Cyan
		g.btnNormal = ebiten.NewImage(32, 32)
		g.btnNormal.Fill(color.RGBA{0, 255, 255, 255})
		g.btnHover = ebiten.NewImage(32, 32)
		g.btnHover.Fill(color.RGBA{200, 255, 255, 255})
		g.btnPressed = ebiten.NewImage(32, 32)
		g.btnPressed.Fill(color.RGBA{0, 200, 200, 255})
		return
	}
	uiAtlas = ebiten.NewImageFromImage(img)

	bounds := uiAtlas.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	g.Log("UI Button Pack Dimensions: %d x %d", w, h)

	// Slicing logic: Vertical Stack of 3 (Normal, Hover, Pressed)
	if h <= 0 || w <= 0 {
		g.Log("Invalid image dimensions")
		return
	}

	btnH := h / 3

	// Normal: Top
	g.btnNormal = getSubImage(uiAtlas, 0, 0, w, btnH)

	// Hover: Middle
	g.btnHover = getSubImage(uiAtlas, 0, btnH, w, btnH)

	// Pressed: Bottom
	g.btnPressed = getSubImage(uiAtlas, 0, btnH*2, w, btnH)

	// Validate Slicing
	if g.btnNormal == nil {
		g.Log("Slicing failed, creating brown fallback")
		g.btnNormal = ebiten.NewImage(32, 32)
		g.btnNormal.Fill(color.RGBA{139, 69, 19, 255})
		g.btnHover = ebiten.NewImage(32, 32)
		g.btnHover.Fill(color.RGBA{205, 133, 63, 255})
		g.btnPressed = ebiten.NewImage(32, 32)
		g.btnPressed.Fill(color.RGBA{101, 67, 33, 255})
	} else {
		g.Log("UI Button Pack Sliced Successfully")
	}
}

func getSubImage(atlas *ebiten.Image, x, y, w, h int) *ebiten.Image {
	if atlas == nil {
		return nil
	}
	bounds := atlas.Bounds()
	aw, ah := bounds.Dx(), bounds.Dy()
	// Clamp to bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > aw {
		w = aw - x
	}
	if y+h > ah {
		h = ah - y
	}

	if w <= 0 || h <= 0 {
		return nil
	}

	// IMPORTANT: Return a sub-image that shares the underlying texture,
	// wrapped in *ebiten.Image.
	return atlas.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image)
}
