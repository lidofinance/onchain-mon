package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Store struct {
	Prometheus      *prometheus.Registry
	BuildInfo       prometheus.Counter
	PublishedBlocks *prometheus.CounterVec
	SentAlerts      *prometheus.CounterVec
	RedisErrors     prometheus.Counter
	SummaryHandlers *prometheus.HistogramVec
	NotifyChannels  *prometheus.CounterVec
	BlockResets     prometheus.Counter

	UnpublishableBlocks         *prometheus.CounterVec
	LastUnpublishableBlock      prometheus.Gauge
	LastPublishedBlockTimestamp prometheus.Gauge
	BlockPayloadSize            *prometheus.GaugeVec
}

const Status = `status`
const Channel = `channel`
const ConsumerName = `consumerName`
const Reason = `reason`
const Stage = `stage`

const StatusOk = `Ok`
const StatusFail = `Fail`

// ReasonMaxPayload marks a block NATS refused because of its size.
const ReasonMaxPayload = `max_payload`

// Payload stages for BlockPayloadSize — one metric, two lines on a chart.
const StageRaw = `raw`
const StageCompressed = `compressed`

var Commit string

func New(promRegistry *prometheus.Registry, prefix, appName, env string) *Store {
	promRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	store := &Store{
		Prometheus: promRegistry,
		BuildInfo: promauto.With(promRegistry).NewCounter(prometheus.CounterOpts{
			Name: prefix + "_metric_build_info",
			Help: "Build information",
			ConstLabels: prometheus.Labels{
				"name":    appName,
				"env":     env,
				"commit":  Commit,
				"version": runtime.Version(),
			},
		}),
		PublishedBlocks: promauto.With(promRegistry).NewCounterVec(prometheus.CounterOpts{
			Name: prefix + "_blocks_published_total",
			Help: "The total number of published blocks",
		}, []string{Status}),
		SentAlerts: promauto.With(promRegistry).NewCounterVec(prometheus.CounterOpts{
			Name: prefix + "_finding_sent_total",
			Help: "The total number of published findings",
		}, []string{ConsumerName, Status}),
		RedisErrors: promauto.With(promRegistry).NewCounter(prometheus.CounterOpts{
			Name: prefix + "_redis_error_total",
			Help: "The total number of redis errors",
		}),
		SummaryHandlers: promauto.With(promRegistry).NewHistogramVec(prometheus.HistogramOpts{
			Name:    prefix + "_request_processing_seconds",
			Help:    "Time spent processing request to notification channel",
			Buckets: prometheus.DefBuckets,
		}, []string{Channel}),
		NotifyChannels: promauto.With(promRegistry).NewCounterVec(prometheus.CounterOpts{
			Name: prefix + "_notification_channel_error_total",
			Help: "The total number of network errors of telegram, discord, opsgenie channels",
		}, []string{Channel, Status}),
		BlockResets: promauto.With(promRegistry).NewCounter(prometheus.CounterOpts{
			Name: prefix + "_block_reset_total",
			Help: "The total number of reset blocks",
		}),
		UnpublishableBlocks: promauto.With(promRegistry).NewCounterVec(prometheus.CounterOpts{
			Name: prefix + "_blocks_unpublishable_total",
			Help: "The total number of blocks skipped because NATS refused them",
		}, []string{Reason}),
		LastUnpublishableBlock: promauto.With(promRegistry).NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_last_unpublishable_block_number",
			// The number is the value, not a label: as a label it would spawn a
			// new time series for every block.
			Help: "Number of the last block that could not be published",
		}),
		BlockPayloadSize: promauto.With(promRegistry).NewGaugeVec(prometheus.GaugeOpts{
			Name: prefix + "_block_payload_bytes",
			Help: "Size of the last published block payload, before and after zstd",
		}, []string{Stage}),
		LastPublishedBlockTimestamp: promauto.With(promRegistry).NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_last_published_block_timestamp",
			// A counter cannot tell "nothing published for a while" apart from
			// "no blocks to publish"; this gauge feeds the staleness alert:
			//   time() - <prefix>_last_published_block_timestamp > 120
			Help: "Unix time of the last successfully published block",
		}),
	}

	return store
}

// UnpublishedBlock records a block NATS refused: the counter drives the alert,
// the gauge answers which block it was.
func (s *Store) UnpublishedBlock(reason string, blockNumber int64) {
	s.UnpublishableBlocks.With(prometheus.Labels{Reason: reason}).Inc()
	s.LastUnpublishableBlock.Set(float64(blockNumber))
}
