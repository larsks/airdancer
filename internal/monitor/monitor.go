package monitor

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"
)

// compiledTrigger holds a compiled regex pattern with its associated command
type compiledTrigger struct {
	regex   *regexp.Regexp
	command string
}

// compiledMailbox holds a compiled mailbox configuration
type compiledMailbox struct {
	mailbox       string
	checkInterval int
	triggers      []compiledTrigger
}

// EmailMonitor handles monitoring multiple IMAP mailboxes for new emails
type EmailMonitor struct {
	config      Config
	client      IMAPClient
	mailboxes   []compiledMailbox
	lastUIDs    map[string]uint32
	reconnectCh chan bool

	// Injected dependencies for testability
	dialer   IMAPDialer
	executor CommandExecutor
	logger   Logger
	timer    Timer

	// Control channels for testing
	stopCh chan struct{}
}

// NewEmailMonitor creates a new EmailMonitor with the given configuration and dependencies
func NewEmailMonitor(config Config, dialer IMAPDialer, executor CommandExecutor, logger Logger, timer Timer) (*EmailMonitor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	var mailboxes []compiledMailbox
	for _, mailboxConfig := range config.Monitor {
		var triggers []compiledTrigger
		for j, triggerConfig := range mailboxConfig.Triggers {
			regex, err := regexp.Compile(triggerConfig.RegexPattern)
			if err != nil {
				return nil, fmt.Errorf("%w \"%s\" in trigger %d of mailbox %s: %v", ErrInvalidRegexPattern, triggerConfig.RegexPattern, j, mailboxConfig.Mailbox, err)
			}
			triggers = append(triggers, compiledTrigger{
				regex:   regex,
				command: triggerConfig.Command,
			})
		}

		mailboxes = append(mailboxes, compiledMailbox{
			mailbox:       mailboxConfig.Mailbox,
			checkInterval: config.GetEffectiveCheckInterval(&mailboxConfig),
			triggers:      triggers,
		})
	}

	monitor := &EmailMonitor{
		config:      config,
		mailboxes:   mailboxes,
		lastUIDs:    make(map[string]uint32),
		reconnectCh: make(chan bool, 1),
		dialer:      dialer,
		executor:    executor,
		logger:      logger,
		timer:       timer,
		stopCh:      make(chan struct{}),
	}

	return monitor, nil
}

// NewEmailMonitorWithDefaults creates a new EmailMonitor with default (real) implementations
func NewEmailMonitorWithDefaults(config Config) (*EmailMonitor, error) {
	return NewEmailMonitor(
		config,
		&RealIMAPDialer{},
		&RealCommandExecutor{},
		&RealLogger{},
		&RealTimer{},
	)
}

