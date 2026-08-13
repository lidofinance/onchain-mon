# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`onchain-mon` (Go module `github.com/lidofinance/onchain-mon`) is a two-binary service suite that replaces OpenZeppelin Defender / Forta for Lido: it feeds Ethereum block data into NATS, and forwards bot findings to Telegram / Discord / OpsGenie / Slack with quorum-based deduplication.

Note: the repo was renamed from `finding-forwarder` to `onchain-mon`; the README and some docs still use the old name and a stale `./cmd/service` build path. The real entrypoints are `cmd/feeder` and `cmd/forwarder`.

Go toolchain: `go 1.26.5` (per `go.mod`).

## Commands

```bash
make tools         # install golangci-lint, mockery, goimports, go-jsonschema, govulncheck into ./bin
make vendor        # go mod tidy && go mod vendor && go mod verify (vendor/ is gitignored — local build cache only)
make build         # STALE: points at ./cmd/service which does not exist
make format        # imports + fmt + vet
make check-format  # same check without rewriting files (what CI runs)
make lint          # bin/golangci-lint run --config=.golangci.yml
make fix-lint      # same with --fix
make test          # safe: no network, no credentials
make test-live     # adds -tags=live — hits real RPC and posts to real channels
make vulncheck
make outdated
make generate-databus-objects  # brief/databus/*.dto.json -> generated/databus/*.dto.go
make generate-docker           # lidofinance/onchain-mon:stable
```

Local environments — each is a separate compose stack:

```bash
make up | down | logs                      # mainnet + prometheus + grafana
make up-testnet | down-testnet | logs-testnet   # Hoodi, own ports and Redis DB
make up-prod | down-prod | logs-prod       # three cells emulating production
make down-all
```

`mainnet` and `prod` both bind Redis on 6379 and cannot run at the same time; `testnet` runs alongside either. Shared service definitions live in `docker-compose.base.yaml` and are pulled in via `extends`.

Build the actual binaries (as the Dockerfile does):

```bash
go build -o ./bin/feeder    ./cmd/feeder
go build -o ./bin/forwarder ./cmd/forwarder
```

Run the local stack (Redis + NATS + both services): `docker-compose up -d`. To develop one service locally, comment it out of `docker-compose.yml` and point its env at the dockerized NATS/Redis.

### Tests

```bash
make test        # go test ./cmd/... ./internal/... — safe, no network, no credentials
make test-live   # adds -tags=live — hits real RPC and posts to real channels
go test ./internal/pkg/notifiler/ -run '^TestFormatAlert$' -v   # single test
```

**Live tests are behind a build tag.** `internal/pkg/chain/chain_test.go` and the telegram/discord/opsgenie/slack `*_test.go` files call `env.Read("../../../.env")` and `env.ReadNotificationConfig(..., "../../../notification.yaml")`, then hit **live** RPC and messaging APIs. They carry `//go:build live`, so:

- `make test` / `go test ./...` are safe on a clean checkout — the live files are not built.
- Running `make test-live` can send real messages to real Lido channels. It needs a populated repo-root `.env` and `notification.yaml`.
- `internal/pkg/consumer/consumer_test.go` needs Redis on `127.0.0.1:6379` (DB 15, from `docker-compose.yml`) and **skips** itself when it is unreachable.

Prefer adding new tests in the offline style (construct the notifier directly, no `env.Read`) — see `internal/pkg/notifiler/opsgenie_unit_test.go`.

## Architecture

