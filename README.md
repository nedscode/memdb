# memdb

The memdb library is a simple in-memory store for go structs that allows indexing and storage of items as well as
configurable item expiry and collection at an interface level.

## Important caveats

As you are working with in-memory objects, it can be easy to overlook that you're also indexing these items in a
database.

Just like a real database, if you update an item such that it's index keys would change, you must Put it back in to
update the items indexes in the database, and also to cause update notifications to be sent.

DO NOT under any circumstances update the PRIMARY KEYs (ie keys used to determine the output of the Less()
comparator) without first removing the existing item. Such an act would leave the item stranded in an unknown
location within the index.

## Including

To start using, get memdb `go get git.neds.sh/golib/memdb` and include it in your code:

```golang
import "git.neds.sh/golib/memdb"
```

## Defining you storage struct

Define your struct as normal:

```golang
type car struct {
	make    string
	model   string
	rrp     int
}
```

Then add the required methods to support storage in memdb.

We need a comparator function `Less`. If `a.Less(b) == false && b.Less(a) == false`, then the item is determined to be
equivalent and the same. Storage of multiple equivalent items will overwrite each other. If you can't figure out the
comparison yourself (eg unknown object type), call the Unsure function which will arbitrarily, but consistently
determine the order. 

```golang
func (i *car) Less(other memdb.Indexer) bool {
	switch o := other.(type) {
	case *car:
		if i.make < o.make {
			return true
		}
		if i.make > o.make {
			return false
		}
		if i.model < o.model {
			return true
		}
		return false
	}
	return memdb.Unsure(i, other)
}
```

Then we need an expiry function to determine if the item is to be expired or not, for the moment, we don't want any
items to be expired, so set the function to return false.

```golang
func (i *car) IsExpired() bool {
	return false
}
```

Finally, we need a function that will return the string values of the indexed fields, all indexed fields are returned
and stored as strings:

```golang
func (i *car) GetField(field string) string {
	switch field {
	case "make":
		return i.make
	case "model":
		return i.model
	default:
		return "" // Indicates should not be indexed
	}
}
```

## Creating a storage instance

To create a storage instance initialise it and set the indexed fields. Indexed fields can only be set at the start
before data gets stored. Attempt to set index fields after use will cause a panic.

```golang
	mdb := memdb.NewStore().
		CreateField("make").
		CreateField("model")
```

## Adding items to the store.

Saving items into the store is simple:

```golang
	mdb.Put(&car{make: "Ford", model: "Fiesta", rrp: 27490})
	mdb.Put(&car{make: "Holden", model: "Astra", rrp: 24190})
	mdb.Put(&car{make: "Honda", model: "Jazz", rrp: 19790})
```

## Retrieving an item

In order to retrieve an item, supply an eqivalent item as a search parameter, you only need to create enough fields in
the search object to be deemed equivalent by your Less function:

```golang
	vehicle := mdb.Get(&car{make: "Holden", model: "Astra"}).(*car)
	fmt.Printf("Vehicle RRP is $%d\n", vehicle.rrp)
```

## Looking up items by index

This is where it starts to get interesting, we can lookup items by any of our defined indexed fields:

```golang
	indexers := mdb.Lookup("model", "Astra")
	for _, indexer := range indexers {
	    vehicle := indexer.(*car)
		fmt.Printf("%s %s ($%d rrp)\n", vehicle.make, vehicle.model, vehicle.rrp)
	}
```

## Traversing the database

If you desire to walk the database, you can ascend or descend from the extremities or a certain point using one of the 
following functions:

 * Ascend(iterator)
 * Descend(iterator)
 * AscendStarting(at, iterator)
 * DescendStarting(at, iterator)

Use the functions by providing an iterator that returns false to stop traversal as follows:

```golang
	fmt.Println("Iterating over all cars, ascending:\n")
	count := 0
	mdb.Ascend(func(indexer memdb.Indexer) bool {
		vehicle := indexer.(*car)
		fmt.Printf("%s %s ($%d rrp)\n", vehicle.make, vehicle.model, vehicle.rrp)
		count++
		return true
	})
	fmt.Println("Found %d cars\n", count)
```

## Expiry

Item expiry can be achieved by defining an expiry condition function and scheduling the expiry function.

Say we expanded the car struct to have a sold time

```golang
type car struct {
	make    string
	model   string
	rrp     int
	sold    time.Time
}
```

Then changed the IsExpired method like:

```golang
func (i *car) IsExpired() bool {
	return i.sold.Before(time.Now().Sub(24 * time.Hour))
}
```

Then scheduled the Expire function:

```golang
go func() {
	tick := time.Tick(30 * time.Minute)
	for range tick {
		mdb.Expire()
	}
}
```

Now every 30 minutes, we will expire cars sold more than 24 hours ago from our listings.
