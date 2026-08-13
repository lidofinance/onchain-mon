package consumer

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

// These tests need a Redis instance. They are skipped when it is not reachable,
// so `go test ./...` still works on a clean checkout. Locally Redis comes from
// docker-compose.yml; tests use DB 15 and clean up after themselves.
const testRedisAddr = "127.0.0.1:6379"
const testRedisDB = 15

type testMsg struct {
	jetstream.Msg
	payload []byte
	acked   bool
	nacked  bool
	delay   time.Duration
	settled bool
}

func (m *testMsg) Data() []byte { return m.payload }
func (m *testMsg) Ack() error   { m.acked = true; m.settled = true; return nil }
func (m *testMsg) Nak() error   { m.nacked = true; m.settled = true; return nil }
func (m *testMsg) NakWithDelay(d time.Duration) error {
	m.nacked = true
	m.delay = d
	m.settled = true
	return nil
}

type stubNotifier struct {
	called bool
	err    error
}

func (n *stubNotifier) SendFinding(_ context.Context, _ *databus.FindingDtoJson) error {
	n.called = true
	return n.err
}
func (n *stubNotifier) GetType() registry.NotificationChannel { return registry.Telegram }

// metrics.New registers collectors in the global promauto registry, so a Store
// can only be built once per test binary.
var testMetricsOnce sync.Once
var testMetrics *metrics.Store

func newTestMetrics() *metrics.Store {
	testMetricsOnce.Do(func() {
		testMetrics = metrics.New(prometheus.NewRegistry(), "consumer_test", "test", "test")
	})
	return testMetrics
}

func dialTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: testRedisDB})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis is not reachable at %s: %v", testRedisAddr, err)
	}

	return rdb
}

const testQuorumSize = 2

func newTestConsumer(rdb *redis.Client, notifier *stubNotifier) *Consumer {
	return &Consumer{
		log:         slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1})),
		mtrs:        newTestMetrics(),
		cache:       expirable.NewLRU[string, uint](10, nil, time.Minute),
		redisClient: rdb,
		repo:        NewRepo(rdb, testQuorumSize),
		source:      "test-source",
		name:        "test-consumer",
		byQuorum:    true,
		quorumSize:  testQuorumSize,
		severitySet: registry.FindingMapping{databus.SeverityCritical: true},
		notifier:    notifier,
	}
}

func testFinding(uniqueKey string) *databus.FindingDtoJson {
	return &databus.FindingDtoJson{
		AlertId:     "ALERT-1",
		Name:        "name",
		Description: "description",
		Severity:    databus.SeverityCritical,
		UniqueKey:   uniqueKey,
		BotName:     "bot",
		Team:        "team",
	}
}

func findingPayload(uniqueKey string) []byte {
	return []byte(`{"alertId":"ALERT-1","name":"name","description":"description",` +
		`"severity":"Critical","uniqueKey":"` + uniqueKey + `","botName":"bot","team":"team",` +
		`"blockTimestamp":1,"blockNumber":1}`)
}

// A failed Incr must not populate the LRU: otherwise the redelivered message
// takes the "already seen" path, hits redis.Nil and gets acked without ever
// being counted, silently dropping the finding.
func Test_collect_quorum_count_failed_incr_keeps_cache_clean(t *testing.T) {
	ctx := context.Background()
	unreachable := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	c := newTestConsumer(unreachable, &stubNotifier{})
	countKey := "test:count:incr-fail"

	msg := &testMsg{}
	_, done := c.collectQuorumCount(ctx, msg, testFinding("u1"), countKey)

	if !done || !msg.nacked {
		t.Fatalf("expected nack and done on Incr failure, got done=%v nacked=%v", done, msg.nacked)
	}
	if c.cache.Contains(countKey) {
		t.Fatal("cache must stay clean when Incr fails, otherwise the finding is lost on redelivery")
	}
}

// The first sighting increments the shared counter and nacks on purpose, so a
// single instance can never satisfy quorum by itself. The count key must carry
// a TTL, otherwise it stays in Redis forever.
func Test_collect_quorum_count_first_sighting_nacks_and_sets_ttl(t *testing.T) {
	ctx := context.Background()
	rdb := dialTestRedis(t)
	countKey := "test:count:first"
	rdb.Del(ctx, countKey)
	t.Cleanup(func() { rdb.Del(ctx, countKey) })

	c := newTestConsumer(rdb, &stubNotifier{})
	msg := &testMsg{}
	_, done := c.collectQuorumCount(ctx, msg, testFinding("u2"), countKey)

	if !done || !msg.nacked {
		t.Fatalf("first sighting must nack, got done=%v nacked=%v", done, msg.nacked)
	}
	if v, _ := rdb.Get(ctx, countKey).Uint64(); v != 1 {
		t.Fatalf("expected counter 1, got %d", v)
	}
	if ttl := rdb.TTL(ctx, countKey).Val(); ttl <= 0 || ttl > TTLMins10 {
		t.Fatalf("expected TTL within %v, got %v", TTLMins10, ttl)
	}
}

