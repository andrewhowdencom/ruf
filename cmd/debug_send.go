package cmd

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message from a call to a specific destination.",
	Long:  `Send a message from a call to a specific destination.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		id, _ := cmd.Flags().GetString("id")
		dest, _ := cmd.Flags().GetString("destination")
		destType, _ := cmd.Flags().GetString("type")

		s := buildSourcer()
		urls := viper.GetStringSlice("source.urls")
		var selectedCall *model.Call

		for _, url := range urls {
			source, _, err := s.Source(url)
			if err != nil {
				return fmt.Errorf("could not source calls from %s: %w", url, err)
			}

			for i := range source.Calls {
				if source.Calls[i].ID == id {
					selectedCall = &source.Calls[i]
					break
				}
			}
			if selectedCall != nil {
				break
			}
		}

		if selectedCall == nil {
			return fmt.Errorf("call with id '%s' not found", id)
		}

		// Render content
		contentTmpl, err := template.New("content").Funcs(sprig.TxtFuncMap()).Parse(selectedCall.Content)
		if err != nil {
			return fmt.Errorf("failed to parse content template: %w", err)
		}

		var renderedContent bytes.Buffer
		if err := contentTmpl.Execute(&renderedContent, nil); err != nil {
			return fmt.Errorf("failed to execute content template: %w", err)
		}

		// Render subject if it exists
		renderedSubject := selectedCall.Subject
		if selectedCall.Subject != "" {
			subjectTmpl, err := template.New("subject").Funcs(sprig.TxtFuncMap()).Parse(selectedCall.Subject)
			if err != nil {
				return fmt.Errorf("failed to parse subject template: %w", err)
			}
			var subjectBuffer bytes.Buffer
			if err := subjectTmpl.Execute(&subjectBuffer, nil); err != nil {
				return fmt.Errorf("failed to execute subject template: %w", err)
			}
			renderedSubject = subjectBuffer.String()
		}

		switch destType {
		case "slack":
			slackToken := viper.GetString("slack.app.token")
			slackClient := slack.NewClient(slackToken)
			_, _, err := slackClient.PostMessage(dest, selectedCall.Author, renderedSubject, renderedContent.String())
			if err != nil {
				return fmt.Errorf("failed to send slack message: %w", err)
			}
		case "email":
			emailClient := email.NewClient(
				viper.GetString("email.host"),
				viper.GetInt("email.port"),
				viper.GetString("email.username"),
				viper.GetString("email.password"),
				viper.GetString("email.from"),
			)
			err := emailClient.Send([]string{dest}, selectedCall.Author, renderedSubject, renderedContent.String())
			if err != nil {
				return fmt.Errorf("failed to send email: %w", err)
			}
		default:
			return fmt.Errorf("unknown destination type: %s", destType)
		}

		fmt.Printf("Message sent successfully to %s\n", dest)
		return nil
	},
}

func init() {
	debugCmd.AddCommand(sendCmd)
	sendCmd.Flags().String("id", "", "ID of the call to send")
	sendCmd.Flags().String("destination", "", "Destination to send the message to")
	sendCmd.Flags().String("type", "", "Type of the destination (e.g., slack, email)")

	sendCmd.MarkFlagRequired("id")
	sendCmd.MarkFlagRequired("destination")
	sendCmd.MarkFlagRequired("type")
}
