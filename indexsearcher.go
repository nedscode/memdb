package memdb

// IndexSearcher can return results from an index
type IndexSearcher interface {
	Each(cb Iterator, keys ...string)
	One(keys ...string) interface{}
	Lookup(keys ...string) []interface{}
	All() []interface{}
	FieldKey(a interface{}) FieldKey
	Stats(keys ...string) []Stats
	_id() string
}
