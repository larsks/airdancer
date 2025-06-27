package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
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

type IMAPConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseSSL   bool   `json:"use_ssl"`
	Mailbox  string `json:"mailbox"`
}

type MonitorConfig struct {
	RegexPattern  string `json:"regex_pattern"`
	Command       string `json:"command"`
	CheckInterval int    `json:"check_interval_seconds"`
}

type Config struct {
	IMAP    IMAPConfig    `json:"imap"`
	Monitor MonitorConfig `json:"monitor"`
}

type EmailMonitor struct {
	config      Config
	client      *client.Client
	regex       *regexp.Regexp
	lastUID     uint32
	reconnectCh chan bool
}

func main() {
	configFile := flag.String("config", "", "Path to the configuration file")

	// IMAP flags
	imapServer := flag.String("imap.server", "", "IMAP server address")
	imapPort := flag.Int("imap.port", 993, "IMAP server port")
	imapUsername := flag.String("imap.username", "", "IMAP username")
	imapPassword := flag.String("imap.password", "", "IMAP password")
	imapUseSSL := flag.Bool("imap.use_ssl", true, "Use SSL for IMAP connection")
	imapMailbox := flag.String("imap.mailbox", "", "IMAP mailbox to monitor")

	// Monitor flags
	monitorRegexPattern := flag.String("monitor.regex_pattern", "", "Regex pattern to match in email bodies")
	monitorCommand := flag.String("monitor.command", "", "Command to execute on regex match")
	monitorCheckInterval := flag.Int("monitor.check_interval_seconds", 0, "Interval in seconds to check for new emails")

	flag.Parse()

	config, err := loadConfig(*configFile)
	if err != nil {
		// If the config file doesn't exist and it wasn't specified, just use the defaults.
		if !os.IsNotExist(err) || *configFile != "" {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Override config with flags if they are set
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "imap.server":
			config.IMAP.Server = *imapServer
		case "imap.port":
			config.IMAP.Port = *imapPort
		case "imap.username":
			config.IMAP.Username = *imapUsername
		case "imap.password":
			config.IMAP.Password = *imapPassword
		case "imap.use_ssl":
			config.IMAP.UseSSL = *imapUseSSL
		case "imap.mailbox":
			config.IMAP.Mailbox = *imapMailbox
		case "monitor.regex_pattern":
			config.Monitor.RegexPattern = *monitorRegexPattern
		case "monitor.command":
			config.Monitor.Command = *monitorCommand
		case "monitor.check_interval_seconds":
			config.Monitor.CheckInterval = *monitorCheckInterval
		}
	})

	if config.IMAP.Server == "" {
		log.Fatalf("IMAP server must be set")
	}

	monitor, err := NewEmailMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	log.Println("Starting email monitor...")
	monitor.Start()
}

