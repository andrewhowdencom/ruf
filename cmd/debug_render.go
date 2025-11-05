package cmd

import (
	"fmt"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/processor"
	"github.com/gorhill/cronexpr"
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

		p := processor.NewTemplateProcessor()

		subject, err := p.Process(callToRender.Subject, nil)
		if err != nil {
			return fmt.Errorf("failed to render subject: %w", err)
		}

		content, err := p.Process(callToRender.Content, nil)
		if err != nil {
			return fmt.Errorf("failed to render content: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Subject:", subject)
		fmt.Fprintln(cmd.OutOrStdout(), "Content:", content)

		// Calculate and display the next send time.
		var next time.Time
		for _, trigger := range callToRender.Triggers {
			if trigger.Cron != "" {
				expr, err := cronexpr.Parse(trigger.Cron)
				if err != nil {
					// Should have been caught by validation, but handle anyway.
					return fmt.Errorf("invalid cron expression: %w", err)
				}
				nextRun := expr.Next(time.Now())
				if next.IsZero() || nextRun.Before(next) {
					next = nextRun
				}
			}
			if !trigger.ScheduledAt.IsZero() {
				if next.IsZero() || trigger.ScheduledAt.Before(next) {
					next = trigger.ScheduledAt
				}
			}
		}

		if !next.IsZero() {
			fmt.Fprintln(cmd.OutOrStdout(), "Next Send:", next.Format(time.RFC1123))
		}

		return nil
	},
}

func init() {
	debugCmd.AddCommand(debugRenderCmd)
}
