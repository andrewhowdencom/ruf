package slack

import "github.com/andrewhowdencom/ruf/internal/model"

// MockClient is a mock implementation of the Client interface for testing.
type MockClient struct {
	PostMessageFunc   func(channel, author, subject, text string, campaign model.Campaign) (string, string, error)
	NotifyAuthorFunc  func(authorEmail, channelId, messageTimestamp, channelName string) error
	DeleteMessageFunc func(channel, timestamp string) error
	GetChannelIDFunc  func(channelName string) (string, error)

	postMessageCalls []struct {
		Destination string
		Author      string
		Subject     string
		Text        string
		Campaign    model.Campaign
	}
}

// NewMockClient creates a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		PostMessageFunc: func(channel, author, subject, text string, campaign model.Campaign) (string, string, error) {
			return "C1234567890", "1234567890.123456", nil
		},
		NotifyAuthorFunc: func(authorEmail, channelId, messageTimestamp, channelName string) error {
			return nil
		},
		DeleteMessageFunc: func(channel, timestamp string) error {
			return nil
		},
		GetChannelIDFunc: func(channelName string) (string, error) {
			return "C1234567890", nil
		},
	}
}

// PostMessage calls the PostMessageFunc.
func (m *MockClient) PostMessage(destination, author, subject, text string, campaign model.Campaign) (string, string, error) {
	m.postMessageCalls = append(m.postMessageCalls, struct {
		Destination string
		Author      string
		Subject     string
		Text        string
		Campaign    model.Campaign
	}{destination, author, subject, text, campaign})
	return m.PostMessageFunc(destination, author, subject, text, campaign)
}

// NotifyAuthor calls the NotifyAuthorFunc.
func (m *MockClient) NotifyAuthor(authorEmail, channelId, messageTimestamp, channelName string) error {
	return m.NotifyAuthorFunc(authorEmail, channelId, messageTimestamp, channelName)
}

// DeleteMessage calls the DeleteMessageFunc.
func (m *MockClient) DeleteMessage(channel, timestamp string) error {
	return m.DeleteMessageFunc(channel, timestamp)
}

// GetChannelID calls the GetChannelIDFunc.
func (m *MockClient) GetChannelID(channelName string) (string, error) {
	return m.GetChannelIDFunc(channelName)
}

// PostMessageCalls returns the recorded calls to PostMessage.
func (m *MockClient) PostMessageCalls() []struct {
	Destination string
	Author      string
	Subject     string
	Text        string
	Campaign    model.Campaign
} {
	return m.postMessageCalls
}
