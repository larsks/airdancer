package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// MockHTTPClient implements HTTPClient interface for testing
type MockHTTPClient struct {
	responses map[string]*http.Response
	requests  []*http.Request
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*http.Response),
		requests:  make([]*http.Request, 0),
	}
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	key := req.Method + " " + req.URL.Path
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}

	// Default response
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(`{"status":"error","message":"Not found"}`)),
	}, nil
}

func (m *MockHTTPClient) AddResponse(method, path string, statusCode int, body string) {
	key := method + " " + path
	m.responses[key] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func (m *MockHTTPClient) GetLastRequest() *http.Request {
	if len(m.requests) == 0 {
		return nil
	}
	return m.requests[len(m.requests)-1]
}

func (m *MockHTTPClient) GetRequestCount() int {
	return len(m.requests)
}

func TestParseArgs(t *testing.T) {
	// Save original working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	// Create temporary directory for testing
	tempDir := t.TempDir()
	os.Chdir(tempDir)

	tests := []struct {
		name        string
		args        []string
		want        *CommandArgs
		wantErr     bool
		wantCommand string
	}{
		{
			name:        "no arguments shows help",
			args:        []string{},
			wantCommand: "help",
			wantErr:     false,
		},
		{
			name:        "help flag",
			args:        []string{"--help"},
			wantCommand: "help",
			wantErr:     false,
		},
		{
			name:        "version flag",
			args:        []string{"--version"},
			wantCommand: "version",
			wantErr:     false,
		},
		{
			name:        "status command with no args",
			args:        []string{"status"},
			wantCommand: "status",
			wantErr:     false,
		},
		{
			name:        "blink command with flags",
			args:        []string{"blink", "switch1", "--period", "2.0", "--duration", "30", "--duty-cycle", "0.7"},
			wantCommand: "blink",
			wantErr:     false,
		},
		{
			name:        "server-url flag",
			args:        []string{"--server-url", "http://example.com:8080", "status"},
			wantCommand: "status",
			wantErr:     false,
		},
		{
			name:    "invalid flag",
			args:    []string{"--invalid-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for each test to avoid conflicts
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.Usage = func() {} // Suppress usage output

			got, err := ParseArgsWithFlagSet(tt.args, fs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Command != tt.wantCommand {
					t.Errorf("ParseArgs() command = %v, want %v", got.Command, tt.wantCommand)
				}
				if got.Config == nil {
					t.Errorf("ParseArgs() config is nil")
				}
			}
		})
	}
}

func TestParseArgsWithConfig(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.toml")

	// Create test config file
	configContent := `server-url = "http://test.example.com:9090"`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	args := []string{"--config", configFile, "status"}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	got, err := ParseArgsWithFlagSet(args, fs)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if got.Config.ServerURL != "http://test.example.com:9090" {
		t.Errorf("ParseArgs() config.ServerURL = %v, want %v", got.Config.ServerURL, "http://test.example.com:9090")
	}
}

func TestParseArgsWithNonExistentConfig(t *testing.T) {
	args := []string{"--config", "/nonexistent/config.toml", "status"}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	_, err := ParseArgsWithFlagSet(args, fs)
	if err == nil {
		t.Error("ParseArgs() expected error for non-existent config file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("ParseArgs() error = %v, want config file not found error", err)
	}
}

func TestCLIExecute(t *testing.T) {
	tests := []struct {
		name       string
		cmdArgs    *CommandArgs
		setupMock  func(*MockHTTPClient)
		wantErr    bool
		wantOutput string
	}{
		{
			name: "help command",
			cmdArgs: &CommandArgs{
				Command: "help",
				Config:  &Config{ServerURL: "http://test.com"},
			},
			wantOutput: "dancerctl - Command line tool",
		},
		{
			name: "status command with no args (acts like switches)",
			cmdArgs: &CommandArgs{
				Command: "status",
				Config:  &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("GET", "/switch/all", 200, `{
					"status": "ok",
					"data": {
						"count": 2,
						"switches": {
							"sw1": {"state": "off", "currentState": false},
							"sw2": {"state": "on", "currentState": true}
						},
						"groups": {
							"group1": {"switches": ["sw1", "sw2"], "summary": false, "state": "off"}
						}
					}
				}`)
			},
			wantOutput: "Switches (2 total)",
		},
		{
			name: "blink command success",
			cmdArgs: &CommandArgs{
				Command:   "blink",
				Args:      []string{"sw1"},
				Period:    2.0,
				Duration:  30,
				DutyCycle: 0.7,
				Config:    &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("POST", "/switch/sw1", 200, `{"status": "ok"}`)
			},
			wantOutput: "Blink started for switch: sw1",
		},
		{
			name: "on command success",
			cmdArgs: &CommandArgs{
				Command:  "on",
				Args:     []string{"sw1"},
				Duration: 60,
				Config:   &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("POST", "/switch/sw1", 200, `{"status": "ok"}`)
			},
			wantOutput: "Switch turned on: sw1",
		},
		{
			name: "toggle command success",
			cmdArgs: &CommandArgs{
				Command: "toggle",
				Args:    []string{"sw1"},
				Config:  &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("POST", "/switch/sw1", 200, `{"status": "ok"}`)
			},
			wantOutput: "Switch toggled: sw1",
		},
		{
			name: "status command success",
			cmdArgs: &CommandArgs{
				Command: "status",
				Args:    []string{"sw1"},
				Config:  &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("GET", "/switch/sw1", 200, `{
					"status": "ok",
					"data": {
						"state": "on",
						"currentState": true,
						"duration": 60,
						"period": 2.0,
						"dutyCycle": 0.5
					}
				}`)
			},
			wantOutput: "Switch: sw1",
		},
		{
			name: "API error response",
			cmdArgs: &CommandArgs{
				Command: "status",
				Config:  &Config{ServerURL: "http://test.com"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("GET", "/switch/all", 400, `{"status": "error", "message": "Invalid request"}`)
			},
			wantErr: true,
		},
		{
			name: "invalid command",
			cmdArgs: &CommandArgs{
				Command: "invalid",
				Config:  &Config{ServerURL: "http://test.com"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient()
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			var stdout, stderr bytes.Buffer
			cli := NewCLI(tt.cmdArgs.Config, mockClient, &stdout, &stderr)

			err := cli.Execute(tt.cmdArgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("CLI.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantOutput != "" {
				output := stdout.String()
				if !strings.Contains(output, tt.wantOutput) {
					t.Errorf("CLI.Execute() output = %v, want to contain %v", output, tt.wantOutput)
				}
			}
		})
	}
}

func TestCLICommandValidation(t *testing.T) {
	tests := []struct {
		name    string
		cmdArgs *CommandArgs
		wantErr bool
		errMsg  string
	}{
		{
			name: "blink with no args",
			cmdArgs: &CommandArgs{
				Command: "blink",
				Args:    []string{},
				Config:  &Config{ServerURL: "http://test.com"},
			},
			wantErr: true,
			errMsg:  "exactly one switch argument",
		},
		{
			name: "blink with too many args",
			cmdArgs: &CommandArgs{
				Command: "blink",
				Args:    []string{"sw1", "sw2"},
				Config:  &Config{ServerURL: "http://test.com"},
			},
			wantErr: true,
			errMsg:  "exactly one switch argument",
		},
		{
			name: "status with too many args",
			cmdArgs: &CommandArgs{
				Command: "status",
				Args:    []string{"sw1", "extra"},
				Config:  &Config{ServerURL: "http://test.com"},
			},
			wantErr: true,
			errMsg:  "zero or one switch argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient()
			var stdout, stderr bytes.Buffer
			cli := NewCLI(tt.cmdArgs.Config, mockClient, &stdout, &stderr)

			err := cli.Execute(tt.cmdArgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("CLI.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("CLI.Execute() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestCLIHTTPRequests(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       *CommandArgs
		setupMock     func(*MockHTTPClient)
		wantMethod    string
		wantPath      string
		wantBodyJSON  string
		wantServerURL string
	}{
		{
			name: "blink request",
			cmdArgs: &CommandArgs{
				Command:   "blink",
				Args:      []string{"sw1"},
				Period:    2.0,
				Duration:  30,
				DutyCycle: 0.7,
				Config:    &Config{ServerURL: "http://custom.com:8080"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("POST", "/switch/sw1", 200, `{"status": "ok"}`)
			},
			wantMethod:    "POST",
			wantPath:      "/switch/sw1",
			wantBodyJSON:  `{"state":"blink","duration":30,"period":2,"dutyCycle":0.7}`,
			wantServerURL: "http://custom.com:8080",
		},
		{
			name: "status request with no args",
			cmdArgs: &CommandArgs{
				Command: "status",
				Config:  &Config{ServerURL: "http://localhost:9090"},
			},
			setupMock: func(m *MockHTTPClient) {
				m.AddResponse("GET", "/switch/all", 200, `{
					"status": "ok",
					"data": {"count": 0, "switches": {}}
				}`)
			},
			wantMethod:    "GET",
			wantPath:      "/switch/all",
			wantServerURL: "http://localhost:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient()
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			var stdout, stderr bytes.Buffer
			cli := NewCLI(tt.cmdArgs.Config, mockClient, &stdout, &stderr)

			err := cli.Execute(tt.cmdArgs)
			if err != nil {
				t.Errorf("CLI.Execute() error = %v", err)
				return
			}

			if mockClient.GetRequestCount() == 0 {
				t.Error("Expected HTTP request to be made")
				return
			}

			req := mockClient.GetLastRequest()
			if req.Method != tt.wantMethod {
				t.Errorf("Request method = %v, want %v", req.Method, tt.wantMethod)
			}

			if req.URL.Path != tt.wantPath {
				t.Errorf("Request path = %v, want %v", req.URL.Path, tt.wantPath)
			}

			expectedURL := tt.wantServerURL + tt.wantPath
			if req.URL.String() != expectedURL {
				t.Errorf("Request URL = %v, want %v", req.URL.String(), expectedURL)
			}

			if tt.wantBodyJSON != "" {
				bodyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
				}
				// Normalize JSON for comparison
				bodyStr := strings.TrimSpace(string(bodyBytes))
				if bodyStr != tt.wantBodyJSON {
					t.Errorf("Request body = %v, want %v", bodyStr, tt.wantBodyJSON)
				}
			}
		})
	}
}

func TestConfig(t *testing.T) {
	// Test default values
	cfg := NewConfig()
	if cfg.ServerURL != defaultServerURL {
		t.Errorf("NewConfig().ServerURL = %v, want %v", cfg.ServerURL, defaultServerURL)
	}

	// Test default config file path
	defaultPath := getDefaultConfigFile()
	if defaultPath == "" {
		t.Error("getDefaultConfigFile() returned empty string")
	}
	if !strings.Contains(defaultPath, ".config/dancer/dancer.toml") {
		t.Errorf("getDefaultConfigFile() = %v, want path containing .config/dancer/dancer.toml", defaultPath)
	}
}

func TestConfigLoadWithDefaults(t *testing.T) {
	tempDir := t.TempDir()

	// Save and restore original HOME
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tempDir)

	cfg := NewConfig()
	// Set the config file to the default path for this test
	cfg.ConfigFile = getDefaultConfigFile()

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	// Test with non-existent default config file (should not error)
	err := cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfig() with non-existent default config should not error: %v", err)
	}

	// Should use default server URL
	if cfg.ServerURL != defaultServerURL {
		t.Errorf("LoadConfig() ServerURL = %v, want %v", cfg.ServerURL, defaultServerURL)
	}
}

func TestConfigLoadWithExplicitFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "explicit-config.toml")

	// Create test config file
	configContent := `server-url = "http://explicit.example.com:8080"`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg := NewConfig()
	cfg.ConfigFile = configFile

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err = cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfig() error = %v", err)
	}

	if cfg.ServerURL != "http://explicit.example.com:8080" {
		t.Errorf("LoadConfig() ServerURL = %v, want %v", cfg.ServerURL, "http://explicit.example.com:8080")
	}
}

func TestConfigLoadWithNonExistentExplicitFile(t *testing.T) {
	cfg := NewConfig()
	cfg.ConfigFile = "/nonexistent/config.toml"

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err := cfg.LoadConfigWithFlagSet(fs)
	if err == nil {
		t.Error("LoadConfig() expected error for non-existent explicit config file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("LoadConfig() error = %v, want config file not found error", err)
	}
}

// Benchmark tests
func BenchmarkParseArgs(b *testing.B) {
	args := []string{"--server-url", "http://localhost:8080", "blink", "sw1", "--period", "2.0"}

	for i := 0; i < b.N; i++ {
		// Create a new flag set for each benchmark iteration
		fs := pflag.NewFlagSet("benchmark", pflag.ContinueOnError)
		fs.Usage = func() {} // Suppress usage output

		_, err := ParseArgsWithFlagSet(args, fs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCLIExecute(b *testing.B) {
	// Use help command for benchmark since it doesn't make HTTP requests
	cmdArgs := &CommandArgs{
		Command: "help",
		Config:  &Config{ServerURL: "http://localhost:8080"},
	}

	mockClient := NewMockHTTPClient()
	var stdout, stderr bytes.Buffer
	cli := NewCLI(cmdArgs.Config, mockClient, &stdout, &stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		stderr.Reset()
		err := cli.Execute(cmdArgs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
