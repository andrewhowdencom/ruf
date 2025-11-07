package worker

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/processor"
)

// ProcessCall handles the processing of a single call, including rendering, sending, and recording the status.
func ProcessCall(call *model.Call, store kv.Storer, slackClient slack.Client, emailClient email.Client, dryRun bool) error {
	slog.Debug("processing call", "call_id", call.ID)
	effectiveScheduledAt := call.ScheduledAt

	dest := call.Destinations[0]
	if len(dest.To) == 0 {
		slog.Warn("skipping call with no address in `to`", "call_id", call.ID)
		return nil
	}

	for _, to := range dest.To {
		hasBeenSent, err := store.HasBeenSent(call.Campaign.ID, call.ID, dest.Type, to)
		if err != nil {
			return fmt.Errorf("failed to check if call has been sent: %w", err)
		}
		if hasBeenSent {
			slog.Debug("skipping call that has already been sent", "call_id", call.ID, "destination", to, "type", dest.Type)
			continue
		}

		// Define the processor stacks for each destination type
		var subjectProcessor, contentProcessor processor.ProcessorStack
		switch dest.Type {
		case "slack":
			subjectProcessor = processor.ProcessorStack{
				processor.NewTemplateProcessor(),
			}
			contentProcessor = processor.ProcessorStack{
				processor.NewTemplateProcessor(),
				processor.NewMarkdownToSlackProcessor(),
			}
		case "email":
			subjectProcessor = processor.ProcessorStack{
				processor.NewTemplateProcessor(),
			}
			contentProcessor = processor.ProcessorStack{
				processor.NewTemplateProcessor(),
				processor.NewMarkdownToHTMLProcessor(),
			}
		default:
			return fmt.Errorf("unsupported destination type: %s", dest.Type)
		}

		data := make(map[string]interface{})
		if call.Data != nil {
			for k, v := range call.Data {
				data[k] = v
			}
		}
		data["ScheduledAt"] = effectiveScheduledAt

		subject, err := subjectProcessor.Process(call.Subject, data)
		if err != nil {
			slog.Error("failed to process subject", "error", err)
			store.AddSentMessage(call.Campaign.ID, call.ID, &kv.SentMessage{
				SourceID:     call.ID,
				ScheduledAt:  effectiveScheduledAt,
				Status:       kv.StatusFailed,
				Type:         dest.Type,
				Destination:  to,
				CampaignName: call.Campaign.Name,
			})
			continue
		}
		content, err := contentProcessor.Process(call.Content, data)
		if err != nil {
			slog.Error("failed to process content", "error", err)
			store.AddSentMessage(call.Campaign.ID, call.ID, &kv.SentMessage{
				SourceID:     call.ID,
				ScheduledAt:  effectiveScheduledAt,
				Status:       kv.StatusFailed,
				Type:         dest.Type,
				Destination:  to,
				CampaignName: call.Campaign.Name,
			})
			continue
		}

		if dryRun {
			slog.Info("dry run: would send message", "call_id", call.ID, "campaign", call.Campaign.Name, "subject", subject, "destination", to, "type", dest.Type, "scheduled_at", effectiveScheduledAt)
			continue
		}

		switch dest.Type {
		case "slack":
			slog.Info("sending slack message", "call_id", call.ID, "destination", to, "scheduled_at", effectiveScheduledAt)
			channelID, timestamp, err := slackClient.PostMessage(to, call.Author, subject, content, call.Campaign)
			sentMessage := &kv.SentMessage{
				SourceID:     call.ID,
				ScheduledAt:  effectiveScheduledAt,
				Timestamp:    timestamp,
				Destination:  to,
				Type:         dest.Type,
				CampaignName: call.Campaign.Name,
			}

			if err != nil {
				sentMessage.Status = kv.StatusFailed
				slog.Error("failed to send slack message", "error", err)
			} else {
				sentMessage.Status = kv.StatusSent
				slog.Info("sent slack message", "call_id", call.ID, "destination", to, "scheduled_at", effectiveScheduledAt)

				if call.Author != "" {
					err := slackClient.NotifyAuthor(call.Author, channelID, timestamp, to)
					if err != nil {
						slog.Error("failed to send author notification", "error", err)
					}
				}
			}

			if err := store.AddSentMessage(call.Campaign.ID, call.ID, sentMessage); err != nil {
				return err
			}
		case "email":
			slog.Info("sending email", "call_id", call.ID, "recipient", to, "scheduled_at", effectiveScheduledAt)
			err := emailClient.Send([]string{to}, call.Author, subject, content, call.Campaign)
			sentMessage := &kv.SentMessage{
				SourceID:     call.ID,
				ScheduledAt:  effectiveScheduledAt,
				Destination:  to,
				Type:         dest.Type,
				CampaignName: call.Campaign.Name,
			}

			if err != nil {
				sentMessage.Status = kv.StatusFailed
				slog.Error("failed to send email", "error", err)
			} else {
				sentMessage.Status = kv.StatusSent
				slog.Info("sent email", "call_id", call.ID, "recipient", to, "scheduled_at", effectiveScheduledAt)
			}

			if err := store.AddSentMessage(call.Campaign.ID, call.ID, sentMessage); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported destination type: %s", dest.Type)
		}
	}

	return nil
}
