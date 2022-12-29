package util

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func AskUser(msg string) bool {
	var q string

	fmt.Printf("%s [y/N] ", msg)
	fmt.Scanln(&q)

	if len(q) > 0 &&
		strings.ToLower(q[:1]) == "y" {
		return true
	}

	fmt.Println("Aborted")
	os.Exit(0)
	return false
}

// The slice must be sorted in ascending order
func ContainsString(slice []string, item string) bool {
	if !sort.StringsAreSorted(slice) {
		sort.Strings(slice)
	}
	idx := sort.SearchStrings(slice, item)
	if idx == len(slice) {
		return false
	}
	return slice[idx] == item
}

// The slice must be sorted in ascending order
func ContainsInt(slice []int, item int) bool {
	if !sort.IntsAreSorted(slice) {
		sort.Ints(slice)
	}
	idx := sort.SearchInts(slice, item)
	if idx == len(slice) {
		return false
	}
	return slice[idx] == item
}

func InsertString(slice []string, item string) []string {
	if idx := sort.SearchStrings(slice, item); idx == len(slice) {
		slice = append(slice, item)
	} else {
		slice = append(slice[:idx+1], slice[idx:]...)
		slice[idx] = item
	}
	return slice
}
