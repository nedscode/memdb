package memdb

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func reflectiveArray(search string, val reflect.Value, path []string) string {
	if search == "" {
		if val.CanInterface() {
			return fmt.Sprintf("%v", val.Interface())
		}
		return ""
	}

	pos, err := strconv.ParseInt(search, 10, 32)
	if err != nil {
		return ""
	}

	if int(pos) >= val.Len() {
		return ""
	}

	f := val.Index(int(pos))
	if f.CanInterface() {
		return reflective(f.Interface(), path[1:])
	} else if len(path) == 1 {
		return staticVal(f.Kind(), f)
	}
	return ""
}

func reflectiveStruct(search string, val reflect.Value, path []string) string {
	if search == "" {
		if val.CanInterface() {
			return fmt.Sprintf("%v", val.Interface())
		}
		return ""
	}

	val = reflect.Indirect(val)
	vt := val.Type()
	n := vt.NumField()
	for i := 0; i < n; i++ {
		ft := vt.Field(i)
		nom := strings.ToLower(ft.Name)
		if nom == search {
			f := val.Field(i)
			if f.CanInterface() {
				return reflective(f.Interface(), path[1:])
			} else if len(path) == 1 {
				return staticVal(f.Kind(), f)
			}
			return ""
		}
	}
	return ""
}

func reflective(a interface{}, path []string) string {
	search := ""
	if len(path) > 0 {
		search = strings.ToLower(path[0])
	}

	// Use reflection to find a field with the specific name (case insensitive)
	val := reflect.ValueOf(a)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}

	vk := val.Kind()
	switch vk {
	case reflect.Struct:
		return reflectiveStruct(search, val, path)

	case reflect.Slice:
		if val.IsNil() {
			return ""
		}
		fallthrough
	case reflect.Array:
		return reflectiveArray(search, val, path)

	default:
		if search != "" {
			return ""
		}
		return staticVal(vk, val)
	}
}

func staticVal(vk reflect.Kind, val reflect.Value) string {
	switch vk {
	case reflect.Bool:
		if val.Bool() {
			return "true"
		}
		return "false"

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return strconv.FormatInt(val.Int(), 10)

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return strconv.FormatUint(val.Uint(), 10)

	case reflect.Float32:
		return strconv.FormatFloat(val.Float(), 'g', 10, 32)

	case reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'g', 10, 64)

	default:
		if val.CanInterface() {
			return fmt.Sprintf("%v", val.Interface())
		}
		return fmt.Sprintf("%v", val.String())
	}
}
