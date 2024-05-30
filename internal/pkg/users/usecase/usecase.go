package usecase

import (
	"context"

	"github.com/lidofinance/finding-forwarder/internal/pkg/users"
	"github.com/lidofinance/finding-forwarder/internal/pkg/users/entity"
)

type usecase struct {
}

func New() users.Usecase {
	return &usecase{}
}

func (u *usecase) Get(ctx context.Context, ID int64) (*entity.User, error) {
	return &entity.User{
		ID: 1,
	}, nil
}
