
# Notification Configuration Guide

This configuration file is used to set up notification channels for various services like Telegram, Discord, and OpsGenie. It defines the severity levels for events and the consumers that process messages from NATS. The configuration allows you to manage different notification channels, assign severity filters, and control the quorum settings for each consumer.

## Main Sections of the Configuration

### 1. **severity_levels**

This section defines the available severity levels for filtering events based on their importance. These severity levels are referenced in the `notification_channels` section to specify which events a consumer should handle.

Available severity levels:
- `info`
- `low`
- `medium`
- `high`
- `critical`

Example:
```yaml
severity_levels:
  - id: "info"
  - id: "low"
  - id: "medium"
  - id: "high"
  - id: "critical"
```

### 2. **telegram_channels**

This section describes all available Telegram channels for sending notifications.

Fields:
- `id`: A unique identifier for the Telegram channel. This will be used to reference the channel in the `notification_channels` section.
- `description`: A brief description of what this channel is used for (e.g., which team or type of alerts).
- `token`: The Telegram bot token used to send notifications.
- `chat_id`: The Telegram chat ID where notifications will be sent.

Example:
```yaml
telegram_channels:
  - id: "Telegram1"
    description: "Main Telegram channel for Team A alerts"
    token: "YOUR_TELEGRAM_BOT_TOKEN_1"
    chat_id: "YOUR_CHAT_ID_1"
```

### 3. **discord_channels**

This section defines Discord channels that send notifications via webhooks.

Fields:
- `id`: A unique identifier for the Discord channel.
- `description`: A description of what the channel is used for (e.g., what type of alerts it handles).
- `webhook_url`: The Discord webhook URL used to send notifications.

Example:
```yaml
discord_channels:
  - id: "Discord1"
    description: "Discord channel for Team A's critical alerts"
    webhook_url: "YOUR_DISCORD_WEBHOOK_URL_1"
```

### 4. **opsgenie_channels**

This section lists OpsGenie channels for sending alerts using the OpsGenie API.

Fields:
- `id`: A unique identifier for the OpsGenie channel.
- `description`: A description explaining the purpose of this OpsGenie channel.
- `api_key`: The API key used to send notifications via OpsGenie.

Example:
```yaml
opsgenie_channels:
  - id: "OpsGenie1"
    description: "OpsGenie integration for Team A critical incidents"
    api_key: "YOUR_OPSGENIE_API_KEY_1"
```

### 5. **teams**

The `teams` section defines the teams and their corresponding consumers that will listen to specific NATS subjects and send notifications using the previously defined channels.

Fields:
- `name`: The name of the team.
- `nats_subject`: The default NATS subject that this team listens to for events. It can be overridden at the consumer level if necessary.
- `notification_channels`: A list of consumers that will process messages and send notifications.

### 6. **notification_channels**

Each `notification_channels` entry describes a consumer that listens for messages and sends notifications. **The `consumerName` must be unique** across all teams. This is a requirement, as NATS does not allow multiple consumers with the same name.

Fields:
- `consumerName`: A unique name for this consumer. It **must be unique** across the entire configuration.
- `type`: The type of notification channel (`Telegram`, `Discord`, or `OpsGenie`).
- `channel_id`: A reference to the channel defined in either `telegram_channels`, `discord_channels`, or `opsgenie_channels` using its `id`.
- `severity_refs`: A list of severity levels this consumer should process. These levels must be defined in the `severity_levels` section.
- `by_quorum`: A boolean value (`true` or `false`) indicating whether a quorum is required before the notification is sent.
- `nats_subject`: (Optional) A specific NATS subject that this consumer will listen to. If not provided, the team's default `nats_subject` will be used.

Example:
```yaml
notification_channels:
  - consumerName: "TelegramAlerts1"
    type: "Telegram"
    channel_id: "Telegram1"
    severity_refs:
      - "high"
      - "critical"
    by_quorum: true
    nats_subject: "nats.teamA.custom"
```

### Example Configuration for Team "protocol"

This is an example setup for a team called "protocol" that uses multiple Telegram, Discord, and OpsGenie channels with different severity levels and quorum settings.

```yaml
severity_levels:
  - id: "info"
  - id: "low"
  - id: "medium"
  - id: "high"
  - id: "critical"
  - id: "unknown"

telegram_channels:
  - id: "Telegram1"
    description: "Telegram channel for debug messages (severity: unknown, no quorum)"
    token: "YOUR_TELEGRAM_BOT_TOKEN_1"
    chat_id: "YOUR_CHAT_ID_1"
  - id: "Telegram2"
    description: "Telegram channel for all messages >= low, no quorum"
    token: "YOUR_TELEGRAM_BOT_TOKEN_2"
    chat_id: "YOUR_CHAT_ID_2"
  - id: "Telegram3"
    description: "Telegram channel for messages >= low, with quorum"
    token: "YOUR_TELEGRAM_BOT_TOKEN_3"
    chat_id: "YOUR_CHAT_ID_3"
  - id: "Telegram4"
    description: "Telegram channel for high and critical messages, with quorum"
    token: "YOUR_TELEGRAM_BOT_TOKEN_4"
    chat_id: "YOUR_CHAT_ID_4"

discord_channels:
  - id: "Discord1"
    description: "Discord channel for debug messages (severity: unknown, no quorum)"
    webhook_url: "YOUR_DISCORD_WEBHOOK_URL_1"
  - id: "Discord2"
    description: "Discord channel for all messages >= low, no quorum"
    webhook_url: "YOUR_DISCORD_WEBHOOK_URL_2"

opsgenie_channels:
  - id: "OpsGenie1"
    description: "OpsGenie channel for high and critical messages"
    api_key: "YOUR_OPSGENIE_API_KEY_1"

teams:
  - name: "protocol"
    nats_subject: "nats.protocol.findings"

    notification_channels:
      - consumerName: "TelegramDebug"
        type: "Telegram"
        channel_id: "Telegram1"
        severity_refs:
          - "unknown"
        by_quorum: false

      - consumerName: "TelegramAllNoQuorum"
        type: "Telegram"
        channel_id: "Telegram2"
        severity_refs:
          - "low"
          - "medium"
          - "high"
          - "critical"
        by_quorum: false

      - consumerName: "TelegramAllWithQuorum"
        type: "Telegram"
        channel_id: "Telegram3"
        severity_refs:
          - "low"
          - "medium"
          - "high"
          - "critical"
        by_quorum: true

      - consumerName: "TelegramHighCritical"
        type: "Telegram"
        channel_id: "Telegram4"
        severity_refs:
          - "high"
          - "critical"
        by_quorum: true

      - consumerName: "DiscordDebug"
        type: "Discord"
        channel_id: "Discord1"
        severity_refs:
          - "unknown"
        by_quorum: false

      - consumerName: "DiscordAllNoQuorum"
        type: "Discord"
        channel_id: "Discord2"
        severity_refs:
          - "low"
          - "medium"
          - "high"
          - "critical"
        by_quorum: false

      - consumerName: "OpsGenieHighCritical"
        type: "OpsGenie"
        channel_id: "OpsGenie1"
        severity_refs:
          - "high"
          - "critical"
        by_quorum: true
```

