package stego

import (
	"bytes"
	"errors" // Import required for errors.Is
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestEmbedAndExtract(t *testing.T) {
	// 1. Create a dummy carrier image (10x10 pixels)
	// Capacity: 10x10 = 100 pixels * 3 channels = 300 bits total.
	// Header Overhead: 32 bits.
	// Available for Data: 268 bits / 8 = 33 bytes max.
	carrier := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	
	// Fill with a uniform color to ensure we aren't relying on zero-values
	draw.Draw(carrier, carrier.Bounds(), &image.Uniform{color.NRGBA{R: 100, G: 100, B: 100, A: 255}}, image.Point{}, draw.Src)

	// 2. Define secret data
	secret := []byte("Hello World!") // 12 bytes

	// 3. Embed
	stegoImg, err := Embed(carrier, secret)
	if err != nil {
		t.Fatalf("Failed to embed data: %v", err)
	}

	// 4. Extract
	extracted, err := Extract(stegoImg)
	if err != nil {
		t.Fatalf("Failed to extract data: %v", err)
	}

	// 5. Compare
	if !bytes.Equal(secret, extracted) {
		t.Errorf("Extracted data mismatch.\nExpected: %v\nGot: %v", secret, extracted)
	}
}

func TestCapacityCheck(t *testing.T) {
	// 2x2 image = 4 pixels * 3 channels = 12 bits total.
	// This is not even enough for the 32-bit length header.
	carrier := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	data := []byte("A")

	_, err := Embed(carrier, data)
	
	// Fix: Use errors.Is because the error is wrapped with context
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("Expected error wrapping ErrMessageTooLarge, got %v", err)
	}
}