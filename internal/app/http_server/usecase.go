package server

import (
	"net/http"

	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/pkg/telegram"
	usersUsecase "github.com/lidofinance/finding-forwarder/internal/pkg/telegram/usecase"
)

type usecase struct {
	User telegram.Usecase
}

func Usecase(cfg *env.AppConfig) *usecase {
	return &usecase{
		User: usersUsecase.New(cfg.TelegramBotToken, http.Client{}),
	}
}
