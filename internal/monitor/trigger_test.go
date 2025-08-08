package monitor

import (
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/go-imap"
)

func TestTriggerCompilation(t *testing.T) {
	tests := []struct {
		name          string
		triggerConfig TriggerConfig
		shouldError   bool
		errorContains string
	}{
		{
			name: "valid regex pattern with case insensitive default",
			triggerConfig: TriggerConfig{
				RegexPattern: "test.*pattern",
				Command:      "echo test",
			},
			shouldError: false,
		},
		{
			name: "valid header patterns",
			triggerConfig: TriggerConfig{
				To:      "lars",
				From:    "amazon",
				Subject: "delivered:",
				Command: "echo matched",
			},
			shouldError: false,
		},
		{
			name: "case sensitive pattern",
			triggerConfig: TriggerConfig{
				RegexPattern: "TEST",
				IgnoreCase:   boolPtr(false),
				Command:      "echo test",
			},
			shouldError: false,
		},
		{
			name: "invalid regex pattern",
			triggerConfig: TriggerConfig{
				RegexPattern: "[invalid",
				Command:      "echo test",
			},
			shouldError:   true,
			errorContains: "invalid regex pattern",
		},
		{
			name: "invalid from pattern",
			triggerConfig: TriggerConfig{
				From:    "[invalid",
				Command: "echo test",
			},
			shouldError:   true,
			errorContains: "invalid 'from' pattern",
		},
		{
			name: "final trigger",
			triggerConfig: TriggerConfig{
				RegexPattern: "test",
				Final:        true,
				Command:      "echo final",
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				IMAP: IMAPConfig{
					Server: "test.example.com",
					Port:   993,
				},
				Monitor: []MailboxConfig{
					{
						Mailbox:  "INBOX",
						Triggers: []TriggerConfig{tt.triggerConfig},
					},
				},
			}

			_, err := NewEmailMonitor(*config, &MockIMAPDialer{}, &MockCommandExecutor{}, &MockLogger{}, &MockTimer{})

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v", err)
				}
			}
		})
	}
}

