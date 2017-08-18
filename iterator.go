package memdb

import "time"

// Iterator is a callback function definition that processes each item iteratively from functions like
// Ascend, Descend etc
type Iterator func(i interface{}) bool

// InfoIterator is a callback function definition that processes each item iteratively from Info function
type InfoIterator func(uid UID, i interface{}, fetched, updated time.Time, reads, writes uint64) bool
