package server

import (
	"github.com/lidofinance/finding-forwarder/internal/pkg/users"
	usersUsecase "github.com/lidofinance/finding-forwarder/internal/pkg/users/usecase"
)

type usecase struct {
	User users.Usecase
}

// nolint
func Usecase() *usecase {
	return &usecase{
		User: usersUsecase.New(),
	}
}
