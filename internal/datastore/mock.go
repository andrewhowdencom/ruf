package datastore

import (
	"fmt"
	"strings"
	"sync"

	"github.com/andrewhowdencom/ruf/internal/kv"
)

// MockStore is a mock implementation of the Storer interface.
type MockStore struct {
	sentMessages map[string]*kv.SentMessage
	mu           sync.Mutex
}

// NewMockStore creates a new MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		sentMessages: make(map[string]*kv.SentMessage),
	}
}

// AddSentMessage adds a new sent message to the mock store.
func (s *MockStore) AddSentMessage(campaignID, callID string, sm *kv.SentMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm.ID = s.generateID(campaignID, callID, sm.Type, sm.Destination)
	s.sentMessages[sm.ID] = sm

	// if the status is not set, default to sent
	if sm.Status == "" {
		sm.Status = kv.StatusSent
	}
	return nil
}

// HasBeenSent checks if a message with the given sourceID and scheduledAt time has been sent.
func (s *MockStore) HasBeenSent(campaignID, callID, destType, destination string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.generateID(campaignID, callID, destType, destination)
	sm, ok := s.sentMessages[id]
	return ok && (sm.Status == kv.StatusSent || sm.Status == kv.StatusDeleted), nil
}

func (s *MockStore) generateID(campaignID, callID, destType, destination string) string {
	parts := []string{
		campaignID,
		callID,
		destType,
		destination,
	}
	return strings.Join(parts, "@")
}

// ListSentMessages retrieves all sent messages from the mock store.
func (s *MockStore) ListSentMessages() ([]*kv.SentMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sentMessages []*kv.SentMessage
	for _, sm := range s.sentMessages {
		sentMessages = append(sentMessages, sm)
	}
	return sentMessages, nil
}

// GetSentMessage retrieves a single sent message from the mock store.
func (s *MockStore) GetSentMessage(id string) (*kv.SentMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm, ok := s.sentMessages[id]
	if !ok {
		return nil, fmt.Errorf("message with id '%s' not found", id)
	}
	return sm, nil
}

// DeleteSentMessage removes a sent message from the mock store.
func (s *MockStore) DeleteSentMessage(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm, ok := s.sentMessages[id]
	if !ok {
		return fmt.Errorf("message with id '%s' not found", id)
	}
	sm.Status = kv.StatusDeleted
	return nil
}

// Close is a no-op for the mock store.
func (s *MockStore) Close() error {
	return nil
}
