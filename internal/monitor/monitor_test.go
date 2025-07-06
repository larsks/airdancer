package monitor

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
)

//go:embed testdata/test-email-with-pattern.txt
var testEmailWithPattern string

//go:embed testdata/test-email-without-match.txt
var testEmailWithoutMatch string

// Mock implementations for testing

type MockLiteral struct {
	content string
	pos     int
}

func (m *MockLiteral) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.content) {
		return 0, io.EOF
	}
	n = copy(p, m.content[m.pos:])
	m.pos += n
	return n, nil
}

func (m *MockLiteral) Len() int {
	return len(m.content)
}

type MockIMAPClient struct {
	loginErr        error
	selectErr       error
	searchErr       error
	uidSearchErr    error
	fetchErr        error
	uidFetchErr     error
	closeErr        error
	loginCalled     bool
	selectCalled    bool
	searchCalled    bool
	uidSearchCalled bool
	fetchCalled     bool
	uidFetchCalled  bool
	closeCalled     bool

	// Return values
	mailboxStatus *imap.MailboxStatus
	searchResults []uint32
	messages      []*imap.Message
}

func (m *MockIMAPClient) Login(username, password string) error {
	m.loginCalled = true
	return m.loginErr
}

func (m *MockIMAPClient) Select(name string, readOnly bool) (*imap.MailboxStatus, error) {
	m.selectCalled = true
	if m.selectErr != nil {
		return nil, m.selectErr
	}
	if m.mailboxStatus == nil {
		return &imap.MailboxStatus{Messages: 0}, nil
	}
	return m.mailboxStatus, nil
}

func (m *MockIMAPClient) Search(criteria *imap.SearchCriteria) ([]uint32, error) {
	m.searchCalled = true
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults, nil
}

func (m *MockIMAPClient) UidSearch(criteria *imap.SearchCriteria) ([]uint32, error) {
	m.uidSearchCalled = true
	if m.uidSearchErr != nil {
		return nil, m.uidSearchErr
	}
	return m.searchResults, nil
}

func (m *MockIMAPClient) Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	m.fetchCalled = true
	if m.fetchErr != nil {
		return m.fetchErr
	}
	go func() {
		defer close(ch)
		for _, msg := range m.messages {
			ch <- msg
		}
	}()
	return nil
}

func (m *MockIMAPClient) UidFetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	m.uidFetchCalled = true
	if m.uidFetchErr != nil {
		return m.uidFetchErr
	}
	go func() {
		defer close(ch)
		for _, msg := range m.messages {
			ch <- msg
		}
	}()
	return nil
}

func (m *MockIMAPClient) Close() error {
	m.closeCalled = true
	return m.closeErr
}

type MockIMAPDialer struct {
	dialTLSErr    error
	dialErr       error
	dialTLSCalled bool
	dialCalled    bool
	client        IMAPClient
}

func (m *MockIMAPDialer) DialTLS(addr string) (IMAPClient, error) {
	m.dialTLSCalled = true
	if m.dialTLSErr != nil {
		return nil, m.dialTLSErr
	}
	return m.client, nil
}

func (m *MockIMAPDialer) Dial(addr string) (IMAPClient, error) {
	m.dialCalled = true
	if m.dialErr != nil {
		return nil, m.dialErr
	}
	return m.client, nil
}

type MockCommandExecutor struct {
	executeErr    error
	executeCalled bool
	lastCommand   string
	lastEnv       []string
	lastStdin     string
}

func (m *MockCommandExecutor) Execute(command string, env []string, stdin io.Reader) error {
	m.executeCalled = true
	m.lastCommand = command
	m.lastEnv = env
	if stdin != nil {
		stdinBytes, _ := io.ReadAll(stdin)
		m.lastStdin = string(stdinBytes)
	}
	return m.executeErr
}

type MockLogger struct {
	printfCalls  []string
	printlnCalls []string
}

func (m *MockLogger) Printf(format string, v ...any) {
	m.printfCalls = append(m.printfCalls, fmt.Sprintf(format, v...))
}

