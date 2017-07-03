package main

import (
	"git.neds.sh/golib/memdb"

	"fmt"
)

type car struct {
	make    string
	model   string
	sales   float64
	expired bool
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
	return i.expired
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

func (i *car) String() string {
	return fmt.Sprintf("%s %s [$%0.02f/m]", i.make, i.model, i.sales)
}

func main() {
	mdb := memdb.NewStore().
		CreateField("make").
		CreateField("model")

	mdb.Store(&car{make: "Ford", model: "Fiesta", sales: 1375449.73})
	mdb.Store(&car{make: "Ford", model: "Focus", sales: 7033248.90})
	mdb.Store(&car{make: "Holden", model: "Astra", sales: 8613642.89})
	mdb.Store(&car{make: "Holden", model: "Cruze", sales: 6072660.32})
	mdb.Store(&car{make: "Honda", model: "Jazz", sales: 7899950.33})
	mdb.Store(&car{make: "Honda", model: "Civic", sales: 9082843.40})
	mdb.Store(&car{make: "Hyundai", model: "i20", sales: 5341543.43})
	mdb.Store(&car{make: "Hyundai", model: "i30", sales: 1171906.40})
	mdb.Store(&car{make: "Kia", model: "Rio", sales: 4473199.22})
	mdb.Store(&car{make: "Kia", model: "Sportage", sales: 2428186.91})
	mdb.Store(&car{make: "Mitsubishi", model: "ASX", sales: 480031.27})
	mdb.Store(&car{make: "Mitsubishi", model: "Mirage", sales: 9487237.84})
	mdb.Store(&car{make: "Mitsubishi", model: "Outlander", sales: 8152048.82})
	mdb.Store(&car{make: "Nissan", model: "Juke", sales: 6436598.01})
	mdb.Store(&car{make: "Nissan", model: "Micra", sales: 5039032.35})
	mdb.Store(&car{make: "Renault", model: "Clio", sales: 110842.73})
	mdb.Store(&car{make: "Renault", model: "Megane", sales: 8131321.16})
	mdb.Store(&car{make: "Suzuki", model: "Jimny", sales: 8388076.64, expired: true})
	mdb.Store(&car{make: "Suzuki", model: "Swift", sales: 6270911.37})
	mdb.Store(&car{make: "Vauxhall", model: "Astra", sales: 9883699.82})

	indexers := mdb.Lookup("model", "Astra")
	fmt.Println("Found Astra models:")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car).String())
	}

	indexer := mdb.Get(&car{make: "Kia", model: "Rio"}).(*car)
	fmt.Printf("The Kia Rio made $%0.02f in sales this month\n", indexer.sales)

	mdb.Delete(&car{make: "Nissan", model: "Juke"})

	indexers = mdb.Lookup("make", "Nissan")
	fmt.Println("Found Nissan makes:")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car).String())
	}

	fmt.Println("Iterating over cars > Nissan:")
	mdb.AscendStarting(&car{make: "Nissan"}, func(indexer memdb.Indexer) bool {
		c, _ := indexer.(*car)
		if c.make == "Suzuki" {
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
