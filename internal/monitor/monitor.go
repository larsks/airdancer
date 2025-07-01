package monitor

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// EmailMonitor handles monitoring an IMAP mailbox for new emails
type EmailMonitor struct {
	config      Config
	client      *client.Client
	regex       *regexp.Regexp
	lastUID     uint32
	reconnectCh chan bool
}

// NewEmailMonitor creates a new EmailMonitor with the given configuration
func NewEmailMonitor(config Config) (*EmailMonitor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(config.Monitor.RegexPattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRegexPattern, err)
	}

	monitor := &EmailMonitor{
		config:      config,
		regex:       regex,
		reconnectCh: make(chan bool, 1),
	}

	return monitor, nil
}

// Start begins monitoring the IMAP mailbox for new emails
func (em *EmailMonitor) Start() {
	log.Println("starting email monitor...")

	for {
		err := em.connect()
		if err != nil {
			log.Printf("connection failed: %v. Retrying in 30 seconds...", err)
			time.Sleep(30 * time.Second)
			continue
		}

		log.Println("connected to IMAP server")

		// Get initial state
		err = em.initializeLastUID()
		if err != nil {
			log.Printf("failed to initialize: %v", err)
			em.disconnect()
			continue
		}

		// Start monitoring
		err = em.monitor()
		if err != nil {
			log.Printf("monitor error: %v. Reconnecting...", err)
		}

		em.disconnect()
		time.Sleep(5 * time.Second)
	}
}

// Stop stops the email monitor
func (em *EmailMonitor) Stop() {
	em.disconnect()
}

// connect establishes a connection to the IMAP server
func (em *EmailMonitor) connect() error {
	var c *client.Client
	var err error

	address := fmt.Sprintf("%s:%d", em.config.IMAP.Server, em.config.IMAP.Port)
	log.Printf("connecting to %s", address)

	if em.config.IMAP.UseSSL {
		c, err = client.DialTLS(address, &tls.Config{})
	} else {
		c, err = client.Dial(address)
	}

	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	if err := c.Login(em.config.IMAP.Username, em.config.IMAP.Password); err != nil {
		c.Close()
		return fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}

	em.client = c
	return nil
}

// disconnect closes the IMAP connection
func (em *EmailMonitor) disconnect() {
	if em.client != nil {
		em.client.Close()
		em.client = nil
	}
}

// initializeLastUID gets the UID of the most recent message to track new emails
func (em *EmailMonitor) initializeLastUID() error {
	mailbox := em.config.IMAP.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	mbox, err := em.client.Select(mailbox, false)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMailboxNotFound, err)
	}

	// If the mailbox is empty, there's no UID
	if mbox.Messages == 0 {
		em.lastUID = 0
		log.Printf("mailbox is empty, starting with UID: 0")
		return nil
	}

	// Search for all messages to get the sequence numbers
	criteria := imap.NewSearchCriteria()
	criteria.SeqNum = new(imap.SeqSet)
	criteria.SeqNum.AddRange(1, mbox.Messages)
	uids, err := em.client.Search(criteria)
	if err != nil {
		return err
	}

	if len(uids) == 0 {
		em.lastUID = 0
		log.Printf("mailbox is empty, starting with UID: 0")
		return nil
	}

	// Fetch the UID of the last message
	seqset := new(imap.SeqSet)
	seqset.AddNum(uids[len(uids)-1])

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- em.client.Fetch(seqset, []imap.FetchItem{imap.FetchUid}, messages)
	}()

	msg := <-messages
	em.lastUID = msg.Uid

	if err := <-done; err != nil {
		return err
	}

	log.Printf("initialized with last UID: %d (UidNext: %d)", em.lastUID, mbox.UidNext)
	return nil
}

// monitor continuously checks for new messages
func (em *EmailMonitor) monitor() error {
	checkInterval := time.Duration(em.config.Monitor.CheckInterval) * time.Second
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := em.checkForNewMessages()
			if err != nil {
				return err
			}
		}
	}
}

