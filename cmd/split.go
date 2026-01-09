package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Beastly713/horcrux/pkg/crypto/secrets"
	"github.com/Beastly713/horcrux/pkg/format"
	"github.com/Beastly713/horcrux/pkg/pipeline"
	"github.com/Beastly713/horcrux/pkg/shamir"
	"github.com/spf13/cobra"
)

var (
	totalParts   int
	threshold    int
	destDir      string
	isHeaderless bool
)

var splitCmd = &cobra.Command{
	Use:   "split [file]",
	Short: "Split a file into encrypted horcruxes",
	Long: `Split a file into N encrypted fragments (horcruxes). 
You need T fragments to recover the file.

Example:
  horcrux split diary.txt -n 5 -t 3
  
  This creates 5 files. Any 3 are needed to recover diary.txt.`,
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

		// 3. Generate Encryption Key (Ephemeral)
		// AES-GCM uses 32-byte keys for AES-256
		keySecret, err := secrets.NewSecret(32)
		if err != nil {
			return fmt.Errorf("failed to generate secure key: %w", err)
		}
		defer keySecret.Destroy() // Ensure memory is cleared on exit

		fmt.Println("Generating key and splitting...")

		// 4. Split the Key (Shamir's Secret Sharing)
		// This returns parts with the X-coordinate embedded in the last byte.
		keyFragments, err := shamir.Split(keySecret.Bytes(), totalParts, threshold)
		if err != nil {
			return fmt.Errorf("failed to split key: %w", err)
		}

		// 5. Process the File (Read -> Compress -> Encrypt -> Shard)
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

		// 6. Write Horcruxes
		originalFilename := filepath.Base(filePath)
		timestamp := time.Now().Unix()

		for i := 0; i < totalParts; i++ {
			index := i + 1 // 1-based index for user friendliness and Shamir X-coord

			// Construct the output filename
			// e.g. diary_1_of_5.horcrux
			ext := filepath.Ext(originalFilename)
			nameNoExt := originalFilename[:len(originalFilename)-len(ext)]
			outName := fmt.Sprintf("%s_%d_of_%d.horcrux", nameNoExt, index, totalParts)
			outPath := filepath.Join(destDir, outName)

			outFile, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("failed to create output file %s: %w", outPath, err)
			}
			defer outFile.Close()

			// Construct the Header
			// Note: In headerless mode, this struct is ignored by the Writer,
			// but we populate it for code consistency and validation.
			header := &format.Header{
				OriginalFilename: originalFilename,
				Timestamp:        timestamp,
				Index:            index,
				Total:            totalParts,
				Threshold:        threshold,
				KeyFragment:      keyFragments[i],
			}

			// Initialize the Format Writer
			writer := format.NewWriter(outFile)

			// Write to disk
			// We access .Data assuming sharding.Shard is a struct wrapper
			if err := writer.Write(header, fileShards[i].Data, isHeaderless); err != nil {
				return fmt.Errorf("failed to write horcrux %d: %w", index, err)
			}

			fmt.Printf("Created %s\n", outName)
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
	splitCmd.Flags().BoolVar(&isHeaderless, "headerless", false, "Paranoiac mode: do not write metadata headers (file will look like random noise)")

	splitCmd.MarkFlagRequired("shards")
	splitCmd.MarkFlagRequired("threshold")
}