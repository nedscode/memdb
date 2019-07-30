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

// Store implements Storer, indexed storage for various items
//
// Just like a real database, if you update an item such that it's index keys would change, you must Put it back in to
// update the items indexes in the database, and also to cause update notifications to be sent.
//
// DO NOT under any circumstances update the PRIMARY KEYs (ie keys used to determine the output of the Less()
// comparator) without first removing the existing item. Such an act would leave the item stranded in an unknown
// location within the index.
type Store struct {
	Storer
	sync.RWMutex

	backing *btree.BTree
	indexes map[string]*Index
	cIndex  *Index
	index   map[string]map[string][]*wrap
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
	accessNotifiers []NotifyFunc

	ticker *time.Ticker
}

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
	s.index = map[string]map[string][]*wrap{}
	s.indexes = map[string]*Index{}
	s.happens = happens

	go func() {
		for h := range happens {
			s.emit(h.event, h.old, h.new, h.stats)
		}
	}()

	go func() {
		// Give initial callers time to call ExpireInterval before we start the first tick
		time.Sleep(100 * time.Millisecond)

		// If there's no ticker set, create a default one
		if s.ticker == nil {
			// About 2.6 times per minute, shouldn't hit the same time every minute
			s.Lock()
			s.ticker = time.NewTicker(23272 * time.Millisecond)
			s.Unlock()
		}

		for range s.ticker.C {
			s.Expire()
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
func (s *Store) IsExpired(a interface{}, now time.Time, stats Stats) bool {
	if s.expirer != nil {
		return s.expirer.IsExpired(a, now, stats)
	}

	if ai, ok := a.(Expirable); ok {
		return ai.IsExpired(now, stats)
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
	s.CreateIndex(fields...)
	s.cIndex.unique = true
	return s
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

	var err error
	if metaPersister, ok := persister.(persist.MetaPersister); ok {
		err = metaPersister.MetaLoad(func(id string, item interface{}, meta *persist.Meta) {
			w := s.wrapIt(item)
			w.uid = UID(id)
			w.stats.Size = meta.Size
			s.addWrap(w)
		})
	} else {
		err = persister.Load(func(id string, item interface{}) {
			w := s.wrapIt(item)
			w.uid = UID(id)
			s.addWrap(w)
		})
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
		w.stats.read(time.Now())
		s.happens <- &happening{
			event: Access,
			old:   w.item,
			new:   w.item,
			stats: w.stats,
		}

		return w.item
	}

	return nil
}

// InPrimaryKey finds a the primary key index to perform queries upon
func (s *Store) InPrimaryKey() IndexSearcher {
	return s.In(s.primaryKey...)
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

// Info calls provided callback function from start (lowest order) of items until end or iterator function returns
// false, includes statistical information for all items in callback.
func (s *Store) Info(cb InfoIterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.AscendRange, nil, nil, s.cbWrap(cb))
}

// Ascend calls provided callback function from start (lowest order) of items until end or iterator function returns
// false
func (s *Store) Ascend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.AscendRange, nil, nil, s.cbWrap(cb))
}

// AscendStarting calls provided callback function from item equal to at until end or iterator function returns false
func (s *Store) AscendStarting(at interface{}, cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.AscendRange, &wrap{storer: s, item: at}, nil, s.cbWrap(cb))
}

// Descend calls provided callback function from end (highest order) of items until start or iterator function returns
// false
func (s *Store) Descend(cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.DescendRange, nil, nil, s.cbWrap(cb))
}

// DescendStarting calls provided callback function from item equal to at until start or iterator function returns false
func (s *Store) DescendStarting(at interface{}, cb Iterator) {
	s.RLock()
	defer s.RUnlock()
	traverse(s.backing.DescendRange, &wrap{storer: s, item: at}, nil, s.cbWrap(cb))
}

// ExpireInterval allows setting of a new auto-expire interval (after the current one ticks)
func (s *Store) ExpireInterval(interval time.Duration) {
	s.Lock()
	defer s.Unlock()
	s.ticker = time.NewTicker(interval)
}

