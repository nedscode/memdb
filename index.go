package memdb

import (
	"strings"
	"time"
)

// Index implements IndexSearcher and represents a list of indexes
type Index struct {
	IndexSearcher

	n      int
	id     string
	fields []string
	store  *Store
	unique bool
}

// Each calls iterator for every matched element
// Items are not guaranteed to be in any particular order
func (idx *Index) Each(cb Iterator, keys ...string) {
	if idx == nil {
		return
	}

	idx.store.RLock()
	defer idx.store.RUnlock()

	values := idx.find(keys)
	if values == nil {
		return
	}

	now := time.Now()
	for _, wrapped := range values {
		wrapped.fetched = now
		wrapped.reads++
		if !cb(wrapped.item) {
			return
		}
	}
}

// One is like Lookup, except just returns the first item found
func (idx *Index) One(keys ...string) interface{} {
	if idx == nil {
		return nil
	}

	idx.store.RLock()
	defer idx.store.RUnlock()

	values := idx.find(keys)
	if len(values) > 0 {
		wrapped := values[0]
		wrapped.fetched = time.Now()
		wrapped.reads++
		return wrapped.item
	}
	return nil
}

// Lookup returns the list of items from the index that match given key
// Returned items are not guaranteed to be in any particular order
func (idx *Index) Lookup(keys ...string) []interface{} {
	if idx == nil {
		return nil
	}

	idx.store.RLock()
	defer idx.store.RUnlock()

	values := idx.find(keys)
	if values == nil {
		return nil
	}

	now := time.Now()
	c := make([]interface{}, len(values))
	for i, wrapped := range values {
		c[i] = wrapped.item
		wrapped.fetched = now
		wrapped.reads++
	}
	return c
}

func (idx *Index) _id() string {
	if idx == nil {
		return ""
	}

	return idx.id
}

func (idx *Index) find(keys []string) []*wrap {
	if idx == nil {
		return nil
	}

	if len(keys) != len(idx.fields) {
		return nil
	}

	s := idx.store

	index, ok := s.index[idx.id]
	if !ok {
		return nil
	}

	key := strings.Join(keys, "\000")

	values, ok := index[key]
	if !ok {
		return nil
	}

	return values
}
