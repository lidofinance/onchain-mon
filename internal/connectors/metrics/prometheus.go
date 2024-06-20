package metrics

import (
	"fmt"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Store struct {
	Prometheus      *prometheus.Registry
	BuildInfo       prometheus.Counter
	PublishedAlerts *prometheus.CounterVec
	SentAlerts      *prometheus.CounterVec
	SummaryHandlers *prometheus.SummaryVec
}

const Status = `status`
const Channel = `channel`

const StatusOk = `Ok`
const StatusFail = `Fail`

var Commit string

func New(promRegistry *prometheus.Registry, prefix, appName, env string) *Store {
	store := &Store{
		Prometheus: promRegistry,
		BuildInfo: promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_metric_build_info", prefix),
			Help: "Build information",
			ConstLabels: prometheus.Labels{
				"name":    appName,
				"env":     env,
				"commit":  Commit,
				"version": runtime.Version(),
			},
		}),
		PublishedAlerts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_finding_published_total", prefix),
			Help: "The total number of published findings",
		}, []string{Status}),
		SentAlerts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_finding_sent_total", prefix),
			Help: "The total number of set findings",
		}, []string{Channel, Status}),
		SummaryHandlers: promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:       fmt.Sprintf("%s_request_processing_seconds", prefix),
			Help:       "Time spent processing request to notification channel",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{Channel}),
	}

	store.Prometheus.MustRegister(
		store.BuildInfo,
		store.PublishedAlerts,
		store.SentAlerts,
		store.SummaryHandlers,
	)

	return store
}
