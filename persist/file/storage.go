package filepersist

import (
	"github.com/nedscode/memdb"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Storage is a simple memdb Persister that stores and loads files as JSON from a folder on a drive somewhere,
// to use this persister, you should ensure your Indexers are JSON Marshalable.
type Storage struct {
	folder  string
	factory memdb.FactoryFunc
}

// NewFileStorage creates a new Storage Persister at the designated folder
// folder is the directory to store the files in
// factory is a factory function that can instantiate a new instance of an Indexer
func NewFileStorage(folder string, factory memdb.FactoryFunc) (*Storage, error) {
	if err := os.Mkdir(folder, 0755); err != nil && os.IsNotExist(err) {
		return nil, err
	}

	test := path.Join(folder, "test")
	if err := ioutil.WriteFile(test, []byte("test"), 0644); err != nil {
		return nil, err
	}
	os.Remove(test)

	return &Storage{
		folder:  folder,
		factory: factory,
	}, nil
}

type container struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Item json.RawMessage `json:"item"`
}

// Save is an implementation of the Persister.Save method
func (s *Storage) Save(id string, indexer memdb.Indexer) {
	data, err := json.Marshal(indexer)
	if err != nil {
		fmt.Printf("Indexer objects must be JSON marshallable to use FilePersist\n%#v\n", err)
		return
	}

	data, err = json.Marshal(&container{
		ID:   id,
		Type: fmt.Sprintf("%T", indexer),
		Item: data,
	})
	if err != nil {
		fmt.Printf("Unable to marshal container\n%#v\n", err)
		return
	}

	name := path.Join(s.folder, id+".json")
	err = ioutil.WriteFile(name, data, 0644)
	if err != nil {
		fmt.Printf("Failed to write indexer object to file %s\n%#v\n", name, err)
		return
	}
}

// Load is an implementation of the Persister.Load method
func (s *Storage) Load(loadFunc memdb.LoadFunc) {
	dir, err := ioutil.ReadDir(s.folder)
	if err != nil {
		fmt.Printf("Unable to read directory %s\n%#v\n", s.folder, err)
		return
	}

	for _, fi := range dir {
		nom := strings.Split(fi.Name(), ".")
		if len(nom) == 2 && len(nom[0]) == 12 && nom[1] == "json" {
			name := path.Join(s.folder, fi.Name())

			data, err := ioutil.ReadFile(name)
			if err != nil {
				fmt.Printf("Unable to read file %s\n%#v\n", name, err)
				continue
			}

			c := &container{}
			err = json.Unmarshal(data, c)
			if err != nil {
				fmt.Printf("Unable to decode container\n%#v\n", err)
				continue
			}

			item := s.factory(c.Type)
			if item == nil {
				fmt.Printf("Unable to get factory for type %s\n", c.Type)
				continue
			}

			err = json.Unmarshal(c.Item, item)
			if err != nil {
				fmt.Printf("Unable to unmarshal item for type %s\n%#v\n", c.Type, err)
				continue
			}

			if indexer, ok := item.(memdb.Indexer); ok {
				loadFunc(c.ID, indexer)
			} else {
				fmt.Printf("Unable to load item of type %s (%T). It is not a Indexer\n", c.Type, item)
			}
		}
	}
}

// Remove is an implementation of the Persister.Remove method
func (s *Storage) Remove(id string) {
	name := path.Join(s.folder, id+".json")
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Failed to remove file %s\n%#v\n", name, err)
		return
	}
}
