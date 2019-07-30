package memdb

import (
	"github.com/google/btree"

	"fmt"
	"sync"
	"time"
)

// Stats contains the access statistics for a stored item
type Stats struct {
	Created  time.Time
	Accessed time.Time
	Modified time.Time
	Reads    uint64
	Writes   uint64
	Size     uint64
	w        *wrap
}

func (s *Stats) read(t time.Time) {
	s.w.Lock()
	defer s.w.Unlock()

	s.Accessed = t
	s.Reads++
}

func (s *Stats) written(t time.Time) {
	s.w.Lock()
	defer s.w.Unlock()

	s.Modified = t
	s.Writes++
}

func (s *Stats) set(from Stats) {
	s.w.Lock()
	defer s.w.Unlock()

	s.Created = from.Created
	s.Accessed = from.Accessed
	s.Modified = from.Modified
	s.Reads = from.Reads
	s.Writes = from.Writes
	s.Size = from.Size
}

// IsZero returns whether the statistic has an item or not
func (s *Stats) IsZero() bool {
	return s.w == nil
}

type wrap struct {
	sync.Mutex

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
