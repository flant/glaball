package sort

import (
	"fmt"

	"github.com/flant/glaball/pkg/client"

	"github.com/ahmetb/go-linq/v3"
)

var (
	// We use negative values because these fields don't exist in any struct
	byHost   = FieldIndex{-1, 3}
	byLen    = FieldIndex{-1, 2}
	empty    = FieldIndex{-1, 1}
	notFound = FieldIndex{-1, 0}
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

func FromChannel(ch chan interface{}, opt *Options) (results []Result) {
	FromChannelQuery(ch, opt).ToSlice(&results)
	return results
}

func FromChannelQuery(ch chan interface{}, opt *Options) linq.Query {
	var orderedQuery linq.OrderedQuery
	// Initialize GroupBy key with `empty` value
	groupBy := empty
	// Initialize _first_ OrderBy key with `empty` value
	first := empty

	// Create `json` tag names tree of current struct fields
	// to get their values fast (by index)
	t := JsonFieldIndexTree(opt.StructType)
	// Add some additional fields
	t.Insert("host", byHost)
	t.Insert("count", byLen)

	query := linq.FromChannel(ch)
	if opt.GroupBy != "" {
		groupBy = notFound
		// Set GroupBy key to some field name if struct has it
		if v, ok := t.Get(opt.GroupBy); ok {
			groupBy = v.(FieldIndex)
		}
		query = query.GroupBy(GroupBy(groupBy))
	}

	if opt.OrderBy != nil {
		first = notFound
		// Set _first_ OrderBy key to some field name if struct has it
		if v, ok := t.Get(opt.OrderBy[0]); ok {
			first = v.(FieldIndex)
		}
	}

	switch o := opt; {
	case o.GroupBy != "" && o.OrderBy[0] == "count":
		orderedQuery = query.OrderBy(ByLen())                 // Order by count first if it is declared
		orderedQuery = orderedQuery.ThenByDescending(ByKey()) // and then by GroupBy key
	default:
		orderedQuery = query.OrderByDescending(OrderBy(groupBy, first)) // Otherwise use GroupBy key and _first_ OrderBy key
	}

	// Add additional order if we have other OrderBy keys
	for _, key := range opt.OrderBy[1:] {
		idx := notFound
		if v, ok := t.Get(key); ok {
			idx = v.(FieldIndex)
		}
		orderedQuery = orderedQuery.ThenByDescending(OrderBy(groupBy, idx))
	}

	// Set ascending or descending order. Default is descending.
	switch opt.SortBy {
	case "asc":
		query = orderedQuery.Reverse()
	default:
		query = orderedQuery.Query
	}

	query = query.Select(func(i interface{}) interface{} {
		// Check if we have a group or single element
		switch v := i.(type) {
		case Element:
			var key interface{}
			// Use specific key if we want to order results by host project name
			switch opt.OrderBy[0] {
			case "host":
				key = v.Host.Project
			default:
				key = ValidFieldValue(t, opt.OrderBy, v.Struct)
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

	return query
}

func GroupBy(groupBy FieldIndex) (func(i interface{}) interface{}, func(i interface{}) interface{}) {
	return func(i interface{}) interface{} {
			return FieldIndexValue(groupBy, i.(Element).Struct)
		},
		func(i interface{}) interface{} { return i }
}

func OrderBy(groupBy, orderBy FieldIndex) func(i interface{}) interface{} {
	switch v := orderBy; {
	case v.Equal(byHost):
		// Return Host.Project name
		return ByHost()
	case v.Equal(byLen):
		// Return length of the group if we have GroupBy key
		if !groupBy.Equal(empty) {
			return ByLen()
		}
		// Return count == 1 if we don't have any groups
		return func(i interface{}) interface{} { return 1 }
	case orderBy.Equal(notFound):
		// Return length of the group if we have GroupBy key
		if !groupBy.Equal(empty) {
			return ByLen()
		}
		// Return count == 1 if we don't have any groups
		return func(i interface{}) interface{} { return 1 }
	}

	// Order by GroupBy key if it exists
	if !groupBy.Equal(empty) {
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
func ByFieldIndex(n FieldIndex) func(i interface{}) interface{} {
	return func(i interface{}) interface{} {
		if v, ok := i.(Element); ok {
			return FieldIndexValue(n, v.Struct)
		}

		return FieldIndexValue(n, i)
	}
}
