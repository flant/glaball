// TODO: refactoring

package sort

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/armon/go-radix"
)

// Represents an index of struct field for (reflect.Value).FieldByIndex func
type FieldIndex []int

// Check if struct really has a field (it must have positive indices)
func (f FieldIndex) Negative() bool {
	for i := 0; i < len(f); i++ {
		return f[i] < 0
	}
	return true
}

// Check equality of FieldIndex slices
func (f FieldIndex) Equal(other FieldIndex) bool {
	if len(f) != len(other) {
		return false
	}
	for i, v := range f {
		if v != other[i] {
			return false
		}
	}
	return true
}

// Get actual value of struct field by its index
func FieldIndexValue(i FieldIndex, v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	// Get actual value if field is a pointer
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		panic(fmt.Errorf("invalid struct: %#v", v))
	}
	rf := rv.FieldByIndex(i)
	// Get actual value if field is a pointer
	if rf.Kind() == reflect.Ptr {
		rf = rf.Elem()
	}
	if !rf.IsValid() {
		panic(fmt.Errorf("invalid struct field: %q for %q", rf.Type().Name(), rv.Type()))
	}
	return rf.Interface()
}

// Get actual value of any valid struct field in _keys_ slice
// Panic if all fields are invalid or not found
func ValidFieldValue(t *radix.Tree, keys []string, v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	// Get actual value if field is a pointer
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	for _, k := range keys {
		if i, ok := t.Get(k); ok {
			idx := i.(FieldIndex)
			// Check if field really exists
			if idx.Negative() {
				continue
			}
			rf := rv.FieldByIndex(idx)
			// Get actual value if field is a pointer
			if rf.Kind() == reflect.Ptr {
				rf = rf.Elem()
			}
			// Ignore invalid fields
			if !rf.IsValid() {
				continue
			}
			return rf.Interface()
		}
	}

	panic(fmt.Sprintf("field not found: %q: %q", keys, rv.Type()))
}

// Create a radix tree of struct fields' json tags and their indices
func JsonFieldIndexTree(v interface{}) *radix.Tree {
	t := radix.New()
	jsonFieldIndexTree(reflect.TypeOf(v), t, nil, nil)
	return t
}

// Create a radix tree of struct fields' json tags and their indices
func jsonFieldIndexTree(rt reflect.Type, t *radix.Tree, prefix []string, index FieldIndex) {
	switch k := rt.Kind(); {
	case k == reflect.Ptr: // Get actual value if field is a pointer
		rt = rt.Elem()
	case k != reflect.Struct:
		panic(fmt.Errorf("invalid struct: %#q", rt))
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		ftyp := f.Type
		// Get actual value if field is a pointer
		if ftyp.Kind() == reflect.Ptr {
			ftyp = f.Type.Elem()
		}
		// Get json tag of each valid struct field and insert it into tree
		if tag, ok := f.Tag.Lookup("json"); ok {
			tagName := strings.Split(tag, ",")[0]
			// Set tag name like `parent.child`
			pfx := append(prefix, tagName)
			idx := append(index, i)
			t.Insert(strings.Join(pfx, "."), idx)
			// Go through if field is a struct
			if ftyp.Kind() == reflect.Struct {
				jsonFieldIndexTree(f.Type, t, pfx, idx)
			}
		}
	}
}
