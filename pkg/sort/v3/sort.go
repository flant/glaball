package sort

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/flant/glaball/pkg/client"
	"github.com/hashicorp/go-hclog"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/samber/lo"
)

var (
	mapper = reflectx.NewMapper("json")

	byHostFI = reflectx.FieldInfo{
		Path: "host",
		Name: "host",
	}

	byLenFI = reflectx.FieldInfo{
		Path: "count",
		Name: "count",
	}
)

type Options struct {
	SortBy, GroupBy string

	OrderBy []string
}

type Result[T any] struct {
	Key      string
	Elements Elements[T]
	Cached   Cached
}

func (r Result[T]) Count() int {
	return len(r.Elements)
}

type Element[T any] struct {
	Host   *client.Host
	Struct T
	Cached Cached
}

type Cached bool

func (c Cached) String() string {
	if c {
		return "yes"
	}
	return "no"
}

type Elements[T any] []Element[T]

func (e Elements[T]) Hosts() client.Hosts {
	s := make(client.Hosts, len(e))
	for i, v := range e {
		s[i] = v.Host
	}
	return s
}

func (e Elements[T]) Cached() Cached {
	for _, v := range e {
		if !v.Cached {
			return false
		}
	}
	return true
}

type ByLen[T any] []Result[T]

func (a ByLen[T]) Len() int           { return len(a) }
func (a ByLen[T]) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLen[T]) Less(i, j int) bool { return a[i].Count() < a[j].Count() }

func FromChannel[T any](ch <-chan Element[T], opt *Options) ([]Result[T], error) {
	var (
		query   = make(map[string][]Element[T])
		groupBy *reflectx.FieldInfo
		first   *reflectx.FieldInfo
		typ     T
	)
	rt := reflect.TypeOf(typ)
	m := mapper.TypeMap(rt)
	m.Paths[byHostFI.Name] = &byHostFI
	m.Names[byHostFI.Name] = &byHostFI
	m.Paths[byLenFI.Path] = &byLenFI
	m.Names[byLenFI.Name] = &byLenFI

	// TODO: check for errors before http requests
	validFieldsFI := make([]*reflectx.FieldInfo, 0, len(opt.OrderBy))
	if f := opt.OrderBy; f != nil {
		for i := 0; i < len(f); i++ {
			if fi := m.GetByPath(f[i]); fi != nil {
				validFieldsFI = append(validFieldsFI, fi)
			}
		}
		if len(validFieldsFI) == 0 {
			return nil, fmt.Errorf("invalid struct field: %q: %q", opt.OrderBy, rt)
		}
		first = validFieldsFI[0]
	}

	byKeyFn := func(p1, p2 *Result[T]) bool {
		return p1.Key < p2.Key
	}

	byHostFn := func(p1, p2 *Result[T]) bool {
		return p1.Elements.Hosts().Projects(true)[0] < p2.Elements.Hosts().Projects(true)[0]
	}

	byLenFn := func(p1, p2 *Result[T]) bool {
		return p1.Count() < p2.Count()
	}

	// byFieldIndexFn := func(p1, p2 *Element[T], fi *reflectx.FieldInfo) bool {
	// 	return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(p1.Struct), fi.Index).String() <
	// 		reflectx.FieldByIndexesReadOnly(reflect.ValueOf(p2.Struct), fi.Index).String()
	// }

	var ms *multiSorter[T]
	switch {
	case first.Name == byHostFI.Name:
		// Return Host.Project name
		ms = OrderedBy[T](byHostFn)
	case first.Name == byLenFI.Name:
		// Return length of the group
		ms = OrderedBy[T](byLenFn)
	// case groupBy != nil:
	// 	ms = OrderedBy[T](byKeyFn)
	default:
		ms = OrderedBy[T](byKeyFn)
	}

	// for _, key := range opt.OrderBy[1:] {
	// 	v := m.GetByPath(key)
	// 	if v == nil {
	// 		return nil, fmt.Errorf("invalid struct field: %s", key)
	// 	}
	// 	ms.less = append(ms.less, byFieldIndexFn[T](v.Index))
	// }

	if groupBy != nil && first != nil && first.Name == byLenFI.Name {
		ms = OrderedBy[T](byLenFn, byKeyFn)
	}
	ms.reverse = opt.SortBy == "asc"

	slice := lo.ChannelToSlice(ch)
	// TODO: multisorter for elements[]
	// ms.Sort(slice)

	if opt.GroupBy != "" {
		groupBy = m.GetByPath(opt.GroupBy)
		if groupBy == nil {
			return nil, fmt.Errorf("invalid struct field: %s", opt.GroupBy)
		}
		query = lo.GroupBy(slice, func(item Element[T]) string {
			return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(item.Struct), groupBy.Index).String()
		})
		result := lo.MapToSlice(query, func(key string, value []Element[T]) Result[T] {
			return Result[T]{
				Key:      key,
				Elements: value,
				Cached:   Elements[T](value).Cached(),
			}
		})

		// TODO: sort results

		return result, nil
	} else {
		// TODO: sort slice
		result := make([]Result[T], len(slice))
		for i, v := range slice {
			rv := reflect.ValueOf(v.Struct)
			for _, fi := range validFieldsFI {
				// TODO: тут должно быть что-то другое, множественная сортировка так не будет работать
				if fv := reflectx.FieldByIndexesReadOnly(rv, fi.Index).Interface(); fv != nil {
					hclog.L().Debug("item", "field", hclog.Fmt("%v", fv))
					result[i] = Result[T]{
						Key:      fmt.Sprintf("%v", fv),
						Elements: []Element[T]{v},
						Cached:   Elements[T]([]Element[T]{v}).Cached(),
					}
				}
			}

		}
		return result, nil
		// NOT VALID
		hclog.L().Debug("got slice", "len", len(slice))
		query = lo.SliceToMap(slice, func(item Element[T]) (string, []Element[T]) {
			rv := reflect.ValueOf(item.Struct)
			// TODO: not valid if duplicate
			for _, fi := range validFieldsFI {
				if fv := reflectx.FieldByIndexesReadOnly(rv, fi.Index).Interface(); fv != nil {
					hclog.L().Debug("item", "field", hclog.Fmt("%v", fv))
					return fmt.Sprintf("%v", fv), []Element[T]{item}
				}
			}
			// should never panic
			panic(fmt.Errorf("field not found: %q: %q", opt.OrderBy, rv.Type()))
		})
	}
	return nil, nil
}

