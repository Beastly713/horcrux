package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var bindCmd = &cobra.Command{
	Use:   "bind [directory]",
	Short: "Reconstruct the original file from horcruxes",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		destination, _ := cmd.Flags().GetString("destination")

		fmt.Printf("[Phase 1 Stub] Binding horcruxes in %s (output: %s)\n", dir, destination)
	},
}

func init() {
	rootCmd.AddCommand(bindCmd)
	bindCmd.Flags().StringP("destination", "d", "", "Directory to output the reconstructed file")
}