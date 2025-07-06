package monitor

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// IMAPClient interface abstracts the IMAP client for testing
type IMAPClient interface {
	Login(username, password string) error
	Select(name string, readOnly bool) (*imap.MailboxStatus, error)
	Search(criteria *imap.SearchCriteria) ([]uint32, error)
	UidSearch(criteria *imap.SearchCriteria) ([]uint32, error)
	Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error
	UidFetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error
	Close() error
}

// IMAPDialer interface abstracts IMAP connection creation
type IMAPDialer interface {
	DialTLS(addr string) (IMAPClient, error)
	Dial(addr string) (IMAPClient, error)
}

// CommandExecutor interface abstracts command execution for testing
type CommandExecutor interface {
	Execute(command string, env []string, stdin io.Reader) error
}

// Logger interface abstracts logging for testing
type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
}

// Timer interface abstracts time operations for testing
type Timer interface {
	NewTicker(d time.Duration) Ticker
	Sleep(d time.Duration)
}

// Ticker interface abstracts ticker for testing
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// RealIMAPDialer implements IMAPDialer using the real go-imap client
type RealIMAPDialer struct{}

func (r *RealIMAPDialer) DialTLS(addr string) (IMAPClient, error) {
	c, err := client.DialTLS(addr, &tls.Config{})
	if err != nil {
		return nil, err
	}
	return &RealIMAPClient{c}, nil
}

func (r *RealIMAPDialer) Dial(addr string) (IMAPClient, error) {
	c, err := client.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &RealIMAPClient{c}, nil
}

// RealIMAPClient wraps the real IMAP client to implement our interface
type RealIMAPClient struct {
	*client.Client
}

func (r *RealIMAPClient) Close() error {
	return r.Client.Close()
}

// RealCommandExecutor implements CommandExecutor using os/exec
type RealCommandExecutor struct{}

func (r *RealCommandExecutor) Execute(command string, env []string, stdin io.Reader) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w \"%s\": %v", ErrCommandExecution, command, err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("command execution failed: %v, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
		} else {
			log.Printf("command executed successfully. stdout: %s, stderr: %s", stdout.String(), stderr.String())
		}
	}()

	return nil
}

// RealLogger implements Logger using the standard log package
type RealLogger struct{}

func (r *RealLogger) Printf(format string, v ...any) {
	log.Printf(format, v...)
}

func (r *RealLogger) Println(v ...any) {
	log.Println(v...)
}

// RealTimer implements Timer using real time operations
type RealTimer struct{}

func (r *RealTimer) NewTicker(d time.Duration) Ticker {
	return &RealTicker{time.NewTicker(d)}
}

func (r *RealTimer) Sleep(d time.Duration) {
	time.Sleep(d)
}

// RealTicker implements Ticker using time.Ticker
type RealTicker struct {
	*time.Ticker
}

func (r *RealTicker) C() <-chan time.Time {
	return r.Ticker.C
}

func (r *RealTicker) Stop() {
	r.Ticker.Stop()
}
