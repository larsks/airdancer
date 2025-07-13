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

//go:embed testdata/global-defaults-config.toml
var globalDefaultsConfigTOML []byte

//go:embed testdata/default-action-config.toml
var defaultActionConfigTOML []byte

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
		assert.Equal(t, "event", config.Buttons[0].Driver)
		assert.Equal(t, "/dev/input/event0:EV_KEY:115", config.Buttons[0].Spec)
		assert.NotNil(t, config.Buttons[0].ClickAction)
		assert.Equal(t, "reboot", *config.Buttons[0].ClickAction)
		assert.Nil(t, config.Buttons[0].ShortPressAction)
		assert.Nil(t, config.Buttons[0].LongPressAction)

		// Button 2 assertions
		assert.Equal(t, "Button 2", config.Buttons[1].Name)
		assert.Equal(t, "event", config.Buttons[1].Driver)
		assert.Equal(t, "/dev/input/event1:EV_KEY:114", config.Buttons[1].Spec)
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
				{Name: "b1", Driver: "event", Spec: "/dev/input/event0:EV_KEY:115"},
				{Name: "b2", Driver: "gpio", Spec: "GPIO16"},
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
				{Driver: "event", Spec: "/dev/input/event0:EV_KEY:115"},
			},
		}
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with button missing name")
		assert.Equal(t, "button 0: name is required", err.Error())
	})

	t.Run("button missing driver", func(t *testing.T) {
		config := &Config{
			Buttons: []ButtonConfig{
				{Name: "b1", Spec: "/dev/input/event0:EV_KEY:115"},
			},
		}
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with button missing driver")
		assert.Equal(t, "button 0 (b1): driver is required", err.Error())
	})

	t.Run("button missing spec", func(t *testing.T) {
		config := &Config{
			Buttons: []ButtonConfig{
				{Name: "b1", Driver: "event"},
			},
		}
		err := config.Validate()
		assert.Error(t, err, "Validation should fail with button missing spec")
		assert.Equal(t, "button 0 (b1): spec is required", err.Error())
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
		assert.Equal(t, "button 0 (Button 1): driver is required", err.Error())
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

func TestGlobalDefaults(t *testing.T) {
	t.Run("config with global defaults", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "global-defaults-config-*.toml")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write(globalDefaultsConfigTOML)
		require.NoError(t, err, "Failed to write to temp file")
		tmpFile.Close()

		config := NewConfig()
		config.ConfigFile = tmpFile.Name()

		err = config.LoadConfig()
		assert.NoError(t, err, "LoadConfig() with global defaults should not fail")

		// Check global defaults are loaded
		assert.NotNil(t, config.ClickInterval)
		assert.Equal(t, 300*time.Millisecond, *config.ClickInterval)
		assert.NotNil(t, config.ShortPressDuration)
		assert.Equal(t, 500*time.Millisecond, *config.ShortPressDuration)
		assert.NotNil(t, config.LongPressDuration)
		assert.Equal(t, 2*time.Second, *config.LongPressDuration)
		assert.NotNil(t, config.Timeout)
		assert.Equal(t, 8*time.Second, *config.Timeout)

		// Check buttons are loaded correctly
		require.Len(t, config.Buttons, 3, "Should load 3 buttons")
		
		// Button 1 has no timing overrides
		assert.Nil(t, config.Buttons[0].ClickInterval)
		assert.Nil(t, config.Buttons[0].ShortPressDuration)
		assert.Nil(t, config.Buttons[0].LongPressDuration)
		assert.Nil(t, config.Buttons[0].Timeout)
		
		// Button 2 has click_interval override
		assert.NotNil(t, config.Buttons[1].ClickInterval)
		assert.Equal(t, 800*time.Millisecond, *config.Buttons[1].ClickInterval)
		assert.Nil(t, config.Buttons[1].ShortPressDuration)
		assert.Nil(t, config.Buttons[1].LongPressDuration)
		assert.Nil(t, config.Buttons[1].Timeout)
		
		// Button 3 has all timing overrides
		assert.NotNil(t, config.Buttons[2].ClickInterval)
		assert.Equal(t, 200*time.Millisecond, *config.Buttons[2].ClickInterval)
		assert.NotNil(t, config.Buttons[2].ShortPressDuration)
		assert.Equal(t, 1*time.Second, *config.Buttons[2].ShortPressDuration)
		assert.NotNil(t, config.Buttons[2].LongPressDuration)
		assert.Equal(t, 3*time.Second, *config.Buttons[2].LongPressDuration)
		assert.NotNil(t, config.Buttons[2].Timeout)
		assert.Equal(t, 10*time.Second, *config.Buttons[2].Timeout)
	})
}

func TestDefaultActionConfig(t *testing.T) {
	t.Run("config with default actions", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "default-action-config-*.toml")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write(defaultActionConfigTOML)
		require.NoError(t, err, "Failed to write to temp file")
		tmpFile.Close()

		config := NewConfig()
		config.ConfigFile = tmpFile.Name()

		err = config.LoadConfig()
		assert.NoError(t, err, "LoadConfig() with default actions should not fail")

		// Check global default action is loaded
		assert.NotNil(t, config.DefaultAction)
		assert.Equal(t, "echo 'Global default action executed'", *config.DefaultAction)

		// Check buttons are loaded correctly
		require.Len(t, config.Buttons, 3, "Should load 3 buttons")
		
		// Button 1 has click_action but no default_action
		assert.NotNil(t, config.Buttons[0].ClickAction)
		assert.Equal(t, "echo 'Button 1 clicked'", *config.Buttons[0].ClickAction)
		assert.Nil(t, config.Buttons[0].DefaultAction)
		
		// Button 2 has its own default_action
		assert.NotNil(t, config.Buttons[1].DefaultAction)
		assert.Equal(t, "echo 'Button 2 default action'", *config.Buttons[1].DefaultAction)
		
		// Button 3 has no actions at all
		assert.Nil(t, config.Buttons[2].ClickAction)
		assert.Nil(t, config.Buttons[2].DefaultAction)
	})
}
