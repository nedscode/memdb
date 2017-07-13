package memdb

import (
	"github.com/google/btree"

	"fmt"
	"math"
	"math/rand"
	"time"
)

type wrap struct {
	id      string
	indexer Indexer
	values  []string
}

// ID generates a unique ID for a wrap instance
func (w *wrap) ID() string {
	if w.id == "" {
		safeChars := "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz"

		var (
			now   = float64(time.Now().UnixNano()) / float64(time.Millisecond)
			n     = len(safeChars)
			scale = float64(n)
			week  = float64(86400000 * 7)
			weeks = math.Floor(now / week)
			ofs   = now - weeks*week
			id    = make([]byte, 12)
		)

		id[0] = safeChars[int64(weeks/scale)%int64(scale)]
		id[1] = safeChars[int64(weeks)%int64(scale)]

		for i := 1; i < 3; i++ {
			r := ofs / week * scale
			ofs -= r * week / scale
			scale *= float64(n)
			id[i+1] = safeChars[int64(r)]
		}
		for i := 4; i < 12; i++ {
			id[i] = safeChars[rand.Int31n(int32(n-1))]
		}
		w.id = string(id)
	}

	return w.id
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
