package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dispatcherCmd represents the dispatcher command
var dispatcherCmd = &cobra.Command{
	Use:   "dispatcher",
	Short: "Dispatcher commands for running and watching.",
	Long:  `A parent command to group the 'run' and 'watch' dispatcher commands.`,
}

func init() {
	rootCmd.AddCommand(dispatcherCmd)
	dispatcherCmd.PersistentFlags().Bool("dry-run", false, "Enable dry run mode")
	viper.BindPFlag("dispatcher.dry_run", dispatcherCmd.PersistentFlags().Lookup("dry-run"))
}
