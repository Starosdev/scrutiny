# Notifications

As documented in [example.scrutiny.yaml](https://github.com/Staros-Labs/scrutiny/blob/master/example.scrutiny.yaml#L59-L75)
there are multiple ways to configure notifications for Scrutiny.

Scrutiny supports multiple notification engines:

- [Shoutrrr](https://github.com/nicholas-fedor/shoutrrr) for the existing `discord://`, `smtp://`, `telegram://`, and similar URL formats
- [Apprise](https://github.com/caronc/apprise) for explicit `apprise+...` targets such as `apprise+mailto://...`
- `script://` for local notification scripts
- raw `http://` or `https://` webhook posts

Scrutiny preserves the existing Shoutrrr URL contract. Apprise is additive, not a replacement, so only URLs prefixed with `apprise+` are routed through the Apprise CLI.

The official Scrutiny web and omnibus container images include the Apprise runtime required for `apprise+...` targets.

If you are troubleshooting a Shoutrrr target, use their documentation: https://nicholas-fedor.github.io/shoutrrr/
If you are troubleshooting an Apprise target, use the Apprise documentation: https://appriseit.com/


# Quiet Hours

Queued alerts are sent as a digest after quiet hours end. The background check runs at **Missed Ping Check Interval** (default: 5 minutes), even when missed-ping alerts are disabled. Failed or rate-limited digests stay queued for the next check. The queue is held in memory and is lost on server restart. Partial delivery failures can repeat a digest to targets that already succeeded.

Quiet-hours digests combine alert categories and retain the legacy failure type `MissedPing` in scripts and webhooks, including digests containing temperature alerts. The subject and message identify the queued alerts.

# Drive Temperature Notifications

Enable this feature in **Display & Notifications**. The global defaults are 55°C for 30 minutes; equality counts as hot. A duration of 0 alerts on the first hot upload. Configure targets as for other notifications. Direct deliveries to scripts and webhooks receive failure type `Temperature`; quiet-hours digests use `MissedPing` as described above.

- Evaluation happens only on successful collector uploads. A collector interval longer than the configured duration delays the alert until the next upload. Fatal smartctl uploads never evaluate temperature.
- Readings of 0 or lower are unknown: they neither start nor reset a streak and do not trigger an alert. A valid reading below the threshold re-arms the device.
- After restarting the server, seeding uses up to 24 hours of raw history tagged with the exact device ID and requires **Store Temperature History** to be enabled and the current upload to appear in the query. History from another device sharing a WWN cannot seed an alert. Empty, stale, or failed queries fall back to timing from the current upload. Keep collector clocks synchronized with the server.
- Missing readings do not prove continuous heat; elapsed time spans gaps between valid observations. Durations longer than 24 hours may need additional time after a restart. Notification state is held in memory, so an ongoing excursion can notify once again after each restart.
- An upload while muted/disabled resets that device's timer. Changes to the threshold or duration reset it on the next evaluated upload. Resuming starts a fresh timer without reusing old history.
- Quiet hours queue one alert for the digest described above. Rate-limited or failed direct dispatches retry on the next hot upload. As with other notifications, partial delivery failures can repeat a message to targets that already succeeded.
- If no targets are configured, alerts remain eligible for retry so adding a target can deliver an ongoing excursion. The notification gate logs the missing-target warning once across devices and digest retries, until a delivery succeeds or the server restarts. Target settings are still checked on each retry.
- **Repeat Notifications** does not apply to temperature alerts. A sustained hot excursion sends one alert; cooling below the threshold re-arms silently, without a recovery notification or hysteresis margin.
- History seeding queries time out after 10 seconds. Debug logging explains when seeding is skipped because the upload timestamp is missing/ahead of the server or the current point is not yet visible in InfluxDB. The timer then starts with the current upload.

To test restart seeding, use a nonzero duration and a hot history older than that duration, then restart the web app and upload another hot reading. Using duration 0 only verifies immediate delivery, not seeding. These settings live under `metrics` in the Settings API: `notify_on_temperature`, `temperature_threshold_celsius` (1–150), and `temperature_duration_minutes` (whole minutes, 0–153722867).

Omitting the threshold or duration in a Settings API request applies 55°C or 30 minutes respectively. An explicit duration of 0 means immediate delivery. The API rejects out-of-range values, including a threshold of 0, even when alerts are disabled. The UI preserves input while typing and rounds to whole Celsius on blur or save. When saving with alerts disabled, invalid hidden temperature values use the defaults; valid values are preserved.

## Upgrade note: temperature history storage

Device-ID tagging first shipped in v1.39.0. Older history and history affected by the AnalogJ identity repair may lack the current device-ID tag and cannot seed temperature notifications. The repair does not add or correct that tag. With new tagged uploads, affected old points age out of the 24-hour seeding window. There is no 24-hour wait for alerts: fresh hot readings start the normal duration timer immediately.

This identity fix applies to notification seeding. Dashboard temperature charts (`GET /api/summary/temp`) still group history by WWN and can mix devices with shared WWNs; correcting raw and downsampled chart history is separate follow-up work.

The temperature notification release also repairs the legacy database key `store_temperature_history`, migrating it to `collector.store_temperature_history`. Affected installations could show history storage enabled while uploads did not store history. The migration preserves the existing preference (and prefers the canonical key if both exist), so enabled storage resumes on subsequent uploads. Existing history is retained; missing past readings cannot be reconstructed.

# SMTP Notifications

The SMTP `timeout` URL parameter must be positive and defaults to `10s`. It now covers connection establishment and the entire SMTP conversation, including TLS, all recipients, and message submission. Slow but working servers may exhaust this shared budget; increase it (for example, `timeout=30s`) when needed. Zero and negative values are rejected before dialing. Timeout failures remain eligible for the normal notification retry behavior.

# Webhook Notifications

Raw HTTP/HTTPS webhook requests time out after 10 seconds. Only HTTP 2xx responses count as successful delivery; other statuses and timeouts are failures. Direct temperature alerts retry failed dispatches on the next hot upload.

# Script Notifications

While the Shoutrrr library supports many popular providers for sending notifications Scrutiny also supports a "script" based
notification system, allowing you to execute a custom script whenever a notification needs to be sent. 
Scripts have a 30-second execution timeout and a further 1-second allowance for closing inherited output pipes. A timeout is a delivery failure; direct temperature alerts retry on the next hot upload. The direct process is terminated on timeout, but this does not guarantee termination of every descendant. Both stdout and stderr continue streaming to raw process stdout with the existing prefix and stream labels, independently of the application logger.

Data is provided to this script using the following environmental variables:

```
SCRUTINY_SUBJECT - 	eg. "Scrutiny SMART error (%s) detected on device: %s"
SCRUTINY_DATE 
SCRUTINY_FAILURE_TYPE - EmailTest, SmartFail, ScrutinyFail, MissedPing, Heartbeat, Temperature
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

# Apprise Targets

Apprise targets must be explicit and prefixed with `apprise+` so Scrutiny can route them through the Apprise CLI without changing the existing `notify.urls` contract.

Apprise has a 30-second execution timeout and a further 1-second allowance for closing inherited pipes. Its output remains buffered for failure diagnostics.

Notification targets execute in parallel, so delivery waits for the slowest complete target operation. These transport budgets are not upload deadlines: history seeding, database work, and waiting behind another upload can add time.

For the audited Shoutrrr v0.17.0 Matrix password-login path, sender construction can make two sequential HTTP requests with 10-second deadlines, followed by the router's 10-second send wait: approximately 30 seconds of transport execution. This is not a total device-lock or upload deadline.

Examples:

- `apprise+mailto://example.com?user=alerts@example.com&pass=app-password&to=admin@example.com`
- `apprise+gotify://gotify-host/token`
- `apprise+https://discord.com/api/webhooks/123/token`
- `apprise+https://hooks.slack.com/services/T000/B000/XXXX`
- `apprise+tgram://123456789:ABCDEF/123456789/`

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

This test route exercises the same notification pipeline used by Scrutiny events, including Shoutrrr targets, explicit `apprise+...` targets, scripts, and raw webhooks.

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

# Collector-Side Error Notifications

Scrutiny notifies you when the collector fails to read SMART data from a drive via `smartctl`, or when a device scan fails entirely. This is distinct from SMART attribute threshold failures — it alerts you when the collection process itself errors.

**Per-device errors** occur when `smartctl` successfully scans and finds a device but fails to retrieve its SMART data. These are reported to `POST /api/device/:wwn/collector-error` and include device context in the notification.

**Scan-level errors** occur when `smartctl --scan` itself fails (no device context available). These are reported to `POST /api/collector/scan-error`.

Common causes:

- Collector not running as root or without `SYS_RAWIO`/`SYS_ADMIN` capabilities
- Drive physically failing to respond to commands
- Unsupported drive type or interface

No additional configuration is required. If notification URLs are configured, collector errors are sent through the same channels as SMART failures.

# HTML Email Delivery

For SMTP notification URLs, Scrutiny sends multipart email with separate
plain-text and HTML bodies when an HTML payload is available.

Current HTML-capable email paths include:

- scheduled reports
- test notifications
- collector-side error notifications
- missed ping digests
- heartbeat notifications
- performance degradation notifications
- replacement risk notifications
- MDADM degradation notifications

If an email arrives as plain text when you expected HTML:

- confirm the destination is an SMTP URL or an HTML-capable Apprise target
- inspect the raw message and verify `Content-Type: multipart/alternative`
- verify the specific notification type actually populates an HTML body
- if running in Docker, confirm the collector and web containers can access the
  required devices and metadata consistently; missing device mappings can lead
  to incorrect or stale alert state even when email delivery itself is working
