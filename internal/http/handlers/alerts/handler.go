package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/utils/deps"
)

type handler struct {
	log        deps.Logger
	jetStream  jetstream.JetStream
	streamName string
}

func New(log deps.Logger, stream jetstream.JetStream, streamName string) *handler {
	return &handler{
		log:        log,
		jetStream:  stream,
		streamName: streamName,
	}
}

func (h *handler) Handler(w http.ResponseWriter, r *http.Request) {
	var payload *models.AlertBatch

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		bb, _ := alert.MarshalBinary()

		// TODO in future we have to set up queue for correct alert routing by teams
		_, publishErr := h.jetStream.PublishAsync(fmt.Sprintf(`%s.new`, h.streamName), bb)
		if publishErr != nil {
			// TODO metircs alert
			h.log.Error(fmt.Errorf(`could not publish alert to JetStream: error %w`, publishErr))
		}
	}

	_, _ = w.Write([]byte("OK"))
}
