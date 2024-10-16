
# Notification System Configuration

This configuration file defines the settings for a notification system that forwards messages to various channels such as Telegram, Discord, and OpsGenie. The configuration includes settings for severity levels, channels, and consumers. Below is an explanation of how to configure the YAML file properly.

## Sections

### 1. **Severity Levels**
This section defines the different severity levels used to categorize messages. These severity levels will be referenced by the consumers to filter which messages they will process.

Example:
```yaml
severity_levels:
  - id: Unknown
  - id: Info
  - id: Low
  - id: Medium
  - id: High
  - id: Critical
```

- **id**: The identifier for the severity level. These values will be used to define which severity levels a consumer should process.

### 2. **Telegram Channels**
Define the Telegram channels where notifications will be sent. Each channel must have a unique ID, a description, and authentication details (bot token and chat ID).

Example:
```yaml
telegram_channels:
  - id: Telegram1
    description: "Telegram channel for debug messages (severity: Unknown, no quorum)"
    bot_token: YOUR_TELEGRAM_BOT_TOKEN_1
    chat_id: YOUR_CHAT_ID_1
  - id: Telegram2
    description: Telegram channel for all messages >= Low, no quorum
    bot_token: YOUR_TELEGRAM_BOT_TOKEN_2
    chat_id: YOUR_CHAT_ID_2
```

- **id**: Unique identifier for the Telegram channel.
- **description**: A short description of the channel and its intended purpose.
- **bot_token**: The Telegram bot token used to authenticate the bot.
- **chat_id**: The chat ID where the messages will be sent.

### 3. **Discord Channels**
Define the Discord channels where notifications will be sent. Each channel must have a unique ID, description, and a webhook URL for sending messages.

Example:
```yaml
discord_channels:
  - id: Discord1
    description: "Discord channel for debug messages (severity: Unknown, no quorum)"
    webhook_url: YOUR_DISCORD_WEBHOOK_URL_1
```

- **id**: Unique identifier for the Discord channel.
- **description**: A short description of the channel and its intended purpose.
- **webhook_url**: The Discord webhook URL used to send messages to the channel.

### 4. **OpsGenie Channels**
Define the OpsGenie channels where critical alerts will be sent. Each channel must have a unique ID, description, and an API key.

Example:
```yaml
opsgenie_channels:
  - id: OpsGenie1
    description: OpsGenie channel for High and Critical messages
    api_key: YOUR_OPSGENIE_API_KEY_1
```

- **id**: Unique identifier for the OpsGenie channel.
- **description**: A short description of the channel and its intended purpose.
- **api_key**: The API key used to authenticate with OpsGenie.

### 5. **Consumers**
Consumers are responsible for listening to specific NATS subjects and forwarding messages to the appropriate channels. Each consumer has its own configuration, including the channel to which it sends notifications, the severity levels it processes, and whether it operates based on a quorum.

Example:
```yaml
consumers:
  - consumerName: TelegramDebug
    type: Telegram
    channel_id: Telegram1
    severities:
      - Unknown
    by_quorum: false
    subjects:
      - findings.protocol.steth
      - findings.protocol.arb
      - findings.protocol.opt
```

- **consumerName**: The name of the consumer, which will be used to generate the unique consumer name as `<teamName>_<consumerName>_<botName>`.
- **type**: The type of channel the consumer forwards messages to (`Telegram`, `Discord`, or `OpsGenie`).
- **channel_id**: The ID of the channel where the messages will be sent (references `telegram_channels`, `discord_channels`, or `opsgenie_channels`).
- **severities**: The list of severity levels this consumer will process (references `severity_levels`).
- **by_quorum**: A boolean flag indicating whether the consumer requires a quorum to process messages. `true` means the consumer will wait for quorum.
- **subjects**: The list of NATS subjects that this consumer listens to. The second part of the subject is the team name, and the third part is the bot name.

### Example Consumer Breakdown