func loadConfig(filename string) (Config, error) {
	var config Config

	if filename == "" {
		return config, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(data, &config)
	return config, err
}

func NewEmailMonitor(config Config) (*EmailMonitor, error) {
	regex, err := regexp.Compile(config.Monitor.RegexPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	monitor := &EmailMonitor{
		config:      config,
		regex:       regex,
		reconnectCh: make(chan bool, 1),
	}

	return monitor, nil
}

func (em *EmailMonitor) Start() {
	for {
		err := em.connect()
		if err != nil {
			log.Printf("Connection failed: %v. Retrying in 30 seconds...", err)
			time.Sleep(30 * time.Second)
			continue
		}

		log.Println("Connected to IMAP server")

		// Get initial state
		err = em.initializeLastUID()
		if err != nil {
			log.Printf("Failed to initialize: %v", err)
			em.disconnect()
			continue
		}

		// Start monitoring
		err = em.monitor()
		if err != nil {
			log.Printf("Monitor error: %v. Reconnecting...", err)
		}

		em.disconnect()
		time.Sleep(5 * time.Second)
	}
}

func (em *EmailMonitor) connect() error {
	var c *client.Client
	var err error

	address := fmt.Sprintf("%s:%d", em.config.IMAP.Server, em.config.IMAP.Port)

	if em.config.IMAP.UseSSL {
		c, err = client.DialTLS(address, &tls.Config{})
	} else {
		c, err = client.Dial(address)
	}

	if err != nil {
		return err
	}

	if err := c.Login(em.config.IMAP.Username, em.config.IMAP.Password); err != nil {
		c.Close()
		return err
	}

	em.client = c
	return nil
}

func (em *EmailMonitor) disconnect() {
	if em.client != nil {
		em.client.Close()
		em.client = nil
	}
}

func (em *EmailMonitor) initializeLastUID() error {
	mailbox := em.config.IMAP.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	mbox, err := em.client.Select(mailbox, false)
	if err != nil {
		return err
	}

	// Get the highest UID in the mailbox
	if mbox.Messages == 0 {
		em.lastUID = 0
		log.Printf("Mailbox is empty, starting with UID: 0")
		return nil
	}

	// Search for all messages to get the highest UID
	seqset := new(imap.SeqSet)
	seqset.AddRange(1, mbox.Messages)

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- em.client.Fetch(seqset, []imap.FetchItem{imap.FetchUid}, messages)
	}()

	var highestUID uint32
	for msg := range messages {
		if msg.Uid > highestUID {
			highestUID = msg.Uid
		}
	}

	if err := <-done; err != nil {
		return err
	}

	em.lastUID = highestUID
	log.Printf("Initialized with last UID: %d (UidNext: %d)", em.lastUID, mbox.UidNext)
	return nil
}

func (em *EmailMonitor) monitor() error {
	mailbox := em.config.IMAP.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

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

func (em *EmailMonitor) checkForNewMessages() error {
	mailbox := em.config.IMAP.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	mbox, err := em.client.Select(mailbox, false)
	if err != nil {
		return err
	}

	// If no messages in mailbox, nothing to do
	if mbox.Messages == 0 {
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
		return nil
	}

	log.Printf("Found %d new messages (UIDs: %v)", len(newUIDs), newUIDs)

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
			log.Printf("Error processing message UID %d: %v", msg.Uid, err)
		}

		processedUIDs = append(processedUIDs, msg.Uid)
		if msg.Uid > em.lastUID {
			em.lastUID = msg.Uid
		}
	}

	if err := <-done; err != nil {
		return err
	}

	log.Printf("Processed messages with UIDs: %v, new lastUID: %d", processedUIDs, em.lastUID)
	return nil
}

func (em *EmailMonitor) processMessage(msg *imap.Message) error {
	if msg == nil || msg.Envelope == nil {
		log.Println("Skipping message with nil envelope")
		return nil
	}

	from := "<unknown>"
	if len(msg.Envelope.From) > 0 {
		from = msg.Envelope.From[0].Address()
	}

	log.Printf("Processing message from: %s, Subject: %s", from, msg.Envelope.Subject)

	// Get message body
	for _, part := range msg.Body {
		body, err := em.extractTextFromPart(part)
		if err != nil {
			log.Printf("Error extracting text: %v", err)
			continue
		}

		if body == "" {
			continue
		}

		// Check if body matches regex
		if em.regex.MatchString(body) {
			log.Printf("Regex match found in message from: %s", from)

			err := em.executeCommand(msg, body)
			if err != nil {
				log.Printf("Error executing command: %v", err)
			}

			return nil
		}
	}

	return nil
}

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

func (em *EmailMonitor) executeCommand(msg *imap.Message, body string) error {
	if em.config.Monitor.Command == "" {
		log.Println("No command configured")
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
		log.Printf("Command execution failed: %v, Output: %s", err, string(output))
		return err
	}

	log.Printf("Command executed successfully. Output: %s", string(output))
	return nil
}
