package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
)

type boolPtrValue struct{ v **bool }

func NewBoolPtrValue(p **bool) *boolPtrValue {
	return &boolPtrValue{p}
}

func (f *boolPtrValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err == nil {
		*f.v = gitlab.Bool(v)
	}
	return err
}

func (f *boolPtrValue) Get() interface{} {
	if *f.v != nil {
		return **f.v
	}
	return nil
}

func (f *boolPtrValue) String() string {
	return fmt.Sprintf("%v", *f.v)
}

func (f *boolPtrValue) Type() string {
	return "bool"
}

type stringPtrValue struct{ v **string }

func NewStringPtrValue(p **string) *stringPtrValue {
	return &stringPtrValue{p}
}

func (f *stringPtrValue) Set(s string) error {
	if s != "" {
		*f.v = gitlab.String(s)
	}

	return nil
}

func (f *stringPtrValue) String() string {
	if *f.v == nil {
		return "<nil>"
	}
	return string(**f.v)
}

func (f *stringPtrValue) Type() string {
	return "string"
}

type enumPtrValue struct {
	v       **string
	options []string
}

func NewEnumPtrValue(p **string, options ...string) *enumPtrValue {
	return &enumPtrValue{p, options}
}

func (f *enumPtrValue) Set(s string) error {
	for _, v := range f.options {
		if v == s {
			*f.v = gitlab.String(s)
			return nil
		}
	}

	return fmt.Errorf("enum value must be one of %s, got '%s'", strings.Join(f.options, ","), s)
}

func (f *enumPtrValue) String() string {
	if *f.v == nil {
		return "<nil>"
	}
	return string(**f.v)
}

func (f *enumPtrValue) Type() string {
	return "string"
}

type timePtrValue struct{ v **time.Time }

func NewTimePtrValue(p **time.Time) *timePtrValue {
	return &timePtrValue{p}
}

func (f *timePtrValue) Set(s string) error {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		*f.v = gitlab.Time(t)
	}

	return err
}

func (f *timePtrValue) String() string {
	if *f.v == nil {
		return "<nil>"
	}
	t := **f.v
	return t.String()
}

func (f *timePtrValue) Type() string {
	return "date"
}

type intPtrValue struct{ v **int }

func NewIntPtrValue(p **int) *intPtrValue {
	return &intPtrValue{p}
}

func (f *intPtrValue) Set(s string) error {
	v, err := strconv.Atoi(s)
	if err == nil {
		*f.v = gitlab.Int(v)
	}

	return err
}

func (f *intPtrValue) String() string {
	if *f.v == nil {
		return "<nil>"
	}
	return fmt.Sprint(**f.v)
}

func (f *intPtrValue) Type() string {
	return "int"
}

type visibilityPtrValue struct{ v **gitlab.VisibilityValue }

func NewVisibilityPtrValue(p **gitlab.VisibilityValue) *visibilityPtrValue {
	return &visibilityPtrValue{p}
}

func (f *visibilityPtrValue) Set(s string) error {
	options := []string{
		string(gitlab.PrivateVisibility),
		string(gitlab.InternalVisibility),
		string(gitlab.PublicVisibility),
	}

	for _, opt := range options {
		if s == opt {
			*f.v = gitlab.Visibility(gitlab.VisibilityValue(opt))
			return nil
		}
	}

	return fmt.Errorf("visibility value must be one of %s, got '%s'", strings.Join(options, ","), s)

}

func (f *visibilityPtrValue) String() string {
	if *f.v == nil {
		return "<nil>"
	}
	return string(**f.v)
}

func (f *visibilityPtrValue) Type() string {
	return "string"
}
