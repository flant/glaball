// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort

import (
	"sort"
)

type lessFunc[T any] func(p1, p2 *Result[T]) bool

// multiSorter implements the Sort interface, sorting the elements within.
type multiSorter[T any] struct {
	elements []Result[T]
	less     []lessFunc[T]
	reverse  bool
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (ms *multiSorter[T]) Sort(elements []Result[T]) {
	ms.elements = elements
	if ms.reverse {
		sort.Sort(sort.Reverse(ms))
	} else {
		sort.Sort(ms)
	}
}

// OrderedBy returns a Sorter that sorts using the less functions, in order.
// Call its Sort method to sort the data.
func OrderedBy[T any](less ...lessFunc[T]) *multiSorter[T] {
	return &multiSorter[T]{
		less: less,
	}
}

// Len is part of sort.Interface.
func (ms *multiSorter[T]) Len() int {
	return len(ms.elements)
}

// Swap is part of sort.Interface.
func (ms *multiSorter[T]) Swap(i, j int) {
	ms.elements[i], ms.elements[j] = ms.elements[j], ms.elements[i]
}

// Less is part of sort.Interface. It is implemented by looping along the
// less functions until it finds a comparison that discriminates between
// the two items (one is less than the other). Note that it can call the
// less functions twice per call. We could change the functions to return
// -1, 0, 1 and reduce the number of calls for greater efficiency: an
// exercise for the reader.
func (ms *multiSorter[T]) Less(i, j int) bool {
	p, q := &ms.elements[i], &ms.elements[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			// p < q, so we have a decision.
			return true
		case less(q, p):
			// p > q, so we have a decision.
			return false
		}
		// p == q; try the next comparison.
	}
	// All comparisons to here said "equal", so just return whatever
	// the final comparison reports.
	return ms.less[k](p, q)
}

func (ms *multiSorter[T]) Chain(less ...lessFunc[T]) {
	ms.less = append(ms.less, less...)
}

// ExampleMultiKeys demonstrates a technique for sorting a struct type using different
// sets of multiple fields in the comparison. We chain together "Less" functions, each of
// which compares a single field.
// func Example_sortMultiKeys() {
// 	// Closures that order the Change structure.
// 	user := func(c1, c2 *Change) bool {
// 		return c1.user < c2.user
// 	}
// 	language := func(c1, c2 *Change) bool {
// 		return c1.language < c2.language
// 	}
// 	increasingLines := func(c1, c2 *Change) bool {
// 		return c1.lines < c2.lines
// 	}
// 	decreasingLines := func(c1, c2 *Change) bool {
// 		return c1.lines > c2.lines // Note: > orders downwards.
// 	}

// 	// Simple use: Sort by user.
// 	OrderedBy(user).Sort(changes)
// 	fmt.Println("By user:", changes)

// 	// More examples.
// 	OrderedBy(user, increasingLines).Sort(changes)
// 	fmt.Println("By user,<lines:", changes)

// 	OrderedBy(user, decreasingLines).Sort(changes)
// 	fmt.Println("By user,>lines:", changes)

// 	OrderedBy(language, increasingLines).Sort(changes)
// 	fmt.Println("By language,<lines:", changes)

// 	OrderedBy(language, increasingLines, user).Sort(changes)
// 	fmt.Println("By language,<lines,user:", changes)

// Output:
// By user: [{dmr C 100} {glenda Go 200} {gri Go 100} {gri Smalltalk 80} {ken C 150} {ken Go 200} {r Go 100} {r C 150} {rsc Go 200}]
// By user,<lines: [{dmr C 100} {glenda Go 200} {gri Smalltalk 80} {gri Go 100} {ken C 150} {ken Go 200} {r Go 100} {r C 150} {rsc Go 200}]
// By user,>lines: [{dmr C 100} {glenda Go 200} {gri Go 100} {gri Smalltalk 80} {ken Go 200} {ken C 150} {r C 150} {r Go 100} {rsc Go 200}]
// By language,<lines: [{dmr C 100} {ken C 150} {r C 150} {gri Go 100} {r Go 100} {glenda Go 200} {ken Go 200} {rsc Go 200} {gri Smalltalk 80}]
// By language,<lines,user: [{dmr C 100} {ken C 150} {r C 150} {gri Go 100} {r Go 100} {glenda Go 200} {ken Go 200} {rsc Go 200} {gri Smalltalk 80}]

// }
