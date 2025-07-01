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