// A later sighting only reads the counter and hands control back to the caller.
func Test_collect_quorum_count_later_sighting_returns_count(t *testing.T) {
	ctx := context.Background()
	rdb := dialTestRedis(t)
	countKey := "test:count:later"
	rdb.Del(ctx, countKey)
	t.Cleanup(func() { rdb.Del(ctx, countKey) })

	c := newTestConsumer(rdb, &stubNotifier{})
	c.collectQuorumCount(ctx, &testMsg{}, testFinding("u3"), countKey)
	rdb.Incr(ctx, countKey) // another instance saw the same finding

	msg := &testMsg{}
	count, done := c.collectQuorumCount(ctx, msg, testFinding("u3"), countKey)

	if done {
		t.Fatal("expected the handler to continue")
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
	if msg.settled {
		t.Fatal("message must not be settled when the handler continues")
	}
}

// An expired count key means the finding is stale: ack it and drop the cache
// entry instead of counting it again.
func Test_collect_quorum_count_expired_key_acks(t *testing.T) {
	ctx := context.Background()
	rdb := dialTestRedis(t)
	countKey := "test:count:expired"
	rdb.Del(ctx, countKey)
	t.Cleanup(func() { rdb.Del(ctx, countKey) })

	c := newTestConsumer(rdb, &stubNotifier{})
	c.cache.Add(countKey, uint(1)) // in LRU, but not in Redis

	msg := &testMsg{}
	_, done := c.collectQuorumCount(ctx, msg, testFinding("u4"), countKey)

	if !done || !msg.acked {
		t.Fatalf("expected ack on redis.Nil, got done=%v acked=%v", done, msg.acked)
	}
	if c.cache.Contains(countKey) {
		t.Fatal("expired key must be removed from the cache")
	}
}

// Quorum is reached locally, but SetSendingStatus loses the race: another
// instance already claimed the send. The message must go back to NATS instead
// of hanging until AckWait and burning a MaxDeliver attempt.
func Test_consume_handler_lost_send_race_nacks_message(t *testing.T) {
	ctx := context.Background()
	rdb := dialTestRedis(t)

	const key = "u-race"
	countKey := "test-consumer:finding:" + key + ":count"
	statusKey := "test-consumer:finding:" + key + ":status"
	rdb.Del(ctx, countKey, statusKey)
	t.Cleanup(func() { rdb.Del(ctx, countKey, statusKey) })

	notifier := &stubNotifier{}
	// The handler sees quorum (2 >= 2), but the Lua script demands more and
	// refuses to hand over the send.
	c := newTestConsumer(rdb, notifier)
	c.repo = NewRepo(rdb, 7)

	rdb.Set(ctx, countKey, 2, time.Minute)
	rdb.Set(ctx, statusKey, string(StatusNotSend), time.Minute)
	c.cache.Add(countKey, uint(1))

	msg := &testMsg{payload: findingPayload(key)}
	c.GetConsumeHandler(ctx)(msg)

	if !msg.settled {
		t.Fatal("message must be settled, otherwise it hangs for the whole AckWait")
	}
	if !msg.nacked {
		t.Fatalf("expected nack, got acked=%v", msg.acked)
	}
	if msg.delay != ResendQuorumMsgAfter {
		t.Fatalf("expected delay %v, got %v", ResendQuorumMsgAfter, msg.delay)
	}
	if notifier.called {
		t.Fatal("notifier must not be called when another instance owns the send")
	}
}

// Findings below the consumer's severity set are acked without touching Redis.
func Test_consume_handler_skips_unwanted_severity(t *testing.T) {
	ctx := context.Background()
	rdb := dialTestRedis(t)

	notifier := &stubNotifier{}
	c := newTestConsumer(rdb, notifier)
	c.severitySet = registry.FindingMapping{databus.SeverityHigh: true}

	msg := &testMsg{payload: findingPayload("u-severity")}
	c.GetConsumeHandler(ctx)(msg)

	if !msg.acked {
		t.Fatalf("expected ack for filtered severity, got acked=%v nacked=%v", msg.acked, msg.nacked)
	}
	if notifier.called {
		t.Fatal("notifier must not be called for filtered findings")
	}
}
