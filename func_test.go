package memdb

import (
	"github.com/google/btree"
	"github.com/nedscode/memdb/persist"

	"context"
	"flag"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var expired = 0

type X struct {
	a int
	b string
	c string
}

// Storage is a mock memdb Persister that stores and loads from a map for testing purposes.
type Storage struct {
	sync.Mutex
	Store map[string]interface{}
}

func NewMockStorage() *Storage {
	return &Storage{
		Store: map[string]interface{}{},
	}
}

// Save is an implementation of the Persister.Save method
func (s *Storage) Save(id string, item interface{}) error {
	s.Lock()
	defer s.Unlock()
	s.Store[id] = item
	return nil
}

// Load is an implementation of the Persister.Load method
func (s *Storage) Load(loadFunc persist.LoadFunc) error {
	s.Lock()
	defer s.Unlock()
	for id, item := range s.Store {
		loadFunc(id, item)
	}
	return nil
}

// Remove is an implementation of the Persister.Remove method
func (s *Storage) Remove(id string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.Store, id)
	return nil
}

var (
	sim   bool
	qseed int
)

func init() {
	flag.BoolVar(&sim, "simulate", sim, "Simulate with black box test")
	flag.IntVar(&qseed, "seed", 0, "Seed for randomiser")
	flag.Parse()
}

func (x *X) Less(o interface{}) bool {
	return x.a < o.(*X).a
}
func (x *X) IsExpired(now, fetched, updated time.Time) bool {
	if expired < 0 {
		return now.UnixNano()%1000000 > 995000
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

func TestPutAll(t *testing.T) {
	s := NewStore()
	items := []interface{}{
		&X{a: 1},
		&X{a: 2},
		&X{a: 3},
		&X{a: 4},
		&X{a: 5},
		&X{a: 6},
		&X{a: 1},
	}
	s.PutAll(items)

	n := s.Len()
	if n != 6 {
		t.Errorf("Expected 6 items imported when PutAll (got %d)", n)
	}
}

func TestGet(t *testing.T) {
	s := NewStore()
	orig := &X{a: 1}
	s.Put(orig)
	v := s.Get(&X{a: 1})
	if v != orig {
		t.Errorf("Gotten value should be same as original instance")
	}

	v = s.Get(&X{a: 2})
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
		t.Errorf("Length of looked up values should be 2 (was %d)", len(vals))
	}
	a := vals[0].(*X)
	b := vals[1].(*X)
	if (a.a != 1 || b.a != 2) && (a.a != 2 || b.a != 1) {
		t.Errorf("Expected to return 1 and 2 (got %#v)", vals)
	}

	val := s.In("b").One("test").(*X)
	if val.a != 1 && val.a != 2 {
		t.Errorf("Expected One to return 1 or 2 (got %#v)", val)
	}
}

func TestLookupInvalidField(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.In("c").Lookup("test")
	if vals != nil {
		t.Errorf("Lookup of invalid field should be nil (was %#v)", vals)
	}

	vals = s.In("b").Lookup("dumb")
	if vals != nil {
		t.Errorf("Lookup of non-present key should be nil (was %#v)", vals)
	}
}

func TestTraverse(t *testing.T) {
	s := NewStore()
	p := NewMockStorage()

	s.CreateIndex("b")
	s.CreateIndex("c")
	s.Persistent(p)

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

	n = len(p.Store)
	if n != 4 {
		t.Errorf("Expected 4 items in persitent store (got %d)", n)
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

	iter := func(i interface{}) bool {
		got += i.(*X).b
		return i != stop
	}

	performTraversals(t, s, iter, &got, &stop, v2, v3, v4)

	expired = 4
	now := time.Now()
	if !v4.IsExpired(now, now, now) {
		t.Errorf("Expected v4 to be expired")
	}

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

	n = len(p.Store)
	if n != 3 {
		t.Errorf("Expected 3 items in persitent store (got %d)", n)
	}

	s.Delete(v2)

	got = ""
	s.Ascend(iter)
	expect = "one:eight:"
	if got != expect {
		t.Errorf("Deleted item not removed expected %s (got %s)", expect, got)
	}

	n = len(p.Store)
	if n != 2 {
		t.Errorf("Expected 2 items in persitent store (got %d)", n)
	}

	verifyPersistentStore(p, t)

	s2 := NewStore()
	s2.Persistent(p)

	n = s2.Len()
	if n != 2 {
		t.Errorf("Expected 2 items in new store (got %d)", n)
	}

	got = ""
	s.Ascend(iter)
	expect = "one:eight:"
	if got != expect {
		t.Errorf("Wrong items found in new store expected %s (got %s)", expect, got)
	}
}

func verifyPersistentStore(p *Storage, t *testing.T) {
	foundOne := false
	foundEight := false
	foundOther := false

	for _, v := range p.Store {
		if v.(*X).a == 1 {
			foundOne = true
		} else if v.(*X).a == 8 {
			foundEight = true
		} else {
			foundOther = true
		}
	}

	if !foundOne || !foundEight || foundOther {
		t.Errorf("Wrong items found in persistent store")
	}
}

func performTraversals(
	t *testing.T,
	s Storer,
	iter func(i interface{}) bool,
	got *string,
	stop **X,
	v2 *X,
	v3 *X,
	v4 *X,
) {
	*got = ""
	s.Ascend(iter)
	expect := "one:two:four:eight:"
	if *got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, *got)
	}

	*got = ""
	s.Descend(iter)
	expect = "eight:four:two:one:"
	if *got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, *got)
	}

	*got = ""
	s.DescendStarting(v3, iter)
	expect = "two:one:"
	if *got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, *got)
	}

	*got = ""
	s.AscendStarting(v3, iter)
	expect = "four:eight:"
	if *got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, *got)
	}

	*got = ""
	*stop = v2
	s.Ascend(iter)
	expect = "one:two:"
	if *got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, *got)
	}

	*got = ""
	*stop = v4
	s.Descend(iter)
	expect = "eight:four:"
	if *got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, *got)
	}

	*got = ""
	*stop = v4
	s.AscendStarting(v3, iter)
	expect = "four:"
	if *got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, *got)
	}

	*got = ""
	*stop = v2
	s.DescendStarting(v3, iter)
	expect = "two:"
	if *got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, *got)
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
	s.In("c").Each(func(i interface{}) bool {
		total += i.(*X).a
		return true
	}, "ZZZ")

	if total != 14 {
		t.Errorf("Expected total of 14 when adding up matching ZZZ items (got %d)", total)
	}
}

