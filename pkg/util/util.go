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

func ContainsString(slice []string, item string) bool {
	idx := sort.SearchStrings(slice, item)
	if idx == len(slice) {
		return false
	}
	return slice[idx] == item
}

func ContainsInt(slice []int, item int) bool {
	idx := sort.SearchInts(slice, item)
	if idx == len(slice) {
		return false
	}
	return slice[idx] == item
}