func (m *MockLogger) Println(v ...any) {
	m.printlnCalls = append(m.printlnCalls, fmt.Sprint(v...))
}

type MockTicker struct {
	ch      chan time.Time
	stopped bool
}

func (m *MockTicker) C() <-chan time.Time {
	return m.ch
}

func (m *MockTicker) Stop() {
	m.stopped = true
	close(m.ch)
}

type MockTimer struct {
	tickerCh      chan time.Time
	ticker        *MockTicker
	sleepDuration time.Duration
}

func (m *MockTimer) NewTicker(d time.Duration) Ticker {
	m.ticker = &MockTicker{ch: make(chan time.Time, 1)}
	m.tickerCh = m.ticker.ch
	return m.ticker
}

func (m *MockTimer) Sleep(d time.Duration) {
	m.sleepDuration = d
}

func (m *MockTimer) TriggerTick() {
	if m.tickerCh != nil {
		select {
		case m.tickerCh <- time.Now():
		default:
		}
	}
}

// Test functions

func TestNewEmailMonitor(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedError error
	}{
		{
			name: "valid config",
			config: Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: "test.*pattern",
				},
			},
			expectedError: nil,
		},
		{
			name: "invalid config - missing server",
			config: Config{
				IMAP: IMAPConfig{
					Port: 993,
				},
				Monitor: MonitorConfig{
					RegexPattern: "test.*pattern",
				},
			},
			expectedError: ErrMissingIMAPServer,
		},
		{
			name: "invalid regex pattern",
			config: Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: "[invalid", // Invalid regex
				},
			},
			expectedError: ErrInvalidRegexPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor, err := NewEmailMonitor(
				tt.config,
				&MockIMAPDialer{},
				&MockCommandExecutor{},
				&MockLogger{},
				&MockTimer{},
			)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if !errors.Is(err, tt.expectedError) && !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Errorf("Expected error containing %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if monitor == nil {
					t.Error("Expected monitor to be created")
				}
			}
		})
	}
}

func TestEmailMonitorConnect(t *testing.T) {
	tests := []struct {
		name          string
		useSSL        bool
		dialTLSErr    error
		dialErr       error
		loginErr      error
		expectedError error
	}{
		{
			name:          "successful SSL connection",
			useSSL:        true,
			expectedError: nil,
		},
		{
			name:          "successful non-SSL connection",
			useSSL:        false,
			expectedError: nil,
		},
		{
			name:          "SSL dial error",
			useSSL:        true,
			dialTLSErr:    errors.New("connection failed"),
			expectedError: ErrConnectionFailed,
		},
		{
			name:          "non-SSL dial error",
			useSSL:        false,
			dialErr:       errors.New("connection failed"),
			expectedError: ErrConnectionFailed,
		},
		{
			name:          "login error",
			useSSL:        true,
			loginErr:      errors.New("auth failed"),
			expectedError: ErrAuthenticationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				IMAP: IMAPConfig{
					Server:   "imap.example.com",
					Port:     993,
					Username: "user@example.com",
					Password: "password",
					UseSSL:   tt.useSSL,
				},
				Monitor: MonitorConfig{
					RegexPattern: "test",
				},
			}

			mockClient := &MockIMAPClient{loginErr: tt.loginErr}
			mockDialer := &MockIMAPDialer{
				dialTLSErr: tt.dialTLSErr,
				dialErr:    tt.dialErr,
				client:     mockClient,
			}

			monitor, err := NewEmailMonitor(config, mockDialer, &MockCommandExecutor{}, &MockLogger{}, &MockTimer{})
			if err != nil {
				t.Fatalf("Failed to create monitor: %v", err)
			}

			err = monitor.connect()

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error containing %v, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Errorf("Expected error containing %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}

				// Verify correct dial method was called
				if tt.useSSL {
					if !mockDialer.dialTLSCalled {
						t.Error("Expected DialTLS to be called")
					}
				} else {
					if !mockDialer.dialCalled {
						t.Error("Expected Dial to be called")
					}
				}

				// Verify login was called
				if !mockClient.loginCalled {
					t.Error("Expected Login to be called")
				}
			}
		})
	}
}

