// Package memdb is designed to allow configurable indexing of values from a structure
package memdb

import (
	"github.com/google/btree"
	"github.com/nedscode/memdb/persist"

	"fmt"
	"strings"
	"sync"
	"time"
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

// Expirer can determine if an item is expired given a current time, last fetched
// and last updated time
type Expirer interface {
	IsExpired(a interface{}, now, fetched, updated time.Time) bool
}

// Fielder can get the string value for a given item's named field
type Fielder interface {
	GetField(a interface{}, field string) string
}

// Storer provides the functionality of a memdb store.
type Storer interface {
	Indexer
	SetIndexer(indexer Indexer)
	SetComparator(comparator Comparator)
	SetExpirer(expirer Expirer)
	SetFielder(fielder Fielder)

	PrimaryKey(fields ...string) *Store
	CreateIndex(fields ...string) *Store
	Unique() *Store
	Reversed(order ...bool) *Store

	Persistent(persister persist.Persister) error

	Get(search interface{}) interface{}
	Put(indexer interface{}) (interface{}, error)
	Delete(search interface{}) (interface{}, error)

	In(fields ...string) IndexSearcher
	Ascend(cb Iterator)
	AscendStarting(at interface{}, cb Iterator)
	Descend(cb Iterator)
	DescendStarting(at interface{}, cb Iterator)

	Expire() int

	Len() int
	Indexes() [][]string
	Keys(fields ...string) []string

	On(event Event, notify NotifyFunc)
}

// IndexSearcher can return results from an index
type IndexSearcher interface {
	Each(cb Iterator, keys ...string)
	One(keys ...string) interface{}
	Lookup(keys ...string) []interface{}
	_id() string
}

// Store implements Storer, indexed storage for various items
//
// Just like a real database, if you update an item such that it's index keys would change, you must Put it back in to
// update the items indexes in the database, and also to cause update notifications to be sent.
//
// DO NOT under any circumstances update the PRIMARY KEYs (ie keys used to determine the output of the Less()
// comparator) without first removing the existing item. Such an act would leave the item stranded in an unknown
// location within the index.
type Store struct {
	sync.RWMutex

	backing *btree.BTree
	indexes map[string]*Index
	cIndex  *Index
	index   map[string]map[string][]interface{}
	happens chan *happening
	used    bool

	primaryKey []string
	reversed   bool
	comparator Comparator
	expirer    Expirer
	fielder    Fielder

	persister persist.Persister

	insertNotifiers []NotifyFunc
	updateNotifiers []NotifyFunc
	removeNotifiers []NotifyFunc
	expiryNotifiers []NotifyFunc
}

type happening struct {
	event Event
	old   interface{}
	new   interface{}
}

// Index represent a list of indexes
type Index struct {
	n      int
	id     string
	fields []string
	store  *Store
	unique bool
}

// Event is a type of event emitted by the class, see the On() method
type Event int

// String describes the event type
func (e Event) String() string {
	switch e {
	case Insert:
		return "Insert event"
	case Update:
		return "Update event"
	case Remove:
		return "Remove event"
	case Expiry:
		return "Expiry event"
	default:
		break
	}
	return "Unknown event"
}

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

type noIndexer struct{}

func (x *noIndexer) Less(_ interface{}) bool {
	return true
}
func (x *noIndexer) IsExpired() bool {
	return false
}
func (x *noIndexer) GetField(_ string) string {
	return ""
}

var (
	none = &noIndexer{}
)

// NotifyFunc is an event receiver that gets called when events happen
type NotifyFunc func(event Event, old, new interface{})

// NewStore returns an initialized store for you to use
func NewStore() Storer {
	s := &Store{}
	s.Init()
	return s
}

// Init will initialize a store
func (s *Store) Init() {
	if s.happens != nil {
		return
	}

	happens := make(chan *happening, 100000)

	s.backing = btree.New(2)
	s.index = map[string]map[string][]interface{}{}
	s.indexes = map[string]*Index{}
	s.happens = happens

	go func() {
		for h := range happens {
			s.emit(h.event, h.old, h.new)
		}
	}()
}

// Less is a comparator function that checks if one item is less than another
func (s *Store) Less(a interface{}, b interface{}) bool {
	less := func() bool {
		if s.comparator != nil {
			return s.comparator.Less(a, b)
		}

		if ai, ok := a.(Indexable); ok {
			if bi, ok := b.(Indexable); ok {
				return ai.Less(bi)
			}
		}

		if len(s.primaryKey) > 0 {
			aid := s.getFieldsValue(a, s.primaryKey)
			bid := s.getFieldsValue(b, s.primaryKey)
			return aid < bid
		}

		// Arbitrary ordering
		return Unsure(a, b)
	}()

	if s.reversed {
		return !less
	}
	return less
}

// IsExpired is an expirer function that checks if an item should be expired out of the store
func (s *Store) IsExpired(a interface{}, now, fetched, updated time.Time) bool {
	if s.expirer != nil {
		return s.expirer.IsExpired(a, now, fetched, updated)
	}

	if ai, ok := a.(Indexable); ok {
		return ai.IsExpired(now, fetched, updated)
	}

	// Never expire
	return false
}

// GetField is a fielder function that returns a string value for a field name
func (s *Store) GetField(a interface{}, field string) string {
	if s.fielder != nil {
		return s.fielder.GetField(a, field)
	}

	if ai, ok := a.(Indexable); ok {
		return ai.GetField(field)
	}

	path := strings.Split(field, ".")
	return reflective(a, path)
}

// SetIndexer sets the comparator, expirer and fielder for this store
// If you override the default comparator, the Store's primary key will no longer determine item ordering
func (s *Store) SetIndexer(indexer Indexer) {
	s.comparator = indexer
	s.expirer = indexer
	s.fielder = indexer
}

// SetComparator sets just the comparator for this store
// If you override the default comparator, the Store's primary key will no longer determine item ordering
func (s *Store) SetComparator(comparator Comparator) {
	s.comparator = comparator
}

// SetExpirer sets just the expirer for this store
func (s *Store) SetExpirer(expirer Expirer) {
	s.expirer = expirer
}

// SetFielder sets just the fielder for this store
func (s *Store) SetFielder(fielder Fielder) {
	s.fielder = fielder
}

// PrimaryKey sets the primary key for this store, will not work if a custom comparator is being used
func (s *Store) PrimaryKey(fields ...string) *Store {
	if s.used {
		panic("Cannot change primary key on in-use store")
	}

	s.primaryKey = fields
	return s.CreateIndex(fields...)
}

// Reversed flips the meaning of the comparator
// Can supply an optional boolean value to set reversal order, or if unspecified, sets to true
// Effectively this swaps the insert order of the store, so that less items are stored after greater items
func (s *Store) Reversed(order ...bool) *Store {
	if s.used {
		panic("Cannot change store order on in-use store")
	}

	if len(order) > 0 {
		s.reversed = order[0]
	} else {
		s.reversed = true
	}

	return s
}

// CreateIndex adds a new index to the list of indexes before the store is populated
func (s *Store) CreateIndex(fields ...string) *Store {
	if s.used {
		panic("Cannot create index on in-use store")
	}

	id := strings.Join(fields, "\000")
	index := &Index{
		n:      len(s.indexes),
		id:     id,
		fields: fields,
		store:  s,
	}
	s.indexes[id] = index
	s.cIndex = index
	return s
}

// Unique makes the current index unique
// Making an index unique will force the delete of all but the last inserted item in the index upon Put()
func (s *Store) Unique() *Store {
	if s.used {
		panic("Cannot create index on in-use store")
	}
	if s.cIndex != nil {
		s.cIndex.unique = true
	}
	return s
}

// Persistent adds a persister to the database and loads up the existing records, call after all indexes are setup but
// before you begin using it.
func (s *Store) Persistent(persister persist.Persister) error {
	if s.used {
		panic("Cannot make persist on in-use store")
	}

	s.used = true
	s.persister = persister

	s.Lock()
	defer s.Unlock()

	var loaderErr error
	err := persister.Load(func(id string, indexer interface{}) {
		if idx, ok := indexer.(Indexable); ok {
			w := s.wrapIt(idx)
			w.uid = UID(id)
			s.addWrap(w)
		} else {
			loaderErr = fmt.Errorf("Error converting item %T to Indexer", indexer)
		}
	})

	if err == nil {
		err = loaderErr
	}

	return err
}

// Get returns an item equal to the passed item from the store
func (s *Store) Get(search interface{}) interface{} {
	s.RLock()
	defer s.RUnlock()

	found := s.backing.Get(&wrap{
		storer: s,
		item:   search,
	})
	if found == nil {
		return nil
	}

	if w, ok := found.(*wrap); ok {
		w.fetched = time.Now()
		w.reads++
		return w.item
	}

	return nil
}

// In finds a simple or compound index to perform queries upon
func (s *Store) In(fields ...string) IndexSearcher {
	s.RLock()
	defer s.RUnlock()

	id := strings.Join(fields, "\000")
	if f, ok := s.indexes[id]; ok {
		return f
	}

	var idx *Index
	return idx
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
	for _, indexer := range values {
		if !cb(indexer) {
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
		return values[0]
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
	c := make([]interface{}, len(values))
	copy(c, values)
	return c
}

func (idx *Index) _id() string {
	if idx == nil {
		return ""
	}

	return idx.id
}

func (idx *Index) find(keys []string) []interface{} {
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

func cbWrap(cb Iterator) btree.ItemIterator {
	return func(i btree.Item) bool {
		if w, ok := i.(*wrap); ok {
			return cb(w.item)
		}
		return true
	}
}

func traverse(traverse func(btree.Item, btree.Item, btree.ItemIterator), a, b btree.Item, iterator btree.ItemIterator) {
	traverse(a, b, iterator)
}

// Ascend calls provided callback function from start (lowest order) of items until end or iterator function returns
// false
func (s *Store) Ascend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.AscendRange, nil, nil, cbWrap(cb))
}

// AscendStarting calls provided callback function from item equal to at until end or iterator function returns false
func (s *Store) AscendStarting(at interface{}, cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.AscendRange, &wrap{storer: s, item: at}, nil, cbWrap(cb))
}

// Descend calls provided callback function from end (highest order) of items until start or iterator function returns
// false
func (s *Store) Descend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.DescendRange, nil, nil, cbWrap(cb))
}

