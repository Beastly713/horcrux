package stego

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// ErrMessageTooLarge indicates the carrier image is too small to hold the data.
var ErrMessageTooLarge = errors.New("message too large for carrier image")

// ErrNoHiddenData indicates the extraction failed to find a valid length prefix.
var ErrNoHiddenData = errors.New("could not extract hidden data (invalid length prefix)")

// Embed hides the data byte slice inside the carrier image using LSB encoding.
// It returns a new image containing the hidden data.
func Embed(carrier image.Image, data []byte) (image.Image, error) {
	bounds := carrier.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	numPixels := width * height

	// 4 bytes for length + data
	totalBitsRequired := (4 + len(data)) * 8
	totalBitsAvailable := numPixels * 3

	if totalBitsRequired > totalBitsAvailable {
		return nil, fmt.Errorf("%w: need %d pixels, have %d", ErrMessageTooLarge, totalBitsRequired/3, numPixels)
	}

	// Create a mutable NRGBA copy
	output := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(output, output.Bounds(), carrier, bounds.Min, draw.Src)

	// Prepare payload: [Length (32-bit)] + [Data]
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	fullPayload := append(lengthBuf, data...)

	bitIndex := 0
	payloadBits := len(fullPayload) * 8

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if bitIndex >= payloadBits {
				return output, nil
			}

			// Direct access since we created the NRGBA
			c := output.NRGBAAt(x, y)

			// Helper to set LSB of a channel
			setLSB := func(val *uint8) {
				if bitIndex >= payloadBits {
					return
				}
				byteIdx := bitIndex / 8
				bitPos := 7 - (bitIndex % 8)
				bit := (fullPayload[byteIdx] >> bitPos) & 1

				*val = (*val & 0xFE) | bit
				bitIndex++
			}

			setLSB(&c.R)
			setLSB(&c.G)
			setLSB(&c.B)

			output.SetNRGBA(x, y, c)
		}
	}

	return output, nil
}

// Extract retrieves the hidden byte slice from a stego image.
func Extract(stegoImage image.Image) ([]byte, error) {
	bounds := stegoImage.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// State machine variables
	var (
		bitIndex      = 0
		lengthBits    = make([]uint8, 32)
		data          []byte
		dataLen       uint32
		readingLength = true // First phase: read 32 bits of length
	)

	// Optimization: check if we can direct-read NRGBA
	nrgbaImg, isNRGBA := stegoImage.(*image.NRGBA)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Get RGB channels
			var r, g, b uint8

			if isNRGBA {
				// Fast path (avoids color conversion loss)
				// Note: NRGBA struct fields are relative to (0,0), but At expects absolute.
				// However, nrgbaImg.NRGBAAt takes relative coordinates if we use x,y 0..w,h?
				// No, image.NRGBA.NRGBAAt(x, y) uses (x-Rect.Min.X). 
				// Since we iterate 0..width, we must add Min.
				c := nrgbaImg.NRGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
				r, g, b = c.R, c.G, c.B
			} else {
				// Slow path (fallback for other image types)
				c := color.NRGBAModel.Convert(stegoImage.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
				r, g, b = c.R, c.G, c.B
			}

			channels := []uint8{r, g, b}

			for _, val := range channels {
				bit := val & 1

				if readingLength {
					lengthBits[bitIndex] = bit
					bitIndex++

					if bitIndex == 32 {
						// Finished reading length
						for i := 0; i < 32; i++ {
							if lengthBits[i] == 1 {
								dataLen |= (1 << (31 - i))
							}
						}

						// Sanity check
						if dataLen == 0 || int(dataLen)*8 > (width*height*3)-32 {
							return nil, ErrNoHiddenData
						}

						data = make([]byte, dataLen)
						readingLength = false
						bitIndex = 0 // Reset for data reading
					}
				} else {
					// Reading Data
					if bitIndex >= int(dataLen)*8 {
						return data, nil
					}

					bytePos := bitIndex / 8
					bitPos := 7 - (bitIndex % 8)

					if bit == 1 {
						data[bytePos] |= (1 << bitPos)
					}
					bitIndex++
				}
			}
		}
	}

	// If we finish the loop but haven't returned, check if we got everything
	if !readingLength && bitIndex >= int(dataLen)*8 {
		return data, nil
	}

	return nil, ErrNoHiddenData
}