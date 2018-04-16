package main

import (
	"fmt"
	"math"
	"os"
	"time"
)

var safeChars = "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz"
var safeCharsIdx map[byte]float64 = map[byte]float64{}
var timeFrame float64 = 86400000 * 7

func init() {
	for i, c := range safeChars {
		safeCharsIdx[byte(c)] = float64(i)
	}
}

// Date gets the date from a memdb id
func Date(what string) *time.Time {
	a := safeCharsIdx[what[0]]
	b := safeCharsIdx[what[1]]
	t := (a*55 + b) * timeFrame

	m := float64(55)
	for i := 1; i <= 7; i++ {
		c := safeCharsIdx[what[1+i]]
		t += c * timeFrame / m
		m *= 55
	}

	t = math.Ceil(t)
	tm := time.Unix(int64(math.Floor(t/1000)), (int64(t)%1000)*int64(time.Millisecond))
	return &tm
}

func main() {
	fmt.Println(Date(os.Args[1]))
}
