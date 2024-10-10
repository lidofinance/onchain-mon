## 09.10.2024
1. Added stage consumers

## 08.10.2024
1. Split up feeder, forwarder to independent bin applications
2. Added mechanism for using UniqueKey for collecting quorum
3. Renamed repo from "finding-forwarder" to "onchain-mon"

## 04.10.2024
1. Removed Forta integration.
2. Fixed issues with Telegram markdown formatting.
3. Improved error handling: if FF fails to send a message with Telegram markdown, it will now send it as plain text.
4. Implemented length checks for Telegram: messages exceeding 4,096 characters will be truncated.
5. Implemented length checks for Discord: messages exceeding 2,000 characters will be truncated.

## 24.09.2024
1. Fix sending network alerts though telegram

## 23.09.2024
1. Add lru for quorum
2. Tun docker-compose-file
3. Upgrade GO 1.23.1
4. Increased MaxMsgSize for Nats for 3Mb
5. Lint project

## 16.09.2024
1. Added redis
2. Added quorum powered by redis
3. Added retry for sending message to Telegram, Discord, OpsGenie

## 14.09.2024
1. Added feeder

## 22.06.2024
1. Added DevOps independent consumer
2. Updated readme.md
3. Changed ```request_processing_seconds``` metric type from summary to histogram

## 21.06.2024
1. Added worker for each team

## 20.06.2024
1. Added version, commit to metric_build
2. Update dependencies
3. Update dependencies in tools

## 19.06.2024
1. Moved from logrus to default slog logger
2. Split up worker and service from one binary app
3. Added finding_published_total, finding_sent_total metrics, request_processing_seconds

## 17.06.2024
1. Added reconnect feature for nats client
2. Added swagger 200, 400 responses for /alert handler

## 07.06.2024
1. Optimized nats consumer worker
2. Optimized alert handlers
3. Added pprof profile handler
4. Added forta-local-config.yaml
5. Fixed crush for wrong app-name for prometheus prefix metric name
6. Updated dependencies

## 06.06.2024
1. Added sending alertAlias to OpsGenie
2. Fix alert handler

## 05.06.2024
1. Added Nats
2. Added workers for sending alert to telegram, discord and, opsGenie
3. Load environment variables inside docker from shell

## 31.05.2024
1. Added method for sending messages into telegram chat
2. Added method for sending messages into discord chat
3. Added method for sending messages into opsGenie chat
4. Set up linter rules
5. Preparation for redis-queue task

## 30.05.2024
1. Forked from go-template
2. Added forta-webhook support