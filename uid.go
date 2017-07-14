package memdb

import (
	"math"
	"math/rand"
	"time"
)

// UID is a unique ID generated from a timestamp and random entropy
type UID string

// NewUID creates a new UID that you can use for a wrapped Indexer or anything else
func NewUID() UID {
	safeChars := "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz"

	var (
		now   = float64(time.Now().UnixNano())
		n     = len(safeChars)
		scale = float64(n)
		week  = float64(86400000000000 * 7)
		weeks = math.Floor(now / week)
		ofs   = now - weeks*week
		id    = make([]byte, 12)
	)

	id[0] = safeChars[int64(weeks/scale)%int64(scale)]
	id[1] = safeChars[int64(weeks)%int64(scale)]

	for i := 2; i < 7; i++ {
		r := math.Floor(ofs / week * scale)
		ofs -= r * week / scale
		scale *= float64(n)
		id[i] = safeChars[int64(r)]
	}

	for i := 7; i < 12; i++ {
		id[i] = safeChars[rand.Int31n(int32(n))]
	}

	return UID(id)
}
