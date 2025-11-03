package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debugCallsCmd = &cobra.Command{
	Use:   "calls",
	Short: "List all scheduled calls from all sources.",
	Long:  `List all scheduled calls from all sources.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := buildSourcer()
		if err != nil {
			return fmt.Errorf("failed to build sourcer: %w", err)
		}

		urls := viper.GetStringSlice("source.urls")
		var allCalls []*model.Call

		for _, url := range urls {
			source, _, err := s.Source(url)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error sourcing from %s: %v\n", url, err)
				continue
			}
			if source == nil {
				continue
			}
			for i := range source.Calls {
				allCalls = append(allCalls, &source.Calls[i])
			}
		}

		output, err := json.MarshalIndent(allCalls, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal calls to JSON: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), string(output))

		return nil
	},
}

func init() {
	debugCmd.AddCommand(debugCallsCmd)
}
