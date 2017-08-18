package memdb

import "time"

// Indexer can be passed to a Storer's SetIndexer. It is a Comparator, Expirer and Fielder
type Indexer interface {
	Comparator
	Expirer
	Fielder
}

// Comparator can perform a Less comparison of 2 items
type Comparator interface {
	Less(a interface{}, b interface{}) bool
}

// Expirer can determine if an item is expired given a current time, last fetched
// and last updated time
type Expirer interface {
	IsExpired(a interface{}, now, fetched, updated time.Time) bool
}

// Fielder can get the string value for a given item's named field
type Fielder interface {
	GetField(a interface{}, field string) string
}

var (
	none = &wrap{}
)
