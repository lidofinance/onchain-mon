package alerts

import (
	"encoding/json"
	"net/http"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/utils/deps"
)

type handler struct {
	log deps.Logger
}

func New(log deps.Logger) *handler {
	return &handler{
		log: log,
	}
}

func (h *handler) Handler(w http.ResponseWriter, r *http.Request) {
	var payload *models.AlertBatch

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	out, _ := json.Marshal(payload)
	h.log.Info(string(out))

	bb, _ := json.Marshal(payload)
	_, _ = w.Write(bb)
}
