package cmd

import (
	"github.com/spf13/cobra"
)

// dispatcherCmd represents the dispatcher command
var dispatcherCmd = &cobra.Command{
	Use:   "dispatcher",
	Short: "Dispatcher commands for running and watching.",
	Long:  `A parent command to group the 'run' and 'watchdog' dispatcher commands.`,
}

func init() {
	rootCmd.AddCommand(dispatcherCmd)
}
