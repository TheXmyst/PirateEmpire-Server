package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

// ColorTransfer applies color correction from reference image to target image
type ColorTransfer struct {
	refHistR, refHistG, refHistB []float64
	refCDFR, refCDFG, refCDFB     []float64
}

// NewColorTransfer creates a new color transfer processor from a reference image
func NewColorTransfer(refImg image.Image) *ColorTransfer {
	ct := &ColorTransfer{
		refHistR: make([]float64, 256),
		refHistG: make([]float64, 256),
		refHistB: make([]float64, 256),
		refCDFR:  make([]float64, 256),
		refCDFG:  make([]float64, 256),
		refCDFB:  make([]float64, 256),
	}

	// Build histograms from reference image
	bounds := refImg.Bounds()
	pixelCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := refImg.At(x, y).RGBA()
			// Skip transparent pixels
			if a == 0 {
				continue
			}
			// Convert from 16-bit to 8-bit
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			ct.refHistR[r8]++
			ct.refHistG[g8]++
			ct.refHistB[b8]++
			pixelCount++
		}
	}

	// Normalize histograms
	if pixelCount > 0 {
		for i := 0; i < 256; i++ {
			ct.refHistR[i] /= float64(pixelCount)
			ct.refHistG[i] /= float64(pixelCount)
			ct.refHistB[i] /= float64(pixelCount)
		}
	}

	// Build Cumulative Distribution Functions (CDF)
	ct.refCDFR[0] = ct.refHistR[0]
	ct.refCDFG[0] = ct.refHistG[0]
	ct.refCDFB[0] = ct.refHistB[0]
	for i := 1; i < 256; i++ {
		ct.refCDFR[i] = ct.refCDFR[i-1] + ct.refHistR[i]
		ct.refCDFG[i] = ct.refCDFG[i-1] + ct.refHistG[i]
		ct.refCDFB[i] = ct.refCDFB[i-1] + ct.refHistB[i]
	}

	return ct
}

