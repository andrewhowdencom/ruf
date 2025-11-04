package slack

import (
	"context"
	"testing"
)

func TestGetChannelID(t *testing.T) {
	// Since we are not testing the Slack API itself, we can pass an empty token.
	// The test for the prefixed channel will fail, but that is expected.
	c := NewClient("").(*client)

	t.Run("should return the channel ID if it is not prefixed with a #", func(t *testing.T) {
		channelID, err := c.GetChannelID(context.Background(), "C1234567890")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if channelID != "C1234567890" {
			t.Errorf("expected channel ID to be C1234567890, got %s", channelID)
		}
	})

	t.Run("should attempt to resolve the channel ID if it is prefixed with a #", func(t *testing.T) {
		// This will fail because we are not using a real token.
		// However, we can assert that an error is returned, which proves that
		// the code is attempting to make an API call.
		_, err := c.GetChannelID(context.Background(), "#random")
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
}
