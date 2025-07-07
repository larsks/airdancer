package buttonwatcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_LoadConfig(t *testing.T) {
	// Create a temporary TOML file for testing
	content := `
[[buttons]]
name = "Test Button 1"
device = "/dev/input/event0"
event_type = "EV_KEY"
event_code = 101
click_action = "echo 'click'"
short_press_duration = "1s"
short_press_action = "echo 'short'"
long_press_duration = "3s"
long_press_action = "echo 'long'"
timeout = "10s"

[[buttons]]
name = "Test Button 2"
device = "/dev/input/event1"
event_type = "EV_KEY"
event_code = 102
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.toml")
	err := os.WriteFile(configFile, []byte(content), 0600)
	require.NoError(t, err)

	// Create a new config and load from the file
	cfg := NewConfig()
	err = cfg.LoadConfig(configFile)
	require.NoError(t, err)

	// Validate the loaded configuration
	err = cfg.Validate()
	require.NoError(t, err)

	// Assert that the configuration was loaded correctly
	assert.Len(t, cfg.Buttons, 2)

	// Check the first button
	assert.Equal(t, "Test Button 1", cfg.Buttons[0].Name)
	assert.Equal(t, "/dev/input/event0", cfg.Buttons[0].Device)
	assert.Equal(t, "EV_KEY", cfg.Buttons[0].EventType)
	assert.Equal(t, uint32(101), cfg.Buttons[0].EventCode)
	assert.Equal(t, "echo 'click'", cfg.Buttons[0].ClickAction)
	assert.Equal(t, time.Second, cfg.Buttons[0].ShortPressDuration)
	assert.Equal(t, "echo 'short'", cfg.Buttons[0].ShortPressAction)
	assert.Equal(t, 3*time.Second, cfg.Buttons[0].LongPressDuration)
	assert.Equal(t, "echo 'long'", cfg.Buttons[0].LongPressAction)
	assert.Equal(t, 10*time.Second, cfg.Buttons[0].Timeout)

	// Check the second button
	assert.Equal(t, "Test Button 2", cfg.Buttons[1].Name)
	assert.Equal(t, "/dev/input/event1", cfg.Buttons[1].Device)
}

func TestConfig_Validate(t *testing.T) {
	testCases := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				Buttons: []ButtonConfig{
					{Name: "b1", Device: "d1"},
					{Name: "b2", Device: "d2"},
				},
			},
			expectErr: false,
		},
		{
			name:      "No buttons",
			config:    &Config{Buttons: []ButtonConfig{}},
			expectErr: true,
		},
		{
			name: "Button with no name",
			config: &Config{
				Buttons: []ButtonConfig{
					{Device: "d1"},
				},
			},
			expectErr: true,
		},
		{
			name: "Button with no device",
			config: &Config{
				Buttons: []ButtonConfig{
					{Name: "b1"},
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