func TestEmailMonitorInitializeLastUID(t *testing.T) {
	tests := []struct {
		name          string
		mailboxStatus *imap.MailboxStatus
		searchResults []uint32
		messages      []*imap.Message
		selectErr     error
		searchErr     error
		fetchErr      error
		expectedUID   uint32
		expectedError bool
	}{
		{
			name:          "empty mailbox",
			mailboxStatus: &imap.MailboxStatus{Messages: 0},
			expectedUID:   0,
		},
		{
			name:          "mailbox with messages",
			mailboxStatus: &imap.MailboxStatus{Messages: 3, UidNext: 5},
			searchResults: []uint32{1, 2, 3},
			messages:      []*imap.Message{{Uid: 3}},
			expectedUID:   3,
		},
		{
			name:          "select error",
			selectErr:     errors.New("mailbox not found"),
			expectedError: true,
		},
		{
			name:          "search error",
			mailboxStatus: &imap.MailboxStatus{Messages: 1},
			searchErr:     errors.New("search failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				IMAP: IMAPConfig{
					Server:  "imap.example.com",
					Port:    993,
					Mailbox: "INBOX",
				},
				Monitor: MonitorConfig{
					RegexPattern: "test",
				},
			}

			mockClient := &MockIMAPClient{
				mailboxStatus: tt.mailboxStatus,
				searchResults: tt.searchResults,
				messages:      tt.messages,
				selectErr:     tt.selectErr,
				searchErr:     tt.searchErr,
				fetchErr:      tt.fetchErr,
			}

			monitor, err := NewEmailMonitor(config, &MockIMAPDialer{}, &MockCommandExecutor{}, &MockLogger{}, &MockTimer{})
			if err != nil {
				t.Fatalf("Failed to create monitor: %v", err)
			}

			monitor.client = mockClient

			err = monitor.initializeLastUID()

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if monitor.lastUID != tt.expectedUID {
					t.Errorf("Expected lastUID %d, got %d", tt.expectedUID, monitor.lastUID)
				}
			}
		})
	}
}

func TestEmailMonitorProcessMessage(t *testing.T) {
	tests := []struct {
		name            string
		message         *imap.Message
		regexPattern    string
		command         string
		expectedCommand bool
		expectedError   bool
	}{
		{
			name:            "nil message",
			message:         nil,
			regexPattern:    "test",
			expectedCommand: false,
		},
		{
			name: "nil envelope",
			message: &imap.Message{
				Envelope: nil,
			},
			regexPattern:    "test",
			expectedCommand: false,
		},
		{
			name: "regex match with command",
			message: &imap.Message{
				Envelope: &imap.Envelope{
					Subject: "Test Subject",
					From:    []*imap.Address{{PersonalName: "Test", MailboxName: "test", HostName: "example.com"}},
				},
				Body: map[*imap.BodySectionName]imap.Literal{
					{}: &MockLiteral{content: testEmailWithPattern},
				},
			},
			regexPattern:    "pattern",
			command:         "echo 'matched'",
			expectedCommand: true,
		},
		{
			name: "no regex match",
			message: &imap.Message{
				Envelope: &imap.Envelope{
					Subject: "Test Subject",
					From:    []*imap.Address{{PersonalName: "Test", MailboxName: "test", HostName: "example.com"}},
				},
				Body: map[*imap.BodySectionName]imap.Literal{
					{}: &MockLiteral{content: testEmailWithoutMatch},
				},
			},
			regexPattern:    "different.*pattern",
			expectedCommand: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: tt.regexPattern,
					Command:      tt.command,
				},
			}

			mockExecutor := &MockCommandExecutor{}
			mockLogger := &MockLogger{}

			monitor, err := NewEmailMonitor(config, &MockIMAPDialer{}, mockExecutor, mockLogger, &MockTimer{})
			if err != nil {
				t.Fatalf("Failed to create monitor: %v", err)
			}

			err = monitor.processMessage(tt.message)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}

			if tt.expectedCommand {
				if !mockExecutor.executeCalled {
					t.Error("Expected command to be executed")
				}
				if mockExecutor.lastCommand != tt.command {
					t.Errorf("Expected command %q, got %q", tt.command, mockExecutor.lastCommand)
				}
			} else {
				if mockExecutor.executeCalled {
					t.Error("Expected command not to be executed")
				}
			}
		})
	}
}

