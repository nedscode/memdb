package filepersist

import (
	"github.com/nedscode/memdb"

	"os"
	"testing"
)

type X struct {
	A int    `json:"a"`
	B string `json:"b"`
	C string `json:"c"`
	X bool   `json:"x"`
}

func (x *X) Less(o memdb.Indexer) bool {
	return x.A < o.(*X).A
}

func (x *X) IsExpired() bool {
	return x.X
}

func (x *X) GetField(f string) string {
	if f == "c" {
		return x.C
	}
	return x.B
}

func TestStorage(t *testing.T) {
	s, err := NewFileStorage("/tmp/filestore", func(indexerType string) interface{} {
		if indexerType != "*filepersist.X" {
			t.Errorf("Unexpected indexerType: %s", indexerType)
		}
		return &X{}
	})

	if err != nil {
		t.Errorf("Unexpected error creating new storage: %#v", err)
	}

	a := &X{
		1,
		"a",
		"Z",
		false,
	}

	id := "123456789012"
	s.Save(id, a)

	if fi, err := os.Stat("/tmp/filestore/" + id + ".json"); err != nil {
		t.Errorf("Expected file to be written")
	} else {
		if fi.Size() < 30 {
			t.Errorf("Expected save file to be bigger")
		}
	}

	s.Load(func(idIn string, indexer memdb.Indexer) {
		if idIn != id {
			t.Errorf("Didn't get expected ID on load %s (expected %s)", idIn, id)
		}

		if x, ok := indexer.(*X); !ok {
			t.Errorf("Didn't get expected type on load %T (expected *X)", indexer)
		} else {
			if x.A != a.A {
				t.Errorf("Didn't get expected field on load A = %d (expected %d)", x.A, a.A)
			}
			if x.B != a.B {
				t.Errorf("Didn't get expected field on load A = %s (expected %s)", x.B, a.B)
			}
			if x.C != a.C {
				t.Errorf("Didn't get expected field on load c = %s (expected %s)", x.C, a.C)
			}
		}
	})

	s.Remove(id)

	if _, err := os.Stat("/tmp/filestore/" + id + ".json"); err == nil || !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted")
	}

}
