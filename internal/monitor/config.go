package monitor

import (
	"fmt"

	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

// IMAPConfig holds IMAP server configuration
type IMAPConfig struct {
	Server               string `mapstructure:"server"`
	Port                 int    `mapstructure:"port"`
	Username             string `mapstructure:"username"`
	Password             string `mapstructure:"password"`
	UseSSL               bool   `mapstructure:"use-ssl"`
	RetryIntervalSeconds *int   `mapstructure:"retry-interval-seconds"`
}

// TriggerConfig holds trigger configuration
type TriggerConfig struct {
	RegexPattern string `mapstructure:"regex-pattern"`
	To           string `mapstructure:"to"`
	From         string `mapstructure:"from"`
	Subject      string `mapstructure:"subject"`
	IgnoreCase   *bool  `mapstructure:"ignore-case"`
	Final        bool   `mapstructure:"final"`
	Command      string `mapstructure:"command"`
}

// MailboxConfig holds configuration for a single mailbox
type MailboxConfig struct {
	Mailbox       string          `mapstructure:"mailbox"`
	CheckInterval *int            `mapstructure:"check-interval-seconds"`
	Triggers      []TriggerConfig `mapstructure:"triggers"`
}

// Config holds the complete configuration for the email monitor
type Config struct {
	ConfigFile    string          `mapstructure:"config-file"`
	IMAP          IMAPConfig      `mapstructure:"imap"`
	CheckInterval *int            `mapstructure:"check-interval-seconds"`
	Monitor       []MailboxConfig `mapstructure:"monitor"`
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	defaultCheckInterval := 30
	defaultRetryInterval := 30
	return &Config{
		IMAP: IMAPConfig{
			Port:                 993,
			UseSSL:               true,
			RetryIntervalSeconds: &defaultRetryInterval,
		},
		CheckInterval: &defaultCheckInterval,
		Monitor:       []MailboxConfig{},
	}
}

// AddFlags adds command-line flags for all configuration options
func (c *Config) AddFlags(fs *pflag.FlagSet) {
	// Config file flag
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "Config file to use")

	// IMAP flags
	fs.StringVar(&c.IMAP.Server, "imap.server", c.IMAP.Server, "IMAP server address")
	fs.IntVar(&c.IMAP.Port, "imap.port", c.IMAP.Port, "IMAP server port")
	fs.StringVar(&c.IMAP.Username, "imap.username", c.IMAP.Username, "IMAP username")
	fs.StringVar(&c.IMAP.Password, "imap.password", c.IMAP.Password, "IMAP password")
	fs.BoolVar(&c.IMAP.UseSSL, "imap.use-ssl", c.IMAP.UseSSL, "Use SSL for IMAP connection")
	if c.IMAP.RetryIntervalSeconds != nil {
		fs.IntVar(c.IMAP.RetryIntervalSeconds, "imap.retry-interval-seconds", *c.IMAP.RetryIntervalSeconds, "Retry interval in seconds when IMAP connection fails")
	}

	// Global check interval flag
	if c.CheckInterval != nil {
		fs.IntVar(c.CheckInterval, "check-interval", *c.CheckInterval, "Global interval in seconds to check for new emails")
	}
}

// LoadConfig loads configuration using the common config loader.
func (c *Config) LoadConfig(configFile string) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(configFile)

	// Set default values
	loader.SetDefaults(map[string]any{
		"imap.server":                 c.IMAP.Server,
		"imap.port":                   c.IMAP.Port,
		"imap.username":               c.IMAP.Username,
		"imap.password":               c.IMAP.Password,
		"imap.use-ssl":                c.IMAP.UseSSL,
		"imap.retry-interval-seconds": 30,
		"check-interval-seconds":      30,
	})

	return loader.LoadConfig(c)
}

// LoadConfigFromStruct loads configuration with proper precedence using the common pattern.
// This is the preferred method that follows the same pattern as API and UI.
func (c *Config) LoadConfigFromStruct() error {
	return c.LoadConfigWithFlagSet(pflag.CommandLine)
}

// LoadConfigWithFlagSet loads configuration with proper precedence using a custom flag set (for testing).
func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)

	// Set default values using the struct defaults
	loader.SetDefaults(map[string]any{
		"imap.server":                 "",
		"imap.port":                   993,
		"imap.username":               "",
		"imap.password":               "",
		"imap.use-ssl":                true,
		"imap.retry-interval-seconds": 30,
		"check-interval-seconds":      30,
	})

	return loader.LoadConfigWithFlagSet(c, fs)
}

// Validate checks that required configuration values are set
func (c *Config) Validate() error {
	if c.IMAP.Server == "" {
		return fmt.Errorf("%w: server is empty", ErrMissingIMAPServer)
	}
	if c.IMAP.Port <= 0 {
		return fmt.Errorf("%w: port is %d", ErrInvalidIMAPPort, c.IMAP.Port)
	}
	if len(c.Monitor) == 0 {
		return fmt.Errorf("%w: no monitor configurations provided", ErrMissingRegexPattern)
	}
	for i, mailbox := range c.Monitor {
		if mailbox.Mailbox == "" {
			return fmt.Errorf("%w: mailbox is empty in monitor %d", ErrMissingRegexPattern, i)
		}
		if len(mailbox.Triggers) == 0 {
			return fmt.Errorf("%w: no triggers configured for mailbox %s", ErrMissingRegexPattern, mailbox.Mailbox)
		}
		for j, trigger := range mailbox.Triggers {
			// At least one trigger condition must be specified
			if trigger.RegexPattern == "" && trigger.To == "" && trigger.From == "" && trigger.Subject == "" {
				return fmt.Errorf("%w: no trigger conditions specified in trigger %d of mailbox %s", ErrMissingRegexPattern, j, mailbox.Mailbox)
			}
		}
	}
	return nil
}

// GetEffectiveCheckInterval returns the effective check interval for a mailbox
// It uses the mailbox-specific value if set, otherwise falls back to the global value
func (c *Config) GetEffectiveCheckInterval(mailbox *MailboxConfig) int {
	if mailbox.CheckInterval != nil {
		return *mailbox.CheckInterval
	}
	if c.CheckInterval != nil {
		return *c.CheckInterval
	}
	return 30 // Default fallback
}

// GetEffectiveRetryInterval returns the effective retry interval for IMAP connections
func (c *Config) GetEffectiveRetryInterval() int {
	if c.IMAP.RetryIntervalSeconds != nil {
		return *c.IMAP.RetryIntervalSeconds
	}
	return 30 // Default fallback
}
