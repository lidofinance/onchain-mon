package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Store struct {
	Prometheus *prometheus.Registry
	BuildInfo  prometheus.Counter
}

func New(promRegistry *prometheus.Registry, prefix, appName, env string) *Store {
	store := &Store{
		Prometheus: promRegistry,
		BuildInfo: promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_metric_build_info", prefix),
			Help: "Build information",
			ConstLabels: prometheus.Labels{
				"name": appName,
				"env":  env,
			},
		}),
	}

	_ = store.Prometheus.Register(store.BuildInfo)

	return store
}
