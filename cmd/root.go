package cmd

import (
	"github.com/spf13/cobra"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "cs-builder",
	Short: "C# monorepo build helper",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to cs-builder.yaml (overrides CS_BUILDER_CONFIG)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
