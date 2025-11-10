package bbolt_test

import (
	"os"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/kv/bbolt"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestScheduledCallPersistence(t *testing.T) {
	dbPath := "persistence_test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second) // Truncate for clean comparison
	call := &kv.ScheduledCall{
		Call: model.Call{
			ID: "test-persistence-call",
		},
		ScheduledAt: now,
	}

	// Add the call to the datastore
	err = store.AddScheduledCall(call)
	assert.NoError(t, err)

	// Retrieve the call from the datastore
	retrievedCall, err := store.GetScheduledCall("test-persistence-call")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedCall)

	// Assert that the ScheduledAt field was persisted correctly
	assert.Equal(t, now, retrievedCall.ScheduledAt, "The ScheduledAt field should be persisted and retrieved correctly.")
}