```
cmd/
  feeder/main.go        # binary 1: chain -> NATS JetStream
  forwarder/main.go     # binary 2: NATS findings -> quorum -> notification channels
internal/
  app/
    feeder/             # block polling loop, zstd-compressed publish, gap recovery
    forwarder/          # creates one JetStream durable consumer per (consumer x subject)
    server/             # chi router, /health /metrics /debug (pprof)
  connectors/           # logger (slog+sentry), metrics (prometheus), nats, redis
  env/
    env.go              # AppConfig from .env / shell (viper, sync.Once)
    notify_config.go    # notification.yaml parsing + ValidateConfig + CollectNatsSubjects
    notification_channels.go  # builds notifiler instances keyed by channel id
  http/handlers/health/ # the only handler left
  pkg/
    chain/              # JSON-RPC client (retry-go), entity/ = raw eth types
    consumer/           # THE core: quorum/dedup/cooldown state machine + Redis repo
    notifiler/          # FindingSender impls: telegram, discord, opsgenie, slack
  utils/                # pointers, registry (channel + severity types), text
generated/databus/      # GENERATED from brief/databus/*.dto.json — DO NOT EDIT
brief/databus/          # JSON Schemas: block.dto.json, finding.dto.json (source of truth)
infra/                  # local stack config; runtime state next to it is gitignored
  nats/nats.conf        # max_payload must stay in sync with nats.MaxMsgSize
  prometheus/           # scrape config with the env/source/service labels the dashboard needs
  grafana/provisioning/ # datasource (uid from $GRAFANA_DS_UID) + dashboard provider
  grafana/dashboards/   # gitignored — copy of the production dashboard from the ansible repo
.github/workflows/checks.yml  # format, lint, vulncheck, test (with Redis service), docker build
```

### Data flow

1. **Feeder** polls `eth_getBlockByNumber` + `eth_getBlockReceipts`, builds `databus.BlockDtoJson`, zstd-compresses it, and `PublishAsync`es to `BLOCK_TOPIC` (e.g. `blocks.mainnet.l1`) with `WithMsgID(blockHash)` for idempotency. It tracks `prevBlockNumber` and self-heals: on >2min without the next block it calls `recoverMissedBlocks` to batch-fetch and publish the gap; the timer is re-armed relative to the block timestamp (`EtaNextBlock` 12s) rather than a fixed tick.
2. **Bots** (external repo `lidofinance/testing-forta-bots`) consume block data and publish `databus.FindingDtoJson` to `findings.<team>.<bot>`.
3. **Forwarder** creates/updates a single JetStream stream `NatsStream` (`InterestPolicy`, `MaxAge` 10min, 4MB max msg) whose subjects come from `CollectNatsSubjects(notificationConfig)`, then one durable consumer per subject per configured notification consumer.

### Quorum model (the part that needs care)

Three forwarder instances run on separate VMs and share Redis; `QUORUM_SIZE` (2-of-3 in prod) decides when a finding is actually sent. `SOURCE` **must be unique per instance** — the forwarder refuses to start without it. All of this lives in `internal/pkg/consumer/consumer.go` (`GetConsumeHandler`) and `repo.go`:

