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
		DeviceID:       "d290f1ee-6c54-4b01-90e6-d701748f0851",
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

	safe := "d290f1ee6c544b0190e6d701748f0851"
	expectedTopics := []string{
		"homeassistant/sensor/scrutiny/" + safe + "_temperature/config",
		"homeassistant/sensor/scrutiny/" + safe + "_status/config",
		"homeassistant/sensor/scrutiny/" + safe + "_power_on_hours/config",
		"homeassistant/sensor/scrutiny/" + safe + "_power_cycle_count/config",
		"homeassistant/binary_sensor/scrutiny/" + safe + "_problem/config",
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

	safe := "d290f1ee6c544b0190e6d701748f0851"
	require.Equal(t, "Temperature", payload["name"])
	require.Equal(t, "temperature", payload["device_class"])
	require.Equal(t, "\u00b0C", payload["unit_of_measurement"])
	require.Equal(t, "measurement", payload["state_class"])
	require.Equal(t, "{{ value_json.temperature }}", payload["value_template"])
	require.Equal(t, "scrutiny/availability", payload["availability_topic"])
	require.Equal(t, "scrutiny/device/"+safe+"/state", payload["state_topic"])
	require.Equal(t, "scrutiny_"+safe+"_temperature", payload["unique_id"])
	require.Equal(t, "sensor.scrutiny_"+safe+"_temperature", payload["default_entity_id"])
	require.Nil(t, payload["object_id"], "object_id should no longer be present")
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

	safe := "d290f1ee6c544b0190e6d701748f0851"
	require.Equal(t, "binary_sensor.scrutiny_"+safe+"_problem", payload["default_entity_id"])
	require.Nil(t, payload["object_id"], "object_id should no longer be present")
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
	require.Equal(t, "scrutiny_d290f1ee6c544b0190e6d701748f0851", identifiers[0])
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
	// Device with only DeviceID set (minimal data)
	device := &models.Device{
		DeviceID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}
	messages := BuildDiscoveryMessages(device, "homeassistant")
	require.Len(t, messages, 5)

	// Check that the device name falls back to "Drive <DeviceID>"
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[0].Payload), &payload))
	devInfo := payload["device"].(map[string]interface{})
	require.Equal(t, "Drive a1b2c3d4-e5f6-7890-abcd-ef1234567890", devInfo["name"])

	// Serial number and firmware should be absent when empty
	_, hasSerial := devInfo["serial_number"]
	require.False(t, hasSerial, "serial_number should be omitted when empty")
	_, hasFw := devInfo["sw_version"]
	require.False(t, hasFw, "sw_version should be omitted when empty")
}

func TestBuildDiscoveryMessages_DeviceNameOnly(t *testing.T) {
	// Device with DeviceName but no ModelName
	device := &models.Device{
		DeviceID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
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
		DeviceID:   "d290f1ee-6c54-4b01-90e6-d701748f0851",
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
		DeviceID:  "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
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
	// 5 for DeviceID-based topics + 5 for legacy WWN-based topics
	require.Len(t, messages, 10, "should produce 10 removal messages (5 DeviceID + 5 legacy WWN)")
}

func TestBuildRemoveMessages_CountNoWWN(t *testing.T) {
	device := &models.Device{
		DeviceID: "d290f1ee-6c54-4b01-90e6-d701748f0851",
	}
	messages := BuildRemoveMessages(device, "homeassistant")
	require.Len(t, messages, 5, "should produce 5 removal messages when no WWN")
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

	// First 5 removal topics (DeviceID-based) should match discovery topics exactly
	for i, msg := range discovery {
		require.Equal(t, msg.Topic, removal[i].Topic, "removal topic should match discovery topic at index %d", i)
	}
}

func TestBuildRemoveMessages_LegacyWWNTopics(t *testing.T) {
	device := testDevice()
	removal := BuildRemoveMessages(device, "homeassistant")

	// Last 5 messages should be legacy WWN-based topics
	legacySafe := "5000cca264eb01d7"
	expectedLegacyTopics := []string{
		"homeassistant/sensor/scrutiny/" + legacySafe + "_temperature/config",
		"homeassistant/sensor/scrutiny/" + legacySafe + "_status/config",
		"homeassistant/sensor/scrutiny/" + legacySafe + "_power_on_hours/config",
		"homeassistant/sensor/scrutiny/" + legacySafe + "_power_cycle_count/config",
		"homeassistant/binary_sensor/scrutiny/" + legacySafe + "_problem/config",
	}

	for i, expected := range expectedLegacyTopics {
		require.Equal(t, expected, removal[5+i].Topic, "legacy WWN removal topic mismatch at index %d", i)
	}
}

func TestBuildDiscoveryMessages_CustomTopicPrefix(t *testing.T) {
	device := testDevice()
	messages := BuildDiscoveryMessages(device, "custom_prefix")

	for _, msg := range messages {
		require.True(t, strings.HasPrefix(msg.Topic, "custom_prefix/"), "topic should use custom prefix: %s", msg.Topic)
	}
}

func TestSafeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0x5000cca264eb01d7", "5000cca264eb01d7"},
		{"5000cca264eb01d7", "5000cca264eb01d7"},
		{"0xABCDEF", "ABCDEF"},
		{"d290f1ee-6c54-4b01-90e6-d701748f0851", "d290f1ee6c544b0190e6d701748f0851"},
		{"no-prefix-with-dashes", "noprefixwithdashes"},
	}

	for _, tt := range tests {
		result := safeID(tt.input)
		require.Equal(t, tt.expected, result, "safeID(%q)", tt.input)
	}
}

func TestStateTopic(t *testing.T) {
	result := stateTopic("d290f1ee-6c54-4b01-90e6-d701748f0851")
	require.Equal(t, "scrutiny/device/d290f1ee6c544b0190e6d701748f0851/state", result)
}
