package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

const entityIDFormat = "scrutiny_%s_%s"

// DiscoveryMessage represents an MQTT discovery message to publish.
type DiscoveryMessage struct {
	Topic   string
	Payload string // JSON string (empty string = remove from HA)
}

// deviceInfo builds the HA device grouping block shared by all entities for a drive.
func deviceInfo(device *models.Device) map[string]interface{} {
	name := device.Label
	if name == "" {
		if device.ModelName != "" && device.DeviceName != "" {
			name = fmt.Sprintf("%s (%s)", device.ModelName, device.DeviceName)
		} else if device.ModelName != "" {
			name = device.ModelName
		} else if device.DeviceName != "" {
			name = device.DeviceName
		} else {
			name = fmt.Sprintf("Drive %s", device.WWN)
		}
	}

	info := map[string]interface{}{
		"identifiers":  []string{fmt.Sprintf("scrutiny_%s", device.WWN)},
		"name":         name,
		"manufacturer": device.Manufacturer,
		"model":        device.ModelName,
		"via_device":   "scrutiny",
	}
	if device.SerialNumber != "" {
		info["serial_number"] = device.SerialNumber
	}
	if device.Firmware != "" {
		info["sw_version"] = device.Firmware
	}
	return info
}

// safeWWN strips the 0x prefix from a WWN for use in MQTT topics.
func safeWWN(wwn string) string {
	return strings.TrimPrefix(wwn, "0x")
}

// stateTopic returns the MQTT topic for a device's state updates.
func stateTopic(wwn string) string {
	return fmt.Sprintf("scrutiny/device/%s/state", wwn)
}

// BuildDiscoveryMessages generates HA MQTT Discovery config messages for all entities of a device.
func BuildDiscoveryMessages(device *models.Device, topicPrefix string) []DiscoveryMessage {
	safe := safeWWN(device.WWN)
	devInfo := deviceInfo(device)
	st := stateTopic(device.WWN)

	messages := []DiscoveryMessage{
		buildSensorDiscovery(topicPrefix, safe, "temperature", devInfo, st, &sensorConfig{
			Name:          "Temperature",
			DeviceClass:   "temperature",
			UnitOfMeasure: "\u00b0C",
			StateClass:    "measurement",
			ValueTemplate: "{{ value_json.temperature }}",
			Icon:          "mdi:thermometer",
		}),
		buildSensorDiscovery(topicPrefix, safe, "status", devInfo, st, &sensorConfig{
			Name:          "Health Status",
			ValueTemplate: "{{ value_json.status }}",
			Icon:          "mdi:harddisk",
		}),
		buildSensorDiscovery(topicPrefix, safe, "power_on_hours", devInfo, st, &sensorConfig{
			Name:          "Power On Hours",
			DeviceClass:   "duration",
			UnitOfMeasure: "h",
			StateClass:    "total_increasing",
			ValueTemplate: "{{ value_json.power_on_hours }}",
			Icon:          "mdi:clock-outline",
		}),
		buildSensorDiscovery(topicPrefix, safe, "power_cycle_count", devInfo, st, &sensorConfig{
			Name:          "Power Cycle Count",
			StateClass:    "total_increasing",
			ValueTemplate: "{{ value_json.power_cycle_count }}",
			Icon:          "mdi:restart",
		}),
		buildBinarySensorDiscovery(topicPrefix, safe, "problem", devInfo, st),
	}

	return messages
}

// BuildRemoveMessages generates empty-payload messages to remove a device from HA discovery.
func BuildRemoveMessages(device *models.Device, topicPrefix string) []DiscoveryMessage {
	safe := safeWWN(device.WWN)

	topics := []string{
		fmt.Sprintf("%s/sensor/scrutiny/%s_temperature/config", topicPrefix, safe),
		fmt.Sprintf("%s/sensor/scrutiny/%s_status/config", topicPrefix, safe),
		fmt.Sprintf("%s/sensor/scrutiny/%s_power_on_hours/config", topicPrefix, safe),
		fmt.Sprintf("%s/sensor/scrutiny/%s_power_cycle_count/config", topicPrefix, safe),
		fmt.Sprintf("%s/binary_sensor/scrutiny/%s_problem/config", topicPrefix, safe),
	}

	messages := make([]DiscoveryMessage, len(topics))
	for i, topic := range topics {
		messages[i] = DiscoveryMessage{Topic: topic, Payload: ""}
	}
	return messages
}

type sensorConfig struct {
	Name           string
	DeviceClass    string
	UnitOfMeasure  string
	StateClass     string
	ValueTemplate  string
	Icon           string
	EntityCategory string
}

func buildSensorDiscovery(topicPrefix, safeWwn, entityID string, devInfo map[string]interface{}, st string, cfg *sensorConfig) DiscoveryMessage {
	id := fmt.Sprintf(entityIDFormat, safeWwn, entityID)

	payload := map[string]interface{}{
		"name":               cfg.Name,
		"unique_id":          id,
		"object_id":          id,
		"state_topic":        st,
		"value_template":     cfg.ValueTemplate,
		"availability_topic": availabilityTopic,
		"device":             devInfo,
	}

	if cfg.DeviceClass != "" {
		payload["device_class"] = cfg.DeviceClass
	}
	if cfg.UnitOfMeasure != "" {
		payload["unit_of_measurement"] = cfg.UnitOfMeasure
	}
	if cfg.StateClass != "" {
		payload["state_class"] = cfg.StateClass
	}
	if cfg.Icon != "" {
		payload["icon"] = cfg.Icon
	}
	if cfg.EntityCategory != "" {
		payload["entity_category"] = cfg.EntityCategory
	}

	topic := fmt.Sprintf("%s/sensor/scrutiny/%s_%s/config", topicPrefix, safeWwn, entityID)
	payloadJSON, _ := json.Marshal(payload)

	return DiscoveryMessage{
		Topic:   topic,
		Payload: string(payloadJSON),
	}
}

func buildBinarySensorDiscovery(topicPrefix, safeWwn, entityID string, devInfo map[string]interface{}, st string) DiscoveryMessage {
	id := fmt.Sprintf(entityIDFormat, safeWwn, entityID)

	payload := map[string]interface{}{
		"name":               "Drive Problem",
		"unique_id":          id,
		"object_id":          id,
		"state_topic":        st,
		"value_template":     "{{ value_json.problem }}",
		"payload_on":         "ON",
		"payload_off":        "OFF",
		"device_class":       "problem",
		"availability_topic": availabilityTopic,
		"device":             devInfo,
		"icon":               "mdi:alert-circle",
	}

	topic := fmt.Sprintf("%s/binary_sensor/scrutiny/%s_%s/config", topicPrefix, safeWwn, entityID)
	payloadJSON, _ := json.Marshal(payload)

	return DiscoveryMessage{
		Topic:   topic,
		Payload: string(payloadJSON),
	}
}