// Process applies color transfer to target image
func (ct *ColorTransfer) Process(targetImg image.Image) image.Image {
	bounds := targetImg.Bounds()
	result := image.NewRGBA(bounds)

	// Build target histograms
	targetHistR := make([]float64, 256)
	targetHistG := make([]float64, 256)
	targetHistB := make([]float64, 256)
	targetCDFR := make([]float64, 256)
	targetCDFG := make([]float64, 256)
	targetCDFB := make([]float64, 256)

	pixelCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := targetImg.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			targetHistR[r8]++
			targetHistG[g8]++
			targetHistB[b8]++
			pixelCount++
		}
	}

	// Normalize target histograms
	if pixelCount > 0 {
		for i := 0; i < 256; i++ {
			targetHistR[i] /= float64(pixelCount)
			targetHistG[i] /= float64(pixelCount)
			targetHistB[i] /= float64(pixelCount)
		}
	}

	// Build target CDF
	targetCDFR[0] = targetHistR[0]
	targetCDFG[0] = targetHistG[0]
	targetCDFB[0] = targetHistB[0]
	for i := 1; i < 256; i++ {
		targetCDFR[i] = targetCDFR[i-1] + targetHistR[i]
		targetCDFG[i] = targetCDFG[i-1] + targetHistG[i]
		targetCDFB[i] = targetCDFB[i-1] + targetHistB[i]
	}

	// Create mapping: for each target value, find corresponding reference value
	mappingR := make([]uint8, 256)
	mappingG := make([]uint8, 256)
	mappingB := make([]uint8, 256)

	for i := 0; i < 256; i++ {
		// Find reference value with closest CDF
		targetCDF := targetCDFR[i]
		bestMatch := 0
		bestDiff := math.Abs(ct.refCDFR[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFR[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingR[i] = uint8(bestMatch)

		targetCDF = targetCDFG[i]
		bestMatch = 0
		bestDiff = math.Abs(ct.refCDFG[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFG[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingG[i] = uint8(bestMatch)

		targetCDF = targetCDFB[i]
		bestMatch = 0
		bestDiff = math.Abs(ct.refCDFB[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFB[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingB[i] = uint8(bestMatch)
	}

	// Apply mapping to target image
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := targetImg.At(x, y).RGBA()
			if a == 0 {
				result.Set(x, y, color.RGBA{0, 0, 0, 0})
				continue
			}
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			// Apply mapping with optional intensity preservation
			newR := mappingR[r8]
			newG := mappingG[g8]
			newB := mappingB[b8]

			result.Set(x, y, color.RGBA{newR, newG, newB, uint8(a >> 8)})
		}
	}

	return result
}

// ProcessWithIntensityPreservation applies color transfer while preserving original intensity
func (ct *ColorTransfer) ProcessWithIntensityPreservation(targetImg image.Image, intensityWeight float64) image.Image {
	bounds := targetImg.Bounds()
	result := image.NewRGBA(bounds)

	// Build target histograms (same as Process)
	targetHistR := make([]float64, 256)
	targetHistG := make([]float64, 256)
	targetHistB := make([]float64, 256)
	targetCDFR := make([]float64, 256)
	targetCDFG := make([]float64, 256)
	targetCDFB := make([]float64, 256)

	pixelCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := targetImg.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			targetHistR[r8]++
			targetHistG[g8]++
			targetHistB[b8]++
			pixelCount++
		}
	}

	if pixelCount > 0 {
		for i := 0; i < 256; i++ {
			targetHistR[i] /= float64(pixelCount)
			targetHistG[i] /= float64(pixelCount)
			targetHistB[i] /= float64(pixelCount)
		}
	}

	targetCDFR[0] = targetHistR[0]
	targetCDFG[0] = targetHistG[0]
	targetCDFB[0] = targetHistB[0]
	for i := 1; i < 256; i++ {
		targetCDFR[i] = targetCDFR[i-1] + targetHistR[i]
		targetCDFG[i] = targetCDFG[i-1] + targetHistG[i]
		targetCDFB[i] = targetCDFB[i-1] + targetHistB[i]
	}

	mappingR := make([]uint8, 256)
	mappingG := make([]uint8, 256)
	mappingB := make([]uint8, 256)

	for i := 0; i < 256; i++ {
		targetCDF := targetCDFR[i]
		bestMatch := 0
		bestDiff := math.Abs(ct.refCDFR[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFR[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingR[i] = uint8(bestMatch)

		targetCDF = targetCDFG[i]
		bestMatch = 0
		bestDiff = math.Abs(ct.refCDFG[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFG[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingG[i] = uint8(bestMatch)

		targetCDF = targetCDFB[i]
		bestMatch = 0
		bestDiff = math.Abs(ct.refCDFB[0] - targetCDF)
		for j := 1; j < 256; j++ {
			diff := math.Abs(ct.refCDFB[j] - targetCDF)
			if diff < bestDiff {
				bestDiff = diff
				bestMatch = j
			}
		}
		mappingB[i] = uint8(bestMatch)
	}

	// Apply mapping with intensity preservation
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := targetImg.At(x, y).RGBA()
			if a == 0 {
				result.Set(x, y, color.RGBA{0, 0, 0, 0})
				continue
			}
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			// Original intensity (luminance)
			origIntensity := 0.299*float64(r8) + 0.587*float64(g8) + 0.114*float64(b8)

			// Mapped colors
			mappedR := float64(mappingR[r8])
			mappedG := float64(mappingG[g8])
			mappedB := float64(mappingB[b8])

			// Mapped intensity
			mappedIntensity := 0.299*mappedR + 0.587*mappedG + 0.114*mappedB

			// Blend: preserve original intensity partially
			if mappedIntensity > 0 {
				ratio := origIntensity / mappedIntensity
				ratio = intensityWeight*ratio + (1-intensityWeight)*1.0
				mappedR *= ratio
				mappedG *= ratio
				mappedB *= ratio
			}

			// Clamp
			newR := uint8(math.Max(0, math.Min(255, mappedR)))
			newG := uint8(math.Max(0, math.Min(255, mappedG)))
			newB := uint8(math.Max(0, math.Min(255, mappedB)))

			result.Set(x, y, color.RGBA{newR, newG, newB, uint8(a >> 8)})
		}
	}

	return result
}

func main() {
	refFile := flag.String("reference", "", "Reference PNG file (style to match)")
	targetFile := flag.String("target", "", "Target PNG file to correct (or directory)")
	outputFile := flag.String("output", "", "Output PNG file (or directory)")
	intensityWeight := flag.Float64("intensity", 0.3, "Intensity preservation weight (0.0 = full transfer, 1.0 = preserve original)")
	flag.Parse()

	if *refFile == "" {
		log.Fatal("Please provide a reference file using -reference")
	}

	if *targetFile == "" {
		log.Fatal("Please provide a target file or directory using -target")
	}

	// Load reference image
	refF, err := os.Open(*refFile)
	if err != nil {
		log.Fatalf("Failed to open reference file: %v", err)
	}
	defer refF.Close()

	refImg, err := png.Decode(refF)
	if err != nil {
		log.Fatalf("Failed to decode reference image: %v", err)
	}

	fmt.Printf("Loaded reference image: %s\n", *refFile)

	// Create color transfer processor
	ct := NewColorTransfer(refImg)
	fmt.Println("Analyzed reference color palette")

	// Process target(s)
	targetInfo, err := os.Stat(*targetFile)
	if err != nil {
		log.Fatalf("Failed to stat target: %v", err)
	}

	var targetFiles []string
	var outputFiles []string

	if targetInfo.IsDir() {
		// Process all PNG files in directory
		entries, err := os.ReadDir(*targetFile)
		if err != nil {
			log.Fatalf("Failed to read directory: %v", err)
		}

		outputDir := *outputFile
		if outputDir == "" {
			outputDir = *targetFile + "_corrected"
		}
		os.MkdirAll(outputDir, 0755)

		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
				targetFiles = append(targetFiles, filepath.Join(*targetFile, entry.Name()))
				outputFiles = append(outputFiles, filepath.Join(outputDir, entry.Name()))
			}
		}
	} else {
		// Single file
		targetFiles = []string{*targetFile}
		if *outputFile == "" {
			ext := filepath.Ext(*targetFile)
			base := (*targetFile)[:len(*targetFile)-len(ext)]
			outputFiles = []string{base + "_corrected.png"}
		} else {
			outputFiles = []string{*outputFile}
		}
	}

	if len(targetFiles) == 0 {
		log.Fatal("No PNG files found to process")
	}

	fmt.Printf("Processing %d file(s)...\n", len(targetFiles))

	for i, targetPath := range targetFiles {
		// Load target image
		targetF, err := os.Open(targetPath)
		if err != nil {
			log.Printf("Failed to open %s: %v", targetPath, err)
			continue
		}

		targetImg, err := png.Decode(targetF)
		targetF.Close()
		if err != nil {
			log.Printf("Failed to decode %s: %v", targetPath, err)
			continue
		}

		// Process
		var result image.Image
		if *intensityWeight > 0 {
			result = ct.ProcessWithIntensityPreservation(targetImg, *intensityWeight)
		} else {
			result = ct.Process(targetImg)
		}

		// Save
		outputPath := outputFiles[i]
		outputF, err := os.Create(outputPath)
		if err != nil {
			log.Printf("Failed to create %s: %v", outputPath, err)
			continue
		}

		err = png.Encode(outputF, result)
		outputF.Close()
		if err != nil {
			log.Printf("Failed to encode %s: %v", outputPath, err)
			continue
		}

		fmt.Printf("✓ Corrected: %s -> %s\n", filepath.Base(targetPath), filepath.Base(outputPath))
	}

	fmt.Println("\nDone! Color correction complete.")
}

