package sort

import (
	"fmt"
	"reflect"

	"github.com/ahmetb/go-linq"
	"github.com/flant/glaball/pkg/client"
	"github.com/jmoiron/sqlx/reflectx"
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

	OrderBy    []string
	StructType interface{}
}

type Result struct {
	Count    int
	Key      string
	Elements Elements
	Cached   Cached
}

type Element struct {
	Host   *client.Host
	Struct interface{}
	Cached Cached
}

type Cached bool

func (c Cached) String() string {
	if c {
		return "yes"
	}
	return "no"
}

type Elements []interface{}

func (e Elements) Hosts() client.Hosts {
	s := make(client.Hosts, 0, len(e))
	for _, v := range e {
		s = append(s, v.(Element).Host)
	}
	return s
}

func (e Elements) Cached() (cached Cached) {
	for _, v := range e {
		if !v.(Element).Cached {
			return false
		}
	}
	return true
}

func (e Elements) Typed() []Element {
	s := make([]Element, 0, len(e))
	for _, v := range e {
		s = append(s, v.(Element))
	}
	return s
}

func FromChannel(ch chan interface{}, opt *Options) ([]Result, error) {
	results := make([]Result, 0)
	query, err := FromChannelQuery(ch, opt)
	if err != nil {
		return nil, err
	}

	query.ToSlice(&results)

	return results, nil
}

func FromChannelQuery(ch chan interface{}, opt *Options) (linq.Query, error) {
	var (
		query        linq.Query
		orderedQuery linq.OrderedQuery
		groupBy      *reflectx.FieldInfo
		first        *reflectx.FieldInfo
	)

	query = linq.FromChannel(ch)
	m := mapper.TypeMap(reflect.TypeOf(opt.StructType))
	m.Paths[byHostFI.Name] = &byHostFI
	m.Names[byHostFI.Name] = &byHostFI
	m.Paths[byLenFI.Path] = &byLenFI
	m.Names[byLenFI.Name] = &byLenFI

	if f := opt.GroupBy; f != "" {
		groupBy = m.GetByPath(f)
		if groupBy == nil {
			return linq.Query{}, fmt.Errorf("invalid struct field: %s", f)
		}
		query = query.GroupBy(GroupBy(groupBy))

	}

	if f := opt.OrderBy; f != nil {
		first = m.GetByPath(f[0])
		if first == nil {
			return linq.Query{}, fmt.Errorf("invalid struct field: %s", f[0])
		}
	}

	orderedQuery = query.OrderByDescending(OrderBy(groupBy, first))

	if groupBy != nil && first != nil && first.Name == byLenFI.Name {
		orderedQuery = query.OrderBy(ByLen())
		orderedQuery = orderedQuery.ThenByDescending(ByKey())
	}

	for _, key := range opt.OrderBy[1:] {
		v := m.GetByPath(key)
		if v == nil {
			return linq.Query{}, fmt.Errorf("invalid struct field: %s", key)
		}
		orderedQuery = orderedQuery.ThenByDescending(OrderBy(groupBy, v))
	}

	query = orderedQuery.Query
	if opt.SortBy == "asc" {
		query = orderedQuery.Reverse()
	}

	query = query.Select(func(i interface{}) interface{} {
		// Check if we have a group or single element
		switch v := i.(type) {
		case Element:
			var key interface{}
			// Use specific key if we want to order results by host project name
			switch first.Name {
			case byHostFI.Name:
				key = v.Host.Project
			default:
				rv := reflect.ValueOf(v.Struct)
				for _, k := range opt.OrderBy {
					key = reflectx.FieldByIndexesReadOnly(rv, m.GetByPath(k).Index).Interface()
					if key != nil && key != v.Struct {
						break
					}
				}
			}
			return Result{
				Count:    1, // Count of single element is always 1
				Key:      fmt.Sprint(key),
				Elements: Elements{v},
				Cached:   v.Cached,
			}
		case linq.Group:
			return Result{
				Count:    len(v.Group),
				Key:      fmt.Sprint(v.Key),
				Elements: Elements(v.Group),
				Cached:   Elements(v.Group).Cached(),
			}
		default:
			return v
		}
	})

	return query, nil
}

func GroupBy(groupBy *reflectx.FieldInfo) (func(i interface{}) interface{}, func(i interface{}) interface{}) {
	return func(i interface{}) interface{} {
			return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(i.(Element).Struct), groupBy.Index).Interface()
		},
		func(i interface{}) interface{} { return i }
}

func OrderBy(groupBy, orderBy *reflectx.FieldInfo) func(i interface{}) interface{} {
	if orderBy.Name == byHostFI.Name {
		// Return Host.Project name
		return ByHost()
	}

	if orderBy.Name == byLenFI.Name {
		// Return length of the group if we have GroupBy key
		if groupBy != nil {
			return ByLen()
		}
		// Return count == 1 if we don't have any groups
		return func(i interface{}) interface{} { return 1 }
	}

	// Order by GroupBy key if it exists
	if groupBy != nil {
		return ByKey()
	}

	// Otherwise order by other keys
	return ByFieldIndex(orderBy)
}

// Order by count
func ByLen() func(i interface{}) interface{} {
	return func(i interface{}) interface{} { return len(i.(linq.Group).Group) }
}

// Order by GroupBy key
func ByKey() func(i interface{}) interface{} {
	return func(i interface{}) interface{} { return i.(linq.Group).Key }
}

// Order by Host.Project key
func ByHost() func(i interface{}) interface{} {
	return func(i interface{}) interface{} {
		if v, ok := i.(Element); ok {
			return v.Host
		}

		return Elements(i.(linq.Group).Group).Hosts().Projects(true)[0]
	}
}

// Order by the field
func ByFieldIndex(fi *reflectx.FieldInfo) func(i interface{}) interface{} {
	return func(i interface{}) interface{} {
		if v, ok := i.(Element); ok {
			return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(v.Struct), fi.Index).Interface()
		}

		return reflectx.FieldByIndexesReadOnly(reflect.ValueOf(i), fi.Index).Interface()
	}
}