// Start begins monitoring all configured IMAP mailboxes for new emails
func (em *EmailMonitor) Start() {
	em.logger.Println("starting email monitor...")

	for {
		select {
		case <-em.stopCh:
			em.logger.Println("email monitor stopped")
			return
		default:
		}

		err := em.connect()
		if err != nil {
			em.logger.Printf("connection failed: %v. Retrying in 30 seconds...", err)
			select {
			case <-em.stopCh:
				em.logger.Println("email monitor stopped during reconnect wait")
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}

		em.logger.Println("connected to IMAP server")

		// Initialize last UIDs for all mailboxes
		err = em.initializeLastUIDs()
		if err != nil {
			em.logger.Printf("failed to initialize: %v", err)
			em.disconnect()
			continue
		}

		// Start monitoring all mailboxes
		err = em.monitorAllMailboxes()
		if err != nil {
			em.logger.Printf("monitor error: %v. Reconnecting...", err)
		}

		em.disconnect()
		select {
		case <-em.stopCh:
			em.logger.Println("email monitor stopped")
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// Stop stops the email monitor
func (em *EmailMonitor) Stop() {
	close(em.stopCh)
	em.disconnect()
}

// connect establishes a connection to the IMAP server
func (em *EmailMonitor) connect() error {
	var c IMAPClient
	var err error

	address := fmt.Sprintf("%s:%d", em.config.IMAP.Server, em.config.IMAP.Port)
	em.logger.Printf("connecting to %s", address)

	if em.config.IMAP.UseSSL {
		c, err = em.dialer.DialTLS(address)
	} else {
		c, err = em.dialer.Dial(address)
	}

	if err != nil {
		return fmt.Errorf("%w to %s: %v", ErrConnectionFailed, address, err)
	}

	if err := c.Login(em.config.IMAP.Username, em.config.IMAP.Password); err != nil {
		c.Close() //nolint:errcheck
		return fmt.Errorf("%w for user %s: %v", ErrAuthenticationFailed, em.config.IMAP.Username, err)
	}

	em.client = c
	return nil
}

// disconnect closes the IMAP connection
func (em *EmailMonitor) disconnect() {
	if em.client != nil {
		em.client.Close() //nolint:errcheck
		em.client = nil
	}
}

// initializeLastUIDs gets the UID of the most recent message in each mailbox to track new emails
func (em *EmailMonitor) initializeLastUIDs() error {
	for _, mailbox := range em.mailboxes {
		uid, err := em.initializeLastUIDForMailbox(mailbox.mailbox)
		if err != nil {
			return err
		}
		em.lastUIDs[mailbox.mailbox] = uid
	}
	return nil
}

// initializeLastUIDForMailbox gets the UID of the most recent message in a specific mailbox
func (em *EmailMonitor) initializeLastUIDForMailbox(mailboxName string) (uint32, error) {
	em.logger.Printf("selecting mailbox %s", mailboxName)
	mbox, err := em.client.Select(mailboxName, false)
	if err != nil {
		return 0, fmt.Errorf("%w \"%s\": %v", ErrMailboxNotFound, mailboxName, err)
	}

	// If the mailbox is empty, there's no UID
	if mbox.Messages == 0 {
		em.logger.Printf("mailbox %s is empty, starting with UID: 0", mailboxName)
		return 0, nil
	}

	// Search for all messages to get the sequence numbers
	criteria := imap.NewSearchCriteria()
	criteria.SeqNum = new(imap.SeqSet)
	criteria.SeqNum.AddRange(1, mbox.Messages)
	uids, err := em.client.Search(criteria)
	if err != nil {
		return 0, err
	}

	if len(uids) == 0 {
		em.logger.Printf("mailbox %s is empty, starting with UID: 0", mailboxName)
		return 0, nil
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
	lastUID := msg.Uid

	if err := <-done; err != nil {
		return 0, err
	}

	em.logger.Printf("initialized mailbox %s with last UID: %d (UidNext: %d)", mailboxName, lastUID, mbox.UidNext)
	return lastUID, nil
}

// monitorAllMailboxes coordinates monitoring of all configured mailboxes
func (em *EmailMonitor) monitorAllMailboxes() error {
	// Group mailboxes by their check interval to optimize monitoring
	intervalGroups := make(map[int][]compiledMailbox)
	for _, mailbox := range em.mailboxes {
		interval := mailbox.checkInterval
		intervalGroups[interval] = append(intervalGroups[interval], mailbox)
	}

	// Start a goroutine for each interval group
	errorCh := make(chan error, len(intervalGroups))
	for interval, mailboxes := range intervalGroups {
		go func(interval int, mailboxes []compiledMailbox) {
			err := em.monitorMailboxGroup(interval, mailboxes)
			if err != nil {
				errorCh <- err
			}
		}(interval, mailboxes)
	}

	// Wait for the first error or stop signal
	select {
	case <-em.stopCh:
		return nil
	case err := <-errorCh:
		return err
	}
}

// monitorMailboxGroup monitors a group of mailboxes with the same check interval
func (em *EmailMonitor) monitorMailboxGroup(checkInterval int, mailboxes []compiledMailbox) error {
	ticker := em.timer.NewTicker(time.Duration(checkInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-em.stopCh:
			return nil
		case <-ticker.C():
			for _, mailbox := range mailboxes {
				err := em.checkForNewMessagesInMailbox(mailbox)
				if err != nil {
					return err
				}
			}
		}
	}
}

// checkForNewMessagesInMailbox looks for new messages in a specific mailbox and processes them
func (em *EmailMonitor) checkForNewMessagesInMailbox(mailbox compiledMailbox) error {
	mailboxName := mailbox.mailbox
	em.logger.Printf("checking for new messages in %s", mailboxName)

	mbox, err := em.client.Select(mailboxName, false)
	if err != nil {
		return err
	}

	// If no messages in mailbox, nothing to do
	if mbox.Messages == 0 {
		em.logger.Printf("no messages in %s", mailboxName)
		return nil
	}

	lastUID := em.lastUIDs[mailboxName]

	// Search for messages with UID greater than lastUID
	criteria := imap.NewSearchCriteria()
	criteria.Uid = new(imap.SeqSet)

	// Only search if we have a valid lastUID
	if lastUID > 0 {
		criteria.Uid.AddRange(lastUID+1, 0)
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
		if uid > lastUID {
			newUIDs = append(newUIDs, uid)
		}
	}

	if len(newUIDs) == 0 {
		em.logger.Printf("no new messages in %s", mailboxName)
		return nil
	}

	em.logger.Printf("found %d new messages in %s (UIDs: %v)", len(newUIDs), mailboxName, newUIDs)

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
		err := em.processMessageInMailbox(msg, mailbox)
		if err != nil {
			em.logger.Printf("error processing message UID %d in %s: %v", msg.Uid, mailboxName, err)
		}

		processedUIDs = append(processedUIDs, msg.Uid)
		if msg.Uid > lastUID {
			em.lastUIDs[mailboxName] = msg.Uid
		}
	}

	if err := <-done; err != nil {
		return err
	}

	em.logger.Printf("processed messages in %s with UIDs: %v, new lastUID: %d", mailboxName, processedUIDs, em.lastUIDs[mailboxName])
	return nil
}

// processMessageInMailbox processes a single email message using the triggers for a specific mailbox
func (em *EmailMonitor) processMessageInMailbox(msg *imap.Message, mailbox compiledMailbox) error {
	if msg == nil || msg.Envelope == nil {
		em.logger.Println("skipping message with nil envelope")
		return nil
	}

	from := "<unknown>"
	if len(msg.Envelope.From) > 0 {
		from = msg.Envelope.From[0].Address()
	}

	em.logger.Printf("processing message from: %s, Subject: %s in mailbox: %s", from, msg.Envelope.Subject, mailbox.mailbox)

	// Get message body
	for _, part := range msg.Body {
		body, err := em.extractTextFromPart(part)
		if err != nil {
			em.logger.Printf("error extracting text: %v", err)
			continue
		}

		if body == "" {
			continue
		}

		// Check if body matches any of the configured triggers for this mailbox
		for i, trigger := range mailbox.triggers {
			if trigger.regex.MatchString(body) {
				em.logger.Printf("regex match found in message from: %s (trigger %d in mailbox %s)", from, i, mailbox.mailbox)

				err := em.executeCommand(msg, body, trigger.command)
				if err != nil {
					return fmt.Errorf("%w: %v", ErrCommandExecution, err)
				}

				// Don't return here - continue checking other triggers
			}
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
func (em *EmailMonitor) executeCommand(msg *imap.Message, body string, command string) error {
	if command == "" {
		em.logger.Println("no command configured")
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

	return em.executor.Execute(command, env, strings.NewReader(body))
}
