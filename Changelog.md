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
1. Added sending alertAlias to OpsGenia
2. Fix alert handler

## 05.06.2024
1. Added Nats
2. Added workers for sending alert to telegram, discord and, opsGenia
3. Load environment variables inside docker from shell

## 31.05.2024
1. Added method for sending messages into telegram chat
2. Added method for sending messages into discord chat
3. Added method for sending messages into opsGenia chat
4. Set up linter rules
5. Preparation for redis-queue task

## 30.05.2024
1. Forked from go-template
2. Added forta-webhook support