1. **TelegramDebug**
    - Processes messages with severity `Unknown` and sends them to the `Telegram1` channel.
    - Does not use a quorum.
    - Listens to subjects: `findings.protocol.steth`, `findings.protocol.arb`, `findings.protocol.opt`.

2. **TelegramForwarder**
    - Processes messages with severity `Low`, `Medium`, `High`, `Critical` and sends them to the `Telegram2` channel.
    - Does not use a quorum.
    - Listens to the same subjects as `TelegramDebug`.

3. **DiscordDebug**
    - Processes messages with severity `Unknown` and sends them to the `Discord1` channel.
    - Does not use a quorum.
    - Listens to subjects: `findings.protocol.steth`, `findings.protocol.arb`, `findings.protocol.opt`.

### Configuration Structure

```yaml
severity_levels:
  - id: Unknown
  - id: Info
  - id: Low
  - id: Medium
  - id: High
  - id: Critical

telegram_channels:
  - id: Telegram1
    description: "Telegram channel for debug messages (severity: Unknown, no quorum)"
    bot_token: YOUR_TELEGRAM_BOT_TOKEN_1
    chat_id: YOUR_CHAT_ID_1
  - id: Telegram2
    description: Telegram channel for all messages >= Low, no quorum
    bot_token: YOUR_TELEGRAM_BOT_TOKEN_2
    chat_id: YOUR_CHAT_ID_2

discord_channels:
  - id: Discord1
    description: "Discord channel for debug messages (severity: Unknown, no quorum)"
    webhook_url: YOUR_DISCORD_WEBHOOK_URL_1

opsgenie_channels:
  - id: OpsGenie1
    description: OpsGenie channel for High and Critical messages
    api_key: YOUR_OPSGENIE_API_KEY_1

consumers:
  - consumerName: TelegramDebug
    type: Telegram
    channel_id: Telegram1
    severities:
      - Unknown
    by_quorum: false
    subjects:
      - findings.protocol.steth
      - findings.protocol.arb
      - findings.protocol.opt

  - consumerName: DiscordDebug
    type: Discord
    channel_id: Discord1
    severities:
      - Unknown
    by_quorum: false
    subjects:
      - findings.protocol.steth
```

### Important Notes:
- **Unique Channel IDs**: Ensure that each `channel_id` in `telegram_channels`, `discord_channels`, and `opsgenie_channels` is unique.
- **Unique Consumer Names**: Each consumer must have a unique `consumerName` for proper operation.
- **NATS Subjects**: Consumers will listen to the subjects defined in the `subjects` field.

### Visualisation:
**Simple Explanation:**
The consumer named **TelegramForwarder** uses the **Telegram Channel** to send alerts from the following NATS subjects: findings.protocol.steth, findings.protocol.arb, and findings.protocol.opt.

**Detailed Explanation:**
The system will dynamically create three separate consumers: **protocol_TelegramForwarder_steth**, **protocol_TelegramForwarder_arb**, and **protocol_TelegramForwarder_opt**.
Each consumer is responsible for delivering findings specific to its corresponding bot (steth, arb, or opt), using the designated **Telegram Channel** for alert notifications.

```
+---------------------------------------------------+
|                NATS Subjects                      |
|---------------------------------------------------|
| findings.protocol.steth                           |
| findings.protocol.arb                             |
| findings.protocol.opt                             |
+---------------------------------------------------+
              | 
              v
+----------------------------------------------------+
|               Consumer: TelegramForwarder          |
|----------------------------------------------------|
| Type       : Telegram                              |
| Channel ID : Telegram                              |
| Severities : Low, Medium, High, Critical           |
| Quorum     : Yes                                   |
+----------------------------------------------------+
              | 
              v
+---------------------------------------------------+
|            Telegram Channel                       |
|---------------------------------------------------|
| Description: "Telegram channel for all messages   |
|              (>= Low severity, by quorum)"        |
| Bot Token  : YOUR_TELEGRAM_BOT_TOKEN              |
| Chat ID    : YOUR_CHAT_ID                         |
+---------------------------------------------------+
```