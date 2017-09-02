package memdb

import "time"

// Indexable is an item that can be stored in the store
type Indexable interface {
	// Lowest returns the lower of indexer or other (or null if can't be determined)
	Less(other interface{}) bool
	IsExpired(now time.Time, stats Stats) bool
	GetField(field string) string
}
