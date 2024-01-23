package sort

import (
	"fmt"
	"reflect"

	"github.com/jmoiron/sqlx/reflectx"
)

func ValidFieldValue(keys []string, v interface{}) (interface{}, error) {
	m := mapper.TypeMap(reflect.TypeOf(v))
	rv := reflect.ValueOf(v)
	for _, k := range keys {
		if fi := m.GetByPath(k); fi != nil {
			fv := reflectx.FieldByIndexesReadOnly(rv, fi.Index)
			if fi.Field.Type.Kind() == reflect.Ptr {
				fv = fv.Elem()
			}
			if rfv := fv.Interface(); rfv != nil && rfv != v {
				return rfv, nil
			}
		}
	}
	return nil, fmt.Errorf("field not found: %q: %q", keys, rv.Type())
}

func ValidOrderBy(keys []string, v interface{}) bool {
	m := mapper.TypeMap(reflect.TypeOf(v))
	for _, k := range keys {
		if fi := m.GetByPath(k); fi != nil {
			return true
		}
	}
	return false
}
