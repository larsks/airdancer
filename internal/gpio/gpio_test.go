package gpio

import (
	"testing"
)

func TestParsePinConfig(t *testing.T) {
	tests := []struct {
		input    string
		expected PinConfig
	}{
		{
			input:    "GPIO23",
			expected: PinConfig{Name: "GPIO23", Polarity: ActiveHigh},
		},
		{
			input:    "GPIO23:ActiveHigh",
			expected: PinConfig{Name: "GPIO23", Polarity: ActiveHigh},
		},
		{
			input:    "GPIO23:activehigh",
			expected: PinConfig{Name: "GPIO23", Polarity: ActiveHigh},
		},
		{
			input:    "GPIO23:ActiveLow",
			expected: PinConfig{Name: "GPIO23", Polarity: ActiveLow},
		},
		{
			input:    "GPIO23:activelow",
			expected: PinConfig{Name: "GPIO23", Polarity: ActiveLow},
		},
		{
			input:    "GPIO2:InvalidPolarity",
			expected: PinConfig{Name: "GPIO2", Polarity: ActiveHigh}, // defaults to ActiveHigh
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParsePinConfig(tt.input)
			if result.Name != tt.expected.Name {
				t.Errorf("expected name %s, got %s", tt.expected.Name, result.Name)
			}
			if result.Polarity != tt.expected.Polarity {
				t.Errorf("expected polarity %d, got %d", tt.expected.Polarity, result.Polarity)
			}
		})
	}
}

func TestParseMultiplePinConfigs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []PinConfig
	}{
		{
			name:  "mixed pin configurations",
			input: []string{"GPIO18", "GPIO19:ActiveLow", "GPIO20:ActiveHigh", "GPIO21:activelow"},
			expected: []PinConfig{
				{Name: "GPIO18", Polarity: ActiveHigh},
				{Name: "GPIO19", Polarity: ActiveLow},
				{Name: "GPIO20", Polarity: ActiveHigh},
				{Name: "GPIO21", Polarity: ActiveLow},
			},
		},
		{
			name:  "all default polarity",
			input: []string{"GPIO2", "GPIO3", "GPIO4"},
			expected: []PinConfig{
				{Name: "GPIO2", Polarity: ActiveHigh},
				{Name: "GPIO3", Polarity: ActiveHigh},
				{Name: "GPIO4", Polarity: ActiveHigh},
			},
		},
		{
			name:  "all explicit ActiveHigh",
			input: []string{"GPIO5:ActiveHigh", "GPIO6:activehigh"},
			expected: []PinConfig{
				{Name: "GPIO5", Polarity: ActiveHigh},
				{Name: "GPIO6", Polarity: ActiveHigh},
			},
		},
		{
			name:  "all ActiveLow",
			input: []string{"GPIO7:ActiveLow", "GPIO8:activelow"},
			expected: []PinConfig{
				{Name: "GPIO7", Polarity: ActiveLow},
				{Name: "GPIO8", Polarity: ActiveLow},
			},
		},
		{
			name:  "single pin",
			input: []string{"GPIO9:ActiveLow"},
			expected: []PinConfig{
				{Name: "GPIO9", Polarity: ActiveLow},
			},
		},
		{
			name:     "empty list",
			input:    []string{},
			expected: []PinConfig{},
		},
		{
			name:  "functional pin names",
			input: []string{"SPI0_MOSI", "SPI0_MISO:ActiveLow", "P1_12"},
			expected: []PinConfig{
				{Name: "SPI0_MOSI", Polarity: ActiveHigh},
				{Name: "SPI0_MISO", Polarity: ActiveLow},
				{Name: "P1_12", Polarity: ActiveHigh},
			},
		},
		{
			name:  "invalid polarity defaults to ActiveHigh",
			input: []string{"GPIO10:InvalidPolarity", "GPIO11:SomeRandomValue"},
			expected: []PinConfig{
				{Name: "GPIO10", Polarity: ActiveHigh},
				{Name: "GPIO11", Polarity: ActiveHigh},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse each pin configuration (simulating what happens in NewGPIOSwitchCollection)
			var result []PinConfig
			for _, pinSpec := range tt.input {
				config := ParsePinConfig(pinSpec)
				result = append(result, config)
			}

			// Verify the length matches
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d pin configs, got %d", len(tt.expected), len(result))
				return
			}

			// Verify each pin configuration
			for i, expected := range tt.expected {
				if result[i].Name != expected.Name {
					t.Errorf("pin %d: expected name %s, got %s", i, expected.Name, result[i].Name)
				}
				if result[i].Polarity != expected.Polarity {
					t.Errorf("pin %d: expected polarity %d, got %d", i, expected.Polarity, result[i].Polarity)
				}
			}
		})
	}
}
