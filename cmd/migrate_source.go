package cmd

import "github.com/spf13/cobra"

var migrateSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Migrate source files to a newer format.",
	Long:  `Migrate source files to a newer format.`,
}

func init() {
	migrateCmd.AddCommand(migrateSourceCmd)
}
