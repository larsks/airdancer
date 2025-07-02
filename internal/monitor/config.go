package monitor

import (
	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
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
	ConfigFile string        `mapstructure:"config-file"`
	IMAP       IMAPConfig    `mapstructure:"imap"`
	Monitor    MonitorConfig `mapstructure:"monitor"`
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
	// Config file flag
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "Config file to use")
	
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

// LoadConfig loads configuration using the common config loader.
func (c *Config) LoadConfig(configFile string) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(configFile)
	
	// Set default values
	loader.SetDefaults(map[string]interface{}{
		"imap.server":                     c.IMAP.Server,
		"imap.port":                       c.IMAP.Port,
		"imap.username":                   c.IMAP.Username,
		"imap.password":                   c.IMAP.Password,
		"imap.use_ssl":                    c.IMAP.UseSSL,
		"imap.mailbox":                    c.IMAP.Mailbox,
		"monitor.regex_pattern":           c.Monitor.RegexPattern,
		"monitor.command":                 c.Monitor.Command,
		"monitor.check_interval_seconds":  c.Monitor.CheckInterval,
	})
	
	return loader.LoadConfig(c)
}

// LoadConfigFromStruct loads configuration with proper precedence using the common pattern.
// This is the preferred method that follows the same pattern as API and UI.
func (c *Config) LoadConfigFromStruct() error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)
	
	// Set default values using the struct defaults
	loader.SetDefaults(map[string]interface{}{
		"imap.server":                     "",
		"imap.port":                       993,
		"imap.username":                   "",
		"imap.password":                   "",
		"imap.use_ssl":                    true,
		"imap.mailbox":                    "INBOX",
		"monitor.regex_pattern":           "",
		"monitor.command":                 "",
		"monitor.check_interval_seconds":  30,
	})
	
	return loader.LoadConfig(c)
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
