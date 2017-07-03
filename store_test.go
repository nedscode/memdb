package memdb

import (
	"testing"

	"github.com/google/btree"
	"strings"
	"sort"
)

var expired = 0

type X struct {
	a int
	b string
}

func (x *X) Less(o Indexer) bool {
	return x.a < o.(*X).a
}
func (x *X) IsExpired() bool {
	return x.a == expired
}
func (x *X) GetField(f string) string {
	return x.b
}

func TestCreateField(t *testing.T) {
	s := NewStore()
	s.CreateField("test")

	f := s.Fields()
	if len(f) != 1 {
		t.Errorf("Fields length should be 1 (is %d)", len(f))
	}
	if f[0] != "test" {
		t.Errorf("Fields should be []string{\"test\"} (is: %#v)", f)
	}
}

func TestCreateAfterStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	s := NewStore()
	s.Put(&X{})
	s.CreateField("b")
}

func TestGet(t *testing.T) {
	s := NewStore()
	orig := &X{a: 1}
	s.Put(orig)
	v := s.Get(&X{a: 1})
	if orig != v {
		t.Errorf("Gotten value should be same as original instance")
	}
}

func TestNoGet(t *testing.T) {
	s := NewStore()
	orig := &X{a: 1}
	s.Put(orig)
	v := s.Get(&X{a: 2})
	if v != nil {
		t.Errorf("Gotten value should be nil (not present)")
	}
}

func TestLookup(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	s.Put(&X{a: 2, b: "test"})
	s.Put(&X{a: 3, b: "not"})
	vals := s.Lookup("b", "test")
	if len(vals) != 2 {
		t.Errorf("Length of looked up values should be 2 (was %s)", len(vals))
	}
	a := vals[0].(*X)
	b := vals[1].(*X)
	if (a.a == 1 && b.a == 2) || (a.a == 2 && b.a == 1) {
		return
	}
	t.Errorf("Expected to return 1 and 2 (got %#v)", vals)
}

func TestLookupInvalidField(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.Lookup("c", "test")
	if vals != nil {
		t.Errorf("Lookup of invalid field should be nil (was %#v)", vals)
	}
}

func TestLookupNonPresentKey(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.Lookup("b", "dumb")
	if vals != nil {
		t.Errorf("Lookup of non-present key should be nil (was %#v)", vals)
	}
}

func TestTraverse(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	v1 := &X{a: 1, b: "one:"}
	v2 := &X{a: 2, b: "two:"}
	v3 := &X{a: 3, b: "three:"}
	v4 := &X{a: 4, b: "four:"}
	v8 := &X{a: 8, b: "eight:"}

	s.Put(v1)
	s.Put(v2)
	s.Put(v4)
	s.Put(v8)

	n := s.Len()
	if n != 4 {
		t.Errorf("Expected 4 items in length (got %d)", n)
	}

	k := s.Keys("b")
	if len(k) != 4 {
		t.Errorf("Expected 4 items in keys for field (got %#v)", k)
	}

	sort.Strings(k)
	j := strings.Join(k, "")
	if j != "eight:four:one:two:" {
		t.Errorf("Unexpected items in keys for field (got %s)", j)
	}

	var got, expect string
	var stop *X

	iter := func(i Indexer) bool {
		got += i.(*X).b
		return i != stop
	}

	got = ""
	s.Ascend(iter)
	expect = "one:two:four:eight:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.Descend(iter)
	expect = "eight:four:two:one:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.DescendStarting(v3, iter)
	expect = "two:one:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.AscendStarting(v3, iter)
	expect = "four:eight:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v2
	s.Ascend(iter)
	expect = "one:two:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v4
	s.Descend(iter)
	expect = "eight:four:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v4
	s.AscendStarting(v3, iter)
	expect = "four:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v2
	s.DescendStarting(v3, iter)
	expect = "two:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	expired = 4
	s.Expire()

	got = ""
	stop = nil
	s.Ascend(iter)
	expect = "one:two:eight:"
	if got != expect {
		t.Errorf("Expired item not removed expected %s (got %s)", expect, got)
	}

	vals := s.Lookup("b", "four")
	if vals != nil {
		t.Errorf("Expired item found by field (got %#v)", vals)
	}

	s.Delete(v2)

	got = ""
	s.Ascend(iter)
	expect = "one:eight:"
	if got != expect {
		t.Errorf("Deleted item not removed expected %s (got %s)", expect, got)
	}
}

func TestUnsure(t *testing.T) {
	if Unsure("A", "Z") != true {
		t.Errorf("Expected A to be < Z")
	}
}

func TestLess(t *testing.T) {
	v1 := &wrap{&X{a: 1, b: "one:"}}
	vx := btree.Int(5)

	if v1.Less(vx) {
		t.Errorf("Comparison with non-Indexer item should be false")
	}
}
