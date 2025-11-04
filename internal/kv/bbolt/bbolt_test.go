package bbolt_test

import (
	"os"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/kv/bbolt"
	"github.com/stretchr/testify/assert"
)

func TestStore_AddAndGetSentMessage(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	sm := &kv.SentMessage{
		SourceID:    "test-source",
		ScheduledAt: time.Now().UTC().Truncate(time.Second),
		Status:      kv.StatusSent,
	}

	err = store.AddSentMessage("test-campaign", "test-call", sm)
	assert.NoError(t, err)

	retrieved, err := store.GetSentMessage(sm.ID)
	assert.NoError(t, err)
	assert.Equal(t, sm, retrieved)
}

func TestStore_HasBeenSent(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	sm := &kv.SentMessage{
		SourceID:    "test-source",
		ScheduledAt: time.Now().UTC(),
		Status:      kv.StatusSent,
		Type:        "slack",
		Destination: "test-channel",
	}

	err = store.AddSentMessage("test-campaign", "test-call", sm)
	assert.NoError(t, err)

	sent, err := store.HasBeenSent("test-campaign", "test-call", "slack", "test-channel")
	assert.NoError(t, err)
	assert.True(t, sent)

	sent, err = store.HasBeenSent("test-campaign", "test-call", "slack", "other-channel")
	assert.NoError(t, err)
	assert.False(t, sent)
}

func TestStore_DeleteSentMessage(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	sm := &kv.SentMessage{
		SourceID:    "test-source",
		ScheduledAt: time.Now().UTC().Truncate(time.Second),
		Status:      kv.StatusSent,
	}

	err = store.AddSentMessage("test-campaign", "test-call", sm)
	assert.NoError(t, err)

	err = store.DeleteSentMessage(sm.ID)
	assert.NoError(t, err)

	retrieved, err := store.GetSentMessage(sm.ID)
	assert.NoError(t, err)
	assert.Equal(t, kv.StatusDeleted, retrieved.Status)
}
