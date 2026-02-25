package mqtt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

func testDevice() *models.Device {
	return &models.Device{
		WWN:            "0x5000cca264eb01d7",
		DeviceName:     "sda",
		Manufacturer:   "Seagate",
		ModelName:      "ST4000DM000",
		SerialNumber:   "Z305B2LN",
		Firmware:       "0001",
		DeviceProtocol: "ATA",
		Capacity:       4000787030016,
		FormFactor:     "3.5 inches",
	}
}

func TestBuildDiscoveryMessages_Count(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")
	require.Len(t, messages, 5, "should produce 5 discovery messages (4 sensors + 1 binary_sensor)")
}

func TestBuildDiscoveryMessages_Topics(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	expectedTopics := []string{
		"homeassistant/sensor/scrutiny/5000cca264eb01d7_temperature/config",
		"homeassistant/sensor/scrutiny/5000cca264eb01d7_status/config",
		"homeassistant/sensor/scrutiny/5000cca264eb01d7_power_on_hours/config",
		"homeassistant/sensor/scrutiny/5000cca264eb01d7_power_cycle_count/config",
		"homeassistant/binary_sensor/scrutiny/5000cca264eb01d7_problem/config",
	}

	for i, msg := range messages {
		require.Equal(t, expectedTopics[i], msg.Topic, "topic mismatch at index %d", i)
	}
}

func TestBuildDiscoveryMessages_TemperatureSensor(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))

	require.Equal(t, "Temperature", payload["name"])
	require.Equal(t, "temperature", payload["device_class"])
	require.Equal(t, "\u00b0C", payload["unit_of_measurement"])
	require.Equal(t, "measurement", payload["state_class"])
	require.Equal(t, "{{ value_json.temperature }}", payload["value_template"])
	require.Equal(t, "scrutiny/availability", payload["availability_topic"])
	require.Equal(t, "scrutiny/device/0x5000cca264eb01d7/state", payload["state_topic"])
	require.Equal(t, "scrutiny_5000cca264eb01d7_temperature", payload["unique_id"])
}

func TestBuildDiscoveryMessages_StatusSensor(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[1].Payload), &payload))

	require.Equal(t, "Health Status", payload["name"])
	require.Nil(t, payload["device_class"], "status sensor should not have device_class")
	require.Equal(t, "{{ value_json.status }}", payload["value_template"])
}

func TestBuildDiscoveryMessages_PowerOnHoursSensor(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[2].Payload), &payload))

	require.Equal(t, "Power On Hours", payload["name"])
	require.Equal(t, "duration", payload["device_class"])
	require.Equal(t, "h", payload["unit_of_measurement"])
	require.Equal(t, "total_increasing", payload["state_class"])
}

func TestBuildDiscoveryMessages_ProblemBinarySensor(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[4].Payload), &payload))

	require.Equal(t, "Drive Problem", payload["name"])
	require.Equal(t, "problem", payload["device_class"])
	require.Equal(t, "ON", payload["payload_on"])
	require.Equal(t, "OFF", payload["payload_off"])
	require.Equal(t, "{{ value_json.problem }}", payload["value_template"])
}

func TestBuildDiscoveryMessages_DeviceInfo(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	// Check device info block on the first message (all share the same block)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))

	devInfo, ok := payload["device"].(map[string]interface{})
	require.True(t, ok)

	require.Equal(t, "ST4000DM000 (sda)", devInfo["name"])
	require.Equal(t, "Seagate", devInfo["manufacturer"])
	require.Equal(t, "ST4000DM000", devInfo["model"])
	require.Equal(t, "Z305B2LN", devInfo["serial_number"])
	require.Equal(t, "0001", devInfo["sw_version"])
	require.Equal(t, "scrutiny", devInfo["via_device"])

	identifiers, ok := devInfo["identifiers"].([]interface{})
	require.True(t, ok)
	require.Len(t, identifiers, 1)
	require.Equal(t, "scrutiny_0x5000cca264eb01d7", identifiers[0])
}

