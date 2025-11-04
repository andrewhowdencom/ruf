package slack

import "context"

// MockClient is a mock implementation of the Client interface for testing.
type MockClient struct {
	PostMessageFunc   func(ctx context.Context, channel, author, subject, text string) (string, string, error)
	NotifyAuthorFunc  func(ctx context.Context, authorEmail, channelId, messageTimestamp, channelName string) error
	DeleteMessageFunc func(ctx context.Context, channel, timestamp string) error
	GetChannelIDFunc  func(ctx context.Context, channelName string) (string, error)

	PostMessageCount  int
	NotifyAuthorCount int
}

// NewMockClient creates a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		PostMessageFunc: func(ctx context.Context, channel, author, subject, text string) (string, string, error) {
			return "C1234567890", "1234567890.123456", nil
		},
		NotifyAuthorFunc: func(ctx context.Context, authorEmail, channelId, messageTimestamp, channelName string) error {
			return nil
		},
		DeleteMessageFunc: func(ctx context.Context, channel, timestamp string) error {
			return nil
		},
		GetChannelIDFunc: func(ctx context.Context, channelName string) (string, error) {
			return "C1234567890", nil
		},
	}
}

// PostMessage calls the PostMessageFunc.
func (m *MockClient) PostMessage(ctx context.Context, channel, author, subject, text string) (string, string, error) {
	m.PostMessageCount++
	return m.PostMessageFunc(ctx, channel, author, subject, text)
}

// NotifyAuthor calls the NotifyAuthorFunc.
func (m *MockClient) NotifyAuthor(ctx context.Context, authorEmail, channelId, messageTimestamp, channelName string) error {
	m.NotifyAuthorCount++
	return m.NotifyAuthorFunc(ctx, authorEmail, channelId, messageTimestamp, channelName)
}

// DeleteMessage calls the DeleteMessageFunc.
func (m *MockClient) DeleteMessage(ctx context.Context, channel, timestamp string) error {
	return m.DeleteMessageFunc(ctx, channel, timestamp)
}

// GetChannelID calls the GetChannelIDFunc.
func (m *MockClient) GetChannelID(ctx context.Context, channelName string) (string, error) {
	return m.GetChannelIDFunc(ctx, channelName)
}
