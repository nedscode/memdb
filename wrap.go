package memdb

import (
	"github.com/google/btree"

	"fmt"
)

type wrap struct {
	uid     UID
	indexer Indexer
	values  []string
}

// UID generates a unique UID for a wrap instance
func (w *wrap) UID() UID {
	if w.uid == "" {

		w.uid = NewUID()
	}

	return w.uid
}

func (w *wrap) Less(than btree.Item) bool {
	a := w.indexer
	if wb, ok := than.(*wrap); ok {
		return a.Less(wb.indexer)
	}
	return false
}

// Unsure calculates if one of different typed objects is less than another in an arbitrary but consistent way.
func Unsure(a interface{}, b interface{}) bool {
	return fmt.Sprintf("%#v", a) < fmt.Sprintf("%#v", b)
}