func TestEmailMonitorExecuteCommand(t *testing.T) {
	config := Config{
		IMAP: IMAPConfig{
			Server: "imap.example.com",
			Port:   993,
		},
		Monitor: MonitorConfig{
			RegexPattern: "test",
			Command:      "echo 'test command'",
		},
	}

	mockExecutor := &MockCommandExecutor{}
	mockLogger := &MockLogger{}

	monitor, err := NewEmailMonitor(config, &MockIMAPDialer{}, mockExecutor, mockLogger, &MockTimer{})
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	message := &imap.Message{
		Uid: 123,
		Envelope: &imap.Envelope{
			Subject: "Test Subject",
			From:    []*imap.Address{{PersonalName: "Test", MailboxName: "test", HostName: "example.com"}},
			Date:    time.Now(),
		},
	}

	body := "test email body"

	err = monitor.executeCommand(message, body)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockExecutor.executeCalled {
		t.Error("Expected Execute to be called")
	}

	if mockExecutor.lastCommand != config.Monitor.Command {
		t.Errorf("Expected command %q, got %q", config.Monitor.Command, mockExecutor.lastCommand)
	}

	if mockExecutor.lastStdin != body {
		t.Errorf("Expected stdin %q, got %q", body, mockExecutor.lastStdin)
	}

	// Check environment variables
	found := false
	for _, env := range mockExecutor.lastEnv {
		if strings.HasPrefix(env, "EMAIL_FROM=") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected EMAIL_FROM environment variable to be set")
	}
}

func TestEmailMonitorExecuteCommandNoCommand(t *testing.T) {
	config := Config{
		IMAP: IMAPConfig{
			Server: "imap.example.com",
			Port:   993,
		},
		Monitor: MonitorConfig{
			RegexPattern: "test",
			Command:      "", // No command configured
		},
	}

	mockExecutor := &MockCommandExecutor{}
	mockLogger := &MockLogger{}

	monitor, err := NewEmailMonitor(config, &MockIMAPDialer{}, mockExecutor, mockLogger, &MockTimer{})
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	message := &imap.Message{
		Uid: 123,
		Envelope: &imap.Envelope{
			Subject: "Test Subject",
		},
	}

	err = monitor.executeCommand(message, "test body")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if mockExecutor.executeCalled {
		t.Error("Expected Execute not to be called when no command is configured")
	}

	// Check that "no command configured" was logged
	found := false
	for _, msg := range mockLogger.printlnCalls {
		if strings.Contains(msg, "no command configured") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'no command configured' to be logged")
	}
}

func TestEmailMonitorStop(t *testing.T) {
	config := Config{
		IMAP: IMAPConfig{
			Server: "imap.example.com",
			Port:   993,
		},
		Monitor: MonitorConfig{
			RegexPattern: "test",
		},
	}

	mockClient := &MockIMAPClient{}
	monitor, err := NewEmailMonitor(config, &MockIMAPDialer{}, &MockCommandExecutor{}, &MockLogger{}, &MockTimer{})
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	monitor.client = mockClient

	// Test that Stop() closes the stop channel and calls disconnect
	monitor.Stop()

	// Verify client.Close was called
	if !mockClient.closeCalled {
		t.Error("Expected Close to be called on client")
	}

	// Verify stop channel is closed by trying to read from it
	select {
	case <-monitor.stopCh:
		// Expected - channel should be closed
	default:
		t.Error("Expected stop channel to be closed")
	}
}
