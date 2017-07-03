// Package memdb is designed to allow configurable indexing of values from a structure
package memdb

import (
	"github.com/google/btree"

	"sync"
)

// Store implements an indexed storage for an Indexer item
//
// Bad things happen if the item's indexed fields change mid-flight. Care should be taken to always return the same
// value for fields [ie: Indexer.GetField(field)] as when first stored, even if they are changed, or else delete the
// item pre-change and re-store it post-change.
type Store struct {
	sync.RWMutex

	fields  []string
	backing *btree.BTree
	index   map[string]map[string][]Indexer
	used    bool
}

// NewStore returns an initialized store for you to use
func NewStore() *Store {
	return &Store{
		backing: btree.New(2),
		index:   map[string]map[string][]Indexer{},
	}
}

// CreateField adds a field to the list of indexed fields before the store is populated
func (s *Store) CreateField(field string) *Store {
	if s.used {
		panic("Cannot create field on in-use store")
	}
	s.fields = append(s.fields, field)
	return s
}

// Get returns an item equal to the passed item from the store
func (s *Store) Get(search Indexer) Indexer {
	s.RLock()
	defer s.RUnlock()

	found := s.backing.Get(&wrap{search})
	if found == nil {
		return nil
	}

	if w, ok := found.(*wrap); ok {
		return w.indexer
	}

	return nil
}

// Lookup returns the list of items from the indexed field that match given key
// Returned items are not guaranteed to be in any particular order
func (s *Store) Lookup(field, key string) []Indexer {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.index[field]
	if !ok {
		return nil
	}

	values, ok := index[key]
	if !ok {
		return nil
	}

	c := make([]Indexer, len(values))
	copy(c, values)
	return c
}

// Ascend calls provided callback function from start (lowest order) of items until end or iterator function returns false
func (s *Store) Ascend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()

	s.backing.Ascend(func(i btree.Item) bool {
		if w, ok := i.(*wrap); ok {
			return cb(w.indexer)
		}
		return true
	})
}

// AscendStarting calls provided callback function from item equal to at until end or iterator function returns false
func (s *Store) AscendStarting(at Indexer, cb Iterator) {
	s.RLock()
	defer s.RUnlock()

	s.backing.AscendGreaterOrEqual(&wrap{at}, func(item btree.Item) bool {
		if w, ok := item.(*wrap); ok {
			return cb(w.indexer)
		}
		return true
	})
}

// Descend calls provided callback function from end (highest order) of items until start or iterator function returns false
func (s *Store) Descend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()

	s.backing.Descend(func(i btree.Item) bool {
		if w, ok := i.(*wrap); ok {
			return cb(w.indexer)
		}
		return true
	})
}

// DescendStarting calls provided callback function from item equal to at until start or iterator function returns false
func (s *Store) DescendStarting(at Indexer, cb Iterator) {
	s.RLock()
	defer s.RUnlock()

	s.backing.DescendLessOrEqual(&wrap{at}, func(item btree.Item) bool {
		if w, ok := item.(*wrap); ok {
			return cb(w.indexer)
		}
		return true
	})
}

// Expire finds all expiring items in the store and deletes them
func (s *Store) Expire() {
	s.Lock()
	defer s.Unlock()

	var rm []Indexer

	s.backing.Ascend(func(item btree.Item) bool {
		if w, ok := item.(*wrap); ok {
			if w.indexer.IsExpired() {
				rm = append(rm, w.indexer)
			}
		}
		return true
	})

	for _, v := range rm {
		s.rm(v)
	}
}

// Store places an indexer item into the store
func (s *Store) Store(indexer Indexer) {
	s.Lock()
	defer s.Unlock()

	s.add(indexer)
}

// Delete removes an item equal to the search item
func (s *Store) Delete(search Indexer) {
	s.Lock()
	defer s.Unlock()

	s.rm(search)
}

// Len returns the number of items in the database
func (s *Store) Len() int {
	s.RLock()
	defer s.RUnlock()

	return s.backing.Len()
}

// Fields returns the list of indexed fields
func (s *Store) Fields() []string {
	s.RLock()
	defer s.RUnlock()

	f := make([]string, len(s.fields))
	copy(f, s.fields)
	return f
}

// Keys returns the list of distinct keys for a field
func (s *Store) Keys(field string) []string {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.index[field]
	if !ok {
		return nil
	}

	keys := make([]string, len(index))
	i := 0
	for key := range index {
		keys[i] = key
		i++
	}
	return keys
}

func (s *Store) add(indexer Indexer) {
	s.used = true
	s.backing.ReplaceOrInsert(&wrap{indexer})
	for _, field := range s.fields {
		key := indexer.GetField(field)
		s.addToIndex(field, key, indexer)
	}
}

func (s *Store) addToIndex(field, key string, indexer Indexer) {
	index, ok := s.index[field]
	if !ok {
		index = map[string][]Indexer{}
		s.index[field] = index
	}

	index[key] = append(index[key], indexer)
}

func (s *Store) rm(indexer Indexer) {
	removed := s.backing.Delete(&wrap{indexer})
	if removed != nil {
		for _, field := range s.fields {
			key := indexer.GetField(field)
			s.rmFromIndex(field, key, indexer)
		}
	}
}

func (s *Store) rmFromIndex(field, key string, indexer Indexer) {
	index, ok := s.index[field]
	if !ok {
		return
	}

	values, ok := index[key]
	if !ok {
		return
	}

	for i, value := range values {
		if indexer == value {
			n := len(values)
			values[i] = values[n-1]
			values = values[:n-1]
			return
		}
	}
}
