package cmd

import (
	"github.com/spf13/cobra"
)

// scheduledCmd represents the scheduled command
var scheduledCmd = &cobra.Command{
	Use:   "scheduled",
	Short: "Manage scheduled calls",
	Long:  `Manage scheduled calls.`,
}

func init() {
	rootCmd.AddCommand(scheduledCmd)
	scheduledCmd.AddCommand(scheduledSkipCmd)
}
