package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client provides a common MQTT client interface for the airdancer project
type Client struct {
	client mqtt.Client
}

// Config holds MQTT client configuration
type Config struct {
	ServerURL         string
	ClientID          string
	MaxRetries        int           // Maximum number of connection retries (0 = infinite)
	InitialRetryDelay time.Duration // Initial delay between retries
	MaxRetryDelay     time.Duration // Maximum delay between retries
	OnConnect         func(*Client) // Callback to execute when connected
}

// ButtonEvent represents a button event from the MQTT topic
type ButtonEvent struct {
	ButtonName string `json:"button_name"`
	EventName  string `json:"event_name"`
	Timestamp  string `json:"timestamp"`
}

// NewClient creates a new MQTT client with the given configuration
// The client will attempt to connect asynchronously and retry if the initial connection fails
func NewClient(config Config) (*Client, error) {
	parsedURL, err := url.Parse(config.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT server URL: %w", err)
	}

	if parsedURL.Scheme != "mqtt" {
		return nil, fmt.Errorf("MQTT server URL must use mqtt:// scheme")
	}

	// Set default retry values if not specified
	initialDelay := config.InitialRetryDelay
	if initialDelay == 0 {
		initialDelay = time.Second
	}
	maxDelay := config.MaxRetryDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.ServerURL)
	opts.SetClientID(config.ClientID)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(config.MaxRetryDelay)
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Printf("Connected to MQTT broker at %s", config.ServerURL)

		// Execute the callback if provided
		if config.OnConnect != nil {
			c := &Client{client: client}
			config.OnConnect(c)
		}
	})

	client := mqtt.NewClient(opts)

	// Start async connection with retry logic
	go func() {
		delay := initialDelay
		attempt := 0
		for {
			if token := client.Connect(); token.Wait() && token.Error() != nil {
				attempt++
				if config.MaxRetries > 0 && attempt >= config.MaxRetries {
					log.Printf("Failed to connect to MQTT broker after %d attempts, giving up: %v", attempt, token.Error())
					return
				}

				log.Printf("Failed to connect to MQTT broker (attempt %d): %v. Retrying in %v...", attempt, token.Error(), delay)
				time.Sleep(delay)

				// Exponential backoff
				delay = delay * 2
				if delay > maxDelay {
					delay = maxDelay
				}
				continue
			}
			// Connection successful
			return
		}
	}()

	return &Client{client: client}, nil
}

// Publish publishes a message to the specified topic
func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	if token := c.client.Publish(topic, qos, retained, payload); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish MQTT message: %w", token.Error())
	}

	return nil
}

// PublishButtonEvent publishes a button event to the appropriate MQTT topic
func (c *Client) PublishButtonEvent(buttonName, eventName string) error {
	event := ButtonEvent{
		ButtonName: buttonName,
		EventName:  eventName,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	topic := fmt.Sprintf("event/button/%s/%s", buttonName, eventName)
	return c.Publish(topic, 0, false, eventJSON)
}

// Subscribe subscribes to a topic with the given message handler
func (c *Client) Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	wrappedHandler := func(client mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
	}

	if token := c.client.Subscribe(topic, qos, wrappedHandler); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to MQTT topic %s: %w", topic, token.Error())
	}

	return nil
}

// IsConnected returns true if the client is connected to the MQTT broker
func (c *Client) IsConnected() bool {
	return c.client != nil && c.client.IsConnected()
}

// Disconnect disconnects from the MQTT broker
func (c *Client) Disconnect(quiesce uint) {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(quiesce)
		log.Printf("Disconnected from MQTT broker")
	}
}