func TestTriggerMatching(t *testing.T) {
	tests := []struct {
		name        string
		trigger     compiledTrigger
		message     testMessage
		body        string
		shouldMatch bool
		description string
	}{
		{
			name: "regex pattern matches body",
			trigger: compiledTrigger{
				bodyRegex: mustCompile("(?i)delivered.*today"),
				command:   "echo matched",
			},
			message: testMessage{
				from:    "amazon@example.com",
				to:      []string{"lars@example.com"},
				subject: "Your order has been delivered",
			},
			body:        "Your package was delivered today at 3 PM",
			shouldMatch: true,
			description: "body contains delivered today",
		},
		{
			name: "from pattern matches",
			trigger: compiledTrigger{
				fromRegex: mustCompile("(?i)amazon"),
				command:   "echo matched",
			},
			message: testMessage{
				from:    "no-reply@amazon.com",
				to:      []string{"lars@example.com"},
				subject: "Order update",
			},
			shouldMatch: true,
			description: "from contains amazon",
		},
		{
			name: "to pattern matches",
			trigger: compiledTrigger{
				toRegex: mustCompile("(?i)lars"),
				command: "echo matched",
			},
			message: testMessage{
				from:    "sender@example.com",
				to:      []string{"lars@example.com", "other@example.com"},
				subject: "Test message",
			},
			shouldMatch: true,
			description: "to contains lars",
		},
		{
			name: "subject pattern matches",
			trigger: compiledTrigger{
				subjectRegex: mustCompile("(?i)delivered:"),
				command:      "echo matched",
			},
			message: testMessage{
				from:    "amazon@example.com",
				to:      []string{"user@example.com"},
				subject: "Delivered: Your package",
			},
			shouldMatch: true,
			description: "subject contains delivered:",
		},
		{
			name: "case sensitive regex no match",
			trigger: compiledTrigger{
				bodyRegex: mustCompile("TEST"), // case sensitive
				command:   "echo matched",
			},
			body:        "This is a test message",
			shouldMatch: false,
			description: "case sensitive TEST doesn't match lowercase test",
		},
		{
			name: "case insensitive regex matches",
			trigger: compiledTrigger{
				bodyRegex: mustCompile("(?i)TEST"), // case insensitive
				command:   "echo matched",
			},
			body:        "This is a test message",
			shouldMatch: true,
			description: "case insensitive TEST matches lowercase test",
		},
		{
			name: "multiple conditions all match",
			trigger: compiledTrigger{
				fromRegex:    mustCompile("(?i)amazon"),
				toRegex:      mustCompile("(?i)lars"),
				subjectRegex: mustCompile("(?i)delivered:"),
				bodyRegex:    mustCompile("(?i)today"),
				command:      "echo all matched",
			},
			message: testMessage{
				from:    "no-reply@amazon.com",
				to:      []string{"lars@example.com"},
				subject: "Delivered: Your order",
			},
			body:        "Package delivered today",
			shouldMatch: true,
			description: "all conditions match",
		},
		{
			name: "one condition fails",
			trigger: compiledTrigger{
				fromRegex:    mustCompile("(?i)amazon"),
				toRegex:      mustCompile("(?i)lars"),
				subjectRegex: mustCompile("(?i)delivered:"),
				bodyRegex:    mustCompile("(?i)yesterday"), // this won't match
				command:      "echo matched",
			},
			message: testMessage{
				from:    "no-reply@amazon.com",
				to:      []string{"lars@example.com"},
				subject: "Delivered: Your order",
			},
			body:        "Package delivered today",
			shouldMatch: false,
			description: "body condition fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse email addresses to get mailbox and host parts
			fromAddr := parseEmailAddress(tt.message.from)
			
			// Create a mock IMAP message
			msg := &imap.Message{
				Envelope: &imap.Envelope{
					From:    []*imap.Address{fromAddr},
					Subject: tt.message.subject,
				},
			}

			// Add To addresses
			for _, to := range tt.message.to {
				toAddr := parseEmailAddress(to)
				msg.Envelope.To = append(msg.Envelope.To, toAddr)
			}

			// Simulate the matching logic from processMessageInMailbox
			matched := true

			// Check body pattern (regex-pattern)
			if tt.trigger.bodyRegex != nil {
				if !tt.trigger.bodyRegex.MatchString(tt.body) {
					matched = false
				}
			}

			// Check From header
			if matched && tt.trigger.fromRegex != nil {
				if !tt.trigger.fromRegex.MatchString(tt.message.from) {
					matched = false
				}
			}

			// Check To header (match against any To address)
			if matched && tt.trigger.toRegex != nil {
				toMatched := false
				for _, toAddr := range tt.message.to {
					if tt.trigger.toRegex.MatchString(toAddr) {
						toMatched = true
						break
					}
				}
				if !toMatched {
					matched = false
				}
			}

			// Check Subject header
			if matched && tt.trigger.subjectRegex != nil {
				if !tt.trigger.subjectRegex.MatchString(tt.message.subject) {
					matched = false
				}
			}

			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v but got match=%v for %s", tt.shouldMatch, matched, tt.description)
			}
		})
	}
}

type testMessage struct {
	from    string
	to      []string
	subject string
}

func mustCompile(pattern string) *regexp.Regexp {
	r, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return r
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func parseEmailAddress(email string) *imap.Address {
	// Simple parsing - for real use would need more sophisticated parsing
	if strings.Contains(email, "@") {
		parts := strings.SplitN(email, "@", 2)
		return &imap.Address{
			MailboxName: parts[0],
			HostName:    parts[1],
		}
	}
	return &imap.Address{
		MailboxName: email,
		HostName:    "",
	}
}
