package cmd

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // Register JPEG decoder
	"image/png"    // Register PNG decoder and encoder
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beastly713/horcrux/pkg/crypto/secrets"
	"github.com/Beastly713/horcrux/pkg/format"
	"github.com/Beastly713/horcrux/pkg/pipeline"
	"github.com/Beastly713/horcrux/pkg/shamir"
	"github.com/Beastly713/horcrux/pkg/stego"
	"github.com/spf13/cobra"
)

var (
	totalParts   int
	threshold    int
	destDir      string
	carrierImage string
	isHeaderless bool
)

var splitCmd = &cobra.Command{
	Use:   "split [file]",
	Short: "Split a file into encrypted horcruxes",
	Long: `Split a file into N encrypted fragments (horcruxes). 
You need T fragments to recover the file.

If --carrier-image is provided, shards will be hidden inside copies of that image 
using steganography and saved as PNG files.

Example:
  horcrux split diary.txt -n 5 -t 3
  horcrux split secrets.pdf -n 3 -t 2 --carrier-image vacation.jpg`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// 1. Validation
		if totalParts < 2 {
			return fmt.Errorf("number of parts (-n) must be at least 2")
		}
		if threshold < 2 {
			return fmt.Errorf("threshold (-t) must be at least 2")
		}
		if threshold > totalParts {
			return fmt.Errorf("threshold cannot be greater than total parts")
		}

		// 2. Prepare Output Directory
		if destDir == "" {
			destDir = filepath.Dir(filePath)
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		// 3. Prepare Carrier Image (if requested)
		var carrier image.Image
		if carrierImage != "" {
			imgFile, err := os.Open(carrierImage)
			if err != nil {
				return fmt.Errorf("failed to open carrier image: %w", err)
			}
			defer imgFile.Close()

			carrier, _, err = image.Decode(imgFile)
			if err != nil {
				return fmt.Errorf("failed to decode carrier image: %w", err)
			}
		}

		// 4. Generate Encryption Key (Ephemeral)
		// AES-GCM uses 32-byte keys for AES-256
		keySecret, err := secrets.NewSecret(32)
		if err != nil {
			return fmt.Errorf("failed to generate secure key: %w", err)
		}
		defer keySecret.Destroy() // Ensure memory is cleared on exit

		fmt.Println("Generating key and splitting...")

		// 5. Split the Key (Shamir's Secret Sharing)
		// This returns parts with the X-coordinate embedded in the last byte.
		keyFragments, err := shamir.Split(keySecret.Bytes(), totalParts, threshold)
		if err != nil {
			return fmt.Errorf("failed to split key: %w", err)
		}

		// 6. Process the File (Read -> Compress -> Encrypt -> Shard)
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		config := pipeline.PipelineConfig{
			Total:     totalParts,
			Threshold: threshold,
		}

		// sharding.Shard is assumed to contain the Data
		fileShards, err := pipeline.SplitPipeline(file, keySecret.Bytes(), config)
		if err != nil {
			return fmt.Errorf("pipeline failed: %w", err)
		}

		if len(fileShards) != len(keyFragments) {
			return fmt.Errorf("mismatch between data shards (%d) and key fragments (%d)", len(fileShards), len(keyFragments))
        }

		// 7. Write Horcruxes
		originalFilename := filepath.Base(filePath)
		timestamp := time.Now().Unix()
		
		// Helper to strip extension for naming
		ext := filepath.Ext(originalFilename)
		nameNoExt := strings.TrimSuffix(originalFilename, ext)

		for i := 0; i < totalParts; i++ {
			index := i + 1 // 1-based index for user friendliness and Shamir X-coord

			// Construct the Header
			header := &format.Header{
				OriginalFilename: originalFilename,
				Timestamp:        timestamp,
				Index:            index,
				Total:            totalParts,
				Threshold:        threshold,
				KeyFragment:      keyFragments[i],
			}

			// Serialize content to memory buffer first
			var contentBuf bytes.Buffer
			writer := format.NewWriter(&contentBuf)

			// Write Header + Body to the buffer
			if err := writer.Write(header, fileShards[i].Data, isHeaderless); err != nil {
				return fmt.Errorf("failed to serialize horcrux %d: %w", index, err)
			}
			contentBytes := contentBuf.Bytes()

			// Determine Output Strategy (Stego vs Standard)
			if carrierImage != "" {
				// --- STEGANOGRAPHY MODE ---
				fmt.Printf("[%d/%d] Embedding into image...\n", index, totalParts)

				stegoImg, err := stego.Embed(carrier, contentBytes)
				if err != nil {
					return fmt.Errorf("failed to embed shard %d: %w", index, err)
				}

				outName := fmt.Sprintf("%s_%d_of_%d.png", nameNoExt, index, totalParts)
				outPath := filepath.Join(destDir, outName)

				outFile, err := os.Create(outPath)
				if err != nil {
					return fmt.Errorf("failed to create output file %s: %w", outPath, err)
				}
				
				// Must encode as PNG to be lossless
				if err := png.Encode(outFile, stegoImg); err != nil {
					outFile.Close()
					return fmt.Errorf("failed to encode png %s: %w", outPath, err)
				}
				outFile.Close()
				fmt.Printf("Created %s\n", outName)

			} else {
				// --- STANDARD MODE ---
				fileExt := ".horcrux"
				if isHeaderless {
					fileExt = ".bin"
				}

				outName := fmt.Sprintf("%s_%d_of_%d%s", nameNoExt, index, totalParts, fileExt)
				outPath := filepath.Join(destDir, outName)

				if err := os.WriteFile(outPath, contentBytes, 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", outPath, err)
				}
				fmt.Printf("Created %s\n", outName)
			}
		}

		fmt.Println("Done! Keep your horcruxes safe.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(splitCmd)

	splitCmd.Flags().IntVarP(&totalParts, "shards", "n", 0, "Total number of horcruxes to make")
	splitCmd.Flags().IntVarP(&threshold, "threshold", "t", 0, "Number of horcruxes required to resurrect")
	splitCmd.Flags().StringVarP(&destDir, "destination", "d", "", "Directory to output horcruxes (default: current directory)")
	splitCmd.Flags().StringVarP(&carrierImage, "carrier-image", "i", "", "Path to an image (jpg/png) to hide the horcruxes inside")
	splitCmd.Flags().BoolVar(&isHeaderless, "headerless", false, "Paranoiac mode: do not write metadata headers")

	splitCmd.MarkFlagRequired("shards")
	splitCmd.MarkFlagRequired("threshold")
}