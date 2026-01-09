package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "horcrux",
	Short: "Split your file into encrypted fragments",
	Long: `Horcrux: A secure tool to split your files into encrypted fragments 
(horcruxes) requiring a specific threshold to reconstruct.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}