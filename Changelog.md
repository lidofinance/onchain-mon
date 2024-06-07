## 07.06.2024
1. Optimized nats consumer worker
2. Optimized alert handlers
3. Added pprof profile handler
4. Added forta-local-config.yaml
5. Fixed crush for wrong app-name for prometheus prefix metric name

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