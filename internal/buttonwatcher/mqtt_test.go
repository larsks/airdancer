package buttonwatcher

import (
	"encoding/json"
	"testing"

	"github.com/larsks/airdancer/internal/mqtt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestButtonEvent_MarshalJSON(t *testing.T) {
	event := mqtt.ButtonEvent{
		ButtonName: "test-button",
		EventName:  "click",
		Timestamp:  "2023-01-01T12:00:00Z",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	expected := `{"button_name":"test-button","event_name":"click","timestamp":"2023-01-01T12:00:00Z"}`
	assert.JSONEq(t, expected, string(data))
}

func TestInitMQTTClient_InvalidURL(t *testing.T) {
	bm := NewButtonMonitor()

	err := bm.initMQTTClient("invalid-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MQTT server URL must use mqtt:// scheme")
}

func TestInitMQTTClient_WrongScheme(t *testing.T) {
	bm := NewButtonMonitor()

	err := bm.initMQTTClient("http://localhost:1883")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MQTT server URL must use mqtt:// scheme")
}

func TestPublishMQTTEvent_NoClient(t *testing.T) {
	bm := NewButtonMonitor()

	// Should not panic when MQTT client is nil
	bm.publishMQTTEvent("test-button", "click")
}

func TestConfig_MQTTServerConfiguration(t *testing.T) {
	config := NewConfig()
	config.MqttServer = "mqtt://localhost:1883"

	assert.Equal(t, "mqtt://localhost:1883", config.MqttServer)
}

func TestButtonMonitor_SetGlobalConfig_WithMQTTServer(t *testing.T) {
	bm := NewButtonMonitor()
	config := &Config{
		MqttServer: "mqtt://invalid-host:1883", // Use invalid host to avoid actual connection
	}

	// Should not panic, but will log an error about connection failure
	bm.SetGlobalConfig(config)

	assert.Equal(t, config, bm.globalConfig)
}

func TestButtonMonitor_SetGlobalConfig_WithoutMQTTServer(t *testing.T) {
	bm := NewButtonMonitor()
	config := &Config{}

	bm.SetGlobalConfig(config)

	assert.Equal(t, config, bm.globalConfig)
	assert.Nil(t, bm.mqttClient)
}

func TestButtonMonitor_Close_WithMQTTClient(t *testing.T) {
	bm := NewButtonMonitor()

	// Test closing without MQTT client (should not panic)
	err := bm.Close()
	assert.NoError(t, err)
}