// DescendStarting calls provided callback function from item equal to at until start or iterator function returns false
func (s *Store) DescendStarting(at interface{}, cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.DescendRange, &wrap{storer: s, item: at}, nil, cbWrap(cb))
}

func (s *Store) findExpired() []interface{} {
	s.RLock()
	defer s.RUnlock()
	now := time.Now()

	var rm []interface{}
	s.backing.Ascend(func(item btree.Item) bool {
		if w, ok := item.(*wrap); ok {
			if s.IsExpired(w.item, now, w.fetched, w.updated) {
				rm = append(rm, w.item)
			}
		}
		return true
	})

	return rm
}

// Expire finds all expiring items in the store and deletes them
func (s *Store) Expire() int {
	rm := s.findExpired()

	s.Lock()
	defer s.Unlock()

	for _, v := range rm {
		old, _ := s.rm(v)
		if old != nil {
			s.happens <- &happening{
				event: Expiry,
				old:   old,
			}
		}
	}

	return len(rm)
}

// Put places an indexer item into the store
func (s *Store) Put(indexer interface{}) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	old, err := s.add(indexer)

	if old == nil {
		s.happens <- &happening{
			event: Insert,
			new:   indexer,
		}
	} else if old != none {
		s.happens <- &happening{
			event: Update,
			old:   old,
			new:   indexer,
		}
	}

	return old, err
}

