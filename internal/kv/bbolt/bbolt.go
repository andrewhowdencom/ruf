package bbolt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"go.etcd.io/bbolt"
)

var (
	sentMessagesBucket   = []byte("sent_messages")
	scheduledCallsBucket = []byte("scheduled_calls")
	slotsBucket          = []byte("slots")
	metaBucket           = []byte("meta")
)

// Store manages the persistence of calls.
type Store struct {
	db *bbolt.DB
}

// NewReadWriteStore creates a new read-write Store and initializes the database.
func NewReadWriteStore() (kv.Storer, error) {
	dbPath, err := xdg.DataFile("ruf/ruf.db")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get db path: %w", kv.ErrDBOperationFailed, err)
	}

	return newStore(dbPath, false)
}

// NewReadOnlyStore creates a new read-only Store and initializes the database.
func NewReadOnlyStore() (kv.Storer, error) {
	dbPath, err := xdg.DataFile("ruf/ruf.db")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get db path: %w", kv.ErrDBOperationFailed, err)
	}

	return newStore(dbPath, true)
}

// NewTestStore creates a new Store for testing purposes.
func NewTestStore(dbPath string) (kv.Storer, error) {
	return newStore(dbPath, false)
}

func newStore(dbPath string, readOnly bool) (kv.Storer, error) {
	options := &bbolt.Options{
		ReadOnly: readOnly,
	}
	db, err := bbolt.Open(dbPath, 0600, options)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open db: %w", kv.ErrDBOperationFailed, err)
	}

	if !readOnly {
		err = db.Update(func(tx *bbolt.Tx) error {
			if _, err := tx.CreateBucketIfNotExists(sentMessagesBucket); err != nil {
				return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, sentMessagesBucket, err)
			}
			if _, err := tx.CreateBucketIfNotExists(scheduledCallsBucket); err != nil {
				return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, scheduledCallsBucket, err)
			}
			if _, err := tx.CreateBucketIfNotExists(slotsBucket); err != nil {
				return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, slotsBucket, err)
			}
			if _, err := tx.CreateBucketIfNotExists(metaBucket); err != nil {
				return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, metaBucket, err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
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
		sm.ShortID = kv.GenerateShortID(sm.ID)

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

// UpdateSentMessage updates an existing sent message in the store.
func (s *Store) UpdateSentMessage(sm *kv.SentMessage) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		buf, err := json.Marshal(sm)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal sent message: %w", kv.ErrSerializationFailed, err)
		}
		if err := b.Put([]byte(sm.ID), buf); err != nil {
			return fmt.Errorf("%w: failed to put sent message: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
}

// Scheduled call management
func (s *Store) AddScheduledCall(call *model.Call) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(scheduledCallsBucket)
		buf, err := json.Marshal(call)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal scheduled call: %w", kv.ErrSerializationFailed, err)
		}
		if err := b.Put([]byte(call.ID), buf); err != nil {
			return fmt.Errorf("%w: failed to put scheduled call: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
}

func (s *Store) GetScheduledCall(id string) (*model.Call, error) {
	var call model.Call
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(scheduledCallsBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("%w: scheduled call with id '%s'", kv.ErrNotFound, id)
		}
		if err := json.Unmarshal(v, &call); err != nil {
			return fmt.Errorf("%w: failed to unmarshal scheduled call: %w", kv.ErrSerializationFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &call, nil
}

func (s *Store) ListScheduledCalls() ([]*model.Call, error) {
	var calls []*model.Call
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(scheduledCallsBucket)
		err := b.ForEach(func(k, v []byte) error {
			var call model.Call
			if err := json.Unmarshal(v, &call); err != nil {
				return fmt.Errorf("%w: failed to unmarshal scheduled call: %w", kv.ErrSerializationFailed, err)
			}
			calls = append(calls, &call)
			return nil
		})
		if err != nil {
			return fmt.Errorf("%w: failed to iterate over scheduled calls: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (s *Store) DeleteScheduledCall(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(scheduledCallsBucket)
		if err := b.Delete([]byte(id)); err != nil {
			return fmt.Errorf("%w: failed to delete scheduled call: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
}

func (s *Store) ClearScheduledCalls() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(scheduledCallsBucket); err != nil {
			return fmt.Errorf("%w: failed to delete bucket '%s': %w", kv.ErrDBOperationFailed, scheduledCallsBucket, err)
		}
		if _, err := tx.CreateBucket(scheduledCallsBucket); err != nil {
			return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, scheduledCallsBucket, err)
		}
		return nil
	})
}

// GetSchemaVersion retrieves the current schema version from the store.
func (s *Store) GetSchemaVersion() (int, error) {
	var version int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(metaBucket)
		v := b.Get([]byte("schema_version"))
		if v == nil {
			return nil
		}
		if err := json.Unmarshal(v, &version); err != nil {
			return fmt.Errorf("%w: failed to unmarshal schema version: %w", kv.ErrSerializationFailed, err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return version, nil
}

// SetSchemaVersion sets the current schema version in the store.
func (s *Store) SetSchemaVersion(version int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(metaBucket)
		buf, err := json.Marshal(version)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal schema version: %w", kv.ErrSerializationFailed, err)
		}
		if err := b.Put([]byte("schema_version"), buf); err != nil {
			return fmt.Errorf("%w: failed to put schema version: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
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
			// If the full ID isn't found, try to find it by short ID.
			found, err := s.getSentMessageByShortID(tx, id)
			if err != nil {
				return err
			}
			sm = *found
			return nil
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

// GetSentMessageByShortID retrieves a single sent message from the store by its short ID.
func (s *Store) GetSentMessageByShortID(shortID string) (*kv.SentMessage, error) {
	var sm *kv.SentMessage
	err := s.db.View(func(tx *bbolt.Tx) error {
		found, err := s.getSentMessageByShortID(tx, shortID)
		if err != nil {
			return err
		}
		sm = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sm, nil
}

func (s *Store) getSentMessageByShortID(tx *bbolt.Tx, shortID string) (*kv.SentMessage, error) {
	var foundMessages []*kv.SentMessage
	b := tx.Bucket(sentMessagesBucket)
	err := b.ForEach(func(k, v []byte) error {
		var sm kv.SentMessage
		if err := json.Unmarshal(v, &sm); err != nil {
			return fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
		}
		if strings.HasPrefix(sm.ShortID, shortID) {
			foundMessages = append(foundMessages, &sm)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to iterate over sent messages: %w", kv.ErrDBOperationFailed, err)
	}
	if len(foundMessages) == 0 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrNotFound, shortID)
	}
	if len(foundMessages) > 1 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrAmbiguousID, shortID)
	}
	return foundMessages[0], nil
}

// DeleteSentMessage removes a sent message from the store.
func (s *Store) DeleteSentMessage(id string) error {
	sm, err := s.GetSentMessage(id)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(sentMessagesBucket)
		sm.Status = kv.StatusDeleted

		buf, err := json.Marshal(sm)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal sent message: %w", kv.ErrSerializationFailed, err)
		}

		if err := b.Put([]byte(sm.ID), buf); err != nil {
			return fmt.Errorf("%w: failed to put sent message: %w", kv.ErrDBOperationFailed, err)
		}
		return nil
	})
}

func (s *Store) ReserveSlot(slot time.Time, callID string) (bool, error) {
	var reserved bool
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(slotsBucket)
		key := []byte(slot.Format(time.RFC3339))
		if v := b.Get(key); v != nil {
			return nil // Slot is already taken
		}

		if err := b.Put(key, []byte(callID)); err != nil {
			return fmt.Errorf("%w: failed to reserve slot: %w", kv.ErrDBOperationFailed, err)
		}
		reserved = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return reserved, nil
}

func (s *Store) ClearAllSlots() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(slotsBucket); err != nil {
			return fmt.Errorf("%w: failed to delete bucket '%s': %w", kv.ErrDBOperationFailed, slotsBucket, err)
		}
		if _, err := tx.CreateBucket(slotsBucket); err != nil {
			return fmt.Errorf("%w: failed to create bucket '%s': %w", kv.ErrDBOperationFailed, slotsBucket, err)
		}
		return nil
	})
}