func NewSorter[T any](keys ...sort.Interface) *Sorter[T] {
	return &Sorter[T]{
		keys: keys,
	}
}

type Sorter[T any] struct {
	keys    []sort.Interface
	reverse bool
}

func (s *Sorter[T]) Sort(data []Result[T]) {
	sort.Slice(data, func(i, j int) bool {
		for _, key := range s.keys {
			if s.reverse {
				// TODO: data
				key = sort.Reverse(key)
			}
			if key.Less(i, j) {
				return true
			}
			if key.Less(j, i) {
				return false
			}
		}
		return false
	})
}

func (s *Sorter[T]) Add(keys ...sort.Interface) {
	s.keys = append(s.keys, keys...)
}

func byFieldIndexFn[T any](fi *reflectx.FieldInfo) func(p1, p2 *Element[T]) bool {
	return func(p1, p2 *Element[T]) bool {
		return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(p1.Struct), fi.Index).String() <
			reflectx.FieldByIndexesReadOnly(reflect.ValueOf(p2.Struct), fi.Index).String()
	}
}

// func FromChannelQuery[T any](ch chan T, opt *Options) (linq.Query, error) {
// 	var (
// 		query        linq.Query
// 		orderedQuery linq.OrderedQuery
// 		groupBy      *reflectx.FieldInfo
// 		first        *reflectx.FieldInfo
// 	)

// 	query = linq.FromChannel(ch)
// 	loq := lo.ChannelToSlice[T](ch)

// 	var typ [0]T
// 	m := mapper.TypeMap(reflect.TypeOf(typ))
// 	m.Paths[byHostFI.Name] = &byHostFI
// 	m.Names[byHostFI.Name] = &byHostFI
// 	m.Paths[byLenFI.Path] = &byLenFI
// 	m.Names[byLenFI.Name] = &byLenFI

// 	if f := opt.GroupBy; f != "" {
// 		groupBy = m.GetByPath(f)
// 		if groupBy == nil {
// 			return linq.Query{}, fmt.Errorf("invalid struct field: %s", f)
// 		}
// 		query = query.GroupBy(GroupBy[T](groupBy))

// 	}