func TestBuildDiscoveryMessages_DeviceInfoConsistency(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "homeassistant")

	// All 5 messages should share the same device block
	var firstDevInfo map[string]interface{}
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	firstDevInfo = payload["device"].(map[string]interface{})

	for i := 1; i < len(messages); i++ {
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(messages[i].Payload), &p))
		devInfo := p["device"].(map[string]interface{})

		firstJSON, _ := json.Marshal(firstDevInfo)
		thisJSON, _ := json.Marshal(devInfo)
		require.JSONEq(t, string(firstJSON), string(thisJSON), "device info mismatch on message %d", i)
	}
}

func TestBuildDiscoveryMessages_MinimalDevice(t *testing.T) {
	// Device with only WWN set (minimal data)
	device := &models.Device{
		WWN: "0x5002538e40a22954",
	}
	messages := BuildDiscoveryMessages(device, "homeassistant")
	require.Len(t, messages, 5)

	// Check that the device name falls back to "Drive <WWN>"
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	devInfo := payload["device"].(map[string]interface{})
	require.Equal(t, "Drive 0x5002538e40a22954", devInfo["name"])

	// Serial number and firmware should be absent when empty
	_, hasSerial := devInfo["serial_number"]
	require.False(t, hasSerial, "serial_number should be omitted when empty")
	_, hasFw := devInfo["sw_version"]
	require.False(t, hasFw, "sw_version should be omitted when empty")
}

func TestBuildDiscoveryMessages_DeviceNameOnly(t *testing.T) {
	// Device with DeviceName but no ModelName
	device := &models.Device{
		WWN:        "0x5002538e40a22954",
		DeviceName: "sda",
	}
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	devInfo := payload["device"].(map[string]interface{})
	require.Equal(t, "sda", devInfo["name"], "should use DeviceName when ModelName is empty")
}

func TestBuildDiscoveryMessages_CustomLabel(t *testing.T) {
	// Device with a user-set Label should use Label as the HA device name
	device := &models.Device{
		WWN:        "0x5000cca264eb01d7",
		DeviceName: "sda",
		ModelName:  "ST4000DM000",
		Label:      "Parity Drive",
	}
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	devInfo := payload["device"].(map[string]interface{})
	require.Equal(t, "Parity Drive", devInfo["name"], "should use Label when set")
}

func TestBuildDiscoveryMessages_ModelNameOnly(t *testing.T) {
	// Device with ModelName but no DeviceName
	device := &models.Device{
		WWN:       "0x5002538e40a22954",
		ModelName: "ST4000DM000",
	}
	messages := BuildDiscoveryMessages(device, "homeassistant")

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	devInfo := payload["device"].(map[string]interface{})
	require.Equal(t, "ST4000DM000", devInfo["name"], "should use ModelName when DeviceName is empty")
}

func TestBuildRemoveMessages_Count(t *testing.T) {
	device := testDevice()
	messages := BuildRemoveMessages(device, "homeassistant")
	require.Len(t, messages, 5, "should produce 5 removal messages")
}

func TestBuildRemoveMessages_EmptyPayloads(t *testing.T) {
	device := testDevice()
	messages := BuildRemoveMessages(device, "homeassistant")

	for _, msg := range messages {
		require.Empty(t, msg.Payload, "removal messages should have empty payloads")
	}
}

func TestBuildRemoveMessages_TopicsMatch(t *testing.T) {
	device := testDevice()
	discovery := BuildDiscoveryMessages(device, "homeassistant")
	removal := BuildRemoveMessages(device, "homeassistant")

	// Removal topics should match discovery topics exactly
	for i, msg := range removal {
		require.Equal(t, discovery[i].Topic, msg.Topic, "removal topic should match discovery topic at index %d", i)
	}
}

func TestBuildDiscoveryMessages_CustomTopicPrefix(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "custom_prefix")

	for _, msg := range messages {
		require.True(t, strings.HasPrefix(msg.Topic, "custom_prefix/"), "topic should use custom prefix: %s", msg.Topic)
	}
}

func TestSafeWWN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0x5000cca264eb01d7", "5000cca264eb01d7"},
		{"5000cca264eb01d7", "5000cca264eb01d7"},
		{"0xABCDEF", "ABCDEF"},
	}

	for _, tt := range tests {
		result := safeWWN(tt.input)
		require.Equal(t, tt.expected, result, "safeWWN(%q)", tt.input)
	}
}

func TestStateTopic(t *testing.T) {
	result := stateTopic("0x5000cca264eb01d7")
	require.Equal(t, "scrutiny/device/0x5000cca264eb01d7/state", result)
}
