package kv

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// Err* are common errors returned by the datastore.
var (
	ErrNotFound            = errors.New("not found")
	ErrDBOperationFailed   = errors.New("db operation failed")
	ErrSerializationFailed = errors.New("serialization failed")
	ErrAmbiguousID         = errors.New("ambiguous ID")
)

// Status represents the status of a call.
type Status string

const (
	// StatusSent means the call has been successfully sent.
	StatusSent Status = "sent"
	// StatusFailed means the call failed to send.
	StatusFailed Status = "failed"
	// StatusDeleted means the call has been deleted.
	StatusDeleted Status = "deleted"
)

// SentMessage represents a message that has been sent.
type SentMessage struct {
	ID           string    `json:"id"`
	ShortID      string    `json:"short_id"`
	SourceID     string    `json:"source_id"`
	ScheduledAt  time.Time `json:"scheduled_at"`
	Timestamp    string    `json:"timestamp,omitempty"` // Slack timestamp
	Destination  string    `json:"destination"`
	Type         string    `json:"type"`
	Status       Status    `json:"status"`
	CampaignName string    `json:"campaign_name"`
}

// Storer is an interface that defines the methods for interacting with the datastore.
type Storer interface {
	AddSentMessage(campaignID, callID string, sm *SentMessage) error
	UpdateSentMessage(sm *SentMessage) error
	HasBeenSent(campaignID, callID, destType, destination string) (bool, error)
	ListSentMessages() ([]*SentMessage, error)
	GetSentMessage(id string) (*SentMessage, error)
	GetSentMessageByShortID(shortID string) (*SentMessage, error)
	DeleteSentMessage(id string) error
	Close() error

	// Slot management
	ReserveSlot(slot time.Time, callID string) (bool, error)
	ClearAllSlots() error
}

// GenerateShortID generates a short ID for a given ID.
func GenerateShortID(id string) string {
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:])[:8]
}
