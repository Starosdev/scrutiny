# Notifications

As documented in [example.scrutiny.yaml](https://github.com/AnalogJ/scrutiny/blob/master/example.scrutiny.yaml#L59-L75)
there are multiple ways to configure notifications for Scrutiny.

Under the hood we use a library called [Shoutrrr](https://github.com/nicholas-fedor/shoutrrr) to send our notifications, and you should use their documentation if you run into
any issues: https://nicholas-fedor.github.io/shoutrrr/


# Script Notifications

While the Shoutrrr library supports many popular providers for sending notifications Scrutiny also supports a "script" based
notification system, allowing you to execute a custom script whenever a notification needs to be sent. 
Data is provided to this script using the following environmental variables:

```
SCRUTINY_SUBJECT - 	eg. "Scrutiny SMART error (%s) detected on device: %s"
SCRUTINY_DATE 
SCRUTINY_FAILURE_TYPE - EmailTest, SmartFail, ScrutinyFail, MissedPing, Heartbeat
SCRUTINY_DEVICE_NAME - eg. /dev/sda
SCRUTINY_DEVICE_TYPE - ATA/SCSI/NVMe
SCRUTINY_DEVICE_SERIAL - eg. WDDJ324KSO
SCRUTINY_MESSAGE - eg. "Scrutiny SMART error notification for device: %s\nFailure Type: %s\nDevice Name: %s\nDevice Serial: %s\nDevice Type: %s\nDate: %s"
SCRUTINY_HOST_ID - (optional) eg. "my-custom-host-id"
```

# Special Characters

`Shoutrrr` supports special characters in the username and password fields, however you'll need to url-encode the
username and the password separately.

- if your username is: `myname@example.com`
- if your password is `124@34$1`

Then your `shoutrrr` url will look something like:

- `smtp://myname%40example%2Ecom:124%4034%241@ms.my.domain.com:587`

# Telegram Topics/Threads

To send notifications to a specific topic (thread) in a Telegram group:

1. Get your chat ID (negative number for groups, e.g., `-123456789`)
2. Get the topic's thread ID from Telegram (e.g., `12345`)
3. Combine them with a colon: `chat_id:thread_id`

**Example configuration:**

```yaml
notify:
  urls:
    - "telegram://BOT_TOKEN@telegram?channels=-123456789:12345"
```

**Common mistake:** Do NOT use `message_thread_id` as a separate parameter:

```yaml
# WRONG - will fail with "message_thread_id is not a valid config key"
- "telegram://token@telegram?channels=-123456789&message_thread_id=12345"

# CORRECT - append thread ID to chat ID with colon
- "telegram://token@telegram?channels=-123456789:12345"
```

**How to find your thread ID:**

1. Open the Telegram topic in a web browser
2. The URL will be: `https://web.telegram.org/a/#-CHATID_THREADID`
3. Extract the thread ID from the URL

# Testing Notifications

You can test that your notifications are configured correctly by posting an empty payload to the notifications health
check API.

```
curl -X POST http://localhost:8080/api/health/notify
```

# MQTT / Home Assistant

Scrutiny supports native Home Assistant integration via MQTT Discovery. When enabled, drives automatically appear as
devices in Home Assistant with sensors for temperature, health status, power-on hours, power cycle count, and a
problem binary sensor.

For setup instructions, see the [Home Assistant Integration](../README.md#home-assistant-integration-mqtt-discovery) section in the README.

## Common Issues

### Drives not appearing in Home Assistant

1. **Verify MQTT is enabled**: Check the Scrutiny logs at startup for `MQTT Home Assistant integration enabled`. If you see `Failed to connect MQTT`, the broker is unreachable.

2. **Check broker connectivity**: Ensure the Scrutiny web server can reach the MQTT broker. If using Docker, remember that `localhost` inside a container refers to the container itself, not the host. Use the container name (e.g., `tcp://mosquitto:1883`) or the Docker gateway IP (e.g., `tcp://172.17.0.1:1883`).

3. **Verify HA MQTT integration**: Home Assistant must have the MQTT integration configured and connected to the **same broker** that Scrutiny publishes to.

4. **Check discovery prefix**: The `topic_prefix` (default: `homeassistant`) must match the discovery prefix configured in your HA MQTT integration (Settings > Integrations > MQTT > Configure > Discovery prefix).

5. **Inspect MQTT messages**: Use `mosquitto_sub` to verify messages are being published:
   ```bash
   # Check discovery messages
   mosquitto_sub -h YOUR_BROKER -t 'homeassistant/#' -v

   # Check state messages
   mosquitto_sub -h YOUR_BROKER -t 'scrutiny/#' -v
   ```

### Stale or incorrect data in Home Assistant

- **After changing a device label**: Labels are pushed immediately to MQTT when updated via the Scrutiny UI. If HA still shows the old name, check that the discovery message was published (see `mosquitto_sub` above).
- **Archived devices still showing**: Archiving a device removes it from HA by publishing empty retained messages. If the device still appears, manually remove it from the HA MQTT integration.
- **State shows "unavailable"**: This means Scrutiny is offline or the MQTT connection was lost. The LWT (Last Will and Testament) mechanism automatically marks entities as unavailable when Scrutiny disconnects.

### Connection keeps dropping

- **Check `client_id`**: If multiple Scrutiny instances connect to the same broker with the same `client_id`, they will kick each other off. Use unique client IDs (e.g., `scrutiny-server1`, `scrutiny-server2`).
- **Check broker logs**: Most brokers log connection/disconnection events. Look for authentication failures or client ID conflicts.
