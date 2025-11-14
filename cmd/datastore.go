package cmd

import (
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
)

var datastoreNewStore = func(readOnly bool) (kv.Storer, error) {
	return datastore.NewStore(readOnly)
}
