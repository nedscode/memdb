package memdb

// FactoryFunc is a function which will return an interface of a named type for decoding the stored Indexer into.
type FactoryFunc func(indexerType string) interface{}

// LoadFunc is a function which will bulk-load the given indexer into the memdb instance at creation time
type LoadFunc func(id string, indexer Indexer)

// Persister is an interface to allow different means of persistent storage to be used with memdb
type Persister interface {
	// Save is called to request persistent save of the indexer with id
	Save(id string, indexer Indexer)

	// Load is called at create time to load all of the persisted items and call loadFunc with each
	Load(loadFunc LoadFunc)

	// Remove is called when an indexer is expired or deleted and needs removal from persistent store
	Remove(id string)
}
