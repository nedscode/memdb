// Package memdb is designed to allow configurable indexing of values from a structure
package memdb

import (
	"github.com/google/btree"

	"sync"
)

// Store implements an indexed storage for an Indexer item
//
// Just like a real database, if you update an item such that it's index keys would change, you must Put it back in to
// update the items indexes in the database, and also to cause update notifications to be sent.
//
// DO NOT under any circumstances update the PRIMARY KEYs (ie keys used to determine the output of the Less()
// comparator) without first removing the existing item. Such an act would leave the item stranded in an unknown
// location within the index.
type Store struct {
	sync.RWMutex

	fields  []string
	backing *btree.BTree
	index   map[string]map[string][]Indexer
	used    bool

	insertNotifiers []NotifyFunc
	updateNotifiers []NotifyFunc
	removeNotifiers []NotifyFunc
	expiryNotifiers []NotifyFunc
}

// Event is a type of event emitted by the class, see the On() method
type Event int

const (
	// Insert Events happen when an item is inserted for the first time
	Insert Event = iota

	// Update Events happen when an existing item is replaced with an new item
	Update

	// Remove Events happen when an existing item is deleted
	Remove

	// Expiry Events happen when items are removed due to being expired
	Expiry
)

// NotifyFunc is an event receiver that gets called when events happen
type NotifyFunc func (event Event, old, new Indexer)

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

	found := s.backing.Get(&wrap{search, nil})
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

	s.backing.AscendGreaterOrEqual(&wrap{at, nil}, func(item btree.Item) bool {
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

	s.backing.DescendLessOrEqual(&wrap{at, nil}, func(item btree.Item) bool {
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
		old := s.rm(v)
		if old != nil {
			s.emit(Expiry, old, nil)
		}
	}
}

// Put places an indexer item into the store
func (s *Store) Put(indexer Indexer) Indexer {
	s.Lock()
	defer s.Unlock()

	old := s.add(indexer)
	if old == nil {
		s.emit(Insert, nil, indexer)
	} else {
		s.emit(Update, old, indexer)
	}
	return old
}

// Delete removes an item equal to the search item
func (s *Store) Delete(search Indexer) Indexer {
	s.Lock()
	defer s.Unlock()

	old := s.rm(search)
	if old != nil {
		s.emit(Remove, old, nil)
	}
	return old
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

// On registers an event handler for an event type
func (s *Store) On(event Event, notify NotifyFunc) {
	switch event {
	case Insert:
		s.insertNotifiers = append(s.insertNotifiers, notify)
	case Update:
		s.updateNotifiers = append(s.updateNotifiers, notify)
	case Remove:
		s.removeNotifiers = append(s.removeNotifiers, notify)
	case Expiry:
		s.expiryNotifiers = append(s.expiryNotifiers, notify)
	default:
		return
	}
}

func (s *Store) emit(event Event, old, new Indexer) {
	var handlers []NotifyFunc
	switch event {
	case Insert:
		handlers = s.insertNotifiers
	case Update:
		handlers = s.updateNotifiers
	case Remove:
		handlers = s.removeNotifiers
	case Expiry:
		handlers = s.expiryNotifiers
	default:
		return
	}

	if handlers != nil && len(handlers) > 0 {
		for _, handler := range handlers {
			handler(event, old, new)
		}
	}
}

func (s *Store) add(indexer Indexer) Indexer {
	// We store a clone of the indexer as it needs to be immutable
	s.used = true
	w := s.wrapIt(indexer)
	found := s.backing.ReplaceOrInsert(w)

	var ow *wrap
	if found != nil {
		ow = found.(*wrap)
	}

	for i, field := range s.fields {
		key := w.fields[i]
		if ow != nil {
			oldKey := ow.fields[i]
			if oldKey != key {
				s.rmFromIndex(field, oldKey, indexer)
				s.addToIndex(field, key, indexer)
			}
		} else {
			s.addToIndex(field, key, indexer)
		}
	}

	if ow != nil {
		return ow.indexer
	}
	return nil
}

func (s *Store) addToIndex(field, key string, indexer Indexer) {
	index, ok := s.index[field]
	if !ok {
		index = map[string][]Indexer{}
		s.index[field] = index
	}

	index[key] = append(index[key], indexer)
}

func (s *Store) rm(indexer Indexer) Indexer {
	removed := s.backing.Delete(&wrap{indexer, nil})
	if removed != nil {
		w := removed.(*wrap)
		for i, field := range s.fields {
			key := w.fields[i]
			s.rmFromIndex(field, key, indexer)
		}
	}

	if removed != nil {
		return removed.(*wrap).indexer
	}
	return nil
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

func (s *Store) wrapIt(indexer Indexer) *wrap {
	fields := make([]string, len(s.fields))
	for i, field := range s.fields {
		fields[i] = indexer.GetField(field)
	}

	return &wrap{
		indexer: indexer,
		fields: fields,
	}
}