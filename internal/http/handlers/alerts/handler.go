package alerts

import (
	"encoding/json"
	"fmt"
	"io"
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

type SendAlertsBadRequest struct {
	Payload *SendAlertsBadRequestBody
}

type SendAlertsBadRequestBody struct {
	Reason string `json:"reason,omitempty"`
}

type SendAlertsOK struct{}

func (h *handler) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		h.log.Error(fmt.Errorf("could not read body: %w", err))

		BadRequest(w, `could not read body`)
		return
	}

	var payload models.AlertBatch
	if err := payload.UnmarshalBinary(body); err != nil {
		h.log.Error(fmt.Errorf("could not unmarshal content: %w", err))

		BadRequest(w, `could not unmarshal content`)
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

	bb, _ := json.Marshal(SendAlertsOK{})
	_, _ = w.Write(bb)
}

func BadRequest(w http.ResponseWriter, reason string) {
	w.WriteHeader(http.StatusBadRequest)

	resp := &SendAlertsBadRequest{
		Payload: &SendAlertsBadRequestBody{
			// Reason: `could not read body`,
			Reason: reason,
		},
	}

	bb, _ := json.Marshal(resp)
	_, _ = w.Write(bb)
}
