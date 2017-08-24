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

// FieldKey represents the key for an item within a field
type FieldKey []string

// NewFieldKey returns a FieldKey from a field representation string [ FieldKey.String() ]
func NewFieldKey(from string) FieldKey {
	return FieldKey(strings.Split(from, "\000"))
}

// Keys are the keys contained in the FieldKey
// It can be used like store.In("field").One(fieldKey.Keys()...)
func (fk FieldKey) Keys() []string {
	return []string(fk)
}

// String returns a representation string for the FieldKey [ can supply to NewFieldKey() ]
func (fk FieldKey) String() string {
	return strings.Join(fk.Keys(), "\000")
}

// FieldKey returns the used key value for the given item for this index
func (idx *Index) FieldKey(a interface{}) FieldKey {
	components := make([]string, len(idx.fields))
	for i, field := range idx.fields {
		components[i] = idx.store.GetField(a, field)
	}
	return FieldKey(components)
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

// All returns the all items from the index
func (idx *Index) All() []interface{} {
	if idx == nil {
		return nil
	}

	idx.store.RLock()
	defer idx.store.RUnlock()

	done := map[string]bool{}
	items := []interface{}{}
	if index, ok := idx.store.index[idx.id]; ok {
		for _, idx := range index {
			for _, wrap := range idx {
				uid := wrap.uid.String()
				if d, ok := done[uid]; !ok || !d {
					items = append(items, wrap.item)
					done[uid] = true
				}
			}
		}
	}

	return items
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
