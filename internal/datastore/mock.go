package datastore

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/andrewhowdencom/ruf/internal/kv"
)

// MockStore is a mock implementation of the Storer interface.
type MockStore struct {
	sentMessages   map[string]*kv.SentMessage
	scheduledCalls map[string]*kv.ScheduledCall
	schemaVersion  int
	mu             sync.Mutex
}

// NewMockStore creates a new MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		sentMessages:   make(map[string]*kv.SentMessage),
		scheduledCalls: make(map[string]*kv.ScheduledCall),
	}
}

// AddSentMessage adds a new sent message to the mock store.
func (s *MockStore) AddSentMessage(campaignID, callID string, sm *kv.SentMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm.ID = s.generateID(campaignID, callID, sm.Type, sm.Destination)
	sm.ShortID = kv.GenerateShortID(sm.ID)
	s.sentMessages[sm.ID] = sm

	// if the status is not set, default to sent
	if sm.Status == "" {
		sm.Status = kv.StatusSent
	}
	return nil
}

// UpdateSentMessage updates an existing sent message in the mock store.
func (s *MockStore) UpdateSentMessage(sm *kv.SentMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentMessages[sm.ID] = sm
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
		// If the full ID isn't found, try to find it by short ID.
		return s.getSentMessageByShortID(id)
	}
	return sm, nil
}

// GetSentMessageByShortID retrieves a single sent message from the mock store by its short ID.
func (s *MockStore) GetSentMessageByShortID(shortID string) (*kv.SentMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getSentMessageByShortID(shortID)
}

func (s *MockStore) getSentMessageByShortID(shortID string) (*kv.SentMessage, error) {
	var foundMessages []*kv.SentMessage
	for _, sm := range s.sentMessages {
		if strings.HasPrefix(sm.ShortID, shortID) {
			foundMessages = append(foundMessages, sm)
		}
	}
	if len(foundMessages) == 0 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrNotFound, shortID)
	}
	if len(foundMessages) > 1 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrAmbiguousID, shortID)
	}
	return foundMessages[0], nil
}

// DeleteSentMessage removes a sent message from the mock store.
func (s *MockStore) DeleteSentMessage(id string) error {
	sm, err := s.GetSentMessage(id)
	if err != nil {
		return err
	}
	sm.Status = kv.StatusDeleted
	return nil
}

// Close is a no-op for the mock store.
func (s *MockStore) Close() error {
	return nil
}

func (m *MockStore) ReserveSlot(slot time.Time, callID string) (bool, error) {
	return true, nil
}

func (m *MockStore) ClearAllSlots() error {
	return nil
}

// GetSchemaVersion retrieves the current schema version from the mock store.
func (s *MockStore) GetSchemaVersion() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schemaVersion, nil
}

// SetSchemaVersion sets the current schema version in the mock store.
func (s *MockStore) SetSchemaVersion(version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemaVersion = version
	return nil
}

// Scheduled call management
func (s *MockStore) AddScheduledCall(call *kv.ScheduledCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduledCalls[call.ID] = call
	return nil
}

func (s *MockStore) GetScheduledCall(id string) (*kv.ScheduledCall, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.scheduledCalls[id]
	if !ok {
		return nil, fmt.Errorf("%w: scheduled call with id '%s'", kv.ErrNotFound, id)
	}
	return call, nil
}

func (s *MockStore) ListScheduledCalls() ([]*kv.ScheduledCall, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var calls []*kv.ScheduledCall
	for _, call := range s.scheduledCalls {
		calls = append(calls, call)
	}
	return calls, nil
}

func (s *MockStore) DeleteScheduledCall(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scheduledCalls, id)
	return nil
}

func (s *MockStore) ClearScheduledCalls() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduledCalls = make(map[string]*kv.ScheduledCall)
	return nil
}
