package server

import (
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/pkg/telegram"
	usersUsecase "github.com/lidofinance/finding-forwarder/internal/pkg/telegram/usecase"
	"net/http"
)

type usecase struct {
	User telegram.Usecase
}

// nolint
func Usecase(cfg *env.AppConfig) *usecase {
	return &usecase{
		User: usersUsecase.New(cfg.TelegramBotToken, http.Client{}),
	}
}