// 	if f := opt.OrderBy; f != nil {
// 		first = m.GetByPath(f[0])
// 		if first == nil {
// 			return linq.Query{}, fmt.Errorf("invalid struct field: %s", f[0])
// 		}
// 	}

// 	orderedQuery = query.OrderByDescending(OrderBy[T](groupBy, first))

// 	if groupBy != nil && first != nil && first.Name == byLenFI.Name {
// 		orderedQuery = query.OrderBy(ByLen())
// 		orderedQuery = orderedQuery.ThenByDescending(ByKey())
// 	}

// 	for _, key := range opt.OrderBy[1:] {
// 		v := m.GetByPath(key)
// 		if v == nil {
// 			return linq.Query{}, fmt.Errorf("invalid struct field: %s", key)
// 		}
// 		orderedQuery = orderedQuery.ThenByDescending(OrderBy[T](groupBy, v))
// 	}

// 	query = orderedQuery.Query
// 	if opt.SortBy == "asc" {
// 		query = orderedQuery.Reverse()
// 	}

// 	query = query.Select(func(i interface{}) interface{} {
// 		// Check if we have a group or single element
// 		switch v := i.(type) {
// 		case Element[T]:
// 			key, err := ValidFieldValue(opt.OrderBy, v.Struct)
// 			if err != nil {
// 				return nil
// 			}
// 			return Result[T]{
// 				Count:    1, // Count of single element is always 1
// 				Key:      fmt.Sprint(key),
// 				Elements: Elements[T]{v},
// 				Cached:   v.Cached,
// 			}
// 		case linq.Group:
// 			// TODO:remove overhead
// 			elements := make(Elements[T], 0, len(v.Group))
// 			for _, v := range v.Group {
// 				elements = append(elements, v.(Element[T]))
// 			}
// 			//
// 			return Result[T]{
// 				Count:    len(v.Group),
// 				Key:      fmt.Sprint(v.Key),
// 				Elements: elements,
// 				Cached:   elements.Cached(),
// 			}
// 		default:
// 			return v
// 		}
// 	})

// 	return query, nil
// }

func GroupBy[T any](groupBy *reflectx.FieldInfo) (func(i interface{}) interface{}, func(i interface{}) interface{}) {
	return func(i interface{}) interface{} {
			return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(i.(Element[T]).Struct), groupBy.Index).Interface()
		},
		func(i interface{}) interface{} { return i }
}

// func OrderBy[T any](groupBy, orderBy *reflectx.FieldInfo) func(i interface{}) interface{} {
// 	if orderBy.Name == byHostFI.Name {
// 		// Return Host.Project name
// 		return ByHost[T]()
// 	}

// 	if orderBy.Name == byLenFI.Name {
// 		// Return length of the group if we have GroupBy key
// 		if groupBy != nil {
// 			return ByLen()
// 		}
// 		// Return count == 1 if we don't have any groups
// 		return func(i interface{}) interface{} { return 1 }
// 	}

// 	// Order by GroupBy key if it exists
// 	if groupBy != nil {
// 		return ByKey()
// 	}

// 	// Otherwise order by other keys
// 	return ByFieldIndex[T](orderBy)
// }

// Order by count
// func ByLen() func(i interface{}) interface{} {
// 	return func(i interface{}) interface{} { return len(i.(linq.Group).Group) }
// }

// Order by GroupBy key
// func ByKey() func(i interface{}) interface{} {
// 	return func(i interface{}) interface{} { return strings.ToLower(fmt.Sprint(i.(linq.Group).Key)) }
// }

// Order by Host.Project key
// func ByHost[T any]() func(i interface{}) interface{} {
// 	return func(i interface{}) interface{} {
// 		if v, ok := i.(Element[T]); ok {
// 			return v.Host
// 		}

// 		return Elements[T](i.(linq.Group).Group).Hosts().Projects(true)[0]
// 	}
// }

// Order by the field
func ByFieldIndex[T any](fi *reflectx.FieldInfo) func(i interface{}) interface{} {
	return func(i interface{}) interface{} {
		if v, ok := i.(Element[T]); ok {
			return strings.ToLower(fmt.Sprint(reflectx.FieldByIndexesReadOnly(reflect.ValueOf(v.Struct), fi.Index).Interface()))
		}

		return strings.ToLower(fmt.Sprint(reflectx.FieldByIndexesReadOnly(reflect.ValueOf(i), fi.Index).Interface()))
	}
}
