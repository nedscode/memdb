// +build !race

package memdb

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func init() {
	if !sim {
		fmt.Println("Will skip longer simulations as -simulate was not specified.")
	}
	if qseed == 0 {
		qseed = int(time.Now().UnixNano()) % 100000
		fmt.Printf("Performing simulation pattern %d. To repeat, rerun with -seed %d\n", qseed, qseed)
	}
}

func TestSimulate_1op_1p(t *testing.T)     { testSimulate(t, 1, 1) }
func TestSimulate_10op_1p(t *testing.T)    { testSimulate(t, 10, 1) }
func TestSimulate_100op_1p(t *testing.T)   { testSimulate(t, 100, 1) }
func TestSimulate_1000op_1p(t *testing.T)  { testSimulate(t, 1000, 1) }
func TestSimulate_10000op_1p(t *testing.T) { testSimulate(t, 10000, 1) }

func TestSimulate_10op_10p(t *testing.T)    { testSimulate(t, 10, 10) }
func TestSimulate_100op_10p(t *testing.T)   { testSimulate(t, 100, 10) }
func TestSimulate_1000op_10p(t *testing.T)  { testSimulate(t, 1000, 10) }
func TestSimulate_10000op_10p(t *testing.T) { testSimulate(t, 10000, 10) }

func TestSimulate_100op_100p(t *testing.T)   { testSimulate(t, 100, 100) }
func TestSimulate_1000op_100p(t *testing.T)  { testSimulate(t, 1000, 100) }
func TestSimulate_10000op_100p(t *testing.T) { testSimulate(t, 10000, 100) }

func TestSimulate_10000op_1000p(t *testing.T) { testSimulate(t, 20000, 1000) }

func TestSimulate_100000op_10000p(t *testing.T) { testSimulate(t, 120000, 10000) }

// Randomly generate operations on a given database with multiple clients to ensure consistency and thread safety.
func testSimulate(t *testing.T, threadCount, parallelism int) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
	}

	if !sim && (threadCount > 1000 || parallelism > 100) {
		t.Skip("Skipping test as -simulate was not specified.")
	}

	rand.Seed(int64(qseed))
	fmt.Printf("Black box simulation with %d thread count & %d parallelism\n", threadCount, parallelism)

	// A list of operations that readers and writers can perform.
	var readerHandlers = []simulateHandler{
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateGetHandler,
		simulateLookupHandler,
		simulateLookupHandler,
		simulateLookupHandler,
		simulateEachHandler,
		simulateEachHandler,
		simulateWalkHandler,
	}
	var writerHandlers = []simulateHandler{
		simulatePutHandler,
		simulatePutHandler,
		simulatePutHandler,
		simulatePutHandler,
		simulatePutHandler,
		simulatePutHandler,
		simulateDeleteHandler,
		simulateDeleteHandler,
		simulateExpiryHandler,
	}

	mdb := NewStore().
		CreateIndex("b").
		CreateIndex("c").
		CreateIndex("b", "c").Unique()

	expired = -aEls

	// Run n threads in parallel, each with their own operation.
	var wg sync.WaitGroup
	var parallels = make(chan bool, parallelism)
	var i int
	for {
		parallels <- true
		wg.Add(1)

		writable := (rand.Int() % 100) < 10 // 10% writers

		// Choose an operation to execute.
		var handler simulateHandler
		if writable {
			handler = writerHandlers[rand.Intn(len(writerHandlers))]
		} else {
			handler = readerHandlers[rand.Intn(len(readerHandlers))]
		}

		// Execute a thread for the given operation.
		go func(writable bool, handler simulateHandler) {
			defer wg.Done()

			// Execute handler.
			handler(mdb)

			// Release a thread back to the scheduling loop.
			<-parallels
		}(writable, handler)

		i++
		if i > threadCount {
			break
		}
	}

	// Wait until all threads are done.
	wg.Wait()
}

type simulateHandler func(mdb *Store)

var (
	aEls   = 5000
	oEls   = 50
	exists = [5000]bool{}
)

// Retrieves a key from the database and verifies that it is what is expected.
func simulatePutHandler(mdb *Store) {
	a := getRand(aEls, 0.1)
	x := &X{
		a: a,
		b: fmt.Sprintf("b%d", getRand(oEls)),
		c: fmt.Sprintf("c%d", getRand(oEls)),
	}

	mdb.Put(x)
	if a < aEls {
		exists[a] = true
	}

	y := mdb.Get(&X{a: a})

	if y != x {
		fmt.Printf("Put:\n%#v\nGot:\n%#v\n", x, y)
		panic("Value mismatch")
	}
}

func getRand(n int, fuzz ...float64) int {
	if len(fuzz) > 0 {
		n = n + int(float64(n)*fuzz[0])
	}
	return int(rand.Int31n(int32(n - 1)))
}

func getPotentialNumber(n int) int {
	s := getRand(n, 0.1)
	if s >= n {
		return s
	}
	if exists[s] {
		return s
	}
	for i := s + 1; i != s; i++ {
		if i >= n {
			i = 0
		}
		if exists[i] {
			return i
		}
	}
	return 0
}

// Deletes a key from the database.
func simulateDeleteHandler(mdb *Store) {
	a := getPotentialNumber(aEls)

	mdb.Delete(&X{a: a})
}

// Expires keys from the database.
func simulateExpiryHandler(mdb *Store) {
	expired = getPotentialNumber(aEls)

	mdb.Expire()
}

// Retrieves a key from the database
func simulateGetHandler(mdb *Store) {
	a := getPotentialNumber(aEls)
	mdb.Get(&X{a: a})
}

// Retrieves a key from the database
func simulateLookupHandler(mdb *Store) {
	s := getRand(3)

	if s == 0 {
		mdb.In("b").Lookup(fmt.Sprintf("b%d", getRand(oEls, 0.1)))
	} else if s == 1 {
		mdb.In("c").Lookup(fmt.Sprintf("c%d", getRand(oEls, 0.1)))
	} else if s == 2 {
		mdb.In("b", "c").Lookup(
			fmt.Sprintf("b%d", getRand(oEls, 0.1)),
			fmt.Sprintf("c%d", getRand(oEls, 0.1)),
		)
	}
}

// Retrieves a key from the database
func simulateEachHandler(mdb *Store) {
	s := getRand(3)

	ea := func(_ interface{}) bool {
		time.Sleep(time.Duration(getRand(1000)) * time.Microsecond)
		return getRand(100) < 98
	}

	if s == 0 {
		mdb.In("b").Each(ea, fmt.Sprintf("b%d", getRand(oEls, 0.1)))
	} else if s == 1 {
		mdb.In("c").Each(ea, fmt.Sprintf("c%d", getRand(oEls, 0.1)))
	} else if s == 2 {
		mdb.In("b", "c").Each(ea,
			fmt.Sprintf("b%d", getRand(oEls, 0.1)),
			fmt.Sprintf("c%d", getRand(oEls, 0.1)),
		)
	}
}

// Walks the database in a random direction
func simulateWalkHandler(mdb *Store) {
	s := getRand(4)
	a := getRand(aEls)

	ea := func(_ interface{}) bool {
		time.Sleep(time.Duration(getRand(200)) * time.Microsecond)
		return getRand(100) < 98
	}

	if s == 0 {
		mdb.Ascend(ea)
	} else if s == 1 {
		mdb.Descend(ea)
	} else if s == 2 {
		mdb.AscendStarting(&X{a: a}, ea)
	} else if s == 3 {
		mdb.DescendStarting(&X{a: a}, ea)
	}
}
