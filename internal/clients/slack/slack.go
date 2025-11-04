package slack

import (
	"fmt"
	"strings"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/slack-go/slack"
)

// Client is an interface that defines the methods for interacting with the Slack API.
type Client interface {
	PostMessage(destination, author, subject, text string, campaign model.Campaign) (string, string, error)
	NotifyAuthor(authorEmail, channelId, messageTimestamp, channelName string) error
	DeleteMessage(channel, timestamp string) error
	GetChannelID(destination string) (string, error)
}

// client is the concrete implementation of the Client interface.
type client struct {
	api *slack.Client
}

// NewClient creates a new Slack client.
func NewClient(token string) Client {
	return &client{
		api: slack.New(token),
	}
}

// PostMessage sends a message to a Slack destination.
func (c *client) PostMessage(destination, author, subject, text string, campaign model.Campaign) (string, string, error) {
	message := text
	if subject != "" {
		message = fmt.Sprintf("*%s*\n%s", subject, text)
	}

	// Default message options.
	options := []slack.MsgOption{
		slack.MsgOptionText(message, false),
	}

	// If an author is specified, try to use their profile for the message.
	if author != "" {
		user, err := c.api.GetUserByEmail(author)
		if err == nil && user != nil {
			// User found, customize username and icon.
			username := user.RealName
			if username == "" {
				username = user.Name
			}
			options = append(options, slack.MsgOptionUsername(username))

			// Use the highest resolution image available.
			if user.Profile.ImageOriginal != "" {
				options = append(options, slack.MsgOptionIconURL(user.Profile.ImageOriginal))
			} else if user.Profile.Image512 != "" {
				options = append(options, slack.MsgOptionIconURL(user.Profile.Image512))
			}
		} else {
			// User not found, fall back to adding attribution in the message body.
			message = fmt.Sprintf("%s\n\n---\nThx: %s", message, author)
			// Overwrite the text option with the updated message.
			options[0] = slack.MsgOptionText(message, false)
		}
	} else if campaign.Name != "" {
		// If no author is specified, use the campaign name and icon.
		options = append(options, slack.MsgOptionUsername(campaign.Name))
		if campaign.IconURL != "" {
			options = append(options, slack.MsgOptionIconURL(campaign.IconURL))
		}
	}

	channelID, err := c.GetChannelID(destination)
	if err != nil {
		return "", "", fmt.Errorf("failed to get channel id for '%s': %w", destination, err)
	}

	// Post the message with the specified options.
	_, timestamp, err := c.api.PostMessage(channelID, options...)
	if err != nil {
		return "", "", fmt.Errorf("failed to post message: %w", err)
	}
	return channelID, timestamp, nil
}

// NotifyAuthor sends a direct message to the author of a message with a permalink to the original message.
func (c *client) NotifyAuthor(authorEmail, channelId, messageTimestamp, channelName string) error {
	user, err := c.api.GetUserByEmail(authorEmail)
	if err != nil {
		return fmt.Errorf("failed to get user by email: %w", err)
	}

	// Open a direct message channel with the user.
	im, _, _, err := c.api.OpenConversation(&slack.OpenConversationParameters{
		Users: []string{user.ID},
	})
	if err != nil {
		return fmt.Errorf("failed to open conversation: %w", err)
	}

	// Get the permalink for the original message.
	permalink, err := c.api.GetPermalink(&slack.PermalinkParameters{
		Channel: channelId,
		Ts:      messageTimestamp,
	})
	if err != nil {
		return fmt.Errorf("failed to get permalink: %w", err)
	}

	// Send the direct message.
	_, _, err = c.api.PostMessage(im.ID, slack.MsgOptionText(fmt.Sprintf("I have just sent your message to %s. You can view it here: %s", channelName, permalink), false))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

// DeleteMessage deletes a message from a Slack channel.
func (c *client) DeleteMessage(channel, timestamp string) error {
	channelID, err := c.GetChannelID(channel)
	if err != nil {
		return fmt.Errorf("failed to get channel id: %w", err)
	}
	_, _, err = c.api.DeleteMessage(channelID, timestamp)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// GetChannelID retrieves the conversation ID for a given destination.
// The destination can be a public channel ("#general"), a user email ("user@example.com"),
// or a user handle ("@username"). If the destination does not match these formats,
// it is assumed to be a raw channel/conversation ID.
func (c *client) GetChannelID(destination string) (string, error) {
	// Handle public/private channel names
	if strings.HasPrefix(destination, "#") {
		var channels []slack.Channel
		params := &slack.GetConversationsParameters{
			Limit: 1000,
			Types: []string{"public_channel", "private_channel"},
		}
		for {
			page, nextCursor, err := c.api.GetConversations(params)
			if err != nil {
				return "", fmt.Errorf("failed to get conversations: %w", err)
			}
			channels = append(channels, page...)
			if nextCursor == "" {
				break
			}
			params.Cursor = nextCursor
		}

		// Normalize channel name for case-insensitive comparison.
		normalizedChannelName := strings.TrimPrefix(strings.ToLower(destination), "#")

		for _, channel := range channels {
			if strings.ToLower(channel.Name) == normalizedChannelName {
				return channel.ID, nil
			}
		}

		return "", fmt.Errorf("channel '%s' not found", destination)
	}

	var user *slack.User
	var err error

	// Handle emails for DMs
	if strings.Contains(destination, "@") && !strings.HasPrefix(destination, "@") {
		user, err = c.api.GetUserByEmail(destination)
		if err != nil {
			return "", fmt.Errorf("failed to get user by email '%s': %w", destination, err)
		}
	} else if strings.HasPrefix(destination, "@") {
		// Handle usernames for DMs (this is inefficient, but the only way)
		users, err := c.api.GetUsers()
		if err != nil {
			return "", fmt.Errorf("failed to list users: %w", err)
		}

		userName := strings.TrimPrefix(destination, "@")
		found := false
		for i := range users {
			if users[i].Name == userName || users[i].Profile.DisplayName == userName {
				user = &users[i]
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("user '%s' not found", destination)
		}
	}

	// If we found a user by email or username, open a DM channel with them.
	if user != nil {
		im, _, _, err := c.api.OpenConversation(&slack.OpenConversationParameters{
			Users: []string{user.ID},
		})
		if err != nil {
			return "", fmt.Errorf("failed to open conversation with user '%s': %w", destination, err)
		}
		return im.ID, nil
	}

	// Otherwise, assume it's a raw ID and return it.
	return destination, nil
}

