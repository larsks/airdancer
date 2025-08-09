package api

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorCollector_NewErrorCollector(t *testing.T) {
	ec := NewErrorCollector()
	if ec == nil {
		t.Fatal("NewErrorCollector returned nil")
	}
	if ec.HasErrors() {
		t.Error("New ErrorCollector should not have errors")
	}
	if ec.Count() != 0 {
		t.Errorf("New ErrorCollector should have count 0, got %d", ec.Count())
	}
}

func TestErrorCollector_Add(t *testing.T) {
	ec := NewErrorCollector()
	
	// Test adding nil error (should be ignored)
	ec.Add("context", nil)
	if ec.HasErrors() {
		t.Error("Adding nil error should not create errors")
	}
	
	// Test adding error with context
	err1 := errors.New("test error 1")
	ec.Add("switch1", err1)
	if !ec.HasErrors() {
		t.Error("Adding error should set HasErrors to true")
	}
	if ec.Count() != 1 {
		t.Errorf("Expected count 1, got %d", ec.Count())
	}
	
	// Test adding error without context
	err2 := errors.New("test error 2")
	ec.Add("", err2)
	if ec.Count() != 2 {
		t.Errorf("Expected count 2, got %d", ec.Count())
	}
}

func TestErrorCollector_AddError(t *testing.T) {
	ec := NewErrorCollector()
	
	// Test adding nil error (should be ignored)
	ec.AddError(nil)
	if ec.HasErrors() {
		t.Error("Adding nil error should not create errors")
	}
	
	// Test adding error
	err := errors.New("test error")
	ec.AddError(err)
	if !ec.HasErrors() {
		t.Error("Adding error should set HasErrors to true")
	}
	if ec.Count() != 1 {
		t.Errorf("Expected count 1, got %d", ec.Count())
	}
}

func TestErrorCollector_Result(t *testing.T) {
	// Test no errors
	ec := NewErrorCollector()
	if result := ec.Result("context"); result != nil {
		t.Errorf("Result with no errors should return nil, got %v", result)
	}
	
	// Test single error with context
	ec = NewErrorCollector()
	err1 := errors.New("test error")
	ec.Add("switch1", err1)
	result := ec.Result("operation failed")
	if result == nil {
		t.Fatal("Result should not be nil with errors")
	}
	expected := "operation failed: switch1: test error"
	if result.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result.Error())
	}
	
	// Test single error without context in Result
	ec = NewErrorCollector()
	ec.Add("switch1", err1)
	result = ec.Result("")
	if result == nil {
		t.Fatal("Result should not be nil with errors")
	}
	expected = "switch1: test error"
	if result.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result.Error())
	}
	
	// Test multiple errors
	ec = NewErrorCollector()
	err2 := errors.New("test error 2")
	ec.Add("switch1", err1)
	ec.Add("switch2", err2)
	result = ec.Result("multiple operations failed")
	if result == nil {
		t.Fatal("Result should not be nil with errors")
	}
	resultStr := result.Error()
	if !strings.Contains(resultStr, "multiple operations failed") {
		t.Errorf("Result should contain context, got %s", resultStr)
	}
	if !strings.Contains(resultStr, "switch1: test error") {
		t.Errorf("Result should contain first error, got %s", resultStr)
	}
	if !strings.Contains(resultStr, "switch2: test error 2") {
		t.Errorf("Result should contain second error, got %s", resultStr)
	}
}

func TestErrorCollector_Errors(t *testing.T) {
	ec := NewErrorCollector()
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	
	ec.AddError(err1)
	ec.Add("context", err2)
	
	errors := ec.Errors()
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}
	
	// First error should be unchanged
	if errors[0].Error() != "error 1" {
		t.Errorf("First error should be 'error 1', got %s", errors[0].Error())
	}
	
	// Second error should have context
	if errors[1].Error() != "context: error 2" {
		t.Errorf("Second error should be 'context: error 2', got %s", errors[1].Error())
	}
}