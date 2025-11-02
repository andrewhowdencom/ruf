package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Client is an interface that defines the methods for interacting with the Slack API.
type Client interface {
	PostMessage(channel, author, subject, text string) (string, string, error)
	NotifyAuthor(authorEmail, channelId, messageTimestamp, channelName string) error
	DeleteMessage(channel, timestamp string) error
	GetChannelID(channelName string) (string, error)
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

// PostMessage sends a message to a Slack channel.
func (c *client) PostMessage(channel, author, subject, text string) (string, string, error) {
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
	}

	channelID, err := c.GetChannelID(channel)
	if err != nil {
		return "", "", fmt.Errorf("failed to get channel id: %w", err)
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
	if !strings.HasPrefix(channelName, "#") {
		channelName = "#" + channelName
	}
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

// GetChannelID retrieves the ID of a channel given its name.
func (c *client) GetChannelID(channelName string) (string, error) {
  if !strings.HasPrefix(channelName, "#") {
		return channelName, nil
	}
  
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
	normalizedChannelName := strings.TrimPrefix(strings.ToLower(channelName), "#")

	for _, channel := range channels {
		if strings.ToLower(channel.Name) == normalizedChannelName {
			return channel.ID, nil
		}
	}

	return "", fmt.Errorf("channel '%s' not found", channelName)
}

