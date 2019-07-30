// Package persist defines interfaces for building Persister implementations for memdb
package persist

// FactoryFunc is a function which will return an interface of a named type for decoding the stored Indexer into.
type FactoryFunc func(indexerType string) interface{}

// LoadFunc is a function which will bulk-load the given indexer into the memdb instance at creation time
type LoadFunc func(id string, indexer interface{})

// Persister is an interface to allow different means of persistent storage to be used with memdb
type Persister interface {
	// Save is called to request persistent save of the indexer with id
	Save(id string, indexer interface{}) error

	// Load is called at create time to load all of the persisted items and call loadFunc with each
	Load(loadFunc LoadFunc) error

	// Remove is called when an indexer is expired or deleted and needs removal from persistent store
	Remove(id string) error
}

// MetaLoadFunc is a function which will bulk-load the given indexer into the memdb instance at creation time
type MetaLoadFunc func(id string, indexer interface{}, meta *Meta)

// MetaPersister is an interface to allow persistent storage with additional metadata
type MetaPersister interface {
	Persister

	// MetaSave is called to request persistent save of the indexer with id
	MetaSave(id string, indexer interface{}) (meta *Meta, err error)

	// MetaLoad is called at create time to load all of the persisted items and call loadFunc with each
	MetaLoad(loadFunc MetaLoadFunc) error
}

// Meta contains metadata
type Meta struct {
	Size uint64
}
