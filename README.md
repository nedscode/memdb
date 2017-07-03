# memdb

The memdb library is a simple in-memory store for go structs that allows indexing and storage of items as well as configurable item expiry and collection at an interface level.

## Usage

The following simple example demonstrates a use-case example for the database (see example/ folder for a more detailed example):

```golang
package main

import (
	"git.neds.sh/golib/memdb"
)

type car struct {
	make    string
	model   string
}

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

func (i *car) IsExpired() bool {
	return false
}

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

func main() {
	mdb := memdb.NewStore().
		CreateField("make").
		CreateField("model")

	mdb.Store(&car{make: "Ford", model: "Fiesta", sales: 1375449.73})
	mdb.Store(&car{make: "Holden", model: "Astra", sales: 8613642.89})
	mdb.Store(&car{make: "Honda", model: "Jazz", sales: 7899950.33})

	indexers := mdb.Lookup("model", "Astra")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car))
	}

	fmt.Println(mdb.Get(&car{make: "Holden", model: "Astra"}).(*car))

	fmt.Println("Iterating over cars > Nissan:")
	mdb.Ascend(func(indexer memdb.Indexer) bool {
		fmt.Println(indexer.(*car))
		return true
	})
}
```
