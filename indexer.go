package memdb

import "time"

// ExpireBool identifies whether an item should expire, or pass through the check
type ExpireBool int

const (
	// ExpireFalse item is not expired
	ExpireFalse ExpireBool = iota
	// ExpireTrue item is expired
	ExpireTrue
	// ExpireNull this check should not influence the expiry of this item
	ExpireNull
)

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

// ExpireFunc is a function that is run to determine if an item is expired
//   return ExpireNull to take no expire action and allow any other Expirers to act
type ExpireFunc func(a interface{}, now time.Time, stats Stats) ExpireBool

// Expirer can determine if an item is expired given a current time, last Accessed
// and last Modified time
type Expirer interface {
	IsExpired(a interface{}, now time.Time, stats Stats) bool
}

// Fielder can get the string value for a given item's named field
type Fielder interface {
	GetField(a interface{}, field string) string
}

var (
	none = &wrap{}
)
