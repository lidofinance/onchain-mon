package alerts

import (
	"encoding/json"
	"net/http"

	"github.com/go-redis/redis"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/utils/deps"
)

type handler struct {
	log            deps.Logger
	redisClient    *redis.Client
	redisQueueName string
}

func New(log deps.Logger, redisClient *redis.Client, redisQueueName string) *handler {
	return &handler{
		log:            log,
		redisClient:    redisClient,
		redisQueueName: redisQueueName,
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
	if err := h.redisClient.LPush(h.redisQueueName, "some data").Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info(string(out))

	bb, _ := json.Marshal(payload)
	_, _ = w.Write(bb)
}
