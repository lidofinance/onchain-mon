package users

import (
	"context"

	"github.com/lidofinance/finding-forwarder/internal/pkg/users/entity"
)

//go:generate ./../../../bin/mockery --name Usecase
type Usecase interface {
	Get(ctx context.Context, ID int64) (*entity.User, error)
}
