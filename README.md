# memdb

The memdb library is a simple in-memory store for go structs that allows indexing and storage of items as well as
configurable item expiry and collection at an interface level.

[![Build Status](https://travis-ci.org/nedscode/memdb.svg?branch=master)](https://travis-ci.org/nedscode/memdb)
[![Go Report Card](https://goreportcard.com/badge/github.com/nedscode/memdb)](https://goreportcard.com/report/github.com/nedscode/memdb)
[![Documentation](https://godoc.org/github.com/nedscode/memdb?status.svg)](http://godoc.org/github.com/nedscode/memdb)
[![Coverage Status](https://coveralls.io/repos/github/nedscode/memdb/badge.svg?branch=master)](https://coveralls.io/github/nedscode/memdb?branch=master)
[![GitHub issues](https://img.shields.io/github/issues/nedscode/memdb.svg)](https://github.com/nedscode/memdb/issues)
[![license](https://img.shields.io/github/license/nedscode/memdb.svg?maxAge=2592000)](https://github.com/nedscode/memdb/LICENSE)

## Important caveats

As you are working with in-memory objects, it can be easy to overlook that you're also indexing these items in a
database.

Just like a real database, if you update an item such that it's index keys would change, you must Put it back in to
update the items indexes in the database, and also to cause update notifications to be sent.

DO NOT under any circumstances update the PRIMARY KEYs (ie keys used to determine the output of the Less()
comparator) without first removing the existing item. Such an act would leave the item stranded in an unknown
location within the index.

## Including

To start using, get memdb `go get github.com/nedscode/memdb` and include it in your code:

```golang
import "github.com/nedscode/memdb"
```

## Defining you storage struct

Define your struct as normal:

```golang
type car struct {
    Make    string
    Model   string
    RRP     int
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
        if i.Make < o.Make {
            return true
        }
        if i.Make > o.Make {
            return false
        }
        if i.Model < o.Model {
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
        return i.Make
    case "model":
        return i.Model
    default:
        return "" // Indicates should not be indexed
    }
}
```

## Creating a storage instance

To create a storage instance initialise it and set the indexed fields.

Indexed fields can only be set at the start before data gets stored. Attempt to set index fields after use will cause a
panic.

```golang
    mdb := memdb.NewStore().
        CreateIndex("make").
        CreateIndex("model")
```

### Compound indexes

You can also create compound indexes by supplying multiple fields:

```golang
    mdb.CreateIndex("make", "model")
```

# Unique indexes

You can also create unique indexes by appending a `Unique()` to the definition.

Putting an item with the same value as a unique index will cause the previous item to be "Updated".

```golang
    type car struct {
        Make    string
        Model   string
        RRP     int
        Vin     string
    }

    mdb.CreateIndex("vin").Unique()
```

## Adding items to the store.

Saving items into the store is simple:

```golang
    mdb.Put(&car{Make: "Ford", Model: "Fiesta", RRP: 27490})
    mdb.Put(&car{Make: "Holden", Model: "Astra", RRP: 24190})
    mdb.Put(&car{Make: "Honda", Model: "Jazz", RRP: 19790})
```

## Retrieving an item

In order to retrieve an item, supply an eqivalent item as a search parameter, you only need to create enough fields in
the search object to be deemed equivalent by your Less function:

```golang
    vehicle := mdb.Get(&car{Make: "Holden", Model: "Astra"}).(*car)
    fmt.Printf("Vehicle RRP is $%d\n", vehicle.RRP)
```

## Looking up items by indexed field

This is where it starts to get interesting, we can lookup items by any of our defined indexed fields:

```golang
    indexers := mdb.In("model").Lookup("Astra")
    for _, indexer := range indexers {
        vehicle := indexer.(*car)
        fmt.Printf("%s %s ($%d rrp)\n", vehicle.Make, vehicle.Model, vehicle.RRP)
    }
```

If you have compound fields, you can search them like:

```golang
    indexers := mdb.In("make", "model").Lookup("Holden", "Astra")
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
        fmt.Printf("%s %s ($%d rrp)\n", vehicle.Make, vehicle.Model, vehicle.RRP)
        count++
        return true
    })
    fmt.Println("Found %d cars\n", count)
```

If you wish to traverse your simple or compound indexed fields, you may also do this via:

```golang
    mdb.In("make", "model").Each(func(indexer memdb.Indexer) bool {
        vehicle := indexer.(*car)
        fmt.Printf("%s %s ($%d rrp)\n", vehicle.Make, vehicle.Model, vehicle.RRP)
}, "Holden", "Astra")
```

## Notification

Item notification can be performed via the On(event, callback) method:

```golang
    notify := func (event memdb.Event, old, new memdb.Indexer) {
        fmt.Printf("Got %#v of %#v -> %#v", event, old, new)
    }
    
    mdb.On(memdb.Insert, notify)
    mdb.On(memdb.Update, notify)
    mdb.On(memdb.Remove, notify)
    mdb.On(memdb.Expiry, notify)
```

## Removal

Items can be removed directly by calling the Delete function

```golang
    mdb.Delete(&car{Make: "Holden", Model: "Astra"})
```

## Expiry

Item expiry can be achieved by defining an expiry condition function and scheduling the expiry function.

Say we expanded the car struct to have a sold time

```golang
type car struct {
    Make    string
    Model   string
    RRP     int
    Sold    time.Time
}
```

Then changed the IsExpired method like:

```golang
func (i *car) IsExpired() bool {
    return i.Sold.Before(time.Now().Sub(24 * time.Hour))
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

## Persistence

Sometimes you want to have your cake and eat it too. While this is an in-memory database,
we also support store-on-put and load-at-create style persistence.

This is achieved by adding a Persister to the store after adding indexes and before beginning
to use it.

There is currently 

```golang
    p := filepersist.NewFileStorage(
        "/tmp/mydata",
        func(indexerType string) interface{} {
            if (indexerType == "*main.car") {
                return &car{}
            }
            return nil
        },
    )
    mdb := memdb.NewStore().
        CreateIndex("make").
        CreateIndex("model").
        Persistent(p)
```


## License

© 2017, Neds International, code is released under GNU LGPL v3.0, see [LICENSE](LICENSE) file.