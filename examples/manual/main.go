package main

import (
	"github.com/nedscode/memdb"

	"fmt"
	"time"
)

// This example explains the method of using memdb via an `Indexable` object.
//
// This allows custom logic to be used in determining equality/sorting, but is not recommended unless you need it.
//
// See the simple example for the recommended way.

type car struct {
	Make    string
	Model   string
	Sales   float64
	Expired bool
}
// Static type check (will stop compile if car is not Indexable, nor Expirable):
var _ memdb.Indexable = (*car)(nil)
var _ memdb.Expirable = (*car)(nil)

func (i *car) Less(other interface{}) bool {
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

func (i *car) IsExpired(_ time.Time, _ memdb.Stats) bool {
	return i.Expired
}

func (i *car) String() string {
	return fmt.Sprintf("%s %s [$%0.02f/m]", i.Make, i.Model, i.Sales)
}

func main() {
	mdb := memdb.NewStore().
		CreateIndex("make").
		CreateIndex("model")

	_, _ = mdb.Put(&car{Make: "Ford", Model: "Fiesta", Sales: 1375449.73})
	_, _ = mdb.Put(&car{Make: "Ford", Model: "Focus", Sales: 7033248.90})
	_, _ = mdb.Put(&car{Make: "Holden", Model: "Astra", Sales: 8613642.89})
	_, _ = mdb.Put(&car{Make: "Holden", Model: "Cruze", Sales: 6072660.32})
	_, _ = mdb.Put(&car{Make: "Honda", Model: "Jazz", Sales: 7899950.33})
	_, _ = mdb.Put(&car{Make: "Honda", Model: "Civic", Sales: 9082843.40})
	_, _ = mdb.Put(&car{Make: "Hyundai", Model: "i20", Sales: 5341543.43})
	_, _ = mdb.Put(&car{Make: "Hyundai", Model: "i30", Sales: 1171906.40})
	_, _ = mdb.Put(&car{Make: "Kia", Model: "Rio", Sales: 4473199.22})
	_, _ = mdb.Put(&car{Make: "Kia", Model: "Sportage", Sales: 2428186.91})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "ASX", Sales: 480031.27})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "Mirage", Sales: 9487237.84})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "Outlander", Sales: 8152048.82})
	_, _ = mdb.Put(&car{Make: "Nissan", Model: "Juke", Sales: 6436598.01})
	_, _ = mdb.Put(&car{Make: "Nissan", Model: "Micra", Sales: 5039032.35})
	_, _ = mdb.Put(&car{Make: "Renault", Model: "Clio", Sales: 110842.73})
	_, _ = mdb.Put(&car{Make: "Renault", Model: "Megane", Sales: 8131321.16})
	_, _ = mdb.Put(&car{Make: "Suzuki", Model: "Jimny", Sales: 8388076.64, Expired: true})
	_, _ = mdb.Put(&car{Make: "Suzuki", Model: "Swift", Sales: 6270911.37})
	_, _ = mdb.Put(&car{Make: "Vauxhall", Model: "Astra", Sales: 9883699.82})

	indexers := mdb.In("model").Lookup("Astra")
	fmt.Println("Found Astra models:")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car).String())
	}

	if found, ok := mdb.Get(&car{Make: "Kia", Model: "Rio"}).(*car); ok {
		fmt.Printf("The Kia Rio made $%0.02f in sales this month\n", found.Sales)
	}

	_, _ = mdb.Delete(&car{Make: "Nissan", Model: "Juke"})

	indexers = mdb.In("make").Lookup("Nissan")
	fmt.Println("Found Nissan makes:")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car).String())
	}

	fmt.Println("Iterating over cars > Nissan:")
	mdb.AscendStarting(&car{Make: "Nissan"}, func(indexer interface{}) bool {
		c, _ := indexer.(*car)
		if c.Make == "Suzuki" {
			// Not interested any more
			return false
		}
		fmt.Println(c.String())
		// Keep going
		return true
	})

	fmt.Println("Expiring expired cars")
	mdb.Expire()
}
