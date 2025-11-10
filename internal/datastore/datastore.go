package datastore

import (
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/kv/bbolt"
	"github.com/andrewhowdencom/ruf/internal/kv/firestore"
	"github.com/spf13/viper"
)

// NewStore creates a new Store and initializes the database.
func NewStore(readOnly bool) (kv.Storer, error) {
	datastoreType := viper.GetString("datastore.type")
	switch datastoreType {
	case "bbolt":
		if readOnly {
			return bbolt.NewReadOnlyStore()
		}
		return bbolt.NewReadWriteStore()
	case "firestore":
		projectID := viper.GetString("datastore.project_id")
		if projectID == "" {
			return nil, fmt.Errorf("datastore.project_id must be set when using firestore")
		}
		return firestore.NewStore(projectID)
	default:
		return nil, fmt.Errorf("unknown datastore type: %s", datastoreType)
	}
}

// NewTestStore creates a new Store for testing purposes.
func NewTestStore(dbPath string) (kv.Storer, error) {
	return bbolt.NewTestStore(dbPath)
}
