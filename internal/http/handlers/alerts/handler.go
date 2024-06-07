package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/utils/deps"
)

type handler struct {
	log        deps.Logger
	natsClient *nats.Conn
	streamName string
}

func New(log deps.Logger, natsClient *nats.Conn, streamName string) *handler {
	return &handler{
		log:        log,
		natsClient: natsClient,
		streamName: streamName,
	}
}

func (h *handler) Handler(w http.ResponseWriter, r *http.Request) {
	contentBytes := make([]byte, r.ContentLength)
	if _, err := r.Body.Read(contentBytes); err != nil {
		if err.Error() != "EOF" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	defer func() {
		r.Body.Close()
		contentBytes = nil
	}()

	var payload models.AlertBatch
	if err := payload.UnmarshalBinary(contentBytes); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	wg := &sync.WaitGroup{}
	for _, alert := range payload.Alerts {
		wg.Add(1)
		go func(finding *models.Alert) {
			defer wg.Done()

			bb, err := json.Marshal(finding)
			if err != nil {
				h.log.Error(fmt.Errorf("could not marshal alert: %w", err))
				return
			}

			// TODO in future we have to set up queue for correct alert routing by teams
			if publishErr := h.natsClient.Publish(fmt.Sprintf(`%s.new`, h.streamName), bb); publishErr != nil {
				// TODO metircs alert
				h.log.Error(fmt.Errorf(`could not publish alert to JetStream: error %w`, publishErr))
			}
		}(alert)
	}

	wg.Wait()

	_, _ = w.Write([]byte("OK"))
}
