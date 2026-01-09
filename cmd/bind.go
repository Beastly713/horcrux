package cmd

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png" // Register PNG decoder
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beastly713/horcrux/pkg/format"
	"github.com/Beastly713/horcrux/pkg/pipeline"
	"github.com/Beastly713/horcrux/pkg/shamir"
	"github.com/Beastly713/horcrux/pkg/stego"
	"github.com/spf13/cobra"
)

var (
	outDir    string
	overwrite bool
)

// bindCmd represents the bind command
var bindCmd = &cobra.Command{
	Use:   "bind [directory]",
	Short: "Reconstruct the original file from a set of horcruxes",
	Long: `Bind looks for .horcrux and .png files in the specified directory 
(or current directory if not provided), validates them, and attempts to 
reconstruct the original file.

You need at least T (threshold) valid horcruxes to succeed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Determine Source Directory
		sourceDir := "."
		if len(args) > 0 {
			sourceDir = args[0]
		}

		// 2. Gather files
		files, err := os.ReadDir(sourceDir)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}

		// internal struct to hold open file references
		type loadedHorcrux struct {
			Path   string
			Header *format.Header
			Body   io.Reader
			File   *os.File // Kept open for standard files, nil for images
		}

		// Group files by an ID (Filename + Timestamp)
		groups := make(map[string][]*loadedHorcrux)

		fmt.Printf("Scanning for horcruxes in %s...\n", sourceDir)

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			ext := strings.ToLower(filepath.Ext(f.Name()))
			if ext != ".horcrux" && ext != ".png" {
				continue
			}

			path := filepath.Join(sourceDir, f.Name())
			file, err := os.Open(path)
			if err != nil {
				fmt.Printf("Skipping unreadable file %s: %v\n", f.Name(), err)
				continue
			}

			var inputReader io.Reader
			var fileToKeepOpen *os.File

			if ext == ".png" {
				// --- STEGANOGRAPHY HANDLING ---
				img, _, err := image.Decode(file)
				file.Close() // Close image file immediately after decoding
				if err != nil {
					fmt.Printf("Skipping invalid image %s: %v\n", f.Name(), err)
					continue
				}

				hiddenData, err := stego.Extract(img)
				if err != nil {
					if err != stego.ErrNoHiddenData {
						fmt.Printf("Failed to extract data from %s: %v\n", f.Name(), err)
					}
					continue
				}

				inputReader = bytes.NewReader(hiddenData)
				fileToKeepOpen = nil

			} else {
				// --- STANDARD HANDLING ---
				inputReader = file
				fileToKeepOpen = file
			}

			// 3. Parse Header
			reader, err := format.NewReader(inputReader)
			if err != nil {
				fmt.Printf("Skipping invalid/headerless file %s: %v\n", f.Name(), err)
				if fileToKeepOpen != nil {
					fileToKeepOpen.Close()
				}
				continue
			}

			groupID := fmt.Sprintf("%s|%d", reader.Header.OriginalFilename, reader.Header.Timestamp)

			lh := &loadedHorcrux{
				Path:   path,
				Header: reader.Header,
				Body:   reader.Body,
				File:   fileToKeepOpen,
			}
			groups[groupID] = append(groups[groupID], lh)
		}

		if len(groups) == 0 {
			return fmt.Errorf("no valid horcruxes found in %s", sourceDir)
		}

		// 4. Process Each Group
		for _, group := range groups {
			refHeader := group[0].Header
			fmt.Printf("\nFound shards for: %s (Threshold: %d/%d)\n", refHeader.OriginalFilename, len(group), refHeader.Threshold)

			defer func(gh []*loadedHorcrux) {
				for _, h := range gh {
					if h.File != nil {
						h.File.Close()
					}
				}
			}(group)

			if len(group) < refHeader.Threshold {
				fmt.Printf("Not enough horcruxes to restore %s. Need %d, found %d.\n", refHeader.OriginalFilename, refHeader.Threshold, len(group))
				continue
			}

			// 5. Reconstruct Key
			fmt.Println("Reconstructing encryption key...")
			keyFragments := make([][]byte, 0, len(group))
			for _, h := range group {
				keyFragments = append(keyFragments, h.Header.KeyFragment)
			}

			key, err := shamir.Combine(keyFragments)
			if err != nil {
				fmt.Printf("Failed to reconstruct key for %s: %v\n", refHeader.OriginalFilename, err)
				continue
			}

			// 6. Reconstruct Body
			fmt.Println("Joining shards and decrypting...")
			shardMap := make(map[int][]byte)
			for _, h := range group {
				data, err := io.ReadAll(h.Body)
				if err != nil {
					fmt.Printf("Failed to read body of %s: %v\n", h.Path, err)
					return err
				}
				// CRITICAL FIX: Convert 1-based Horcrux Index to 0-based RS Index
				// Shamir uses 1..N, ReedSolomon uses 0..N-1
				shardMap[h.Header.Index-1] = data
			}

			plainText, err := pipeline.JoinPipeline(shardMap, key, refHeader.Total, refHeader.Threshold)
			if err != nil {
				fmt.Printf("Reconstruction pipeline failed: %v\n(Did you try to bind corrupted or wrong files?)\n", err)
				continue
			}

			// 7. Write Output
			finalPath := filepath.Join(outDir, refHeader.OriginalFilename)
			if outDir == "" {
				finalPath = refHeader.OriginalFilename
			}

			if _, err := os.Stat(finalPath); err == nil && !overwrite {
				fmt.Printf("File %s already exists. Use --overwrite to replace it.\n", finalPath)
				continue
			}

			if err := os.WriteFile(finalPath, plainText, 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}

			fmt.Printf("Successfully resurrected: %s\n", finalPath)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(bindCmd)

	bindCmd.Flags().StringVarP(&outDir, "destination", "d", "", "Directory to write the resurrected file")
	bindCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing file if present")
}