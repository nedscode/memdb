package memdb

import (
	"github.com/google/btree"

	"fmt"
	"time"
)

type wrap struct {
	storer  Storer
	uid     UID
	item    interface{}
	values  []string
	fetched time.Time
	updated time.Time
	reads   uint64
	writes  uint64
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
