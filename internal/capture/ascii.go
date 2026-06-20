package capture

import (
	"image"
	"strings"
)

// Gradients from darkest to lightest
const asciiGradient = " .:-=+*#%@"

// imageToASCII converts an image to an ASCII string representation.
// It handles resizing via simple nearest-neighbor sampling for speed.
func imageToASCII(img image.Image, targetWidth, targetHeight int) string {
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// If targets are 0, use a default aspect ratio
	if targetWidth <= 0 {
		targetWidth = 80
	}
	if targetHeight <= 0 {
		targetHeight = 24
	}

	// Calculate scaling factors
	xScale := float64(width) / float64(targetWidth)
	yScale := float64(height) / float64(targetHeight)

	var sb strings.Builder
	// Each row has targetWidth chars + 1 newline
	sb.Grow(targetHeight * (targetWidth + 1))

	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			// Find the corresponding pixel in the original image
			srcX := bounds.Min.X + int(float64(x)*xScale)
			srcY := bounds.Min.Y + int(float64(y)*yScale)

			r, g, b, _ := img.At(srcX, srcY).RGBA()

			// Convert to grayscale using luminance formula
			// Y = 0.299R + 0.587G + 0.114B
			// RGBA() returns values in 0-65535, so we scale it down to 0-255
			luminance := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0

			// Map luminance to ascii char
			idx := int((luminance / 255.0) * float64(len(asciiGradient)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(asciiGradient) {
				idx = len(asciiGradient) - 1
			}

			sb.WriteByte(asciiGradient[idx])
		}
		if y < targetHeight-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