func upTo(ms int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
}

func notificateText(t *testing.T, s Storer, text, what string, expect Indexable) {
	st := s.(*Store)
	if expect == nil {
		if st.index["b"][text] != nil {
			t.Errorf("Expected b one: index to be nil")
		}
	} else if len(st.index["b"][text]) != 1 || st.index["b"][text][0].item != expect {
		t.Errorf("Expected b %s: index to be %s", text, what)
	}
}

func TestNotificates(t *testing.T) {
	s := NewStore()
	s.CreateIndex("b")
	v1 := &X{a: 1, b: "one:"}
	v2 := &X{a: 1, b: "two:"}

	var expectEvent Event
	var expectOld, expectNew, expectOne, expectTwo Indexable

	var ctx context.Context
	var done context.CancelFunc

	h := func(event Event, old, new interface{}) {
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

		notificateText(t, s, "one:", "v1", expectOne)
		notificateText(t, s, "two:", "v2", expectTwo)
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
		t.Errorf("Expected exactly one response from compound lookup (got %d)", n)
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

	var updated interface{}
	var ctx context.Context
	var done context.CancelFunc

	s.On(Update, func(_ Event, old, new interface{}) {
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
	if !Unsure("A", "Z") {
		t.Errorf("Expected A to be < Z")
	}
}

func TestLess(t *testing.T) {
	s := NewStore()
	v1 := &wrap{storer: s, item: &X{a: 1, b: "one:"}}
	vx := btree.Int(5)

	if v1.Less(vx) {
		t.Errorf("Comparison with non-Indexer item should be false")
	}
}

func TestEventString(t *testing.T) {
	if Insert.String() != "Insert event" {
		t.Errorf("Insert string incorrect")
	}

	if Update.String() != "Update event" {
		t.Errorf("Update string incorrect")
	}

	if Remove.String() != "Remove event" {
		t.Errorf("Remove string incorrect")
	}

	if Expiry.String() != "Expiry event" {
		t.Errorf("Expiry string incorrect")
	}

	bad := Event(-1)
	if bad.String() != "Unknown event" {
		t.Errorf("Event unknown string incorrect")
	}
}

func TestPersistentAfterUse(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	s := NewStore()
	p := NewMockStorage()
	s.Put(&X{})
	s.Persistent(p)
}

type anon struct {
	id    string
	value int
}

func TestPrimaryKey(t *testing.T) {
	s := NewStore()
	s.PrimaryKey("id")
	s.CreateIndex("value")

	s.Put(&anon{"c", 10})
	s.Put(&anon{"b", 20})
	s.Put(&anon{"a", 40})
	s.Put(&anon{"d", 80})

	order := ""
	s.Ascend(func(i interface{}) bool {
		order += i.(*anon).id
		return true
	})

	if order != "abcd" {
		t.Errorf("Wrong order of items, expected abcd (got %s)", order)
	}
}

func TestReversed(t *testing.T) {
	s := NewStore()
	s.PrimaryKey("id")
	s.CreateIndex("value")
	s.Reversed()

	if !s.(*Store).reversed {
		t.Errorf("Expected store.reversed to be true")
	}

	s.Put(&anon{"b", 10})
	s.Put(&anon{"d", 20})
	s.Put(&anon{"a", 40})
	s.Put(&anon{"c", 80})

	order := ""
	s.Ascend(func(i interface{}) bool {
		order += i.(*anon).id
		return true
	})

	if order != "dcba" {
		t.Errorf("Wrong order of items, expected dcba (got %s)", order)
	}
}
