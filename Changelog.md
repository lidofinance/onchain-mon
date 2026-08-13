## 13.08.2026

### Fixed
1. Consumer: finding could be lost when Redis `Incr` failed — the LRU was marked before the counter was accepted, so the redelivered message took the "already seen" path and got acked without ever being counted
2. Consumer: the count key could stay in Redis forever without a TTL when `Expire` failed — `Decr` left it at 0 instead of dropping it
3. Consumer: a message was left unsettled when another instance claimed the send, so it hung for the whole `AckWait` (30s) and burned one of the 10 `MaxDeliver` attempts
4. Consumer: the `StatusFail` metric was counted twice on a send failure
5. Removed deprecated chi `middleware.RealIP` (GHSA-3fxj-6jh8-hvhx and friends) — `RemoteAddr` is never read, and the services expose only infra endpoints
6. Forwarder: `notification.yaml` is now also mounted at `/app`, so it is found when `ENV=local`

### Changed
7. Update to go1.26.5
8. Update dependencies
9. Update dev tools; `gomodguard` -> `gomodguard_v2`
10. NATS max message size moved into a single constant `nats.MaxMsgSize` and raised to 8 Mb, matching `max_payload` in nats.conf
11. `AppConfig` now matches the keys actually parsed from env — dropped `URL`, `FindingTopic`, `RedisURL`, `RedisDB` and the unused Redis Streams names
12. Reworked `sample.env` and added `sample.testnet.env`; dropped the dead `NATS_PUBLISH_TOPIC`
13. Consumer: extracted `handleWithoutQuorum` and `collectQuorumCount` out of `GetConsumeHandler`

### Added
14. CI workflow: format, lint, vulncheck, build + tests against a Redis service, and a docker image build so a broken Dockerfile surfaces before deploy
15. Consumer: tests for quorum counting and for losing the send race
16. `docker-compose.testnet.yaml` (Hoodi) with its own ports, container names and Redis DB — it runs alongside the mainnet stack
17. `docker-compose.base.yaml` holds the shared redis/nats definitions; mainnet, testnet and prod inherit them via `extends`
18. Makefile targets per environment: `up`/`down`/`logs`, `up-testnet`/`down-testnet`/`logs-testnet`, `up-prod`/`down-prod`/`logs-prod`, `down-all`
19. `make check-format`, `make test` and `make test-live`

### Removed
20. The unused `/` config page: `web/templates` and the `show` handler
21. The `tools` module — its pinned versions had drifted from what `make tools` installs

### Tooling
22. Live RPC/messaging tests now sit behind the `live` build tag, so `go test ./...` no longer posts to real channels — use `make test-live` for those
23. Fixed the `vulncheck` target: it was missing the package pattern, so it never actually scanned anything
24. Fixed the format/imports conflict: `goimports` local-prefixes pointed at golangci-lint's own module, so the IDE and the CLI grouped imports differently
25. Enabled more linters: `errorlint`, `modernize`, `perfsprint`, `usestdlibvars`, `usetesting`
26. Local infrastructure state moved into `infra/` (nats, steth-db); runtime data is gitignored, `nats.conf` stays tracked

## 06.05.2026
1. Added string sanitizer

## 08.04.2026
1. Forwarder: add `details` field to OpsGenie alerts with forwarder attributes (env, source, team, botName, alertId) for flexible routing
2. Forwarder: add unit tests for FormatAlert and OpsGenie AlertPayload serialization

## 24.10.2025
1. Forwarder: add Slack channel notifications for findings.
2. Forwarder: pass environment to notification channels and include it in OpsGenie alerts

## 08.09.2025
1. Update to go1.25.1
2. Update dependencies

## 18.08.2025
1. Update to go1.25
2. Update dependencies
3. Fix linters, format warnings
4. Add docker-compose.prod.yml for emulating prod setup

## 07.08.2025
1. Updated NATS consumers to handle explicit delayed message redelivery for Discord, Telegram, and OpsGenie
2. Updated go-redis/v8 -> go-redis/v9

## 13.05.2025
1. Add support for fetching blocks by number and dynamic time
2. Add support for skipped blocks

## 29.04.2025
1. Forwarder: added Cool down period for sent findings.

## 18.04.2025
1. Feeder: added int blockDto: "From", "TransactionHash" to each receipt

## 04.04.2025
1. Added env variable BLOCK_EXPLORER = {etherscan, hoody.etherscan and etc}

## 21.02.2025
1. Added zstd compression

## 18.02.2025
1. Increased MaxMsgSize for Nats for 4Mb

## 13.02.2025
1. Integrated sentry with slog

## 07.02.2025
1. Fix opsGenia status. Send only P1, P2.

## 09.12.2024
1. Dynamic config feature: filter only desired Findings.

## 05.12.2024
1. Remove PublishedAlerts metrics
2. Added NotifyChannels: `forwarder_notification_channel_error_total`
3. Inc `feeder_blocks_published_total` for each unsuccessful network request

## 28.11.2024
1. Field "uniqueKey" is required for getting quorum.

## 16.10.2024
1. Added dynamic yaml notification config

## 11.10.2024
1. Quorum hash calculation: if unique key is specified - salt it with botId and Team
2. Made alert footer compact (2 lines) and add "happened ~ X seconds ago" text
3. Added code for stage consumers: yet disabled by commenting out
4. Fixed statusKey: from key -> statusKey on statusSent
5. Set expired 1m ttl for statusKey, countKey when instance has sent finding

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