// Expire finds all expiring items in the store and deletes them
func (s *Store) Expire() int {
	rm := s.findExpired()

	s.Lock()
	defer s.Unlock()

	for _, wrapped := range rm {
		old, _ := s.rm(wrapped)
		if old != nil {
			s.happens <- &happening{
				event: Expiry,
				old:   old.item,
				stats: old.stats,
			}
		}
	}

	return len(rm)
}

// PutAll places multiple items into the store on a single lock
func (s *Store) PutAll(items []interface{}) error {
	s.Lock()
	defer s.Unlock()

	errs := 0
	for _, item := range items {
		newWrap, oldWrap, err := s.add(item)

		if oldWrap == nil {
			s.happens <- &happening{
				event: Insert,
				new:   item,
				stats: newWrap.stats,
			}
		} else if oldWrap != none {
			s.happens <- &happening{
				event: Update,
				old:   oldWrap.item,
				new:   item,
				stats: newWrap.stats,
			}
		}

		if err != nil {
			errs++
		}
	}

	if errs > 0 {
		return fmt.Errorf("%d errors occurred during operation", errs)
	}
	return nil
}

// Put places an item into the store, returns the old replaced item (if any)
func (s *Store) Put(item interface{}) (old interface{}, err error) {
	s.Lock()
	defer s.Unlock()

	var newWrap, oldWrap *wrap
	newWrap, oldWrap, err = s.add(item)

	if oldWrap == nil {
		s.happens <- &happening{
			event: Insert,
			new:   item,
			stats: newWrap.stats,
		}
	} else if oldWrap != none {
		old = oldWrap.item
		s.happens <- &happening{
			event: Update,
			old:   old,
			new:   item,
			stats: newWrap.stats,
		}
	}
	return
}

// Delete removes an item equal to the search item, returns the deleted item (if any)
func (s *Store) Delete(search interface{}) (old interface{}, err error) {
	s.Lock()
	defer s.Unlock()

	var oldWrap *wrap
	oldWrap, err = s.rm(search)
	if oldWrap != nil {
		old = oldWrap.item
		s.happens <- &happening{
			event: Remove,
			old:   old,
			stats: oldWrap.stats,
		}
	}
	return
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

type IndexStats struct {
	Key   []string
	Count uint64
	Size  uint64
}

// IndexStats returns the list of distinct keys for an index along with stats of the items held.
// The Size field represents stored (on disk) size of items, if using a persister, and will be 0 otherwise.
func (s *Store) IndexStats(fields ...string) []*IndexStats {
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

	keys := make([]*IndexStats, len(index))
	i := 0
	for key, wraps := range index {
		var size uint64
		if _, ok := s.persister.(persist.MetaPersister); ok {
			for _, wrap := range wraps {
				size += wrap.stats.Size
			}
		}
		keys[i] = &IndexStats{
			Key: strings.Split(key, "\000"),
			Count: uint64(len(wraps)),
			Size: size,
		}
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
	case Access:
		s.accessNotifiers = append(s.accessNotifiers, notify)
	default:
		return
	}
}

func (s *Store) findExpired() []*wrap {
	s.RLock()
	defer s.RUnlock()
	now := time.Now()

	var rm []*wrap
	s.backing.Ascend(func(item btree.Item) bool {
		if w, ok := item.(*wrap); ok {
			// TODO - Possible lock contention here if this calls any store functions
			if s.IsExpired(w.item, now, w.stats) {
				rm = append(rm, w)
			}
		}
		return true
	})

	return rm
}

func (s *Store) emit(event Event, old, new interface{}, stats Stats) {
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
	case Access:
		handlers = s.accessNotifiers
	default:
		return
	}

	if len(handlers) > 0 {
		for _, handler := range handlers {
			handler(event, old, new, stats)
		}
	}
}

func (s *Store) add(item interface{}) (*wrap, *wrap, error) {
	w := s.wrapIt(item)
	ret := s.addWrap(w)

	var err error
	if s.persister != nil {
		id := string(w.UID())
		if metaPersister, ok := s.persister.(persist.MetaPersister); ok {
			var meta *persist.Meta
			meta, err = metaPersister.MetaSave(id, item)
			w.stats.Size = meta.Size
		} else {
			err = s.persister.Save(id, item)
		}
	}

	return w, ret, err
}

