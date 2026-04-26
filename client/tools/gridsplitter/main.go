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

func main() {
	inputFile := flag.String("input", "", "Input PNG file path")
	outputDir := flag.String("output", "slices_grid", "Output directory")
	rows := flag.Int("rows", 3, "Number of rows")
	cols := flag.Int("cols", 3, "Number of columns")
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

	cellW := width / *cols
	cellH := height / *rows

    fmt.Printf("Image: %dx%d. Splitting into %dx%d grid. Cell Size: %dx%d\n", width, height, *cols, *rows, cellW, cellH)

	// Prepare output dir
	_ = os.MkdirAll(*outputDir, 0755)

	names := []string{
        "top_left", "top_center", "top_right",
        "mid_left", "center", "mid_right",
        "bot_left", "bot_center", "bot_right",
    }

	idx := 0
	for y := 0; y < *rows; y++ {
		for x := 0; x < *cols; x++ {
			
            rect := image.Rect(x*cellW, y*cellH, (x+1)*cellW, (y+1)*cellH)
			subImg := image.NewRGBA(image.Rect(0, 0, cellW, cellH))
            
            // Draw crop
            draw.Draw(subImg, subImg.Bounds(), img, rect.Min, draw.Src)

			// Save
            name := ""
            if idx < len(names) {
                name = names[idx]
            } else {
                name = fmt.Sprintf("part_%d", idx)
            }
            
			outName := fmt.Sprintf("%s/%s.png", *outputDir, name)
			outFile, err := os.Create(outName)
			if err != nil {
				log.Printf("Failed to create %s: %v", outName, err)
				continue
			}
			err = png.Encode(outFile, subImg)
			outFile.Close()
			if err == nil {
				fmt.Printf("Saved %s\n", outName)
			}
            idx++
		}
	}
}
