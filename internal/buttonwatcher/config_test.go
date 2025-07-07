package buttonwatcher

import (
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/test-config.toml
var testConfigTOML []byte

//go:embed testdata/invalid-config.toml
var invalidConfigTOML []byte

//go:embed testdata/empty-config.toml
var emptyConfigTOML []byte

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	assert.NotNil(t, config, "NewConfig() should not return nil")
	assert.Empty(t, config.ConfigFile, "NewConfig() ConfigFile should be empty")
	assert.Empty(t, config.Buttons, "NewConfig() Buttons should be empty")
}

func TestConfigAddFlags(t *testing.T) {
	config := NewConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	flag := fs.Lookup("config")
	require.NotNil(t, flag, "AddFlags() should add 'config' flag")
	assert.Equal(t, "", flag.DefValue, "Default config file should be empty")
}

func TestConfigLoadConfig(t *testing.T) {
	t.Run("no config file", func(t *testing.T) {
		config := NewConfig()
		pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
		config.AddFlags(pflag.CommandLine)
		err := config.LoadConfig()
		assert.NoError(t, err, "LoadConfig() without file should not fail")
	})

	t.Run("non-existent file", func(t *testing.T) {
		config := NewConfig()
		config.ConfigFile = "/nonexistent/config.toml"
		err := config.LoadConfig()
		assert.Error(t, err, "LoadConfig() with non-existent file should fail")
	})

	t.Run("valid config file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.toml")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write(testConfigTOML)
		require.NoError(t, err, "Failed to write to temp file")
		tmpFile.Close()

		config := NewConfig()
		config.ConfigFile = tmpFile.Name()

		pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
		config.AddFlags(pflag.CommandLine)

		err = config.LoadConfig()
		assert.NoError(t, err, "LoadConfig() with valid config should not fail")

		require.Len(t, config.Buttons, 2, "Should load 2 buttons")

		// Button 1 assertions
		assert.Equal(t, "Button 1", config.Buttons[0].Name)
		assert.Equal(t, "/dev/input/event0", config.Buttons[0].Device)
		assert.Equal(t, "EV_KEY", config.Buttons[0].EventType)
		assert.Equal(t, uint32(115), config.Buttons[0].EventCode)
		assert.NotNil(t, config.Buttons[0].ClickAction)
		assert.Equal(t, "reboot", *config.Buttons[0].ClickAction)
		assert.Nil(t, config.Buttons[0].ShortPressAction)
		assert.Nil(t, config.Buttons[0].LongPressAction)

		// Button 2 assertions
		assert.Equal(t, "Button 2", config.Buttons[1].Name)
		assert.Equal(t, "/dev/input/event1", config.Buttons[1].Device)
		assert.NotNil(t, config.Buttons[1].ShortPressAction)
		assert.Equal(t, "shutdown", *config.Buttons[1].ShortPressAction)
		assert.NotNil(t, config.Buttons[1].LongPressAction)
		assert.Equal(t, "reboot", *config.Buttons[1].LongPressAction)
		assert.NotNil(t, config.Buttons[1].ShortPressDuration)
		assert.Equal(t, 200*time.Millisecond, *config.Buttons[1].ShortPressDuration)
		assert.NotNil(t, config.Buttons[1].LongPressDuration)
		assert.Equal(t, 2*time.Second, *config.Buttons[1].LongPressDuration)
		assert.NotNil(t, config.Buttons[1].Timeout)
		assert.Equal(t, 5*time.Second, *config.Buttons[1].Timeout)
		assert.Nil(t, config.Buttons[1].ClickAction)
	})
}

func TestConfigValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Buttons: []ButtonConfig{
				{Name: "b1", Device: "d1"},
				{Name: "b2", Device: "d2"},
			},
		}
		err := config.Validate()
		assert.NoError(t, err, "Validation should pass for a valid config")
	})

	t.Run("no buttons", func(t *testing.T) {
		config := NewConfig()
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with no buttons")
		assert.Equal(t, "no buttons configured", err.Error())
	})

	t.Run("button missing name", func(t *testing.T) {
		config := &Config{
			Buttons: []ButtonConfig{
				{Device: "d1"},
			},
		}
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with button missing name")
		assert.Equal(t, "button 0: name is required", err.Error())
	})

	t.Run("button missing device", func(t *testing.T) {
		config := &Config{
			Buttons: []ButtonConfig{
				{Name: "b1"},
			},
		}
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with button missing device")
		assert.Equal(t, "button 0 (b1): device is required", err.Error())
	})

	t.Run("loaded invalid config", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid-config-*.toml")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write(invalidConfigTOML)
		require.NoError(t, err, "Failed to write to temp file")
		tmpFile.Close()

		config := NewConfig()
		config.ConfigFile = tmpFile.Name()

		err = config.LoadConfig()
		require.NoError(t, err, "Loading should not fail, validation is separate")

		err = config.Validate()
		assert.Error(t, err, "Validation should fail for the loaded invalid config")
		assert.Equal(t, "button 0 (Button 1): device is required", err.Error())
	})

	t.Run("loaded empty config", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "empty-config-*.toml")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write(emptyConfigTOML)
		require.NoError(t, err, "Failed to write to temp file")
		tmpFile.Close()

		config := NewConfig()
		config.ConfigFile = tmpFile.Name()

		err = config.LoadConfig()
		require.NoError(t, err, "Loading should not fail, validation is separate")

		err = config.Validate()
		assert.Error(t, err, "Validation should fail for the loaded empty config")
		assert.Equal(t, "no buttons configured", err.Error())
	})
}
