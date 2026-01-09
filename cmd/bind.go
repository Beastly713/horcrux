package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Beastly713/horcrux/pkg/format"
	"github.com/Beastly713/horcrux/pkg/pipeline"
	"github.com/Beastly713/horcrux/pkg/shamir"
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
	Long: `Bind looks for .horcrux files in the specified directory (or current directory if not provided),
validates them, and attempts to reconstruct the original file.

You need at least T (threshold) valid horcruxes to succeed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Determine Source Directory
		sourceDir := "."
		if len(args) > 0 {
			sourceDir = args[0]
		}

		// 2. Gather .horcrux files
		files, err := os.ReadDir(sourceDir)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}

		// internal struct to hold open file references
		type loadedHorcrux struct {
			Path   string
			Header *format.Header
			Body   io.Reader
			File   *os.File
		}

		// Group files by an ID (Filename + Timestamp) to handle multiple split files in one dir
		groups := make(map[string][]*loadedHorcrux)

		fmt.Printf("Scanning for horcruxes in %s...\n", sourceDir)

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".horcrux" {
				continue
			}

			path := filepath.Join(sourceDir, f.Name())
			file, err := os.Open(path)
			if err != nil {
				fmt.Printf("Skipping unreadable file %s: %v\n", f.Name(), err)
				continue
			}

			// We don't defer close here immediately because we need the Reader open for the pipeline.
			// We will close them after processing the group.

			// 3. Parse Header
			reader, err := format.NewReader(file)
			if err != nil {
				// If header parsing fails, it might be a corrupted file or a "headerless" one.
				// For now, we skip it as we cannot bind without metadata.
				fmt.Printf("Skipping invalid/headerless file %s: %v\n", f.Name(), err)
				file.Close()
				continue
			}

			// Create a unique group ID
			groupID := fmt.Sprintf("%s|%d", reader.Header.OriginalFilename, reader.Header.Timestamp)
			
			lh := &loadedHorcrux{
				Path:   path,
				Header: reader.Header,
				Body:   reader.Body, // This is the bufio reader positioned at the start of ciphertext
				File:   file,
			}
			groups[groupID] = append(groups[groupID], lh)
		}

		if len(groups) == 0 {
			return fmt.Errorf("no valid horcruxes found in %s", sourceDir)
		}

		// 4. Process Each Group
		for _, group := range groups {
			// All headers in a group should be identical regarding Total/Threshold
			refHeader := group[0].Header
			fmt.Printf("\nFound shards for: %s (Threshold: %d/%d)\n", refHeader.OriginalFilename, len(group), refHeader.Threshold)

			// Clean up file handles for this group when done
			defer func(gh []*loadedHorcrux) {
				for _, h := range gh {
					h.File.Close()
				}
			}(group)

			// Check Threshold
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
				// Read the binary body from the pre-positioned reader
				data, err := io.ReadAll(h.Body)
				if err != nil {
					fmt.Printf("Failed to read body of %s: %v\n", h.Path, err)
					// We might still succeed if we have enough other shards, but simpler to fail fast here
					return err
				}
				shardMap[h.Header.Index] = data
			}

			// Pipeline: Unshard -> Decrypt -> Decompress
			plainText, err := pipeline.JoinPipeline(shardMap, key, refHeader.Total, refHeader.Threshold)
			if err != nil {
				fmt.Printf("Reconstruction pipeline failed: %v\n(Did you try to bind corrupted or wrong files?)\n", err)
				continue
			}

			// 7. Write Output
			finalPath := filepath.Join(outDir, refHeader.OriginalFilename)
			if outDir == "" {
				// Default to current directory if no output flag set
				finalPath = refHeader.OriginalFilename
			}

			// Check overwrite
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