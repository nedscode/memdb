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

type Y struct {
	Bad chan int `json:"Bad"`
}

func (y *Y) Less(o memdb.Indexer) bool {
	return false
}

func (y *Y) IsExpired() bool {
	return false
}

func (y *Y) GetField(f string) string {
	return ""
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

	s.Load(func(idIn string, indexer interface{}) {
		if idIn != id {
			t.Errorf("Didn't get expected UID on load %s (expected %s)", idIn, id)
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

func TestInvalidFolder(t *testing.T) {
	_, err := NewFileStorage("/invalid/directory", func(indexerType string) interface{} {
		return nil
	})

	if err == nil {
		t.Errorf("Expected error on invalid directory")
	}
}

func TestUnauthFolder(t *testing.T) {
	_, err := NewFileStorage("/dev/zero", func(indexerType string) interface{} {
		return nil
	})

	if err == nil {
		t.Errorf("Expected error on unauthorized folder")
	}
}

func TestSaveUnmarshalable(t *testing.T) {
	s, err := NewFileStorage("/tmp/filestore", func(indexerType string) interface{} {
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error creating new storage: %#v", err)
	}

	bad := &Y{}
	if err = s.Save("123456789012", bad); err == nil {
		t.Errorf("Expected error saving unmarshalable indexer")
	}
}

func TestSaveUnwriteable(t *testing.T) {
	s, err := NewFileStorage("/tmp/filestore", func(indexerType string) interface{} {
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error creating new storage: %#v", err)
	}

	s.folder = "/dev/zero"
	test := &X{}
	if err = s.Save("123456789012", test); err == nil {
		t.Errorf("Expected error saving to bad folder")
	}
}

func TestLoadUnreadable(t *testing.T) {
	s, err := NewFileStorage("/tmp/filestore", func(indexerType string) interface{} {
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error creating new storage: %#v", err)
	}

	s.folder = "/dev/zero"
	if err = s.Load(func(id string, indexer interface{}) {}); err == nil {
		t.Errorf("Expected error saving to bad folder")
	}
}

func TestLoadUnparse(t *testing.T) {
	s, err := NewFileStorage("/tmp/filestore", func(indexerType string) interface{} {
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error creating new storage: %#v", err)
	}

	_, err = s.readFile("/non-existent")
	if err == nil {
		t.Errorf("Expected error reading non-existent file")
	}

	_, err = s.getContainer([]byte("NotJSON"))
	if err == nil {
		t.Errorf("Expected error reading bad JSON")
	}

	_, err = s.newItem("Unknown")
	if err == nil {
		t.Errorf("Expected error factorying unknown type")
	}

	err = s.unmarshalItem([]byte("NotJSON"), &Y{})
	if err == nil {
		t.Errorf("Expected error getting item from bad JSON")
	}

	err = s.removeFile("/tmp")
	if err == nil {
		t.Errorf("Expected error removing not-a-file")
	}
}
