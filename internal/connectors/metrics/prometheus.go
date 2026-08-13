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
}

const Status = `status`
const Channel = `channel`
const ConsumerName = `consumerName`

const StatusOk = `Ok`
const StatusFail = `Fail`

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
	}

	return store
}
