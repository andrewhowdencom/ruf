package cmd

import (
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/templater"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debugRenderCmd = &cobra.Command{
	Use:   "render [CALL_ID]",
	Short: "Render a specific call.",
	Long:  `Render a specific call.`,
	Args:  cobra.ExactArgs(1),
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

		callID := args[0]
		var callToRender *model.Call
		for _, call := range allCalls {
			if call.ID == callID {
				callToRender = call
				break
			}
		}

		if callToRender == nil {
			return fmt.Errorf("call with ID '%s' not found", callID)
		}

		subject, err := templater.Render(callToRender.Subject)
		if err != nil {
			return fmt.Errorf("failed to render subject: %w", err)
		}

		content, err := templater.Render(callToRender.Content)
		if err != nil {
			return fmt.Errorf("failed to render content: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Subject:", subject)
		fmt.Fprintln(cmd.OutOrStdout(), "Content:", content)

		return nil
	},
}

func init() {
	debugCmd.AddCommand(debugRenderCmd)
}
