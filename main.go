package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

type Config struct {
	IMAP struct {
		Server   string `json:"server"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		UseSSL   bool   `json:"use_ssl"`
		Mailbox  string `json:"mailbox"`
	} `json:"imap"`
	Monitor struct {
		RegexPattern  string `json:"regex_pattern"`
		Command       string `json:"command"`
		CheckInterval int    `json:"check_interval_seconds"`
	} `json:"monitor"`
}

type EmailMonitor struct {
	config      Config
	client      *imapclient.Client
	regex       *regexp.Regexp
	lastUID     imap.UID
	reconnectCh chan bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config.json>")
		os.Exit(1)
	}

	configFile := os.Args[1]
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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
		em.startMonitoring()

		err = em.monitor()
		if err != nil {
			log.Printf("Monitor error: %v. Reconnecting...", err)
		}

		em.disconnect()
		time.Sleep(5 * time.Second)
	}
}

func (em *EmailMonitor) connect() error {
	var c *imapclient.Client
	var err error

	address := fmt.Sprintf("%s:%d", em.config.IMAP.Server, em.config.IMAP.Port)
	options := &imapclient.Options{}

	if em.config.IMAP.UseSSL {
		options.TLSConfig = &tls.Config{}
		c, err = imapclient.DialTLS(address, options)
	} else {
		c, err = imapclient.DialStartTLS(address, options)
	}

	if err != nil {
		return err
	}

	if err := c.Login(em.config.IMAP.Username, em.config.IMAP.Password).Wait(); err != nil {
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

	selectData, err := em.client.Select(mailbox, nil).Wait()
	if err != nil {
		return err
	}

	// If mailbox is empty, start with UID 0
	if selectData.NumMessages == 0 {
		em.lastUID = 0
		log.Printf("Mailbox is empty, starting with UID: 0")
		return nil
	}

	// Use SEARCH to find all messages
	searchCmd := em.client.Search(&imap.SearchCriteria{}, nil)
	searchData, err := searchCmd.Wait()
	if err != nil {
		return err
	}

	// Get all sequence numbers from search results
	seqNums := searchData.SeqNums
	if len(seqNums) == 0 {
		em.lastUID = 0
		log.Printf("No messages found in search, starting with UID: 0")
		return nil
	}

	// Get the UID of the last message by fetching just the last sequence number
	lastSeqNum := seqNums[len(seqNums)-1]
	seqSet := imap.SeqSet{}
	seqSet.AddNum(lastSeqNum)

	fetchCmd := em.client.Fetch(seqSet, &imap.FetchOptions{
		UID: true,
	})

	var highestUID imap.UID
	err = fetchCmd.ForEach(func(msg *imapclient.FetchMessageData) error {
		if msg.UID > highestUID {
			highestUID = msg.UID
		}
		return nil
	})

	if err != nil {
		return err
	}

	em.lastUID = highestUID
	log.Printf("Initialized with last UID: %d (UidNext: %d)", em.lastUID, selectData.UIDNext)
	return nil
}

func (em *EmailMonitor) startMonitoring() {
	log.Printf("Starting to monitor for new messages (last processed UID: %d)", em.lastUID)
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

	selectData, err := em.client.Select(mailbox, nil).Wait()
	if err != nil {
		return err
	}

	// If no messages in mailbox, nothing to do
	if selectData.NumMessages == 0 {
		return nil
	}

	// Search for messages with UID greater than lastUID
	criteria := &imap.SearchCriteria{}
	uidSet := imap.UIDSet{}

	if em.lastUID > 0 {
		uidSet.AddRange(em.lastUID+1, 0) // From lastUID+1 to end
	} else {
		uidSet.AddRange(1, 0) // All messages
	}
	criteria.UID = []imap.UIDSet{uidSet}

	searchCmd := em.client.UIDSearch(criteria, nil)
	searchData, err := searchCmd.Wait()
	if err != nil {
		return err
	}

	// Get all UIDs from search results
	allUIDs := searchData.UIDs
	if len(allUIDs) == 0 {
		return nil
	}

	// Filter out UIDs that we've already seen (additional safety check)
	var newUIDs []imap.UID
	for _, uid := range allUIDs {
		if uid > em.lastUID {
			newUIDs = append(newUIDs, uid)
		}
	}

	if len(newUIDs) == 0 {
		return nil
	}

	log.Printf("Found %d new messages (UIDs: %v)", len(newUIDs), newUIDs)

	// Fetch new messages using UID FETCH
	uidSet = imap.UIDSet{}
	for _, uid := range newUIDs {
		uidSet.AddNum(uid)
	}

	fetchCmd := em.client.Fetch(uidSet, &imap.FetchOptions{
		Envelope:      true,
		BodyStructure: &imap.FetchItemBodyStructure{},
		BodySection:   []*imap.FetchItemBodySection{{}}, // Fetch full body
		UID:           true,
	})

	var processedUIDs []imap.UID
	err = fetchCmd.ForEach(func(msg *imapclient.FetchMessageData) error {
		err := em.processMessage(msg)
		if err != nil {
			log.Printf("Error processing message UID %d: %v", msg.UID, err)
		}

		processedUIDs = append(processedUIDs, msg.UID)
		if msg.UID > em.lastUID {
			em.lastUID = msg.UID
		}
		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("Processed messages with UIDs: %v, new lastUID: %d", processedUIDs, em.lastUID)
	return nil
}

func (em *EmailMonitor) processMessage(msg *imapclient.FetchMessageData) error {
	var fromAddr string
	if len(msg.Envelope.From) > 0 {
		fromAddr = msg.Envelope.From[0].Address()
	}

	log.Printf("Processing message from: %s, Subject: %s", fromAddr, msg.Envelope.Subject)

	// Get message body
	for _, bodyData := range msg.BodySection {
		body, err := em.extractTextFromBody(bodyData)
		if err != nil {
			log.Printf("Error extracting text: %v", err)
			continue
		}

		if body == "" {
			continue
		}

		// Check if body matches regex
		if em.regex.MatchString(body) {
			log.Printf("Regex match found in message from: %s", fromAddr)

			err := em.executeCommand(msg, body)
			if err != nil {
				log.Printf("Error executing command: %v", err)
			}

			return nil
		}
	}

	return nil
}

func (em *EmailMonitor) extractTextFromBody(bodyData []byte) (string, error) {
	mr, err := mail.CreateReader(strings.NewReader(string(bodyData)))
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

func (em *EmailMonitor) executeCommand(msg *imapclient.FetchMessageData, body string) error {
	if em.config.Monitor.Command == "" {
		log.Println("No command configured")
		return nil
	}

	var fromAddr string
	if len(msg.Envelope.From) > 0 {
		fromAddr = msg.Envelope.From[0].Address()
	}

	// Set environment variables with message info
	env := os.Environ()
	env = append(env, fmt.Sprintf("EMAIL_FROM=%s", fromAddr))
	env = append(env, fmt.Sprintf("EMAIL_SUBJECT=%s", msg.Envelope.Subject))
	env = append(env, fmt.Sprintf("EMAIL_DATE=%s", msg.Envelope.Date.Format(time.RFC3339)))
	env = append(env, fmt.Sprintf("EMAIL_UID=%d", msg.UID))

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