// Delete removes an item equal to the search item
func (s *Store) Delete(search interface{}) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	old, err := s.rm(search)
	if old != nil {
		s.happens <- &happening{
			event: Remove,
			old:   old,
		}
	}
	return old, err
}

// Len returns the number of items in the database
func (s *Store) Len() int {
	s.RLock()
	defer s.RUnlock()

	return s.backing.Len()
}

// Indexes returns the list of indexed indexes
func (s *Store) Indexes() [][]string {
	s.RLock()
	defer s.RUnlock()

	c := make([][]string, len(s.indexes))
	for _, f := range s.indexes {
		fc := make([]string, len(f.fields))
		copy(fc, f.fields)
		c[f.n] = fc
	}
	return c
}

// Keys returns the list of distinct keys for an index
func (s *Store) Keys(fields ...string) []string {
	f := s.In(fields...)
	if f == nil {
		return nil
	}

	s.RLock()
	defer s.RUnlock()

	index, ok := s.index[f._id()]
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

func (s *Store) emit(event Event, old, new interface{}) {
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

	if len(handlers) > 0 {
		for _, handler := range handlers {
			handler(event, old, new)
		}
	}
}

func (s *Store) add(indexer interface{}) (interface{}, error) {
	w := s.wrapIt(indexer)
	ret := s.addWrap(w)

	var err error
	if s.persister != nil {
		err = s.persister.Save(string(w.UID()), indexer)
	}

	return ret, err
}

func (s *Store) addWrap(w *wrap) interface{} {
	s.used = true
	found := s.backing.ReplaceOrInsert(w)

	var ow *wrap
	if found != nil {
		ow = found.(*wrap)
		w.fetched = ow.fetched
		w.writes = ow.writes
	}
	w.updated = time.Now()
	w.writes++

	var emitted bool
	for _, index := range s.indexes {
		key := w.values[index.n]
		if ow != nil {
			oldKey := ow.values[index.n]
			if oldKey != key {
				s.rmFromIndex(index.id, oldKey, ow.item)
				emitted = s.addToIndex(index.id, key, w.item)
			}
		} else {
			emitted = s.addToIndex(index.id, key, w.item)
		}
	}

	if ow != nil {
		return ow.item
	}
	if emitted {
		return none
	}
	return nil
}

func (s *Store) addToIndex(indexID string, key string, indexer interface{}) (emitted bool) {
	index, ok := s.indexes[indexID]
	if !ok {
		return
	}

	indexItems, ok := s.index[indexID]
	if !ok {
		indexItems = map[string][]interface{}{}
		s.index[indexID] = indexItems
	}

	items := indexItems[key]
	if index.unique && len(items) > 0 {
		// Items have been replaced!
		for _, item := range indexItems[key] {
			rm, _ := s.rm(item)
			if rm != nil {
				s.happens <- &happening{
					event: Update,
					old:   rm,
					new:   indexer,
				}
				emitted = true
			}
		}
		items = nil
	}
	indexItems[key] = append(items, indexer)
	return
}

func (s *Store) rm(item interface{}) (interface{}, error) {
	removed := s.backing.Delete(&wrap{storer: s, item: item})

	var err error
	if removed != nil {
		w := removed.(*wrap)
		if s.persister != nil {
			err = s.persister.Remove(string(w.UID()))
		}

		for _, index := range s.indexes {
			key := w.values[index.n]
			s.rmFromIndex(index.id, key, w.item)
		}
	}

	if removed != nil {
		return removed.(*wrap).item, err
	}
	return nil, err
}

func (s *Store) rmFromIndex(indexID string, key string, item interface{}) {
	index, ok := s.index[indexID]
	if !ok {
		return
	}

	values, ok := index[key]
	if !ok {
		return
	}

	for i, value := range values {
		if item == value {
			n := len(values)
			if n == 1 && i == 0 {
				index[key] = nil
				return
			}
			values[i] = values[n-1]
			index[key] = values[:n-1]
			return
		}
	}
}

func (s *Store) getIndexValue(item interface{}, index *Index) string {
	return s.getFieldsValue(item, index.fields)
}

func (s *Store) getFieldsValue(item interface{}, fields []string) string {
	components := make([]string, len(fields))
	for i, field := range fields {
		components[i] = s.GetField(item, field)
	}
	return strings.Join(components, "\000")
}

func (s *Store) wrapIt(item interface{}) *wrap {
	values := make([]string, len(s.indexes))
	for _, index := range s.indexes {
		values[index.n] = s.getIndexValue(item, index)
	}

	return &wrap{
		storer:  s,
		item:    item,
		values:  values,
		updated: time.Now(),
	}
}
