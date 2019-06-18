package main

import (
	"github.com/nedscode/memdb"
	"time"

	"fmt"
)

// This example describes the recommended way of using memdb via automatic field indexing and sorting.

type car struct {
	Make  string
	Model string
	Sales float64
	Info  carInfo
}

type carInfo struct {
	SKU string
}

func (i *car) String() string {
	return fmt.Sprintf("%s %s [$%0.02f/m]", i.Make, i.Model, i.Sales)
}

func main() {
	mdb := memdb.NewStore().
		PrimaryKey("make", "model").
		CreateIndex("model").
		CreateIndex("info.sku")

	// Expire the items after they've been in the list for 30 seconds:
	mdb.SetExpirer(memdb.AgeExpirer(0, 0, 30*time.Second))

	_, _ = mdb.Put(&car{Make: "Ford", Model: "Fiesta", Sales: 1375449.73, Info: carInfo{SKU: "C7961"}})
	_, _ = mdb.Put(&car{Make: "Ford", Model: "Focus", Sales: 7033248.90, Info: carInfo{SKU: "C0082"}})
	_, _ = mdb.Put(&car{Make: "Holden", Model: "Astra", Sales: 8613642.89, Info: carInfo{SKU: "C8044"}})
	_, _ = mdb.Put(&car{Make: "Holden", Model: "Cruze", Sales: 6072660.32, Info: carInfo{SKU: "C4227"}})
	_, _ = mdb.Put(&car{Make: "Honda", Model: "Jazz", Sales: 7899950.33, Info: carInfo{SKU: "C4736"}})
	_, _ = mdb.Put(&car{Make: "Honda", Model: "Civic", Sales: 9082843.40, Info: carInfo{SKU: "C7488"}})
	_, _ = mdb.Put(&car{Make: "Hyundai", Model: "i20", Sales: 5341543.43, Info: carInfo{SKU: "C7185"}})
	_, _ = mdb.Put(&car{Make: "Hyundai", Model: "i30", Sales: 1171906.40, Info: carInfo{SKU: "C3511"}})
	_, _ = mdb.Put(&car{Make: "Kia", Model: "Rio", Sales: 4473199.22, Info: carInfo{SKU: "C8312"}})
	_, _ = mdb.Put(&car{Make: "Kia", Model: "Sportage", Sales: 2428186.91, Info: carInfo{SKU: "C6626"}})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "ASX", Sales: 480031.27, Info: carInfo{SKU: "C7779"}})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "Mirage", Sales: 9487237.84, Info: carInfo{SKU: "C0424"}})
	_, _ = mdb.Put(&car{Make: "Mitsubishi", Model: "Outlander", Sales: 8152048.82, Info: carInfo{SKU: "C3811"}})
	_, _ = mdb.Put(&car{Make: "Nissan", Model: "Juke", Sales: 6436598.01, Info: carInfo{SKU: "C4627"}})
	_, _ = mdb.Put(&car{Make: "Nissan", Model: "Micra", Sales: 5039032.35, Info: carInfo{SKU: "C2459"}})
	_, _ = mdb.Put(&car{Make: "Renault", Model: "Clio", Sales: 110842.73, Info: carInfo{SKU: "C7004"}})
	_, _ = mdb.Put(&car{Make: "Renault", Model: "Megane", Sales: 8131321.16, Info: carInfo{SKU: "C2054"}})
	_, _ = mdb.Put(&car{Make: "Suzuki", Model: "Jimny", Sales: 8388076.64, Info: carInfo{SKU: "C8471"}})
	_, _ = mdb.Put(&car{Make: "Suzuki", Model: "Swift", Sales: 6270911.37, Info: carInfo{SKU: "C0211"}})
	_, _ = mdb.Put(&car{Make: "Vauxhall", Model: "Astra", Sales: 9883699.82, Info: carInfo{SKU: "C7168"}})

	indexers := mdb.In("model").Lookup("Astra")
	fmt.Println("Found Astra models:")
	for _, indexer := range indexers {
		fmt.Println(indexer.(*car).String())
	}

	if found, ok := mdb.InPrimaryKey().One("Kia", "Rio").(*car); ok {
		fmt.Printf("The Kia Rio made $%0.02f in sales this month\n", found.Sales)
	}

	findSKU := "C3811"
	if found, ok := mdb.In("info.sku").One(findSKU).(*car); ok {
		fmt.Printf("SKU %s (%s %s) made $%0.02f in sales this month\n", findSKU,  found.Make, found.Model, found.Sales)
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
