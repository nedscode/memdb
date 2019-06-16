package memdb

import (
	"strings"
	"testing"
)

type R struct {
	Slice   []*R
	Struct  *R
	Str     string
	Int64   int64
	Uint64  uint64
	Float64 float64
	Float32 float32
	Bool    bool
	Map     map[string]map[int]string
}

func Test_reflective(t *testing.T) {
	r := &R{
		Slice: []*R{
			{
				Str: "Sliced",
			},
		},
		Struct: &R{
			Str:     "Values",
			Int64:   500,
			Uint64:  5000,
			Float64: 5000.0000001,
			Bool:    true,
		},
		Map: map[string]map[int]string {
			"AAA": map[int]string{
				5: "Hello",
			},
		},
		Str:     "Main",
		Int64:   20,
		Uint64:  200,
		Float64: 200.0000001,
		Float32: 0.000000001,
	}

	assertStr(t, r, "str", "Main")
	assertStr(t, r, "slice.0.str", "Sliced")
	assertStr(t, r, "struct.str", "Values")
	assertStr(t, r, "struct.int64", "500")
	assertStr(t, r, "struct.uint64", "5000")
	assertStr(t, r, "struct.float64", "5000")
	assertStr(t, r, "struct.bool", "true")

	assertStr(t, r, "int64", "20")
	assertStr(t, r, "uint64", "200")
	assertStr(t, r, "float64", "200.0000001")
	assertStr(t, r, "float32", "9.999999717e-10")
	assertStr(t, r, "bool", "false")
   	assertStr(t, r, "map.aaa.5", "Hello")

	assertStr(t, r, "struct", "{[] <nil> Values 500 5000 5000.0000001 0 true map[]}")
}

func assertStr(t *testing.T, i interface{}, key, expect string) {
	path := strings.Split(key, ".")
	got := reflective(i, path)
	if got != expect {
		t.Errorf("Expected to find %s at %s (got %s)", expect, key, got)
	}
}
