
# Feeder
<div style="display: flex; align-items: flex-start;">
    <img src="./docs/Feeder.png" alt="Feeder" style="width:200px; height:200px; margin-right: 15px;" />
    <div>
        <p>
            <strong>Feeder</strong> is a component of the <strong>finding-forwarder</strong> software suite. The Feeder serves as an alternative group of software developed by Forta (forta-rpc-scanner, forta-json-rpc), 
            Its primary function is to fetch the latest blockchain data at regular intervals and publish this data to a NATS topic. This allows other components, particularly bots, to subscribe to the topic and process the blockchain information.
        </p>
        <h3>How Feeder Works</h3>
        <ul>
            <li><strong>Feeder</strong> runs continuously, fetching the latest block data from the blockchain every 6 seconds.</li>
            <li>It retrieves transaction receipts and other relevant data, organizes this information, and publishes it to a specific topic defined in the environment variables: <code>BLOCK_TOPIC="blocks.mainnet.l1"</code>.</li>
            <li>Bots within the system subscribe to this topic (<code>blocks.mainnet.l1</code>) to listen for new block data. After processing, they send their findings to different NATS topics following the naming pattern: <code>findings.&lt;team_name&gt;.&lt;bot_name&gt;</code>.</li>
            <li>The processed findings are then managed and sent to notification channels by the <strong>Forwarder</strong> component, ensuring critical information reaches the appropriate channels.</li>
        </ul>
    </div>
</div>

### BlockDto Format
The data published by **Feeder** follows a structured JSON format defined in [block.dto.json](./brief/databus/block.dto.json). Below is the schema:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "BlockDto",
  "type": "object",
  "properties": {
    "number": {
      "type": "integer"
    },
    "timestamp": {
      "type": "integer"
    },
    "parentHash": {
      "type": "string"
    },
    "hash": {
      "type": "string"
    },
    "receipts": {
      "title": "Receipts",
      "type": "array",
      "items": {
        "title": "Receipt",
        "type": "object",
        "properties": {
          "to": {
            "type": "string"
          },
          "logs": {
            "title": "Logs",
            "type": "array",
            "items": {
              "title": "Log",
              "type": "object",
              "properties": {
                "address": {
                  "type": "string"
                },
                "topics": {
                  "title": "Topics",
                  "type": "array",
                  "items": {
                    "type": "string"
                  }
                },
                "data": {
                  "type": "string"
                },
                "blockNumber": {
                  "type": "integer"
                },
                "transactionHash": {
                  "type": "string"
                },
                "transactionIndex": {
                  "type": "integer"
                },
                "blockHash": {
                  "type": "string"
                },
                "logIndex": {
                  "type": "integer"
                },
                "removed": {
                  "type": "boolean"
                }
              },
              "required": [
                "address",
                "topics",
                "data",
                "blockNumber",
                "transactionHash",
                "transactionIndex",
                "blockHash",
                "logIndex",
                "removed"
              ]
            }
          }
        },
        "required": ["logs"]
      }
    }
  },
  "required": ["number", "timestamp", "hash", "parentHash", "receipts"]
}
```

### Key Functionality
1. **Regular Data Fetching:**
   - **Feeder** retrieves the latest blockchain block data every 6 seconds, ensuring that the system is always up-to-date with the latest information.
2. **Publishing to NATS JetStream:**
   - The fetched data is published to a defined NATS topic, which other bots can subscribe to for further processing.
3. **Integration with Finding-Forwarder Workflow:**
   - After bots process the data, the results are published to findings topics, which are then picked up by **Forwarder** for further handling and delivery to notification channels.

### Example Workflow:
1. **Feeder** fetches a new block from the blockchain and publishes it to `blocks.mainnet.l1`.
2. Bots subscribed to `blocks.mainnet.l1` process the block data, extracting relevant findings.
3. The bots send their findings to `findings.<team_name>.<bot_name>`.
4. **Forwarder** listens to these findings topics, applies quorum and filtering, and forwards the important findings to configured notification channels like Telegram, Discord, or OpsGenie.

### Process Flow Diagram

```plaintext
+--------------------------+
|          Feeder          |
| Fetches latest block data|
+-----------+--------------+
            |
            v
+--------------------------+
| Publishes block data to  |
|  blocks.mainnet.l1 topic |
+-----------+--------------+
            |
            v
+-------------------------------+
|         Bots Subscribed       |
| Listen to blocks.mainnet.l1   |
+-----------+-------------------+
            |
            v
+-----------------------------------+
| Process block data and send       |
| findings to findings.<team>.<bot> |
+-----------+-----------------------+
            |
            v
+------------------------------------+
|          Forwarder                 |
| Listens to findings topics         |
| Applies quorum and filtering       |
| Forwards to notification channels  |
| (Telegram, Discord, OpsGenie)      |
+------------------------------------+
```