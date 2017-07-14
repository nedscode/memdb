package memdb

// Iterator is a callback function definition that processes each item iteratively from functions like
// Ascend, Descend etc
type Iterator func(i Indexer) bool

// Indexer is an item that can be stored in the store
type Indexer interface {
	// Lowest returns the lower of indexer or other (or null if can't be determined)
	Less(other Indexer) bool
	IsExpired() bool
	GetField(field string) string
}
