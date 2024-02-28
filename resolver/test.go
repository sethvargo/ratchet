package resolver

import (
	"context"
	"fmt"
)

// Test is a test resolver. It accepts a pre-defined list of results and panics
// if asked to resolve an undefined reference.
type Test struct {
	data    map[string]*TestResult
	upgrade map[string]*TestResult
}

// TestResult represents the result of a resolution. If Err is not nil,
// Resolved is the empty string.
type TestResult struct {
	Resolved string
	Err      error
}

// NewTest creates a new test resolver.
func NewTest(data, upgrade map[string]*TestResult) (*Test, error) {
	if data == nil {
		data = make(map[string]*TestResult, 2)
	}
	return &Test{data: data, upgrade: upgrade}, nil
}

func (t *Test) Resolve(ctx context.Context, value string) (string, error) {
	v, ok := t.data[value]
	if !ok {
		panic(fmt.Sprintf("no test value for %q", value))
	}
	return v.Resolved, v.Err
}

func (t *Test) Upgrade(ctx context.Context, value string) (string, error) {
	v, ok := t.upgrade[value]
	if !ok {
		panic(fmt.Sprintf("no test value for %q", value))
	}
	return v.Resolved, v.Err
}
