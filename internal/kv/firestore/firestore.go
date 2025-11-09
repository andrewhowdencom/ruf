package firestore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store manages the persistence of calls in Firestore.
type Store struct {
	client *firestore.Client
}

// NewStore creates a new Store and initializes the Firestore client.
func NewStore(projectID string) (kv.Storer, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}
	return &Store{client: client}, nil
}

// Close closes the Firestore client connection.
func (s *Store) Close() error {
	return s.client.Close()
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

// AddSentMessage adds a new sent message to the store.
func (s *Store) AddSentMessage(campaignID, callID string, sm *kv.SentMessage) error {
	ctx := context.Background()
	sm.ID = s.generateID(campaignID, callID, sm.Type, sm.Destination)
	sm.ShortID = kv.GenerateShortID(sm.ID)
	_, err := s.client.Collection("sent_messages").Doc(sm.ID).Set(ctx, sm)
	if err != nil {
		return fmt.Errorf("%w: failed to add sent message: %w", kv.ErrDBOperationFailed, err)
	}
	return nil
}

// GetSchemaVersion retrieves the current schema version from the store.
func (s *Store) GetSchemaVersion() (int, error) {
	ctx := context.Background()
	doc, err := s.client.Collection("meta").Doc("schema_version").Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("%w: failed to get schema version: %w", kv.ErrDBOperationFailed, err)
	}

	version, err := doc.DataAt("version")
	if err != nil {
		return 0, fmt.Errorf("%w: failed to get schema version: %w", kv.ErrDBOperationFailed, err)
	}

	return version.(int), nil
}

// SetSchemaVersion sets the current schema version in the store.
func (s *Store) SetSchemaVersion(version int) error {
	ctx := context.Background()
	_, err := s.client.Collection("meta").Doc("schema_version").Set(ctx, map[string]interface{}{
		"version": version,
	})
	if err != nil {
		return fmt.Errorf("%w: failed to set schema version: %w", kv.ErrDBOperationFailed, err)
	}
	return nil
}

// UpdateSentMessage updates an existing sent message in the store.
func (s *Store) UpdateSentMessage(sm *kv.SentMessage) error {
	ctx := context.Background()
	_, err := s.client.Collection("sent_messages").Doc(sm.ID).Set(ctx, sm)
	if err != nil {
		return fmt.Errorf("%w: failed to update sent message: %w", kv.ErrDBOperationFailed, err)
	}
	return nil
}

func (s *Store) ReserveSlot(slot time.Time, callID string) (bool, error) {
	ctx := context.Background()
	key := slot.Format(time.RFC3339)
	docRef := s.client.Collection("slots").Doc(key)

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return err
		}
		if doc.Exists() {
			return fmt.Errorf("slot already reserved")
		}
		return tx.Set(docRef, map[string]string{"callId": callID})
	})

	if err != nil {
		if err.Error() == "slot already reserved" {
			return false, nil
		}
		return false, fmt.Errorf("%w: failed to reserve slot: %w", kv.ErrDBOperationFailed, err)
	}

	return true, nil
}

func (s *Store) ClearAllSlots() error {
	ctx := context.Background()
	ref := s.client.Collection("slots")
	for {
		iter := ref.Limit(100).Documents(ctx)
		numDeleted, err := iter.GetAll()
		if err != nil {
			return fmt.Errorf("%w: failed to iterate documents: %w", kv.ErrDBOperationFailed, err)
		}
		if len(numDeleted) == 0 {
			return nil
		}

		batch := s.client.Batch()
		for _, doc := range numDeleted {
			batch.Delete(doc.Ref)
		}
		_, err = batch.Commit(ctx)
		if err != nil {
			return fmt.Errorf("%w: failed to commit batch delete: %w", kv.ErrDBOperationFailed, err)
		}
	}
}

// HasBeenSent checks if a message with the given sourceID and scheduledAt time has a 'sent' or 'deleted' status.
func (s *Store) HasBeenSent(campaignID, callID, destType, destination string) (bool, error) {
	ctx := context.Background()
	id := s.generateID(campaignID, callID, destType, destination)
	doc, err := s.client.Collection("sent_messages").Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("%w: failed to get sent message: %w", kv.ErrDBOperationFailed, err)
	}

	var sm kv.SentMessage
	if err := doc.DataTo(&sm); err != nil {
		return false, fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
	}

	return sm.Status == kv.StatusSent || sm.Status == kv.StatusDeleted, nil
}

// ListSentMessages retrieves all sent messages from the store.
func (s *Store) ListSentMessages() ([]*kv.SentMessage, error) {
	ctx := context.Background()
	var messages []*kv.SentMessage
	iter := s.client.Collection("sent_messages").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var sm kv.SentMessage
		if err := doc.DataTo(&sm); err != nil {
			return nil, fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
		}
		messages = append(messages, &sm)
	}
	return messages, nil
}

// GetSentMessage retrieves a single sent message from the store.
func (s *Store) GetSentMessage(id string) (*kv.SentMessage, error) {
	ctx := context.Background()
	doc, err := s.client.Collection("sent_messages").Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// If the full ID isn't found, try to find it by short ID.
			return s.GetSentMessageByShortID(id)
		}
		return nil, fmt.Errorf("%w: failed to get sent message: %w", kv.ErrDBOperationFailed, err)
	}

	var sm kv.SentMessage
	if err := doc.DataTo(&sm); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
	}
	return &sm, nil
}

// GetSentMessageByShortID retrieves a single sent message from the store by its short ID.
func (s *Store) GetSentMessageByShortID(shortID string) (*kv.SentMessage, error) {
	ctx := context.Background()
	end := shortID + "~"
	iter := s.client.Collection("sent_messages").Where("ShortID", ">=", shortID).Where("ShortID", "<", end).Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get sent message by short id: %w", kv.ErrDBOperationFailed, err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrNotFound, shortID)
	}
	if len(docs) > 1 {
		return nil, fmt.Errorf("%w: message with short id '%s'", kv.ErrAmbiguousID, shortID)
	}

	var sm kv.SentMessage
	if err := docs[0].DataTo(&sm); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal sent message: %w", kv.ErrSerializationFailed, err)
	}
	return &sm, nil
}

// DeleteSentMessage removes a sent message from the store.
func (s *Store) DeleteSentMessage(id string) error {
	sm, err := s.GetSentMessage(id)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = s.client.Collection("sent_messages").Doc(sm.ID).Update(ctx, []firestore.Update{
		{Path: "Status", Value: kv.StatusDeleted},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("%w: message with id '%s'", kv.ErrNotFound, id)
		}
		return fmt.Errorf("%w: failed to delete sent message: %w", kv.ErrDBOperationFailed, err)
	}
	return nil
}
