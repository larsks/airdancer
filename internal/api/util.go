package api

import (
	"fmt"
	"strings"
)

// ErrorCollector accumulates errors and provides a unified way to handle multiple errors
type ErrorCollector struct {
	errors []error
}

// NewErrorCollector creates a new ErrorCollector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]error, 0),
	}
}

// Add adds an error with optional context to the collector
func (ec *ErrorCollector) Add(context string, err error) {
	if err != nil {
		if context != "" {
			ec.errors = append(ec.errors, fmt.Errorf("%s: %w", context, err))
		} else {
			ec.errors = append(ec.errors, err)
		}
	}
}

// AddError adds an error without context
func (ec *ErrorCollector) AddError(err error) {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// HasErrors returns true if any errors have been collected
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

// Count returns the number of errors collected
func (ec *ErrorCollector) Count() int {
	return len(ec.errors)
}

// Result returns a combined error if any errors were collected, nil otherwise
func (ec *ErrorCollector) Result(context string) error {
	if len(ec.errors) == 0 {
		return nil
	}

	if len(ec.errors) == 1 {
		if context != "" {
			return fmt.Errorf("%s: %w", context, ec.errors[0])
		}
		return ec.errors[0]
	}

	// Multiple errors - create a summary
	errorStrings := make([]string, len(ec.errors))
	for i, err := range ec.errors {
		errorStrings[i] = err.Error()
	}

	combined := strings.Join(errorStrings, "; ")
	if context != "" {
		return fmt.Errorf("%s: %s", context, combined)
	}
	return fmt.Errorf("%s", combined)
}

// Errors returns the slice of collected errors
func (ec *ErrorCollector) Errors() []error {
	return ec.errors
}
