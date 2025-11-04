package bbolt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adrg/xdg"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"go.etcd.io/bbolt"
)

var sentMessagesBucket = []byte("sent_messages")

// Store manages the persistence of calls.
type Store struct {
	db *bbolt.DB
}

// NewStore creates a new Store and initializes the database.
func NewStore() (kv.Storer, error) {
	dbPath, err := xdg.DataFile("ruf/ruf.db")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get db path: %w", kv.ErrDBOperationFailed, err)
	}

	return newStore(dbPath)
}

// NewTestStore creates a new Store for testing purposes.
func NewTestStore(dbPath string) (kv.Storer, error) {
	return newStore(dbPath)
}

func newStore(dbPath string) (kv.Storer, error) {
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open db: %w", kv.ErrDBOperationFailed, err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(sentMessagesBucket)
		if err != nil {
			return fmt.Errorf("%w: failed to create bucket: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// AddSentMessage adds a new sent message to the store.
func (s *Store) AddSentMessage(campaignID, callID string, sm *kv.SentMessage) error {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		sm.ID = s.generateID(campaignID, callID, sm.Type, sm.Destination)

		buf, err := json.Marshal(sm)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal sent message: %w", kv.ErrSerializationFailed, err)
		}

		if err := b.Put([]byte(sm.ID), buf); err != nil {
			return fmt.Errorf("%w: failed to put sent message: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
	return err
}

// HasBeenSent checks if a message with the given sourceID and scheduledAt time has a 'sent' or 'deleted' status.
// It returns false for messages that have a 'failed' status, or do not exist.
func (s *Store) HasBeenSent(campaignID, callID, destType, destination string) (bool, error) {
	var sent bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		id := s.generateID(campaignID, callID, destType, destination)
		v := b.Get([]byte(id))
		if v != nil {
			var sm kv.SentMessage
			if err := json.Unmarshal(v, &sm); err != nil {
				return fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
			}
			if sm.Status == kv.StatusSent || sm.Status == kv.StatusDeleted {
				sent = true
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("%w: failed to check if message has been sent: %w", kv.ErrDBOperationFailed, err)
	}
	return sent, nil
}

func (s *Store) generateID(campaignID, callID, destType, destination string) string {
	parts := []string{
		campaignID,
		callID,
		destType,
		destination,
	}
	return strings.Join(parts, "@")
}

// ListSentMessages retrieves all sent messages from the store.
func (s *Store) ListSentMessages() ([]*kv.SentMessage, error) {
	var sentMessages []*kv.SentMessage
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		err := b.ForEach(func(k, v []byte) error {
			var sm kv.SentMessage
			if err := json.Unmarshal(v, &sm); err != nil {
				return fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
			}
			sentMessages = append(sentMessages, &sm)
			return nil
		})
		if err != nil {
			return fmt.Errorf("%w: failed to iterate over sent messages: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sentMessages, nil
}

// GetSentMessage retrieves a single sent message from the store.
func (s *Store) GetSentMessage(id string) (*kv.SentMessage, error) {
	var sm kv.SentMessage
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("%w: message with id '%s'", kv.ErrNotFound, id)
		}
		if err := json.Unmarshal(v, &sm); err != nil {
			return fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &sm, nil
}

// DeleteSentMessage removes a sent message from the store.
func (s *Store) DeleteSentMessage(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("%w: message with id '%s'", kv.ErrNotFound, id)
		}

		var sm kv.SentMessage
		if err := json.Unmarshal(v, &sm); err != nil {
			return fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
		}

		sm.Status = kv.StatusDeleted

		buf, err := json.Marshal(sm)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal sent message: %w", kv.ErrSerializationFailed, err)
		}

		if err := b.Put([]byte(id), buf); err != nil {
			return fmt.Errorf("%w: failed to put sent message: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
}
