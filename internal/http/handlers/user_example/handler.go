package userexample

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sergeyWh1te/go-template/internal/pkg/users"
	"github.com/sergeyWh1te/go-template/internal/utils/deps"
)

type handler struct {
	log    deps.Logger
	userUc users.Usecase
}

func New(log deps.Logger, userUc users.Usecase) *handler {
	return &handler{
		log:    log,
		userUc: userUc,
	}
}

func (h *handler) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	user, err := h.userUc.Get(r.Context(), int64(1))
	if err != nil {
		h.log.Error(fmt.Errorf(`some eror %w`, err))

		fmt.Fprint(w, "user not found")
		return
	}

	jsonResponse, _ := json.Marshal(user)
	_, _ = w.Write(jsonResponse)
}
