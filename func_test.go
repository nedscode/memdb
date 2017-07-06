package memdb

import (
	"github.com/google/btree"

	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
	"flag"
	"context"
)

var expired = 0

type X struct {
	a int
	b string
	c string
}

var (
	sim bool
	qseed int
)

func init() {
	flag.BoolVar(&sim, "simulate", sim, "Simulate with black box test")
	flag.IntVar(&qseed, "seed", 0, "Seed for randomiser")
	flag.Parse()

	if !sim {
		fmt.Println("Will skip longer simulations as -simulate was not specified.")
	}
	if qseed == 0 {
		qseed = int(time.Now().UnixNano())%100000
		fmt.Printf("Performing simulation pattern %d. To repeat, rerun with -seed %d\n", qseed, qseed)
	}

}

func (x *X) Less(o Indexer) bool {
	return x.a < o.(*X).a
}
func (x *X) IsExpired() bool {
	if expired < 0 {
		return time.Now().UnixNano() % 1000000 > 995000
	}
	return x.a == expired
}
func (x *X) GetField(f string) string {
	if f == "c" {
		return x.c
	}
	return x.b
}

func TestCreateField(t *testing.T) {
	s := NewStore()
	s.CreateIndex("test")

	f := s.Indexes()
	if len(f) != 1 {
		t.Errorf("Index length should be 1 (is %d)", len(f))
	}
	if f[0] == nil || len(f[0]) != 1 || f[0][0] != "test" {
		t.Errorf("Index should be []string{\"test\"} (is: %#v)", f)
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
	s.CreateIndex("b")
}

func TestUniqueAfterStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	s := NewStore()
	s.CreateIndex("b")
	s.Put(&X{})
	s.Unique()
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
	s.CreateIndex("b")
	s.Put(&X{a: 1, b: "test"})
	s.Put(&X{a: 2, b: "test"})
	s.Put(&X{a: 3, b: "not"})
	vals := s.In("b").Lookup("test")
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
	s.CreateIndex("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.In("c").Lookup("test")
	if vals != nil {
		t.Errorf("Lookup of invalid field should be nil (was %#v)", vals)
	}
}

func TestLookupNonPresentKey(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.In("b").Lookup("dumb")
	if vals != nil {
		t.Errorf("Lookup of non-present key should be nil (was %#v)", vals)
	}
}

func TestTraverse(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	s.CreateIndex("c")

	v1 := &X{a: 1, b: "one:", c: "ZZZ"}
	v2 := &X{a: 2, b: "two:", c: "ZZZ"}
	v3 := &X{a: 3, b: "three:", c: "ZZZ"}
	v4 := &X{a: 4, b: "four:", c: "XXX"}
	v8 := &X{a: 8, b: "eight:", c: "ZZZ"}

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

	vals := s.In("b").Lookup("four")
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

func TestEach(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	s.CreateIndex("c")

	v1 := &X{a: 1, b: "one:", c: "ZZZ"}
	v2 := &X{a: 2, b: "two:", c: "ZZZ"}
	v3 := &X{a: 3, b: "three:", c: "ZZZ"}
	v4 := &X{a: 4, b: "four:", c: "XXX"}
	v8 := &X{a: 8, b: "eight:", c: "ZZZ"}

	s.Put(v1)
	s.Put(v2)
	s.Put(v3)
	s.Put(v4)
	s.Put(v8)

	total := 0
	s.In("c").Each(func(i Indexer) bool {
		total += i.(*X).a
		return true
	}, "ZZZ")

	if total != 14 {
		t.Errorf("Expected total of 14 when adding up matching ZZZ items (got %d)", total)
	}
}

func upTo(ms int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Millisecond)
}

func TestNotificates(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	v1 := &X{a: 1, b: "one:"}
	v2 := &X{a: 1, b: "two:"}

	var expectEvent Event
	var expectOld, expectNew, expectOne, expectTwo Indexer

	var ctx context.Context
	var done context.CancelFunc

	h := func(event Event, old, new Indexer) {
		defer done()

		if event != expectEvent {
			t.Errorf("Expected event %#v (got %#v)", expectEvent, event)
		}
		if old != expectOld {
			t.Errorf("Expected %#v old value %#v (got %#v)", event, expectOld, old)
		}
		if new != expectNew {
			t.Errorf("Expected %#v new value %#v (got %#v)", event, expectNew, new)
		}
		if expectOne == nil {
			if s.index["b"]["one:"] != nil {
				t.Errorf("Expected b one: index to be nil")
			}
		} else if len(s.index["b"]["one:"]) != 1 || s.index["b"]["one:"][0] != expectOne {
			t.Errorf("Expected b one: index to be v1")
		}
		if expectTwo == nil {
			if s.index["b"]["two:"] != nil {
				t.Errorf("Expected b two: index to be nil")
			}
		} else if len(s.index["b"]["two:"]) != 1 || s.index["b"]["two:"][0] != expectTwo {
			t.Errorf("Expected b two: index to be v1")
		}
	}

	s.On(Insert, h)
	s.On(Update, h)
	s.On(Remove, h)
	s.On(Expiry, h)

	ctx, done = upTo(10)
	expectEvent = Insert
	expectOld = nil
	expectNew = v1
	expectOne = v1
	expectTwo = nil
	s.Put(v1)
	<-ctx.Done()

	ctx, done = upTo(10)
	expectEvent = Update
	expectOld = v1
	expectNew = v2
	expectOne = nil
	expectTwo = v2
	s.Put(v2)
	<-ctx.Done()

	ctx, done = upTo(10)
	expectEvent = Remove
	expectOld = v2
	expectNew = nil
	expectOne = nil
	expectTwo = nil
	s.Delete(v1) // This is a trick as we asked to delete v1, but v2 is actually getting deleted and should be expected
	<-ctx.Done()

	ctx, done = upTo(10)
	expectEvent = Insert
	expectOld = nil
	expectNew = v1
	expectOne = v1
	expectTwo = nil
	s.Put(v1)
	<-ctx.Done()

	ctx, done = upTo(10)
	expired = 1
	expectEvent = Expiry
	expectOld = v1
	expectNew = nil
	expectOne = nil
	expectTwo = nil
	s.Expire()
	<-ctx.Done()

	expired = 0
}

func TestCompound(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b", "c")
	v1a := &X{a: 1, b: "one", c: "xxx"}
	v1b := &X{a: 2, b: "one", c: "zzz"}
	v2a := &X{a: 3, b: "two", c: "xxx"}
	v2b := &X{a: 4, b: "two", c: "zzz"}

	s.Put(v1a)
	s.Put(v1b)
	s.Put(v2a)
	s.Put(v2b)

	out := s.In("b", "c").Lookup("one", "zzz")
	if n := len(out); n != 1 {
		t.Errorf("Expected exactly one response from compound lookup (got %s)", n)
	}
	if out[0].(*X).a != 2 {
		t.Errorf("Expected a = 2 in compound result (got %#v)", out[0])
	}
}

func TestUnique(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	s.CreateIndex("c").Unique()
	v1a := &X{a: 1, b: "one", c: "a"}
	v1b := &X{a: 2, b: "two", c: "a"}
	v2 := &X{a: 3, b: "three", c: "b"}
	v3 := &X{a: 4, b: "four", c: "c"}

	var updated Indexer
	var ctx context.Context
	var done context.CancelFunc

	s.On(Update, func(_ Event, old, new Indexer) {
		defer done()
		updated = old
	})

	ctx, done = upTo(10)
	s.Put(v1a)
	s.Put(v1b)
	s.Put(v2)
	s.Put(v3)
	<-ctx.Done()


	if n := s.Len(); n != 3 {
		t.Errorf("Expected only 3 items in store (got %d)", n)
	}

	items := s.In("c").Lookup("a")
	if n := len(items); n != 1 {
		t.Errorf("Expected only 1 items in c index for a (got %d)", n)
	}

	if items[0] != v1b {
		t.Errorf("Expected only item in c index for a to be v1b (got %#v)", items[0])
	}

	if updated != v1a {
		t.Errorf("Expected update notification that v1a was replaced (got %#v)", updated)
	}
}

func TestUnsure(t *testing.T) {
	if Unsure("A", "Z") != true {
		t.Errorf("Expected A to be < Z")
	}
}

func TestLess(t *testing.T) {
	v1 := &wrap{&X{a: 1, b: "one:"}, nil}
	vx := btree.Int(5)

	if v1.Less(vx) {
		t.Errorf("Comparison with non-Indexer item should be false")
	}
}
