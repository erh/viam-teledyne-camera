// testgrab is a standalone tool that opens the camera, grabs one frame, and saves it as a JPEG.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"

	"viam-flir-camera/flircamera"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	serial := ""
	if len(os.Args) > 1 {
		serial = os.Args[1]
	}

	fmt.Println("Opening camera...")
	cam, err := flircamera.TestNewCamera(serial)
	if err != nil {
		return err
	}
	defer cam.Close()

	// Auto exposure, auto gain for realistic test.
	fmt.Println("Configuring (auto exposure, auto gain)...")
	if err := cam.Configure(0, 0, true, true); err != nil {
		return err
	}

	fmt.Println("Starting acquisition...")
	if err := cam.Start(); err != nil {
		return err
	}
	defer cam.Stop()

	// Grab several frames to let auto-exposure settle.
	var data []byte
	var width, height int
	for i := 0; i < 20; i++ {
		var err error
		data, width, height, err = cam.GrabRGB()
		if err != nil {
			return err
		}
		// Compute per-channel averages.
		var sumR, sumG, sumB uint64
		numPixels := len(data) / 3
		for p := 0; p < numPixels; p++ {
			sumR += uint64(data[p*3])
			sumG += uint64(data[p*3+1])
			sumB += uint64(data[p*3+2])
		}
		n := float64(numPixels)
		fmt.Printf("Frame %2d: %dx%d  R=%.1f G=%.1f B=%.1f\n",
			i+1, width, height, float64(sumR)/n, float64(sumG)/n, float64(sumB)/n)
	}

	// Print pixel values at various locations.
	spots := [][2]int{{0, 0}, {width / 2, 0}, {width - 1, 0},
		{0, height / 2}, {width / 2, height / 2}, {width - 1, height / 2},
		{0, height - 1}, {width / 2, height - 1}, {width - 1, height - 1}}
	fmt.Println("Pixel samples (last frame):")
	for _, s := range spots {
		i := (s[1]*width + s[0]) * 3
		fmt.Printf("  (%4d,%4d): R=%3d G=%3d B=%3d\n", s[0], s[1], data[i], data[i+1], data[i+2])
	}

	// Compute standard deviation.
	numPx := width * height
	var sumR2, sumG2, sumB2 float64
	for p := 0; p < numPx; p++ {
		sumR2 += float64(data[p*3])
		sumG2 += float64(data[p*3+1])
		sumB2 += float64(data[p*3+2])
	}
	avgR, avgG, avgB := sumR2/float64(numPx), sumG2/float64(numPx), sumB2/float64(numPx)
	var varR, varG, varB float64
	for p := 0; p < numPx; p++ {
		dr := float64(data[p*3]) - avgR
		dg := float64(data[p*3+1]) - avgG
		db := float64(data[p*3+2]) - avgB
		varR += dr * dr
		varG += dg * dg
		varB += db * db
	}
	n := float64(numPx)
	fmt.Printf("StdDev: R=%.1f G=%.1f B=%.1f\n",
		math.Sqrt(varR/n), math.Sqrt(varG/n), math.Sqrt(varB/n))

	// Convert RGB to image.NRGBA
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			img.SetNRGBA(x, y, color.NRGBA{R: data[i], G: data[i+1], B: data[i+2], A: 0xFF})
		}
	}

	outPath := "test.jpg"
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		return err
	}
	fmt.Printf("Saved to %s\n", outPath)
	return nil
}
