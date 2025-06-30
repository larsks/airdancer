package monitor

import "errors"

var (
	// Configuration errors
	ErrMissingIMAPServer    = errors.New("IMAP server must be set")
	ErrInvalidIMAPPort      = errors.New("IMAP port must be non-zero")
	ErrMissingRegexPattern  = errors.New("regex pattern must be set")
	ErrInvalidRegexPattern  = errors.New("invalid regex pattern")
	
	// Connection errors
	ErrConnectionFailed     = errors.New("failed to connect to IMAP server")
	ErrAuthenticationFailed = errors.New("IMAP authentication failed")
	ErrMailboxNotFound      = errors.New("mailbox not found")
	
	// Message processing errors
	ErrMessageProcessing    = errors.New("error processing message")
	ErrCommandExecution     = errors.New("error executing command")
)