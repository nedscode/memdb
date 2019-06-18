package memdb

import "time"

// Expirable is an item that can be expired from the store.
type Expirable interface {
	// IsExpired returns whether the item should be expired or not.
	IsExpired(now time.Time, stats Stats) bool
}

// Indexable is an item that can be stored in the store.
type Indexable interface {
	// Less returns the lower of indexer or other (or null if can't be determined).
	Less(other interface{}) bool
	// GetField returns the value of the given field.
	GetField(field string) string
}
