package util

import (
	"fmt"
	"os"
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
