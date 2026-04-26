package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
)

// Point tracks pixel locations
type Point struct {
	X, Y int
}

func main() {
	inputFile := flag.String("input", "", "Input PNG file path")
	outputDir := flag.String("output", "slices", "Output directory")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Please provide an input file using -input")
	}

	// Open file
	f, err := os.Open(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	// Visited map
	visited := make([][]bool, height)
	for y := 0; y < height; y++ {
		visited[y] = make([]bool, width)
	}

	// Prepare output dir
	_ = os.MkdirAll(*outputDir, 0755)

	assetID := 0

	// Scan
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if visited[y][x] {
				continue
			}

			// Check alpha
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				visited[y][x] = true
				continue
			}

			// Found new unvisited opaque pixel -> Flood Fill / BFS
			region := []Point{}
			queue := []Point{{x, y}}
			visited[y][x] = true

			minX, minY := x, y
			maxX, maxY := x, y

			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				region = append(region, p)

				if p.X < minX { minX = p.X }
				if p.X > maxX { maxX = p.X }
				if p.Y < minY { minY = p.Y }
				if p.Y > maxY { maxY = p.Y }

				// Neighbors
				dirs := []Point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}
				for _, d := range dirs {
					nx, ny := p.X+d.X, p.Y+d.Y
					if nx >= 0 && nx < width && ny >= 0 && ny < height {
						if !visited[ny][nx] {
							_, _, _, na := img.At(nx, ny).RGBA()
							if na > 0 {
								visited[ny][nx] = true
								queue = append(queue, Point{nx, ny})
							}
						}
					}
				}
			}

			// Extract Rectangle
			rectW := maxX - minX + 1
			rectH := maxY - minY + 1
            
            // Filter noise: Minimum size 16x16
            if rectW < 16 || rectH < 16 {
                continue
            }

			subImg := image.NewRGBA(image.Rect(0, 0, rectW, rectH))
            draw.Draw(subImg, subImg.Bounds(), img, image.Point{minX, minY}, draw.Src)

			// Save
			outName := fmt.Sprintf("%s/asset_%d.png", *outputDir, assetID)
            // ... (rest of save logic)
			outFile, err := os.Create(outName)
			if err != nil {
				log.Printf("Failed to create %s: %v", outName, err)
				continue
			}
			err = png.Encode(outFile, subImg)
			outFile.Close()
			if err == nil {
				fmt.Printf("Saved %s (%dx%d)\n", outName, rectW, rectH)
				assetID++
			}
		}
	}
    if assetID == 0 {
        fmt.Println("No assets found! Try adjusting the alpha threshold or input file.")
    } else {
        fmt.Printf("Done. Extracted %d meaningful assets.\n", assetID)
    }
}