- Per consumer, per finding `UniqueKey`, Redis holds `<consumerName>:finding:<key>:count` and `:status` (`not_send` / `sending` / `sent`).
- An in-process `expirable.LRU` guards the `INCR`: the first sighting increments then **nacks** (so a single instance can't satisfy quorum by itself); later sightings only `GET`.
- Below quorum → `NakWithDelay(ResendQuorumMsgAfter)`. At quorum, `SetSendingStatus` runs a **Lua script** to atomically claim the send (`not_send` → `sending`), so exactly one instance delivers.
- After a successful send: `status=sent` plus a 30-min **cooldown** key hashed over `botName + alertId + sha256(description) + consumerName` — the same alert body with a different block number is suppressed.
- On send failure the count is decremented and the status key deleted so another instance can retry.
- `by_quorum: false` consumers (debug channels) skip all of the above and instead use a `SetNX` dedup key (`DedupKeyTTL` 15min) derived from `source + consumerName + uniqueKey`.

`MaxAckPending` is deliberately `6` for quorum consumers and `1` otherwise — 3 instances x 6 = 18 in-flight sends, kept under Telegram's ~20 msg/min per-bot limit. When a notifier returns a `*notifiler.RateLimitedError`, the handler nacks with `rle.ResetAfter + 500ms` instead of dropping the finding. Changing these numbers changes rate-limit behavior in prod.

`GetConsumeHandler` has ~20 exit points and **every one of them must settle the message** (ack / nack / nackDelay / terminate). A path that returns without settling leaves the message hanging for the full `AckWait` (30s) and burns one of the 10 `MaxDeliver` attempts — the failure is silent, so the compiler will not catch it and neither will a passing test.

### Traps worth knowing

- **Redis TTLs inside the Lua script take seconds, not `time.Duration`.** `repo.go` passes args to `EVAL` verbatim, so handing it a `Duration` sends nanoseconds. Everywhere else (`Set`, `SetNX`, `Expire`) go-redis converts for you.
- **Metrics live in the app's own registry.** `metrics.New` builds collectors with `promauto.With(promRegistry)` and `/metrics` is served by `promhttp.HandlerFor(a.Metrics.Prometheus, …)`. These two belong together: switching one without the other silently drops every `*Vec` metric from the endpoint. Go/process collectors are registered explicitly for the same reason.
- **Metric names come from `APP_NAME`** (`[ -]` replaced with `_`). Production runs `APP_NAME: feeder` and `APP_NAME: forwarder`, so the metrics are `feeder_*` / `forwarder_*` and the Grafana dashboard depends on that. The local compose pins the same values rather than reading `.env`.
- **`PORT` and `LOG_FORMAT` are pinned in docker-compose too** — the dashboard's scrape targets assume 8080 and its log panels parse fields with `| json`. A local `.env` saying otherwise breaks both without any error.

### Configuration

Two separate config sources, both read at startup:

- **`.env`** (or shell env when `READ_ENV_FROM_SHELL=true`) → `internal/env/env.go`. Keys are documented in `README.md`; `sample.env` is the template. `MetricsPrefix` is derived from `APP_NAME` with `[ -]` replaced by `_`.
- **`notification.yaml`** → declares `severity_levels`, `*_channels` (with tokens/webhooks), and `consumers`. **`ReadNotificationConfig` ignores the passed path unless `ENV=local`** — for any other env it hard-codes `/etc/forwarder/notification.yaml`, which is why the local compose mounts the file at both `/app/notification.yaml` and `/etc/forwarder/notification.yaml`. `ValidateConfig` enforces unique `consumerName`s, that every consumer's `channel_id` resolves to a declared channel of that `type`, that severities are known, and that `subjects` is non-empty; it also populates the derived `SeveritySet` / `FindingFilterMap`. Samples: `notification.sample.yaml`, `notification.prod.sample.yaml`, `notification.dev.yaml`.

A consumer's NATS subject must be `findings.<team>.<bot>` — `NewConsumers` splits on `.` and requires ≥3 parts to derive the durable name `<team>_<consumerName>_<bot>`. `ValidateConfig` now enforces that shape up front, along with: at least one consumer and one severity level, a non-empty `consumerName`, a non-empty severity set per consumer, and no two consumers resolving to the same durable name (`a_b` + `findings.x.y` and `b` + `findings.x_a.y` both yield `x_a_b_y`, which would make them share one JetStream consumer). Each of these used to pass validation and either crash the forwarder at startup or leave it forwarding nothing.

DTOs are generated, not hand-written: edit `brief/databus/*.dto.json` then run `make generate-databus-objects`.

## Conventions

`docs/code_style.md` is the authoritative style guide. The load-bearing rules:

- **Never panic** outside app/worker initialization.
- Accept interfaces, return concrete structs. Constructors for handler/usecase/repo types return **unexported** structs (`revive`'s `unexported-return` is disabled for this reason) — see `chain.NewChain` and `forwarder.New`.
- Compare strings to `""`, not `len(s) > 0`; check slices with `len(x) == 0`; avoid `else`.
- Test names use `snake_case`, no spaces.
- Mocks go in `internal/pkg/<domain>/mocks` via `mockery` (`bin/mockery`, v3).
- `TODO` comments must link a task.

Lint is strict (`gosec`, `mnd`, `funlen`, `gocritic` with all tags, `lll` at 140 chars, `dupl` at 100). Magic numbers need named constants — this is why the codebase is full of `TTLMins10`, `NackDelayMsg`, `Per6Sec`, etc. `goimports` uses `-local github.com/lidofinance/onchain-mon`, so this module's imports form their own trailing group. Run `make format && make lint` before finishing.

Logging: user-facing/error strings are passed through `text.LeaveOnlyDomainInURLs` before hitting the log to strip RPC URL credentials — keep this when adding log lines that may embed URLs (the RPC error path in `chain.go` does the same).

## Further docs

`feeder.md`, `forwarder.md`, `config.md` cover each component and every config key in detail; `docs/structure.md` documents the intended Go project layout.
