package util

import (
	"fmt"
	"io"
	"strings"
)

type Item struct {
	Key   string
	Value string
}

type Dict []Item

func (d Dict) Keys() []string {
	s := make([]string, len(d))
	for i, v := range d {
		s[i] = v.Key
	}
	return s
}

func (d Dict) Values() []string {
	s := make([]string, len(d))
	for i, v := range d {
		s[i] = v.Value
	}
	return s
}

func (d Dict) Print(w io.Writer, sep string, args ...interface{}) error {
	if len(args) != len(d) {
		return fmt.Errorf("cannot print: wrong number of arguments")
	}
	_, err := fmt.Fprintf(w, strings.Join(d.Values(), sep)+"\n", args...)
	return err
}
