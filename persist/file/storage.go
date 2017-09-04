package filepersist

import (
	"github.com/nedscode/memdb/persist"

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
	factory persist.FactoryFunc
}

// NewFileStorage creates a new Storage Persister at the designated folder
// folder is the directory to store the files in
// factory is a factory function that can instantiate a new instance of an Indexer
func NewFileStorage(folder string, factory persist.FactoryFunc) (*Storage, error) {
	if err := os.MkdirAll(folder, 0755); err != nil && os.IsNotExist(err) {
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

func (s *Storage) writeFile(name string, data []byte) error {
	err := ioutil.WriteFile(name, data, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write indexer object to file %s\n%#v\n", name, err)
	}
	return nil
}

// Save is an implementation of the Persister.Save method
func (s *Storage) Save(id string, indexer interface{}) error {
	data, err := json.Marshal(indexer)
	if err != nil {
		return fmt.Errorf("Indexer objects must be JSON marshallable to use FilePersist storage\n%#v\n", err)
	}

	data, _ = json.Marshal(&container{
		ID:   id,
		Type: fmt.Sprintf("%T", indexer),
		Item: data,
	})

	name := path.Join(s.folder, id+".json")
	return s.writeFile(name, data)
}

func (s *Storage) readFile(name string) ([]byte, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("Unable to read file %s: %#v", name, err)
	}
	return data, nil
}

func (s *Storage) getContainer(data []byte) (*container, error) {
	c := &container{}
	err := json.Unmarshal(data, c)
	if err != nil {
		err = fmt.Errorf("Unable to decode container: %#v", err)
	}
	return c, err
}

func (s *Storage) newItem(t string) (interface{}, error) {
	item := s.factory(t)
	if item == nil {
		return nil, fmt.Errorf("Unable to get factory for type %s", t)
	}
	return item, nil
}

func (s *Storage) unmarshalItem(data []byte, item interface{}) error {
	err := json.Unmarshal(data, item)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal item for type %T: %#v", item, err)
	}
	return nil
}

// Load is an implementation of the Persister.Load method
func (s *Storage) Load(loadFunc persist.LoadFunc) error {
	dir, err := ioutil.ReadDir(s.folder)
	if err != nil {
		return fmt.Errorf("Unable to read directory %s: %#v", s.folder, err)
	}

	var lastErr error
	for _, fi := range dir {
		nom := strings.Split(fi.Name(), ".")
		if len(nom) == 2 && len(nom[0]) == 12 && nom[1] == "json" {
			name := path.Join(s.folder, fi.Name())
			data, err := s.readFile(name)

			var (
				c    *container
				item interface{}
			)

			if err == nil {
				c, err = s.getContainer(data)
			}

			if err == nil {
				item, err = s.newItem(c.Type)
			}

			if err == nil {
				err = s.unmarshalItem(c.Item, item)
			}

			if err == nil {
				loadFunc(c.ID, item)
			}

			if err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

func (s *Storage) removeFile(name string) error {
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to remove file %s\n%#v\n", name, err)
	}
	return nil
}

// Remove is an implementation of the Persister.Remove method
func (s *Storage) Remove(id string) error {
	name := path.Join(s.folder, id+".json")
	return s.removeFile(name)
}
