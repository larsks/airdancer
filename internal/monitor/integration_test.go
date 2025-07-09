package monitor

import (
	"testing"
)

func TestMultipleMailboxConfiguration(t *testing.T) {
	// Test that we can create a configuration with multiple mailboxes
	config := &Config{
		IMAP: IMAPConfig{
			Server:   "imap.example.com",
			Port:     993,
			Username: "test@example.com",
			Password: "password",
			UseSSL:   true,
		},
		CheckInterval: intPtr(60), // Global interval
		Monitor: []MailboxConfig{
			{
				Mailbox:       "INBOX",
				CheckInterval: intPtr(30), // Mailbox-specific interval
				Triggers: []TriggerConfig{
					{
						RegexPattern: "urgent.*alert",
						Command:      "notify-send 'Alert'",
					},
				},
			},
			{
				Mailbox: "SPAM",
				// No CheckInterval specified, should use global
				Triggers: []TriggerConfig{
					{
						RegexPattern: "CRITICAL.*ERROR",
						Command:      "logger 'Critical error'",
					},
				},
			},
		},
	}

	// Test validation
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected config to be valid, got error: %v", err)
	}

	// Test effective check intervals
	if config.GetEffectiveCheckInterval(&config.Monitor[0]) != 30 {
		t.Errorf("Expected INBOX to use mailbox-specific interval 30, got %d", config.GetEffectiveCheckInterval(&config.Monitor[0]))
	}

	if config.GetEffectiveCheckInterval(&config.Monitor[1]) != 60 {
		t.Errorf("Expected SPAM to use global interval 60, got %d", config.GetEffectiveCheckInterval(&config.Monitor[1]))
	}
}

func TestEmptyMailboxValidation(t *testing.T) {
	config := &Config{
		IMAP: IMAPConfig{
			Server: "imap.example.com",
			Port:   993,
		},
		Monitor: []MailboxConfig{
			{
				Mailbox: "", // Empty mailbox name
				Triggers: []TriggerConfig{
					{
						RegexPattern: "test",
						Command:      "echo test",
					},
				},
			},
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail for empty mailbox name")
	}
}

func TestNoTriggersValidation(t *testing.T) {
	config := &Config{
		IMAP: IMAPConfig{
			Server: "imap.example.com",
			Port:   993,
		},
		Monitor: []MailboxConfig{
			{
				Mailbox:  "INBOX",
				Triggers: []TriggerConfig{}, // No triggers
			},
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail for mailbox with no triggers")
	}
}