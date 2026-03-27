package agent

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"

	// Register standard decoders for image.Decode.
	_ "golang.org/x/image/webp"
)

// Image sanitization constants (moved from telegram/image_sanitize.go).
const (
	// imageMaxSide is the maximum pixels per side before resize.
	imageMaxSide = 1200
	// imageSanitizeMaxBytes is the max file size after compression (5MB, Anthropic API limit).
	imageSanitizeMaxBytes = 5 * 1024 * 1024
)

// jpegQualities is the grid of quality levels to try during sanitization.
var jpegQualities = []int{85, 75, 65, 55, 45, 35}

// Ensure standard image decoders are registered.
func init() {
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "\x89PNG", png.Decode, png.DecodeConfig)
}

// SanitizeImage resizes and compresses an image for LLM vision input.
// Applied uniformly to all channels at the agent loop level.
// Pipeline: decode → resize if >1200px → JPEG compress until <5MB.
func SanitizeImage(inputPath string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Resize if either dimension exceeds the max side.
	if w > imageMaxSide || h > imageMaxSide {
		src = fitImage(src, imageMaxSide, imageMaxSide)
	}

	for _, quality := range jpegQualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
			return "", fmt.Errorf("encode jpeg (q=%d): %w", quality, err)
		}
		if buf.Len() <= imageSanitizeMaxBytes {
			outPath := filepath.Join(os.TempDir(), fmt.Sprintf("argoclaw_sanitized_%d.jpg", os.Getpid()))
			if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
				return "", fmt.Errorf("write sanitized image: %w", err)
			}
			return outPath, nil
		}
	}

	return "", fmt.Errorf("image too large even at lowest quality (dimensions: %dx%d)", w, h)
}

// fitImage scales the image to fit within maxW x maxH preserving aspect ratio.
// Uses Catmull-Rom (bicubic) interpolation for quality similar to Lanczos.
func fitImage(src image.Image, maxW, maxH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	ratio := float64(srcW) / float64(srcH)
	newW, newH := maxW, maxH

	if float64(maxW)/float64(maxH) > ratio {
		newW = int(float64(maxH) * ratio)
	} else {
		newH = int(float64(maxW) / ratio)
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
