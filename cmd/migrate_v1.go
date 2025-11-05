package cmd

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// LegacyCall represents the old structure of a call for migration purposes.
type LegacyCall struct {
	ID           string               `json:"id" yaml:"id"`
	Author       string               `json:"author,omitempty" yaml:"author,omitempty"`
	Subject      string               `json:"subject,omitempty" yaml:"subject,omitempty"`
	Content      string               `json:"content" yaml:"content"`
	Destinations []model.Destination  `json:"destinations" yaml:"destinations"`
	ScheduledAt  time.Time            `json:"scheduled_at,omitempty" yaml:"scheduled_at,omitempty"`
	Cron         string               `json:"cron,omitempty" yaml:"cron,omitempty"`
	Recurring    bool                 `json:"recurring,omitempty" yaml:"recurring,omitempty"`
	Delta        string               `json:"delta,omitempty" yaml:"delta,omitempty"`
	Sequence     string               `json:"sequence,omitempty" yaml:"sequence,omitempty"`
	Campaign     model.Campaign       `json:"campaign" yaml:"campaign"`
}

// LegacySource represents the old structure of a source file for migration.
type LegacySource struct {
	Campaign model.Campaign `json:"campaign" yaml:"campaign"`
	Calls    []LegacyCall   `json:"calls" yaml:"calls"`
	Events   []model.Event  `json:"events" yaml:"events"`
}

var migrateV1Cmd = &cobra.Command{
	Use:   "v1 [file]",
	Short: "Migrate a YAML file from the v0 format to the v1 format.",
	Long:  `Migrate a YAML file from the v0 format to the v1 format.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var legacySource LegacySource
		if err := yaml.Unmarshal(data, &legacySource); err != nil {
			return fmt.Errorf("failed to unmarshal legacy YAML: %w", err)
		}

		newCalls := make([]model.Call, len(legacySource.Calls))
		for i, legacyCall := range legacySource.Calls {
			newCall := model.Call{
				ID:           legacyCall.ID,
				Author:       legacyCall.Author,
				Subject:      legacyCall.Subject,
				Content:      legacyCall.Content,
				Destinations: legacyCall.Destinations,
			}

			var triggers []model.Trigger
			if !legacyCall.ScheduledAt.IsZero() {
				triggers = append(triggers, model.Trigger{ScheduledAt: legacyCall.ScheduledAt})
			}
			if legacyCall.Cron != "" {
				triggers = append(triggers, model.Trigger{Cron: legacyCall.Cron})
			}
			if legacyCall.Sequence != "" || legacyCall.Delta != "" {
				triggers = append(triggers, model.Trigger{Sequence: legacyCall.Sequence, Delta: legacyCall.Delta})
			}
			newCall.Triggers = triggers
			newCalls[i] = newCall
		}

		newSource := struct {
			Campaign model.Campaign `json:"campaign" yaml:"campaign"`
			Calls    []model.Call   `json:"calls" yaml:"calls"`
			Events   []model.Event  `json:"events" yaml:"events"`
		}{
			Campaign: legacySource.Campaign,
			Calls:    newCalls,
			Events:   legacySource.Events,
		}

		newData, err := yaml.Marshal(newSource)
		if err != nil {
			return fmt.Errorf("failed to marshal new YAML: %w", err)
		}

		fmt.Fprint(cmd.OutOrStdout(), string(newData))
		return nil
	},
}

func init() {
	migrateCmd.AddCommand(migrateV1Cmd)
}
