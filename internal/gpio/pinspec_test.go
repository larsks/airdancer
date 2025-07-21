package gpio

import (
	"testing"
)

func TestParsePinNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{
			name:     "direct number",
			input:    "18",
			expected: 18,
			wantErr:  false,
		},
		{
			name:     "GPIO prefix uppercase",
			input:    "GPIO18",
			expected: 18,
			wantErr:  false,
		},
		{
			name:     "GPIO prefix lowercase",
			input:    "gpio18",
			expected: 18,
			wantErr:  false,
		},
		{
			name:     "GPIO prefix mixed case",
			input:    "GpIo18",
			expected: 18,
			wantErr:  false,
		},
		{
			name:     "zero pin number",
			input:    "GPIO0",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "large pin number",
			input:    "GPIO27",
			expected: 27,
			wantErr:  false,
		},
		{
			name:     "invalid format - letters only",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - GPIO without number",
			input:    "GPIO",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - GPIO with non-numeric",
			input:    "GPIOabc",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "negative number as string",
			input:    "-5",
			expected: -5,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePinNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePinNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ParsePinNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParsePin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *PinSpec
		wantErr  bool
	}{
		{
			name:  "simple pin number",
			input: "18",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullAuto,
			},
			wantErr: false,
		},
		{
			name:  "GPIO prefix",
			input: "GPIO18",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullAuto,
			},
			wantErr: false,
		},
		{
			name:  "active-low polarity",
			input: "GPIO18:active-low",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveLow,
				PullMode: PullAuto,
			},
			wantErr: false,
		},
		{
			name:  "active-high polarity explicit",
			input: "GPIO18:active-high",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullAuto,
			},
			wantErr: false,
		},
		{
			name:  "pull-up resistor",
			input: "GPIO18:pull-up",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullUp,
			},
			wantErr: false,
		},
		{
			name:  "pull-down resistor",
			input: "GPIO18:pull-down",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullDown,
			},
			wantErr: false,
		},
		{
			name:  "pull-none resistor",
			input: "GPIO18:pull-none",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullNone,
			},
			wantErr: false,
		},
		{
			name:  "pull-auto resistor explicit",
			input: "GPIO18:pull-auto",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullAuto,
			},
			wantErr: false,
		},
		{
			name:  "active-low with pull-up",
			input: "GPIO18:active-low:pull-up",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveLow,
				PullMode: PullUp,
			},
			wantErr: false,
		},
		{
			name:  "all parameters specified",
			input: "GPIO18:active-high:pull-down",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullDown,
			},
			wantErr: false,
		},
		{
			name:  "parameters in different order",
			input: "GPIO18:pull-up:active-low",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveLow,
				PullMode: PullUp,
			},
			wantErr: false,
		},
		{
			name:  "case insensitive parameters",
			input: "GPIO18:ACTIVE-LOW:PULL-UP",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveLow,
				PullMode: PullUp,
			},
			wantErr: false,
		},
		{
			name:  "whitespace in parameters",
			input: "GPIO18: active-low : pull-up ",
			expected: &PinSpec{
				LineNum:  18,
				Polarity: ActiveLow,
				PullMode: PullUp,
			},
			wantErr: false,
		},
		{
			name:    "invalid pin format",
			input:   "invalid:active-low",
			wantErr: true,
		},
		{
			name:    "unknown parameter",
			input:   "GPIO18:unknown-param",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "just colon",
			input:   ":",
			wantErr: true,
		},
		{
			name:    "multiple unknown parameters",
			input:   "GPIO18:active-low:invalid:pull-up",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePin(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if result.LineNum != tt.expected.LineNum {
				t.Errorf("ParsePin() LineNum = %v, want %v", result.LineNum, tt.expected.LineNum)
			}
			if result.Polarity != tt.expected.Polarity {
				t.Errorf("ParsePin() Polarity = %v, want %v", result.Polarity, tt.expected.Polarity)
			}
			if result.PullMode != tt.expected.PullMode {
				t.Errorf("ParsePin() PullMode = %v, want %v", result.PullMode, tt.expected.PullMode)
			}
		})
	}
}

func TestPolarityString(t *testing.T) {
	tests := []struct {
		polarity Polarity
		expected string
	}{
		{ActiveHigh, "active-high"},
		{ActiveLow, "active-low"},
		{Polarity(999), "unknown"}, // Test unknown value
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.polarity.String()
			if result != tt.expected {
				t.Errorf("Polarity.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPullModeString(t *testing.T) {
	tests := []struct {
		pullMode PullMode
		expected string
	}{
		{PullNone, "pull-none"},
		{PullUp, "pull-up"},
		{PullDown, "pull-down"},
		{PullAuto, "pull-auto"},
		{PullMode(999), "unknown"}, // Test unknown value
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.pullMode.String()
			if result != tt.expected {
				t.Errorf("PullMode.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPinSpecString(t *testing.T) {
	tests := []struct {
		name     string
		pinSpec  *PinSpec
		expected string
	}{
		{
			name: "active-high with pull-auto",
			pinSpec: &PinSpec{
				LineNum:  18,
				Polarity: ActiveHigh,
				PullMode: PullAuto,
			},
			expected: "GPIO18:active-high:pull-auto",
		},
		{
			name: "active-low with pull-up",
			pinSpec: &PinSpec{
				LineNum:  27,
				Polarity: ActiveLow,
				PullMode: PullUp,
			},
			expected: "GPIO27:active-low:pull-up",
		},
		{
			name: "active-high with pull-down",
			pinSpec: &PinSpec{
				LineNum:  0,
				Polarity: ActiveHigh,
				PullMode: PullDown,
			},
			expected: "GPIO0:active-high:pull-down",
		},
		{
			name: "active-low with pull-none",
			pinSpec: &PinSpec{
				LineNum:  22,
				Polarity: ActiveLow,
				PullMode: PullNone,
			},
			expected: "GPIO22:active-low:pull-none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pinSpec.String()
			if result != tt.expected {
				t.Errorf("PinSpec.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test round-trip parsing - parse a string, convert back to string, and parse again
func TestPinSpecRoundTrip(t *testing.T) {
	testSpecs := []string{
		"GPIO18",
		"GPIO18:active-low",
		"GPIO18:pull-up",
		"GPIO18:active-low:pull-up",
		"GPIO27:active-high:pull-down",
		"22:active-low:pull-none",
		"0:pull-auto",
	}

	for _, spec := range testSpecs {
		t.Run(spec, func(t *testing.T) {
			// Parse the original spec
			parsed1, err := ParsePin(spec)
			if err != nil {
				t.Fatalf("ParsePin() failed: %v", err)
			}

			// Convert to string and parse again
			stringified := parsed1.String()
			parsed2, err := ParsePin(stringified)
			if err != nil {
				t.Fatalf("ParsePin() on stringified spec failed: %v", err)
			}

			// Compare the two parsed specs
			if parsed1.LineNum != parsed2.LineNum {
				t.Errorf("LineNum mismatch: original=%v, round-trip=%v", parsed1.LineNum, parsed2.LineNum)
			}
			if parsed1.Polarity != parsed2.Polarity {
				t.Errorf("Polarity mismatch: original=%v, round-trip=%v", parsed1.Polarity, parsed2.Polarity)
			}
			if parsed1.PullMode != parsed2.PullMode {
				t.Errorf("PullMode mismatch: original=%v, round-trip=%v", parsed1.PullMode, parsed2.PullMode)
			}
		})
	}
}
