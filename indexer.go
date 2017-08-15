package memdb

import "time"

// Iterator is a callback function definition that processes each item iteratively from functions like
// Ascend, Descend etc
type Iterator func(i interface{}) bool

// Indexable is an item that can be stored in the store
type Indexable interface {
	// Lowest returns the lower of indexer or other (or null if can't be determined)
	Less(other interface{}) bool
	IsExpired(now, fetched, updated time.Time) bool
	GetField(field string) string
}
