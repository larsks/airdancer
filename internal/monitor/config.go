package monitor

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// IMAPConfig holds IMAP server configuration
type IMAPConfig struct {
	Server   string `mapstructure:"server"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	UseSSL   bool   `mapstructure:"use_ssl"`
	Mailbox  string `mapstructure:"mailbox"`
}

// MonitorConfig holds monitoring configuration
type MonitorConfig struct {
	RegexPattern  string `mapstructure:"regex_pattern"`
	Command       string `mapstructure:"command"`
	CheckInterval int    `mapstructure:"check_interval_seconds"`
}

// Config holds the complete configuration for the email monitor
type Config struct {
	IMAP    IMAPConfig    `mapstructure:"imap"`
	Monitor MonitorConfig `mapstructure:"monitor"`
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		IMAP: IMAPConfig{
			Port:    993,
			UseSSL:  true,
			Mailbox: "INBOX",
		},
		Monitor: MonitorConfig{
			CheckInterval: 30,
		},
	}
}

// AddFlags adds command-line flags for all configuration options
func (c *Config) AddFlags(fs *pflag.FlagSet) {
	// IMAP flags
	fs.StringVar(&c.IMAP.Server, "imap.server", c.IMAP.Server, "IMAP server address")
	fs.IntVar(&c.IMAP.Port, "imap.port", c.IMAP.Port, "IMAP server port")
	fs.StringVar(&c.IMAP.Username, "imap.username", c.IMAP.Username, "IMAP username")
	fs.StringVar(&c.IMAP.Password, "imap.password", c.IMAP.Password, "IMAP password")
	fs.BoolVar(&c.IMAP.UseSSL, "imap.use-ssl", c.IMAP.UseSSL, "Use SSL for IMAP connection")
	fs.StringVar(&c.IMAP.Mailbox, "imap.mailbox", c.IMAP.Mailbox, "IMAP mailbox to monitor")

	// Monitor flags
	fs.StringVar(&c.Monitor.RegexPattern, "monitor.regex-pattern", c.Monitor.RegexPattern, "Regex pattern to match in email bodies")
	fs.StringVar(&c.Monitor.Command, "monitor.command", c.Monitor.Command, "Command to execute on regex match")
	fs.IntVar(&c.Monitor.CheckInterval, "monitor.check-interval", c.Monitor.CheckInterval, "Interval in seconds to check for new emails")
}

// LoadConfig loads configuration using viper with support for multiple formats
func (c *Config) LoadConfig(configFile string) error {
	v := viper.New()

	// Set defaults
	v.SetDefault("imap.server", c.IMAP.Server)
	v.SetDefault("imap.port", c.IMAP.Port)
	v.SetDefault("imap.username", c.IMAP.Username)
	v.SetDefault("imap.password", c.IMAP.Password)
	v.SetDefault("imap.use_ssl", c.IMAP.UseSSL)
	v.SetDefault("imap.mailbox", c.IMAP.Mailbox)
	v.SetDefault("monitor.regex_pattern", c.Monitor.RegexPattern)
	v.SetDefault("monitor.command", c.Monitor.Command)
	v.SetDefault("monitor.check_interval_seconds", c.Monitor.CheckInterval)

	// Bind flags to viper
	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("failed to bind flags: %w", err)
	}

	// Read config file if specified
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(c); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// Validate checks that required configuration values are set
func (c *Config) Validate() error {
	if c.IMAP.Server == "" {
		return ErrMissingIMAPServer
	}
	if c.IMAP.Port == 0 {
		return ErrInvalidIMAPPort
	}
	if c.Monitor.RegexPattern == "" {
		return ErrMissingRegexPattern
	}
	return nil
}
