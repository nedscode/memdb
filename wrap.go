package memdb

import (
	"github.com/google/btree"

	"fmt"
	"sync"
	"time"
)

type Stats struct {
	sync.Mutex
	Created  time.Time
	Accessed time.Time
	Modified time.Time
	Reads    uint64
	Writes   uint64
}

func (s Stats) read(t time.Time) {
	s.Lock()
	defer s.Unlock()

	s.Accessed = t
	s.Reads++
}

func (s Stats) written(t time.Time) {
	s.Lock()
	defer s.Unlock()

	s.Modified = t
	s.Writes++
}

func (s Stats) set(from Stats) {
	s.Lock()
	defer s.Unlock()

	s.Created = from.Created
	s.Accessed = from.Accessed
	s.Modified = from.Modified
	s.Reads = from.Reads
	s.Writes = from.Writes
}

type wrap struct {
	storer Storer
	uid    UID
	item   interface{}
	values []string
	stats  Stats
}

// UID generates a unique UID for a wrap instance
func (w *wrap) UID() UID {
	if w.uid == "" {
		w.uid = NewUID()
	}

	return w.uid
}

func (w *wrap) Less(than btree.Item) bool {
	a := w.item
	if wb, ok := than.(*wrap); ok {
		return w.storer.Less(a, wb.item)
	}
	return false
}

// Unsure calculates if one of different typed objects is less than another in an arbitrary but consistent way.
func Unsure(a interface{}, b interface{}) bool {
	return fmt.Sprintf("%#v", a) < fmt.Sprintf("%#v", b)
}
