package util

import (
	"fmt"
	"strings"
)

type enumValue struct {
	v       *string
	options []string
}

func NewEnumValue(p *string, options ...string) *enumValue {
	return &enumValue{p, options}
}

func (f *enumValue) Set(s string) error {
	for _, v := range f.options {
		if v == s {
			*f.v = s
			return nil
		}
	}

	return fmt.Errorf("enum value must be one of %s, got '%s'", strings.Join(f.options, ","), s)
}

func (f *enumValue) String() string {
	return *f.v
}

func (f *enumValue) Type() string {
	return "string"
}