func (s *Store) addWrap(w *wrap) *wrap {
	s.used = true
	w.UID()
	found := s.backing.ReplaceOrInsert(w)

	var ow *wrap
	if found != nil {
		ow = found.(*wrap)
		w.stats = ow.stats
	}

	w.stats.written(time.Now())

	var emitted bool
	for _, index := range s.indexes {
		key := w.values[index.n]
		if ow != nil {
			oldKey := ow.values[index.n]
			s.rmFromIndex(index.id, oldKey, ow)
		}
		emitted = s.addToIndex(index.id, key, w)
	}

	if ow != nil {
		return ow
	}
	if emitted {
		return none
	}
	return nil
}

func (s *Store) addToIndex(indexID string, key string, wrapped *wrap) (emitted bool) {
	index, ok := s.indexes[indexID]
	if !ok {
		return
	}

	indexWraps, ok := s.index[indexID]
	if !ok {
		indexWraps = map[string][]*wrap{}
		s.index[indexID] = indexWraps
	}

	wraps := indexWraps[key]
	if index.unique && len(wraps) > 0 {
		// Items have been replaced!
		for _, indexWrap := range indexWraps[key] {
			rm, _ := s.rm(indexWrap)
			if rm != nil {
				s.happens <- &happening{
					event: Update,
					old:   rm.item,
					new:   wrapped.item,
					stats: wrapped.stats,
				}
				emitted = true
			}
		}
		wraps = nil
	}
	indexWraps[key] = append(wraps, wrapped)
	return
}

func (s *Store) rm(item interface{}) (*wrap, error) {
	var search *wrap

	if wrapped, ok := item.(*wrap); ok {
		search = wrapped
	} else {
		search = &wrap{storer: s, item: item}
	}
	removed := s.backing.Delete(search)

	var err error
	if removed != nil {
		w := removed.(*wrap)
		if s.persister != nil {
			err = s.persister.Remove(string(w.UID()))
		}

		for _, index := range s.indexes {
			key := w.values[index.n]
			s.rmFromIndex(index.id, key, w)
		}
	}

	if removed != nil {
		return removed.(*wrap), err
	}
	return nil, err
}

func (s *Store) rmFromIndex(indexID string, key string, wrapped *wrap) {
	indexWraps, ok := s.index[indexID]
	if !ok {
		return
	}

	wraps, ok := indexWraps[key]
	if !ok {
		return
	}

	for i, wrap := range wraps {
		if wrapped == wrap {
			n := len(wraps)
			if n == 1 && i == 0 {
				indexWraps[key] = nil
				return
			}
			wraps[i] = wraps[n-1]
			indexWraps[key] = wraps[:n-1]
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
	if wrapped, ok := item.(*wrap); ok {
		return wrapped
	}

	values := make([]string, len(s.indexes))
	for _, index := range s.indexes {
		values[index.n] = s.getIndexValue(item, index)
	}

	now := time.Now()
	w := &wrap{
		storer: s,
		item:   item,
		values: values,
	}
	w.stats = Stats{
		w:        w,
		Created:  now,
		Modified: now,
	}
	return w
}

func (s *Store) cbWrap(cb interface{}) btree.ItemIterator {
	now := time.Now()
	return func(i btree.Item) bool {
		if w, ok := i.(*wrap); ok {
			w.stats.read(now)
			if iterator, ok := cb.(Iterator); ok {
				s.happens <- &happening{
					event: Access,
					old:   w.item,
					new:   w.item,
					stats: w.stats,
				}
				return iterator(w.item)
			} else if info, ok := cb.(InfoIterator); ok {
				return info(w.uid, w.item, w.stats)
			}
		}
		return true
	}
}

func traverse(traverse func(btree.Item, btree.Item, btree.ItemIterator), a, b btree.Item, iterator btree.ItemIterator) {
	traverse(a, b, iterator)
}