// checkForNewMessages looks for new messages and processes them
func (em *EmailMonitor) checkForNewMessages() error {
	mailbox := em.config.IMAP.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	log.Printf("checking for new messages in %s", mailbox)

	mbox, err := em.client.Select(mailbox, false)
	if err != nil {
		return err
	}

	// If no messages in mailbox, nothing to do
	if mbox.Messages == 0 {
		log.Println("no messages")
		return nil
	}

	// Search for messages with UID greater than lastUID
	criteria := imap.NewSearchCriteria()
	criteria.Uid = new(imap.SeqSet)

	// Only search if we have a valid lastUID
	if em.lastUID > 0 {
		criteria.Uid.AddRange(em.lastUID+1, 0)
	} else {
		// If lastUID is 0, we want all messages
		criteria.Uid.AddRange(1, 0)
	}

	uids, err := em.client.UidSearch(criteria)
	if err != nil {
		return err
	}

	// Filter out UIDs that we've already seen (additional safety check)
	var newUIDs []uint32
	for _, uid := range uids {
		if uid > em.lastUID {
			newUIDs = append(newUIDs, uid)
		}
	}

	if len(newUIDs) == 0 {
		log.Println("no new messages")
		return nil
	}

	log.Printf("found %d new messages (UIDs: %v)", len(newUIDs), newUIDs)

	// Fetch new messages
	seqset := new(imap.SeqSet)
	for _, uid := range newUIDs {
		seqset.AddNum(uid)
	}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- em.client.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBodyStructure, "BODY[]"}, messages)
	}()

	var processedUIDs []uint32
	for msg := range messages {
		err := em.processMessage(msg)
		if err != nil {
			log.Printf("error processing message UID %d: %v", msg.Uid, err)
		}

		processedUIDs = append(processedUIDs, msg.Uid)
		if msg.Uid > em.lastUID {
			em.lastUID = msg.Uid
		}
	}

	if err := <-done; err != nil {
		return err
	}

	log.Printf("processed messages with UIDs: %v, new lastUID: %d", processedUIDs, em.lastUID)
	return nil
}

// processMessage processes a single email message
func (em *EmailMonitor) processMessage(msg *imap.Message) error {
	if msg == nil || msg.Envelope == nil {
		log.Println("skipping message with nil envelope")
		return nil
	}

	from := "<unknown>"
	if len(msg.Envelope.From) > 0 {
		from = msg.Envelope.From[0].Address()
	}

	log.Printf("processing message from: %s, Subject: %s", from, msg.Envelope.Subject)

	// Get message body
	for _, part := range msg.Body {
		body, err := em.extractTextFromPart(part)
		if err != nil {
			log.Printf("error extracting text: %v", err)
			continue
		}

		if body == "" {
			continue
		}

		// Check if body matches regex
		if em.regex.MatchString(body) {
			log.Printf("regex match found in message from: %s", from)

			err := em.executeCommand(msg, body)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrCommandExecution, err)
			}

			return nil
		}
	}

	return nil
}

// extractTextFromPart extracts text content from an email part
func (em *EmailMonitor) extractTextFromPart(part io.Reader) (string, error) {
	mr, err := mail.CreateReader(part)
	if err != nil {
		return "", err
	}

	var body strings.Builder

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			mediaType, _, _ := h.ContentType()
			if strings.HasPrefix(mediaType, "text/") {
				content, err := io.ReadAll(p.Body)
				if err != nil {
					continue
				}
				body.WriteString(string(content))
			}
		}
	}

	return body.String(), nil
}

// executeCommand runs the configured command when a regex match is found
func (em *EmailMonitor) executeCommand(msg *imap.Message, body string) error {
	if em.config.Monitor.Command == "" {
		log.Println("no command configured")
		return nil
	}

	from := "<unknown>"
	if msg.Envelope != nil && len(msg.Envelope.From) > 0 {
		from = msg.Envelope.From[0].Address()
	}

	// Set environment variables with message info
	env := os.Environ()
	env = append(env, fmt.Sprintf("EMAIL_FROM=%s", from))
	env = append(env, fmt.Sprintf("EMAIL_SUBJECT=%s", msg.Envelope.Subject))
	env = append(env, fmt.Sprintf("EMAIL_DATE=%s", msg.Envelope.Date.Format(time.RFC3339)))
	env = append(env, fmt.Sprintf("EMAIL_UID=%d", msg.Uid))

	// Parse command (simple shell command parsing)
	parts := strings.Fields(em.config.Monitor.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(body)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("command execution failed: %v, Output: %s", err, string(output))
		return err
	}

	log.Printf("command executed successfully. Output: %s", string(output))
	return nil
}
