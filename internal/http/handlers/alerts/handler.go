package alerts

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
)

type handler struct {
	log        *slog.Logger
	natsClient *nats.Conn
	streamName string
}

func New(log *slog.Logger, natsClient *nats.Conn, streamName string) *handler {
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
		reason := `Could not read body`
		h.log.Error(fmt.Sprintf("%s: %v", reason, err))

		BadRequest(w, reason)
		return
	}

	var payload models.AlertBatch
	if alertUnmarshalErr := payload.UnmarshalBinary(body); alertUnmarshalErr != nil {
		reason := `Could not unmarshal content`
		h.log.Error(fmt.Sprintf("%s: %s", reason, alertUnmarshalErr))

		BadRequest(w, reason)
		return
	}

	wg := &sync.WaitGroup{}
	for _, alert := range payload.Alerts {
		wg.Add(1)
		go func(finding *models.Alert) {
			defer wg.Done()

			bb, findingErr := json.Marshal(finding)
			if findingErr != nil {
				h.log.Error(fmt.Sprintf("Could not marshal finding: %v", findingErr))
				return
			}

			// TODO in future we have to set up queue for correct alert routing by teams
			if publishErr := h.natsClient.Publish(fmt.Sprintf(`%s.new`, h.streamName), bb); publishErr != nil {
				// TODO metircs alert
				h.log.Error(fmt.Sprintf("could not publish alert to JetStream: error: %v", publishErr))
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
