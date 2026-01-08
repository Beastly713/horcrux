package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var splitCmd = &cobra.Command{
	Use:   "split [filename]",
	Short: "Split a file into horcruxes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		total, _ := cmd.Flags().GetInt("count")
		threshold, _ := cmd.Flags().GetInt("threshold")

		fmt.Printf("[Phase 1 Stub] Splitting %s into %d files (threshold: %d)\n", path, total, threshold)
	},
}

func init() {
	rootCmd.AddCommand(splitCmd)
	splitCmd.Flags().IntP("count", "n", 5, "Total number of horcruxes to make")
	splitCmd.Flags().IntP("threshold", "t", 3, "Number of horcruxes required to restore")
}