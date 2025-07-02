package monitor

import (
	"testing"
)

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrMissingIMAPServer",
			err:      ErrMissingIMAPServer,
			expected: "IMAP server must be set",
		},
		{
			name:     "ErrInvalidIMAPPort",
			err:      ErrInvalidIMAPPort,
			expected: "IMAP port must be non-zero",
		},
		{
			name:     "ErrMissingRegexPattern",
			err:      ErrMissingRegexPattern,
			expected: "regex pattern must be set",
		},
		{
			name:     "ErrInvalidRegexPattern",
			err:      ErrInvalidRegexPattern,
			expected: "invalid regex pattern",
		},
		{
			name:     "ErrConnectionFailed",
			err:      ErrConnectionFailed,
			expected: "failed to connect to IMAP server",
		},
		{
			name:     "ErrAuthenticationFailed",
			err:      ErrAuthenticationFailed,
			expected: "IMAP authentication failed",
		},
		{
			name:     "ErrMailboxNotFound",
			err:      ErrMailboxNotFound,
			expected: "mailbox not found",
		},
		{
			name:     "ErrMessageProcessing",
			err:      ErrMessageProcessing,
			expected: "error processing message",
		},
		{
			name:     "ErrCommandExecution",
			err:      ErrCommandExecution,
			expected: "error executing command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestErrorsAreNotNil(t *testing.T) {
	errors := []error{
		ErrMissingIMAPServer,
		ErrInvalidIMAPPort,
		ErrMissingRegexPattern,
		ErrInvalidRegexPattern,
		ErrConnectionFailed,
		ErrAuthenticationFailed,
		ErrMailboxNotFound,
		ErrMessageProcessing,
		ErrCommandExecution,
	}

	for i, err := range errors {
		if err == nil {
			t.Errorf("Error at index %d is nil", i)
		}
	}
}
